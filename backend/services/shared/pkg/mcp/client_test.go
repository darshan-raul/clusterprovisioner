package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeMCPServer simulates a streamable-HTTP MCP server.
// It serves: initialize, notifications/initialized (no-op), tools/call.
func fakeMCPServer(t *testing.T, toolResult map[string]any) *httptest.Server {
	t.Helper()
	var calls atomic.Int32
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var req Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "test-session-123")
			resp := Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(`{"protocolVersion":"2025-06-18","serverInfo":{"name":"fake","version":"0"}}`),
			}
			_ = json.NewEncoder(w).Encode(resp)
		case "notifications/initialized":
			// no-op
			w.WriteHeader(204)
		case "tools/call":
			resultBytes, _ := json.Marshal(toolResult)
			resp := Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: json.RawMessage(
					`{"content":[{"type":"text","text":` + string(resultBytes) + `}]}`,
				),
			}
			_ = json.NewEncoder(w).Encode(resp)
		default:
			http.Error(w, "unknown method", 400)
		}
	}))
}

func TestClient_InitializeAndCallTool(t *testing.T) {
	srv := fakeMCPServer(t, map[string]any{"pods": []string{"a", "b"}})
	defer srv.Close()

	c := NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	raw, err := c.CallTool(ctx, "list_pods", map[string]any{"namespace": "default"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !strings.Contains(string(raw), `"pods"`) {
		t.Errorf("result missing 'pods': %s", raw)
	}
	if sid, _ := c.sessionID.Load().(string); sid != "test-session-123" {
		t.Errorf("session id = %q", sid)
	}
}

func TestClient_ReinitializeIsIdempotent(t *testing.T) {
	srv := fakeMCPServer(t, map[string]any{"ok": true})
	defer srv.Close()

	c := NewClient(srv.URL)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if err := c.Initialize(ctx); err != nil {
			t.Fatalf("init %d: %v", i, err)
		}
	}
	if _, err := c.CallTool(ctx, "list_pods", nil); err != nil {
		t.Fatalf("call after reinit: %v", err)
	}
}

func TestClient_PropagatesRPCError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req Request
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		if req.Method == "initialize" {
			w.Header().Set("Mcp-Session-Id", "x")
			_ = json.NewEncoder(w).Encode(Response{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(`{}`)})
			return
		}
		_ = json.NewEncoder(w).Encode(Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: -32601, Message: "method not found"},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	ctx := context.Background()
	_, err := c.CallTool(ctx, "nonexistent", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "method not found") {
		t.Errorf("err = %v", err)
	}
}

// TestClient_SSEResponseWithoutIDLine covers the FastMCP emit format
// where the SSE event has no `id:` line — only `event: message` and
// `data: {...}`. The JSON-RPC id is inside the data payload.
func TestClient_SSEResponseWithoutIDLine(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req Request
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		switch req.Method {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "sse-session")
			fmt.Fprintf(w, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":%d,\"result\":{\"protocolVersion\":\"2025-06-18\"}}\n\n", req.ID)
		case "notifications/initialized":
			w.WriteHeader(204)
		case "tools/call":
			fmt.Fprintf(w, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":%d,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"ok\"}]}}\n\n", req.ID)
		default:
			http.Error(w, "unknown", 400)
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	ctx := context.Background()
	if _, err := c.CallTool(ctx, "list_pods", nil); err != nil {
		t.Fatalf("call: %v", err)
	}
}
