package embedder

import (
	"context"
	"math"
	"testing"
)

func cosineSimilarity(a, b []float32) float32 {
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

func TestMockEmbedder_DeterministicAndNormalized(t *testing.T) {
	m := NewMockEmbedder(64)
	ctx := context.Background()

	v1, err := m.Embed(ctx, "pod nginx is failing in prod")
	if err != nil {
		t.Fatalf("Embed 1 failed: %v", err)
	}
	v2, err := m.Embed(ctx, "pod nginx is failing in prod")
	if err != nil {
		t.Fatalf("Embed 2 failed: %v", err)
	}

	if len(v1) != 64 {
		t.Fatalf("expected dim 64, got %d", len(v1))
	}
	for i := range v1 {
		if v1[i] != v2[i] {
			t.Fatalf("mismatch at index %d: %v vs %v", i, v1[i], v2[i])
		}
	}

	// Check unit norm
	var normSq float32
	for _, val := range v1 {
		normSq += val * val
	}
	if math.Abs(float64(normSq-1.0)) > 1e-4 {
		t.Errorf("expected unit norm ~1.0, got %f", normSq)
	}

	// Related sentence should have higher cosine similarity than unrelated sentence
	vRelated, _ := m.Embed(ctx, "nginx pod failing CrashLoopBackOff")
	vUnrelated, _ := m.Embed(ctx, "database postgres connection pool")

	simRelated := cosineSimilarity(v1, vRelated)
	simUnrelated := cosineSimilarity(v1, vUnrelated)

	if simRelated <= simUnrelated {
		t.Errorf("expected related similarity (%f) > unrelated (%f)", simRelated, simUnrelated)
	}
}
