package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"

	"github.com/strata/orchestrator/internal/store"
)

// fakeStore implements ClusterStore for tests.
type fakeStore struct {
	mu       sync.Mutex
	users    map[string]store.User
	clusters map[string]store.Cluster
	history  []store.ActionHistory
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		users:    map[string]store.User{},
		clusters: map[string]store.Cluster{},
		history:  []store.ActionHistory{},
	}
}

func (f *fakeStore) EnsureUser(_ context.Context, u store.User) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.users[u.ID] = u
	return nil
}

func (f *fakeStore) ListClusters(_ context.Context, userID string) ([]store.Cluster, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]store.Cluster, 0, len(f.clusters))
	for _, c := range f.clusters {
		if c.UserID == userID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (f *fakeStore) GetCluster(_ context.Context, userID, clusterID string) (*store.Cluster, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.clusters[clusterID]
	if !ok || c.UserID != userID {
		return nil, store.ErrNotFound
	}
	return &c, nil
}

// CreateCluster implements ClusterStore for tests.
func (f *fakeStore) CreateCluster(_ context.Context, c store.Cluster, creds store.ClusterCreds) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	c.KubeconfigPath = creds.KubeconfigPath
	c.EncryptedKubeconfig = creds.EncryptedKubeconfig
	c.DEKCiphertext = creds.DEKCiphertext
	f.clusters[c.ID] = c
	return nil
}

// DeleteCluster implements ClusterStore for tests.
func (f *fakeStore) DeleteCluster(_ context.Context, userID, clusterID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.clusters[clusterID]
	if !ok || c.UserID != userID {
		return store.ErrNotFound
	}
	delete(f.clusters, clusterID)
	// cascade history deletion
	var filtered []store.ActionHistory
	for _, a := range f.history {
		if a.ClusterID != clusterID {
			filtered = append(filtered, a)
		}
	}
	f.history = filtered
	return nil
}

func (f *fakeStore) RecordAction(_ context.Context, a store.ActionHistory) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.history = append(f.history, a)
	return nil
}

func (f *fakeStore) ListHistory(_ context.Context, userID, clusterID string, limit int) ([]store.ActionHistory, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var out []store.ActionHistory
	for i := len(f.history) - 1; i >= 0; i-- {
		a := f.history[i]
		if a.UserID != userID {
			continue
		}
		if clusterID != "" && a.ClusterID != clusterID {
			continue
		}
		out = append(out, a)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

// fakeMCP implements MCPCaller for tests. The configured result is
// returned verbatim. failWith, if set, makes CallToolWithHeaders
// return an error.
type fakeMCP struct {
	mu       sync.Mutex
	result   json.RawMessage
	calls    []mcpCall
	failWith error
}

type mcpCall struct {
	Name    string
	Args    map[string]any
	Headers http.Header
}

func newFakeMCP() *fakeMCP {
	return &fakeMCP{result: json.RawMessage(`[{"name":"p1"}]`)}
}

func (f *fakeMCP) CallToolWithHeaders(_ context.Context, name string, args map[string]any, headers http.Header) (json.RawMessage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failWith != nil {
		return nil, f.failWith
	}
	f.calls = append(f.calls, mcpCall{Name: name, Args: args, Headers: headers})
	return f.result, nil
}

// errMCP is a sentinel for tests that need to assert on a specific
// error path.
var errMCP = errors.New("mcp: simulated failure")
