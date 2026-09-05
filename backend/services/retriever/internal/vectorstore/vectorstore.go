package vectorstore

import (
	"context"
)

// Point is an indexed item with an embedding vector, raw text, and metadata.
type Point struct {
	ID       string         `json:"id"`
	Vector   []float32      `json:"vector"`
	Text     string         `json:"text"`
	Metadata map[string]any `json:"metadata"`
}

// SearchResult is a retrieved match with relevance score.
type SearchResult struct {
	ID       string         `json:"id"`
	Text     string         `json:"text"`
	Score    float32        `json:"score"`
	Metadata map[string]any `json:"metadata"`
}

// Store defines operations against a vector database.
type Store interface {
	Upsert(ctx context.Context, collection string, points []Point) error
	Search(ctx context.Context, collection string, vector []float32, topK int, filter map[string]any) ([]SearchResult, error)
	Delete(ctx context.Context, collection, id string) error
	Health(ctx context.Context) error
}
