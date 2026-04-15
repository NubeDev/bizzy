// Package server provides a high-level wrapper around openapi2mcp for building MCP servers.
package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	openapi2mcp "github.com/NubeIO/openapi-mcp"
	"github.com/NubeIO/openapi-mcp/pkg/config"
	"github.com/NubeIO/openapi-mcp/pkg/middleware"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Tool describes a custom hand-written MCP tool.
type Tool struct {
	Name        string
	Description string
	Handler     func(ctx context.Context, params map[string]any) (string, error)
}

// Server wraps the MCP server with config, middleware, and custom tools.
type Server struct {
	cfg         config.Config
	mcpServer   *mcp.Server
	doc         *openapi3.T
	ops         []openapi2mcp.OpenAPIOperation
	chain       middleware.Chain
	customTools []Tool
	toolNames   []string
}

// New creates a new Server from the given config.
// It loads the OpenAPI spec, extracts operations, and prepares the MCP server.
// Call Serve() to start listening.
func New(cfg config.Config) (*Server, error) {
	cfg.ApplyEnv()
	cfg.Defaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	cfg.SetAuthEnv()

	doc, err := openapi2mcp.LoadOpenAPISpec(cfg.Spec)
	if err != nil {
		return nil, fmt.Errorf("loading spec: %w", err)
	}

	ops := openapi2mcp.ExtractOpenAPIOperations(doc)

	// Apply tag filter.
	if len(cfg.Tags) > 0 {
		ops = filterByTags(ops, cfg.Tags)
	}

	// Apply read-only filter.
	if cfg.ReadOnly {
		ops = filterReadOnly(ops)
	}

	return &Server{
		cfg: cfg,
		doc: doc,
		ops: ops,
	}, nil
}

// Use adds middleware to the server's chain.
func (s *Server) Use(mw ...middleware.Middleware) {
	s.chain.Use(mw...)
}

// AddTool registers a custom hand-written tool alongside the OpenAPI-generated ones.
func (s *Server) AddTool(t Tool) {
	s.customTools = append(s.customTools, t)
}

// ToolNames returns the list of registered tool names after Build/Serve.
func (s *Server) ToolNames() []string {
	return s.toolNames
}

// MCPServer returns the underlying MCP server (available after Build).
func (s *Server) MCPServer() *mcp.Server {
	return s.mcpServer
}

// HTTPHandler returns an http.Handler for mounting on custom routers (e.g. Gin).
// Calls Build() if not already built.
func (s *Server) HTTPHandler() http.Handler {
	if s.mcpServer == nil {
		s.Build()
	}
	srv := s.mcpServer
	return mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server {
		return srv
	}, nil)
}

// Build constructs the MCP server and registers all tools.
// Called automatically by Serve(), but can be called early for HTTPHandler().
func (s *Server) Build() {
	impl := &mcp.Implementation{Name: s.cfg.Name, Version: s.cfg.Version}
	s.mcpServer = mcp.NewServer(impl, nil)

	opts := &openapi2mcp.ToolGenOptions{
		ConfirmDangerousActions: s.cfg.ConfirmDangerous,
		Version:                s.cfg.Version,
	}

	// If middleware is present, wrap the default HTTP client to run hooks.
	if s.chain.Len() > 0 {
		chain := &s.chain
		opts.RequestHandler = func(req *http.Request) (*http.Response, error) {
			toolName := req.Header.Get("X-MCP-Tool")
			if err := chain.RunBefore(req.Context(), toolName, nil); err != nil {
				return nil, err
			}
			resp, err := http.DefaultClient.Do(req)
			result := &middleware.Result{}
			if resp != nil {
				result.Text = fmt.Sprintf("HTTP %d", resp.StatusCode)
			}
			chain.RunAfter(req.Context(), toolName, result, err)
			return resp, err
		}
	}

	s.toolNames = openapi2mcp.RegisterOpenAPITools(s.mcpServer, s.ops, s.doc, opts)

	// Register custom tools.
	for _, ct := range s.customTools {
		tool := &mcp.Tool{
			Name:        ct.Name,
			Description: ct.Description,
		}
		handler := ct.Handler
		mcp.AddTool(s.mcpServer, tool, func(ctx context.Context, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			text, err := handler(ctx, args)
			if err != nil {
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
					IsError: true,
				}, nil, nil
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: text}},
			}, nil, nil
		})
		s.toolNames = append(s.toolNames, ct.Name)
	}
}

// Serve builds and starts the MCP server using the configured transport.
func (s *Server) Serve() error {
	s.Build()

	switch s.cfg.Transport {
	case "stdio":
		fmt.Fprintf(os.Stderr, "[%s] serving via stdio (%d tools)\n", s.cfg.Name, len(s.toolNames))
		return s.mcpServer.Run(context.Background(), &mcp.StdioTransport{})
	case "http":
		srv := s.mcpServer
		handler := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server {
			return srv
		}, nil)
		fmt.Fprintf(os.Stderr, "[%s] serving HTTP on %s (%d tools)\n", s.cfg.Name, s.cfg.Addr, len(s.toolNames))
		return http.ListenAndServe(s.cfg.Addr, handler)
	default:
		return fmt.Errorf("unknown transport: %s (expected stdio or http)", s.cfg.Transport)
	}
}

// QuickStart creates and serves an MCP server in one call.
func QuickStart(cfg config.Config) error {
	srv, err := New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	return srv.Serve()
}

// --- helpers ---

func filterByTags(ops []openapi2mcp.OpenAPIOperation, tags []string) []openapi2mcp.OpenAPIOperation {
	tagSet := make(map[string]struct{}, len(tags))
	for _, t := range tags {
		tagSet[t] = struct{}{}
	}
	var filtered []openapi2mcp.OpenAPIOperation
	for _, op := range ops {
		for _, t := range op.Tags {
			if _, ok := tagSet[t]; ok {
				filtered = append(filtered, op)
				break
			}
		}
	}
	return filtered
}

func filterReadOnly(ops []openapi2mcp.OpenAPIOperation) []openapi2mcp.OpenAPIOperation {
	var filtered []openapi2mcp.OpenAPIOperation
	for _, op := range ops {
		if strings.EqualFold(op.Method, "get") || strings.EqualFold(op.Method, "head") || strings.EqualFold(op.Method, "options") {
			filtered = append(filtered, op)
		}
	}
	return filtered
}
