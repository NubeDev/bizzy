// mcpdemo: MCP server that exposes a device API as tools.
//
// Prerequisites: start the fake API server first:
//   cd fakeserver && go run . -addr :9000
//
// Usage:
//   go run .                              # stdio mode (for Claude Code / Copilot)
//   go run . --transport http             # HTTP mode on :8080
//   go run . --base-url http://myhost:9000  # point at a real API
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/NubeIO/openapi-mcp/pkg/config"
	"github.com/NubeIO/openapi-mcp/pkg/middleware"
	"github.com/NubeIO/openapi-mcp/pkg/server"
)

func main() {
	transport := flag.String("transport", "stdio", "MCP transport: stdio or http")
	mcpAddr := flag.String("mcp-addr", ":8080", "MCP HTTP listen address")
	baseURL := flag.String("base-url", "http://localhost:9000", "upstream API base URL")
	flag.Parse()

	cfg := config.Config{
		Name:      "nube-demo",
		Version:   "0.1.0",
		Spec:      specPath(),
		BaseURL:   *baseURL,
		Transport: *transport,
		Addr:      *mcpAddr,
	}

	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("creating MCP server: %v", err)
	}

	// Logging middleware.
	srv.Use(middleware.Before(func(_ context.Context, tool string, _ map[string]any) error {
		fmt.Fprintf(os.Stderr, "[mw] tool called: %s\n", tool)
		return nil
	}))

	// Custom hand-written tool (not from the OpenAPI spec).
	srv.AddTool(server.Tool{
		Name:        "device_summary",
		Description: "Returns a summary of all devices. Shows count, online/offline status.",
		Handler: func(_ context.Context, _ map[string]any) (string, error) {
			resp, err := http.Get(*baseURL + "/api/v1/devices")
			if err != nil {
				return "", fmt.Errorf("calling API: %w", err)
			}
			defer resp.Body.Close()
			var result struct {
				Data  []json.RawMessage `json:"data"`
				Count int               `json:"count"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				return "", fmt.Errorf("decoding response: %w", err)
			}
			online, offline := 0, 0
			for _, raw := range result.Data {
				var d struct{ Online bool }
				json.Unmarshal(raw, &d)
				if d.Online {
					online++
				} else {
					offline++
				}
			}
			return fmt.Sprintf("Device Summary\nTotal:   %d\nOnline:  %d\nOffline: %d",
				result.Count, online, offline), nil
		},
	})

	fmt.Fprintf(os.Stderr, "[demo] MCP server starting (transport=%s, api=%s)\n", *transport, *baseURL)
	if err := srv.Serve(); err != nil {
		log.Fatalf("MCP server failed: %v", err)
	}
}

func specPath() string {
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidate := filepath.Join(dir, "..", "openapi.yaml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	for _, c := range []string{"openapi.yaml", "mcpdemo/openapi.yaml"} {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return "openapi.yaml"
}
