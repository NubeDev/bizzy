package airunner

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mcpToolClient is a lightweight wrapper around the MCP SDK client that
// connects to the platform's own MCP endpoint to list and call tools.
// Used by the server-side agent loop to give non-Claude providers tool access.
type mcpToolClient struct {
	session *mcp.ClientSession
}

// newMCPToolClient connects to the MCP server at mcpURL with the given Bearer token,
// performs the initialization handshake, and returns a ready-to-use client.
func newMCPToolClient(ctx context.Context, mcpURL, token string) (*mcpToolClient, error) {
	transport := &mcp.StreamableClientTransport{
		Endpoint: mcpURL,
		HTTPClient: &http.Client{
			Transport: &authRoundTripper{token: token},
		},
	}

	client := mcp.NewClient(
		&mcp.Implementation{Name: "bizzy-agent-loop", Version: "0.1.0"},
		nil,
	)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, err
	}

	return &mcpToolClient{session: session}, nil
}

// listTools fetches all tools available to the current user from the MCP server.
func (c *mcpToolClient) listTools(ctx context.Context) ([]*mcp.Tool, error) {
	result, err := c.session.ListTools(ctx, nil)
	if err != nil {
		return nil, err
	}
	return result.Tools, nil
}

// callTool executes a tool by name with JSON arguments and returns the text result.
func (c *mcpToolClient) callTool(ctx context.Context, name string, args json.RawMessage) (string, error) {
	result, err := c.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		return "", err
	}

	// Extract text from content blocks.
	var parts []string
	for _, content := range result.Content {
		if tc, ok := content.(*mcp.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	return strings.Join(parts, "\n"), nil
}

func (c *mcpToolClient) close() {
	if err := c.session.Close(); err != nil {
		log.Printf("[mcpclient] close error: %v", err)
	}
}

// authRoundTripper injects a Bearer token into every HTTP request.
type authRoundTripper struct {
	token string
}

func (t *authRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+t.token)
	return http.DefaultTransport.RoundTrip(req)
}
