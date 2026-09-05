package vectorstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// QdrantStore interacts with Qdrant Vector DB via its HTTP REST API.
type QdrantStore struct {
	client    *http.Client
	baseURL   string
	dim       int
	ensuredMu sync.Mutex
	ensured   map[string]bool
}

func NewQdrantStore(baseURL string, dimension int) *QdrantStore {
	if dimension <= 0 {
		dimension = 128
	}
	return &QdrantStore{
		client:  &http.Client{Timeout: 10 * time.Second},
		baseURL: strings.TrimRight(baseURL, "/"),
		dim:     dimension,
		ensured: make(map[string]bool),
	}
}

func (q *QdrantStore) ensureCollection(ctx context.Context, collection string) error {
	q.ensuredMu.Lock()
	if q.ensured[collection] {
		q.ensuredMu.Unlock()
		return nil
	}
	q.ensuredMu.Unlock()

	// Check if collection exists
	getReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/collections/%s", q.baseURL, collection), nil)
	getResp, err := q.client.Do(getReq)
	if err == nil {
		defer getResp.Body.Close()
		if getResp.StatusCode == http.StatusOK {
			q.ensuredMu.Lock()
			q.ensured[collection] = true
			q.ensuredMu.Unlock()
			return nil
		}
	}

	// Create collection
	createBody := map[string]any{
		"vectors": map[string]any{
			"size":     q.dim,
			"distance": "Cosine",
		},
	}
	bodyBytes, _ := json.Marshal(createBody)
	putReq, err := http.NewRequestWithContext(ctx, http.MethodPut, fmt.Sprintf("%s/collections/%s", q.baseURL, collection), bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	putReq.Header.Set("Content-Type", "application/json")
	putResp, err := q.client.Do(putReq)
	if err != nil {
		return fmt.Errorf("create collection %s: %w", collection, err)
	}
	defer putResp.Body.Close()

	if putResp.StatusCode != http.StatusOK && putResp.StatusCode != http.StatusConflict {
		return fmt.Errorf("create collection %s failed: HTTP %d", collection, putResp.StatusCode)
	}

	q.ensuredMu.Lock()
	q.ensured[collection] = true
	q.ensuredMu.Unlock()
	return nil
}

func (q *QdrantStore) Upsert(ctx context.Context, collection string, points []Point) error {
	if err := q.ensureCollection(ctx, collection); err != nil {
		return err
	}

	type qdrantPoint struct {
		ID      string         `json:"id"`
		Vector  []float32      `json:"vector"`
		Payload map[string]any `json:"payload"`
	}

	qPoints := make([]qdrantPoint, len(points))
	for i, p := range points {
		payload := map[string]any{
			"doc_id":   p.ID,
			"text":     p.Text,
			"metadata": p.Metadata,
		}
		// Qdrant point ID can be UUID format
		uuid := stringToUUID(p.ID)
		qPoints[i] = qdrantPoint{
			ID:      uuid,
			Vector:  p.Vector,
			Payload: payload,
		}
	}

	payloadBytes, err := json.Marshal(map[string]any{"points": qPoints})
	if err != nil {
		return fmt.Errorf("marshal qdrant points: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, fmt.Sprintf("%s/collections/%s/points?wait=true", q.baseURL, collection), bytes.NewReader(payloadBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := q.client.Do(req)
	if err != nil {
		return fmt.Errorf("upsert points: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upsert points failed: HTTP %d", resp.StatusCode)
	}
	return nil
}

func (q *QdrantStore) Search(ctx context.Context, collection string, vector []float32, topK int, filter map[string]any) ([]SearchResult, error) {
	if topK <= 0 {
		topK = 5
	}

	searchBody := map[string]any{
		"vector":       vector,
		"limit":        topK,
		"with_payload": true,
	}

	// If metadata filter specified, build Qdrant filter
	if len(filter) > 0 {
		var must []map[string]any
		for k, v := range filter {
			must = append(must, map[string]any{
				"key": "metadata." + k,
				"match": map[string]any{
					"value": v,
				},
			})
		}
		searchBody["filter"] = map[string]any{
			"must": must,
		}
	}

	payloadBytes, err := json.Marshal(searchBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/collections/%s/points/search", q.baseURL, collection), bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := q.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("qdrant search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return []SearchResult{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("qdrant search failed: HTTP %d", resp.StatusCode)
	}

	var parsed struct {
		Result []struct {
			Score   float32 `json:"score"`
			Payload struct {
				DocID    string         `json:"doc_id"`
				Text     string         `json:"text"`
				Metadata map[string]any `json:"metadata"`
			} `json:"payload"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode qdrant search response: %w", err)
	}

	results := make([]SearchResult, len(parsed.Result))
	for i, r := range parsed.Result {
		results[i] = SearchResult{
			ID:       r.Payload.DocID,
			Text:     r.Payload.Text,
			Score:    r.Score,
			Metadata: r.Payload.Metadata,
		}
	}
	return results, nil
}

func (q *QdrantStore) Delete(ctx context.Context, collection, id string) error {
	uuid := stringToUUID(id)
	delBody := map[string]any{
		"points": []string{uuid},
	}
	payloadBytes, _ := json.Marshal(delBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/collections/%s/points/delete?wait=true", q.baseURL, collection), bytes.NewReader(payloadBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := q.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (q *QdrantStore) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/healthz", q.baseURL), nil)
	if err != nil {
		return err
	}
	resp, err := q.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("qdrant unhealthy: HTTP %d", resp.StatusCode)
	}
	return nil
}

func stringToUUID(s string) string {
	h := sha256.Sum256([]byte(s))
	// Convert first 16 bytes to standard UUID format: 8-4-4-4-12
	return fmt.Sprintf("%x-%x-%x-%x-%x", h[0:4], h[4:6], h[6:8], h[8:10], h[10:16])
}
