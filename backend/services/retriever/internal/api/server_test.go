package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/rs/zerolog"

	"github.com/strata/retriever/internal/config"
	"github.com/strata/retriever/internal/embedder"
	"github.com/strata/retriever/internal/vectorstore"
)

func testServer() (*httptest.Server, *Server) {
	log := zerolog.New(os.Stderr)
	cfg := config.Config{DevMode: true}
	emb := embedder.NewMockEmbedder(64)
	st := vectorstore.NewMemoryStore()
	srv := New(cfg, log, nil, emb, st)
	ts := httptest.NewServer(srv.Router())
	return ts, srv
}

func TestHealthz(t *testing.T) {
	ts, _ := testServer()
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestIndexAndRetrieve_MultiTenantIsolation(t *testing.T) {
	ts, _ := testServer()
	defer ts.Close()

	ctx := context.Background()

	// 1. Index document for alice
	docAlice := IndexRequest{
		Collection: "clusters",
		ID:         "cl-alice/pod-1",
		Text:       "nginx ingress controller in production cluster alice",
		Metadata:   map[string]any{"env": "prod"},
	}
	payloadAlice, _ := json.Marshal(docAlice)
	reqAlice, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/index", bytes.NewReader(payloadAlice))
	reqAlice.Header.Set("X-Strata-User", "alice")
	reqAlice.Header.Set("Content-Type", "application/json")

	resAlice, err := http.DefaultClient.Do(reqAlice)
	if err != nil {
		t.Fatal(err)
	}
	resAlice.Body.Close()
	if resAlice.StatusCode != http.StatusOK {
		t.Fatalf("index alice status = %d", resAlice.StatusCode)
	}

	// 2. Index document for bob
	docBob := IndexRequest{
		Collection: "clusters",
		ID:         "cl-bob/pod-2",
		Text:       "postgres database pod in production cluster bob",
		Metadata:   map[string]any{"env": "prod"},
	}
	payloadBob, _ := json.Marshal(docBob)
	reqBob, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/index", bytes.NewReader(payloadBob))
	reqBob.Header.Set("X-Strata-User", "bob")
	reqBob.Header.Set("Content-Type", "application/json")

	resBob, err := http.DefaultClient.Do(reqBob)
	if err != nil {
		t.Fatal(err)
	}
	resBob.Body.Close()
	if resBob.StatusCode != http.StatusOK {
		t.Fatalf("index bob status = %d", resBob.StatusCode)
	}

	// 3. Alice queries for "production" - should ONLY see Alice's pods, NOT Bob's
	qAlice := RetrieveRequest{
		Collection: "clusters",
		Query:      "production pods",
		TopK:       10,
	}
	qBytes, _ := json.Marshal(qAlice)
	searchAlice, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/retrieve", bytes.NewReader(qBytes))
	searchAlice.Header.Set("X-Strata-User", "alice")
	searchAlice.Header.Set("Content-Type", "application/json")

	respSearchAlice, err := http.DefaultClient.Do(searchAlice)
	if err != nil {
		t.Fatal(err)
	}
	defer respSearchAlice.Body.Close()
	if respSearchAlice.StatusCode != http.StatusOK {
		t.Fatalf("retrieve alice status = %d", respSearchAlice.StatusCode)
	}

	var aliceResults struct {
		Chunks []vectorstore.SearchResult `json:"chunks"`
		Count  int                        `json:"count"`
	}
	_ = json.NewDecoder(respSearchAlice.Body).Decode(&aliceResults)

	if len(aliceResults.Chunks) != 1 {
		t.Fatalf("expected 1 result for alice, got %d", len(aliceResults.Chunks))
	}
	if aliceResults.Chunks[0].ID != "cl-alice/pod-1" {
		t.Errorf("expected alice pod, got %s", aliceResults.Chunks[0].ID)
	}

	// 4. Unauthenticated request without token or X-Strata-User must be rejected with 401
	unauthReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/retrieve", bytes.NewReader(qBytes))
	unauthReq.Header.Set("Content-Type", "application/json")
	respUnauth, err := http.DefaultClient.Do(unauthReq)
	if err != nil {
		t.Fatal(err)
	}
	defer respUnauth.Body.Close()
	if respUnauth.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for unauthenticated retrieve, got %d", respUnauth.StatusCode)
	}

	// 5. Delete document
	delReq, _ := http.NewRequestWithContext(ctx, http.MethodDelete, ts.URL+"/index/clusters/cl-alice/pod-1", nil)
	delReq.Header.Set("X-Strata-User", "alice")
	delResp, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatal(err)
	}
	delResp.Body.Close()
	if delResp.StatusCode != http.StatusOK {
		t.Fatalf("delete status = %d", delResp.StatusCode)
	}
}
