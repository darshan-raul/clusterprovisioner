package vectorstore

import (
	"context"
	"testing"
)

func TestMemoryStore_UpsertSearchFilterDelete(t *testing.T) {
	ms := NewMemoryStore()
	ctx := context.Background()

	p1 := Point{
		ID:       "doc-1",
		Vector:   []float32{1.0, 0.0, 0.0},
		Text:     "nginx ingress pod",
		Metadata: map[string]any{"namespace": "prod", "env": "production"},
	}
	p2 := Point{
		ID:       "doc-2",
		Vector:   []float32{0.0, 1.0, 0.0},
		Text:     "postgres database",
		Metadata: map[string]any{"namespace": "db", "env": "production"},
	}
	p3 := Point{
		ID:       "doc-3",
		Vector:   []float32{0.8, 0.2, 0.0},
		Text:     "frontend react pod",
		Metadata: map[string]any{"namespace": "staging", "env": "staging"},
	}

	if err := ms.Upsert(ctx, "user_alice_clusters", []Point{p1, p2, p3}); err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	// Search matching p1
	queryVec := []float32{1.0, 0.0, 0.0}
	results, err := ms.Search(ctx, "user_alice_clusters", queryVec, 2, nil)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].ID != "doc-1" {
		t.Errorf("expected top result doc-1, got %s", results[0].ID)
	}
	if results[0].Score < 0.99 {
		t.Errorf("expected score ~1.0, got %f", results[0].Score)
	}

	// Search with metadata filter: env=production
	filtered, err := ms.Search(ctx, "user_alice_clusters", queryVec, 10, map[string]any{"env": "production"})
	if err != nil {
		t.Fatalf("Filtered search failed: %v", err)
	}
	if len(filtered) != 2 {
		t.Fatalf("expected 2 production results, got %d", len(filtered))
	}
	for _, r := range filtered {
		if r.Metadata["env"] != "production" {
			t.Errorf("expected env production, got %v", r.Metadata["env"])
		}
	}

	// Delete doc-1
	if err := ms.Delete(ctx, "user_alice_clusters", "doc-1"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	afterDelete, err := ms.Search(ctx, "user_alice_clusters", queryVec, 10, nil)
	if err != nil {
		t.Fatalf("Search after delete failed: %v", err)
	}
	if len(afterDelete) != 2 {
		t.Fatalf("expected 2 results after delete, got %d", len(afterDelete))
	}
	for _, r := range afterDelete {
		if r.ID == "doc-1" {
			t.Errorf("doc-1 was not deleted")
		}
	}
}
