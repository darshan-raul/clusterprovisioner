package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/strata/orchestrator/internal/store"
	"github.com/strata/shared/pkg/crypto"
)

func seedAlice(srv *Server) {
	fs := srv.store.(*fakeStore)
	_ = fs.EnsureUser(context.Background(), store.User{ID: "alice", Username: "alice"})
	_ = fs.CreateCluster(context.Background(), store.Cluster{
		ID: "cl-001", UserID: "alice", Name: "demo", Context: "demo-ctx",
	}, store.ClusterCreds{KubeconfigPath: "/etc/strata/kubeconfigs/cl-001"})
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

func TestDeletePod_HappyPath(t *testing.T) {
	ts, srv, priv := testServer(t)
	seedAlice(srv)

	tok := mintToken(t, priv, "test-kid", "alice", "strata-tui", false)
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/clusters/cl-001/pods/test-pod?namespace=prod&grace-period-seconds=5", nil)
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
	if len(m.calls) != 1 {
		t.Fatalf("mcp calls = %d", len(m.calls))
	}
	call := m.calls[0]
	if call.Name != "delete_pod" {
		t.Errorf("call name = %q, want delete_pod", call.Name)
	}
	if call.Args["name"] != "test-pod" || call.Args["namespace"] != "prod" || call.Args["grace_period_seconds"] != 5 {
		t.Errorf("call args = %+v", call.Args)
	}
}

func TestDeletePod_NotFound(t *testing.T) {
	ts, srv, priv := testServer(t)
	seedAlice(srv)

	tok := mintToken(t, priv, "test-kid", "alice", "strata-tui", false)
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/clusters/cl-missing/pods/test-pod", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestApplyManifest_HappyPath(t *testing.T) {
	ts, srv, priv := testServer(t)
	seedAlice(srv)

	tok := mintToken(t, priv, "test-kid", "alice", "strata-tui", false)
	body := `{"manifest":"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm","namespace":"default"}`
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/clusters/cl-001/apply", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	m := srv.mcp.(*fakeMCP)
	if len(m.calls) != 1 {
		t.Fatalf("mcp calls = %d", len(m.calls))
	}
	call := m.calls[0]
	if call.Name != "apply_manifest" {
		t.Errorf("call name = %q, want apply_manifest", call.Name)
	}
	if call.Args["cluster_id"] != "cl-001" || call.Args["namespace"] != "default" {
		t.Errorf("call args = %+v", call.Args)
	}
}

func TestApplyManifest_RequiresManifest(t *testing.T) {
	ts, srv, priv := testServer(t)
	seedAlice(srv)

	tok := mintToken(t, priv, "test-kid", "alice", "strata-tui", false)
	body := `{"manifest":"","namespace":"default"}`
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/clusters/cl-001/apply", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestExecCommand_HappyPath(t *testing.T) {
	ts, srv, priv := testServer(t)
	seedAlice(srv)

	tok := mintToken(t, priv, "test-kid", "alice", "strata-tui", false)
	body := `{"command":"whoami","namespace":"kube-system","container":"main"}`
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/clusters/cl-001/pods/test-pod/exec", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	m := srv.mcp.(*fakeMCP)
	if len(m.calls) != 1 {
		t.Fatalf("mcp calls = %d", len(m.calls))
	}
	call := m.calls[0]
	if call.Name != "exec_command" {
		t.Errorf("call name = %q, want exec_command", call.Name)
	}
	if call.Args["pod"] != "test-pod" || call.Args["command"] != "whoami" || call.Args["container"] != "main" {
		t.Errorf("call args = %+v", call.Args)
	}
}

func TestExecCommand_RequiresCommand(t *testing.T) {
	ts, srv, priv := testServer(t)
	seedAlice(srv)

	tok := mintToken(t, priv, "test-kid", "alice", "strata-tui", false)
	body := `{"namespace":"kube-system"}`
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/clusters/cl-001/pods/test-pod/exec", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestCreateCluster_HappyPath(t *testing.T) {
	ts, srv, priv := testServer(t)
	seedAlice(srv)

	tok := mintToken(t, priv, "test-kid", "alice", "strata-tui", false)
	kubeconfig := `
apiVersion: v1
kind: Config
current-context: dev-ctx
contexts:
- name: dev-ctx
  context:
    cluster: dev-cluster
    user: dev-user
clusters:
- name: dev-cluster
  cluster:
    server: https://127.0.0.1:6443
users:
- name: dev-user
  user:
    token: fake-token
`
	reqBody := `{"name":"staging","kubeconfig":` + strconvQuote(kubeconfig) + `}`
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/clusters", strings.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	var res struct {
		Cluster store.Cluster `json:"cluster"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(res.Cluster.ID, "cl-") {
		t.Errorf("cluster id = %q, expected prefix cl-", res.Cluster.ID)
	}
	if res.Cluster.Name != "staging" {
		t.Errorf("name = %q, want 'staging'", res.Cluster.Name)
	}
	if res.Cluster.Context != "dev-ctx" {
		t.Errorf("context = %q, want 'dev-ctx'", res.Cluster.Context)
	}

	// Verify the stored cluster in the fakeStore has encrypted credentials
	fs := srv.store.(*fakeStore)
	stored, err := fs.GetCluster(context.Background(), "alice", res.Cluster.ID)
	if err != nil {
		t.Fatalf("cluster not in store: %v", err)
	}
	if stored.EncryptedKubeconfig == "" {
		t.Fatal("expected non-empty encrypted kubeconfig in store")
	}

	// Decrypt stored credentials and check they match original
	key := crypto.DeriveKey(srv.cfg.EncryptionSecret)
	decrypted, err := crypto.Decrypt(key, stored.EncryptedKubeconfig)
	if err != nil {
		t.Fatalf("failed to decrypt stored kubeconfig: %v", err)
	}
	if strings.TrimSpace(string(decrypted)) != strings.TrimSpace(kubeconfig) {
		t.Errorf("decrypted = %q, want %q", string(decrypted), kubeconfig)
	}
}

func TestCreateCluster_InvalidYAML(t *testing.T) {
	ts, srv, priv := testServer(t)
	seedAlice(srv)

	tok := mintToken(t, priv, "test-kid", "alice", "strata-tui", false)
	reqBody := `{"name":"staging","kubeconfig":"this is not: yaml: [broken"}`
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/clusters", strings.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestCreateCluster_MissingFields(t *testing.T) {
	ts, srv, priv := testServer(t)
	seedAlice(srv)

	tok := mintToken(t, priv, "test-kid", "alice", "strata-tui", false)
	reqBody := `{"name":"","kubeconfig":"apiVersion: v1"}`
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/clusters", strings.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestDeleteCluster_HappyPath(t *testing.T) {
	ts, srv, priv := testServer(t)
	seedAlice(srv)

	tok := mintToken(t, priv, "test-kid", "alice", "strata-tui", false)
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/clusters/cl-001", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	// Verify cluster is now deleted
	fs := srv.store.(*fakeStore)
	if _, err := fs.GetCluster(context.Background(), "alice", "cl-001"); err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound after deletion, got %v", err)
	}
}

func TestDeleteCluster_NotFound(t *testing.T) {
	ts, srv, priv := testServer(t)
	seedAlice(srv)

	tok := mintToken(t, priv, "test-kid", "alice", "strata-tui", false)
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/clusters/cl-nonexistent", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestDeleteCluster_OtherUser(t *testing.T) {
	ts, srv, priv := testServer(t)
	seedAlice(srv)
	fs := srv.store.(*fakeStore)
	_ = fs.EnsureUser(context.Background(), store.User{ID: "bob", Username: "bob"})

	tok := mintToken(t, priv, "test-kid", "bob", "strata-tui", false)
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/clusters/cl-001", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestListPods_PropagatesEncryptedKubeconfig(t *testing.T) {
	ts, srv, priv := testServer(t)
	fs := srv.store.(*fakeStore)
	_ = fs.EnsureUser(context.Background(), store.User{ID: "alice", Username: "alice"})
	_ = fs.CreateCluster(context.Background(), store.Cluster{
		ID: "cl-enc-mcp", UserID: "alice", Name: "enc-cluster", Context: "ctx",
	}, store.ClusterCreds{
		EncryptedKubeconfig: "test-encrypted-payload",
	})

	tok := mintToken(t, priv, "test-kid", "alice", "strata-tui", false)
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/clusters/cl-enc-mcp/pods", nil)
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
	if len(m.calls) != 1 {
		t.Fatalf("mcp calls = %d", len(m.calls))
	}
	if got := m.calls[0].Args["kubeconfig_encrypted"]; got != "test-encrypted-payload" {
		t.Errorf("args[kubeconfig_encrypted] = %v", got)
	}
	if got := m.calls[0].Headers.Get("X-Strata-Encrypted-Kubeconfig"); got != "test-encrypted-payload" {
		t.Errorf("X-Strata-Encrypted-Kubeconfig = %q", got)
	}
}

func strconvQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func TestListHistory_EndToEnd(t *testing.T) {
	ts, srv, priv := testServer(t)
	seedAlice(srv)

	tok := mintToken(t, priv, "test-kid", "alice", "strata-tui", false)

	// Perform a pod deletion
	delReq, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/clusters/cl-001/pods/nginx-pod?namespace=prod", nil)
	delReq.Header.Set("Authorization", "Bearer "+tok)
	delReq.Header.Set("X-Strata-Client", "tui")
	delResp, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatal(err)
	}
	delResp.Body.Close()
	if delResp.StatusCode != http.StatusOK {
		t.Fatalf("delete pod status = %d", delResp.StatusCode)
	}

	// Query history
	histReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/history", nil)
	histReq.Header.Set("Authorization", "Bearer "+tok)
	histResp, err := http.DefaultClient.Do(histReq)
	if err != nil {
		t.Fatal(err)
	}
	defer histResp.Body.Close()
	if histResp.StatusCode != http.StatusOK {
		t.Fatalf("history status = %d", histResp.StatusCode)
	}

	var histBody struct {
		History []store.ActionHistory `json:"history"`
	}
	if err := json.NewDecoder(histResp.Body).Decode(&histBody); err != nil {
		t.Fatal(err)
	}
	if len(histBody.History) != 1 {
		t.Fatalf("expected 1 history item, got %d", len(histBody.History))
	}
	item := histBody.History[0]
	if item.ActionType != "delete_pod" {
		t.Errorf("expected action delete_pod, got %s", item.ActionType)
	}
	if item.Target != "prod/nginx-pod" {
		t.Errorf("expected target prod/nginx-pod, got %s", item.Target)
	}
	if item.Status != "success" {
		t.Errorf("expected status success, got %s", item.Status)
	}
	if item.ClientType != "tui" {
		t.Errorf("expected client_type tui, got %s", item.ClientType)
	}
}

func TestListClusterHistory_FilterAndNotFound(t *testing.T) {
	ts, srv, priv := testServer(t)
	seedAlice(srv)

	tok := mintToken(t, priv, "test-kid", "alice", "strata-tui", false)

	// Non-existent cluster history returns 404
	badReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/clusters/non-existent/history", nil)
	badReq.Header.Set("Authorization", "Bearer "+tok)
	badResp, err := http.DefaultClient.Do(badReq)
	if err != nil {
		t.Fatal(err)
	}
	badResp.Body.Close()
	if badResp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for non-existent cluster history, got %d", badResp.StatusCode)
	}

	// Valid cluster returns empty list initially
	goodReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/clusters/cl-001/history", nil)
	goodReq.Header.Set("Authorization", "Bearer "+tok)
	goodResp, err := http.DefaultClient.Do(goodReq)
	if err != nil {
		t.Fatal(err)
	}
	defer goodResp.Body.Close()
	if goodResp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for valid cluster history, got %d", goodResp.StatusCode)
	}
	var histBody struct {
		History []store.ActionHistory `json:"history"`
	}
	_ = json.NewDecoder(goodResp.Body).Decode(&histBody)
	if len(histBody.History) != 0 {
		t.Errorf("expected 0 items initially, got %d", len(histBody.History))
	}
}
