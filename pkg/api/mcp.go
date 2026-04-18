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

		var user models.User
		if err := a.DB.Where("token = ?", token).First(&user).Error; err != nil {
			return nil
		}

		// Get user's installed + enabled apps.
		var installs []models.AppInstall
		a.DB.Where("user_id = ? AND enabled = ?", user.ID, true).Find(&installs)

		// Build a per-user MCP server with only their tools/prompts + platform tools.
		return a.MCPFactory.BuildServer(installs, a.DB, user.ID)
	}, nil)
}
