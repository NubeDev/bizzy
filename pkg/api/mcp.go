package api

import (
	"net/http"
	"strings"

	"github.com/NubeDev/bizzy/pkg/models"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// buildMCPHandler creates a single StreamableHTTPHandler that resolves the user
// from the request and builds a per-user MCP server for each session.
func (a *API) buildMCPHandler() http.Handler {
	return mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		// Extract bearer token from the MCP client request.
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			return nil
		}
		token := strings.TrimPrefix(header, "Bearer ")

		user, ok := a.Users.FindOne(func(u models.User) bool {
			return u.Token == token
		})
		if !ok {
			return nil
		}

		// Get user's installed + enabled apps.
		installs := a.AppInstalls.FindFunc(func(ai models.AppInstall) bool {
			return ai.UserID == user.ID && ai.Enabled
		})

		// Build a per-user MCP server with only their tools/prompts.
		return a.MCPFactory.BuildServer(installs)
	}, nil)
}
