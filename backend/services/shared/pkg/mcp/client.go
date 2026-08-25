// Package mcp is the orchestrator's client to FastMCP servers over
// streamable-HTTP transport.
//
// The MCP spec defines a JSON-RPC 2.0 protocol with two transports:
// stdio and HTTP/SSE. Strata uses HTTP/SSE in the backend. The "streamable
// HTTP" variant sends POST requests and reads NDJSON or SSE responses.
//
// Reference: https://modelcontextprotocol.io/specification/2025-06-18/basic/transports
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Client calls tools on an MCP server.
type Client struct {
	baseURL    string
	httpClient *http.Client
	// sessionID is set by Initialize and reused on every subsequent
	// call. Streamable-HTTP requires the session header after init.
	sessionID atomic.Value // string
	// initMu serializes the Initialize handshake.
	initMu sync.Mutex
}

// NewClient returns a client that talks to the MCP server at baseURL
// (e.g. "http://mcp-k8s:8000/mcp").
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Request is one JSON-RPC request to send.
type Request struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      int64          `json:"id"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params,omitempty"`
}

// Response is one JSON-RPC response. Streamable-HTTP returns either
// a single JSON object or an SSE stream of one or more JSON objects.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError is the JSON-RPC 2.0 error object.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *RPCError) Error() string { return fmt.Sprintf("mcp: rpc error %d: %s", e.Code, e.Message) }

// Initialize performs the MCP handshake. Must be called once before
// any tool call. Idempotent.
func (c *Client) Initialize(ctx context.Context) error {
	c.initMu.Lock()
	defer c.initMu.Unlock()
	if sid, _ := c.sessionID.Load().(string); sid != "" {
		return nil
	}
	req := Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "strata-orchestrator",
				"version": "0.1.0",
			},
		},
	}
	resp, headers, err := c.callRaw(ctx, req, nil)
	if err != nil {
		return fmt.Errorf("mcp: initialize: %w", err)
	}
	if resp == nil {
		return errors.New("mcp: initialize: empty response")
	}
	if resp.Error != nil {
		return resp.Error
	}
	if sid := headers.Get("Mcp-Session-Id"); sid != "" {
		c.sessionID.Store(sid)
	}
	// Send notifications/initialized to acknowledge the handshake.
	note := Request{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}
	body, _ := json.Marshal(note)
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	if sid, _ := c.sessionID.Load().(string); sid != "" {
		httpReq.Header.Set("Mcp-Session-Id", sid)
	}
	_, _ = c.httpClient.Do(httpReq)
	return nil
}

// CallTool invokes an MCP tool by name with the given arguments. Returns
// the parsed result.
func (c *Client) CallTool(ctx context.Context, name string, args map[string]any) (json.RawMessage, error) {
	if err := c.Initialize(ctx); err != nil {
		return nil, err
	}
	id := time.Now().UnixNano()
	req := Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  "tools/call",
		Params: map[string]any{
			"name":      name,
			"arguments": args,
		},
	}
	resp, _, err := c.callRaw(ctx, req, nil)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, resp.Error
	}
	return resp.Result, nil
}

// CallToolWithHeaders is CallTool with extra HTTP headers (used to
// forward the user's JWT + the cluster's kubeconfig to the MCP server).
func (c *Client) CallToolWithHeaders(ctx context.Context, name string, args map[string]any, headers http.Header) (json.RawMessage, error) {
	if err := c.Initialize(ctx); err != nil {
		return nil, err
	}
	id := time.Now().UnixNano()
	req := Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  "tools/call",
		Params: map[string]any{
			"name":      name,
			"arguments": args,
		},
	}
	resp, _, err := c.callRaw(ctx, req, headers)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, resp.Error
	}
	return resp.Result, nil
}

// callRaw sends req and parses the response. Handles both regular JSON
// responses and SSE streams (one or more `data:` lines).
func (c *Client) callRaw(ctx context.Context, req Request, extraHeaders http.Header) (*Response, http.Header, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, nil, fmt.Errorf("mcp: marshal: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("mcp: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	if sid, _ := c.sessionID.Load().(string); sid != "" {
		httpReq.Header.Set("Mcp-Session-Id", sid)
	}
	for k, vs := range extraHeaders {
		for _, v := range vs {
			httpReq.Header.Add(k, v)
		}
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, nil, fmt.Errorf("mcp: do: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, resp.Header, fmt.Errorf("mcp: http %d: %s", resp.StatusCode, string(raw))
	}
	ct := resp.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "text/event-stream") {
		return parseSSEResponse(resp.Body, req.ID), resp.Header, nil
	}
	var out Response
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, resp.Header, fmt.Errorf("mcp: decode: %w", err)
	}
	return &out, resp.Header, nil
}

// parseSSEResponse scans an SSE stream and returns the response whose
// JSON-RPC id matches reqID. FastMCP and other MCP servers in practice
// emit events without an SSE “id:“ line — the id lives inside the
// JSON “data:“ payload. We therefore: (a) prefer the SSE “id:“ if
// present, and (b) fall back to scanning the JSON for the matching id.
func parseSSEResponse(r io.Reader, reqID int64) *Response {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 8*1024*1024)
	var currentSSEID string
	var dataLines []string
	flush := func() *Response {
		if len(dataLines) == 0 {
			currentSSEID = ""
			return nil
		}
		raw := []byte(strings.Join(dataLines, "\n"))
		var out Response
		if err := json.Unmarshal(raw, &out); err != nil {
			currentSSEID = ""
			dataLines = nil
			return nil
		}
		currentSSEID = ""
		dataLines = nil
		// Prefer the SSE-level id if the server sent one; otherwise the
		// JSON-RPC id inside the payload is authoritative.
		if currentSSEID != "" && currentSSEID != fmt.Sprintf("%d", out.ID) {
			return nil
		}
		if out.ID == reqID {
			return &out
		}
		return nil
	}
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "":
			if r := flush(); r != nil {
				return r
			}
		case strings.HasPrefix(line, "id: "):
			currentSSEID = strings.TrimPrefix(line, "id: ")
		case strings.HasPrefix(line, "data: "):
			dataLines = append(dataLines, strings.TrimPrefix(line, "data: "))
		}
	}
	// Some servers don't terminate with a blank line. Flush whatever's pending.
	if r := flush(); r != nil {
		return r
	}
	return nil
}

// Close releases the session. Safe to call multiple times.
func (c *Client) Close(ctx context.Context) error {
	sid, _ := c.sessionID.Load().(string)
	if sid == "" {
		return nil
	}
	c.sessionID.Store("")
	return nil
}

// ErrNotInitialized is returned when CallTool is invoked before Initialize.
var ErrNotInitialized = errors.New("mcp: client not initialized")
