package indexer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
)

func TestDocsChunking(t *testing.T) {
	tmpDir := t.TempDir()
	docFile := filepath.Join(tmpDir, "sample.md")
	content := `# Strata Guide
This is the introduction.

## Upgrading EKS
To upgrade EKS, drain Karpenter nodes first.
Then update node groups.

## Troubleshooting Pods
Check pod logs using :logs <pod> command.
`
	if err := os.WriteFile(docFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	chunks, err := LoadAndChunkDocs(tmpDir)
	if err != nil {
		t.Fatalf("LoadAndChunkDocs failed: %v", err)
	}
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}
	if chunks[1].Section != "Upgrading EKS" {
		t.Errorf("expected Upgrading EKS, got %s", chunks[1].Section)
	}
}

func TestIndexClusterResources_CallsRetriever(t *testing.T) {
	indexedPoints := []IndexPoint{}
	userHeaders := []string{}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/index" {
			http.NotFound(w, r)
			return
		}
		userHeaders = append(userHeaders, r.Header.Get("X-Strata-User"))
		var p IndexPoint
		_ = json.NewDecoder(r.Body).Decode(&p)
		indexedPoints = append(indexedPoints, p)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"upserted":true}`))
	}))
	defer ts.Close()

	log := zerolog.New(os.Stderr)
	idx := New(ts.URL, log)

	pods := []PodInfo{
		{
			Name:      "nginx-1",
			Namespace: "prod",
			Node:      "node-1",
			Phase:     "Running",
			Ready:     "1/1",
			Restarts:  0,
			Age:       "1h",
		},
	}
	history := []HistoryInfo{
		{
			ActionType: "delete_pod",
			Target:     "prod/test-pod",
			Status:     "success",
			ClientType: "tui",
			CreatedAt:  "2026-09-05",
		},
	}

	ctx := context.Background()
	count, err := idx.IndexClusterResources(ctx, "alice", "cl-001", "production", "prod-ctx", pods, history)
	if err != nil {
		t.Fatalf("IndexClusterResources failed: %v", err)
	}
	if count != 3 { // 1 summary + 1 pod + 1 history
		t.Fatalf("expected 3 indexed points, got %d", count)
	}
	if len(indexedPoints) != 3 {
		t.Fatalf("retriever received %d points", len(indexedPoints))
	}
	for _, u := range userHeaders {
		if u != "alice" {
			t.Errorf("expected user alice, got %s", u)
		}
	}
}
