package embedder

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"
)

// Embedder generates vector embeddings for text.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	Dimension() int
}

// OpenAIEmbedder calls an OpenAI/LiteLLM-compatible /v1/embeddings endpoint.
type OpenAIEmbedder struct {
	client    *http.Client
	url       string
	model     string
	apiKey    string
	dimension int
}

func NewOpenAIEmbedder(url, model, apiKey string, dimension int) *OpenAIEmbedder {
	if dimension <= 0 {
		dimension = 1536
	}
	return &OpenAIEmbedder{
		client:    &http.Client{Timeout: 10 * time.Second},
		url:       url,
		model:     model,
		apiKey:    apiKey,
		dimension: dimension,
	}
}

func (e *OpenAIEmbedder) Dimension() int {
	return e.dimension
}

func (e *OpenAIEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody := map[string]any{
		"input": text,
		"model": e.model,
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal embedding request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create embedding request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.apiKey)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute embedding request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding service returned HTTP %d", resp.StatusCode)
	}

	var parsed struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode embedding response: %w", err)
	}
	if len(parsed.Data) == 0 || len(parsed.Data[0].Embedding) == 0 {
		return nil, errors.New("empty embedding returned")
	}
	return parsed.Data[0].Embedding, nil
}

// MockEmbedder produces deterministic, L2-normalized pseudo-embeddings.
// Words distribute weights into hash buckets so text with common words
// produces higher cosine similarity.
type MockEmbedder struct {
	dim int
}

func NewMockEmbedder(dimension int) *MockEmbedder {
	if dimension <= 0 {
		dimension = 128
	}
	return &MockEmbedder{dim: dimension}
}

func (m *MockEmbedder) Dimension() int {
	return m.dim
}

func (m *MockEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	vec := make([]float32, m.dim)
	tokens := strings.Fields(strings.ToLower(text))
	if len(tokens) == 0 {
		tokens = []string{""}
	}

	for _, token := range tokens {
		h := sha256.Sum256([]byte(token))
		// Distribute into 4 buckets per token
		for i := 0; i < 4; i++ {
			idx := int(binary.BigEndian.Uint32(h[i*4:i*4+4])) % m.dim
			if idx < 0 {
				idx = -idx
			}
			vec[idx] += 1.0
		}
	}

	// L2 Normalize
	var sumSq float64
	for _, v := range vec {
		sumSq += float64(v * v)
	}
	norm := float32(math.Sqrt(sumSq))
	if norm > 0 {
		for i := range vec {
			vec[i] /= norm
		}
	}

	return vec, nil
}
