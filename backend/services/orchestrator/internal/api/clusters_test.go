package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/strata/orchestrator/internal/store"
)

func seedAlice(srv *Server) {
	fs := srv.store.(*fakeStore)
	_ = fs.EnsureUser(context.Background(), store.User{ID: "alice", Username: "alice"})
	_ = fs.CreateCluster(context.Background(), store.Cluster{
		ID: "cl-001", UserID: "alice", Name: "demo", Context: "demo-ctx",
	}, "/etc/strata/kubeconfigs/cl-001")
}

func TestListClusters_ReturnsUsersClusters(t *testing.T) {
	ts, srv, priv := testServer(t)
	seedAlice(srv)

	tok := mintToken(t, priv, "test-kid", "alice", "strata-tui", false)
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/clusters/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var body struct {
		Clusters []store.Cluster `json:"clusters"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if len(body.Clusters) != 1 || body.Clusters[0].ID != "cl-001" {
		t.Errorf("clusters = %+v", body.Clusters)
	}
}

func TestListPods_HappyPath(t *testing.T) {
	ts, srv, priv := testServer(t)
	seedAlice(srv)

	tok := mintToken(t, priv, "test-kid", "alice", "strata-tui", false)
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/clusters/cl-001/pods", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var pods []map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&pods)
	if len(pods) != 1 || pods[0]["name"] != "p1" {
		t.Errorf("pods = %+v", pods)
	}
	// Verify the fake MCP saw the right call args and headers.
	m := srv.mcp.(*fakeMCP)
	if len(m.calls) != 1 {
		t.Fatalf("mcp calls = %d", len(m.calls))
	}
	if m.calls[0].Name != "list_pods" {
		t.Errorf("mcp tool = %q", m.calls[0].Name)
	}
	if m.calls[0].Args["cluster_id"] != "cl-001" {
		t.Errorf("mcp args = %v", m.calls[0].Args)
	}
	if got := m.calls[0].Headers.Get("X-Strata-Kubeconfig"); got != "/etc/strata/kubeconfigs/cl-001" {
		t.Errorf("X-Strata-Kubeconfig = %q", got)
	}
	if got := m.calls[0].Headers.Get("X-Strata-User"); got != "alice" {
		t.Errorf("X-Strata-User = %q", got)
	}
}

func TestListPods_PassesQueryParams(t *testing.T) {
	ts, srv, priv := testServer(t)
	seedAlice(srv)

	tok := mintToken(t, priv, "test-kid", "alice", "strata-tui", false)
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/clusters/cl-001/pods?namespace=kube-system&label-selector=app%3Dnginx", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	m := srv.mcp.(*fakeMCP)
	if got := m.calls[0].Args["namespace"]; got != "kube-system" {
		t.Errorf("namespace = %v", got)
	}
	if got := m.calls[0].Args["label_selector"]; got != "app=nginx" {
		t.Errorf("label_selector = %v", got)
	}
}

func TestListPods_NotFound(t *testing.T) {
	ts, srv, priv := testServer(t)
	seedAlice(srv)

	tok := mintToken(t, priv, "test-kid", "alice", "strata-tui", false)
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/clusters/cl-missing/pods", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d", resp.StatusCode)
	}
}

func TestListPods_OtherUserForbidden(t *testing.T) {
	ts, srv, priv := testServer(t)
	seedAlice(srv)
	// Bob exists but has no clusters.
	fs := srv.store.(*fakeStore)
	_ = fs.EnsureUser(context.Background(), store.User{ID: "bob", Username: "bob"})

	tok := mintToken(t, priv, "test-kid", "bob", "strata-tui", false)
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/clusters/cl-001/pods", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, bob saw alice's cluster", resp.StatusCode)
	}
}

func TestListPods_MCPErrorBecomes502(t *testing.T) {
	ts, srv, priv := testServer(t)
	seedAlice(srv)

	mcp := srv.mcp.(*fakeMCP)
	mcp.mu.Lock()
	mcp.failWith = errMCP
	mcp.mu.Unlock()

	tok := mintToken(t, priv, "test-kid", "alice", "strata-tui", false)
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/clusters/cl-001/pods", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", resp.StatusCode)
	}
}
