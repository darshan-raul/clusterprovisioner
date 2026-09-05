package vectorstore

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
)

// MemoryStore is an in-memory thread-safe vector store.
type MemoryStore struct {
	mu          sync.RWMutex
	collections map[string]map[string]Point
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		collections: make(map[string]map[string]Point),
	}
}

func (m *MemoryStore) Upsert(_ context.Context, collection string, points []Point) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	col, ok := m.collections[collection]
	if !ok {
		col = make(map[string]Point)
		m.collections[collection] = col
	}

	for _, p := range points {
		col[p.ID] = p
	}
	return nil
}

func (m *MemoryStore) Search(_ context.Context, collection string, queryVec []float32, topK int, filter map[string]any) ([]SearchResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if topK <= 0 {
		topK = 5
	}

	col, ok := m.collections[collection]
	if !ok {
		return []SearchResult{}, nil
	}

	var candidates []SearchResult
	for _, p := range col {
		// Apply metadata filter
		if !matchesFilter(p.Metadata, filter) {
			continue
		}

		score := cosine(queryVec, p.Vector)
		candidates = append(candidates, SearchResult{
			ID:       p.ID,
			Text:     p.Text,
			Score:    score,
			Metadata: p.Metadata,
		})
	}

	// Sort descending by score
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	if len(candidates) > topK {
		candidates = candidates[:topK]
	}
	return candidates, nil
}

func (m *MemoryStore) Delete(_ context.Context, collection, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	col, ok := m.collections[collection]
	if !ok {
		return nil
	}
	delete(col, id)
	return nil
}

func (m *MemoryStore) Health(_ context.Context) error {
	return nil
}

func matchesFilter(meta, filter map[string]any) bool {
	if len(filter) == 0 {
		return true
	}
	if len(meta) == 0 {
		return false
	}
	for k, expectedVal := range filter {
		actualVal, exists := meta[k]
		if !exists {
			return false
		}
		if fmt.Sprintf("%v", actualVal) != fmt.Sprintf("%v", expectedVal) {
			return false
		}
	}
	return true
}

func cosine(a, b []float32) float32 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, magA, magB float32
	for i := range a {
		dot += a[i] * b[i]
		magA += a[i] * a[i]
		magB += b[i] * b[i]
	}
	if magA == 0 || magB == 0 {
		return 0
	}
	return dot / (float32(math.Sqrt(float64(magA))) * float32(math.Sqrt(float64(magB))))
}
