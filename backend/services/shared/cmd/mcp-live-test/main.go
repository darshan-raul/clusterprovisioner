package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/strata/shared/pkg/mcp"
)

func main() {
	url := os.Getenv("MCP_URL")
	if url == "" {
		url = "http://127.0.0.1:18029/mcp"
	}
	c := mcp.NewClient(url)
	ctx := context.Background()
	if err := c.Initialize(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "init:", err)
		os.Exit(1)
	}
	fmt.Println("initialized OK")
	raw, err := c.CallTool(ctx, "list_pods", map[string]any{
		"cluster_id": "cl-mock-01",
		"namespace":  "default",
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "call:", err)
		os.Exit(1)
	}
	out := map[string]any{}
	_ = json.Unmarshal(raw, &out)
	fmt.Printf("result keys: ")
	for k := range out {
		fmt.Printf("%s ", k)
	}
	fmt.Println()
}
