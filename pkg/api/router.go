// Package api provides the REST API router and handlers for the multi-tenant server.
package api

import (
	"github.com/NubeDev/bizzy/pkg/apps"
	"github.com/NubeDev/bizzy/pkg/auth"
	"github.com/NubeDev/bizzy/pkg/jsondb"
	"github.com/NubeDev/bizzy/pkg/models"
	"github.com/gin-gonic/gin"
)

// API holds the shared state for all API handlers.
type API struct {
	Workspaces  *jsondb.Collection[models.Workspace]
	Users       *jsondb.Collection[models.User]
	AppInstalls *jsondb.Collection[models.AppInstall]
	Sessions    *jsondb.Collection[models.Session]
	AppRegistry *apps.Registry
	MCPFactory  *apps.MCPFactory
}

// SetupRouter creates a gin router with all routes mounted.
func (a *API) SetupRouter() *gin.Engine {
	r := gin.Default()

	// Health check (no auth).
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
			"users":  a.Users.Count(),
			"apps":   len(a.AppRegistry.List()),
		})
	})

	// Bootstrap endpoint: create the first workspace + admin user without auth.
	// Only works when no users exist yet.
	r.POST("/bootstrap", a.bootstrap)

	// All other routes require auth.
	authed := r.Group("/", auth.Middleware(a.Users))

	// User self-service.
	authed.GET("/users/me", a.getMe)

	// User management (admin).
	admin := authed.Group("/", auth.RequireAdmin())
	admin.GET("/users/:id", a.getUser)
	admin.DELETE("/users/:id", a.deleteUser)

	// Token management (self or admin).
	authed.POST("/users/:id/token", a.rotateToken)
	authed.DELETE("/users/:id/token", a.revokeToken)

	// Workspace management (admin).
	admin.POST("/workspaces", a.createWorkspace)
	admin.DELETE("/workspaces/:id", a.deleteWorkspace)

	// Workspace listing (scoped by role).
	authed.GET("/workspaces", a.listWorkspaces)
	authed.GET("/workspaces/:id", a.getWorkspace)

	// User creation within a workspace (admin).
	admin.POST("/workspaces/:id/users", a.createUser)
	authed.GET("/workspaces/:id/users", a.listWorkspaceUsers)

	// App catalog.
	authed.GET("/apps", a.listApps)
	authed.GET("/apps/:id", a.getApp)

	// App CRUD (admin).
	admin.POST("/apps", a.createApp)
	admin.PUT("/apps/:id", a.updateApp)
	admin.DELETE("/apps/:id", a.deleteApp)

	// Tool CRUD within an app (admin).
	authed.GET("/apps/:id/tools", a.listAppTools)
	admin.POST("/apps/:id/tools", a.createTool)
	admin.PUT("/apps/:id/tools/:name", a.updateTool)
	admin.DELETE("/apps/:id/tools/:name", a.deleteTool)

	// Prompt CRUD within an app (admin).
	authed.GET("/apps/:id/prompts", a.listAppPrompts)
	admin.POST("/apps/:id/prompts", a.createPrompt)
	admin.DELETE("/apps/:id/prompts/:name", a.deletePrompt)

	// App install/uninstall.
	authed.POST("/apps/:id/install", a.installApp)
	authed.GET("/app-installs", a.listInstalls)
	authed.PATCH("/app-installs/:id", a.updateInstall)
	authed.DELETE("/app-installs/:id", a.uninstallApp)

	// Tools & prompts for current user (REST alternative to MCP).
	authed.GET("/my/tools", a.listMyTools)
	authed.GET("/my/prompts", a.listMyPrompts)
	authed.GET("/my/prompts/:name", a.getPrompt)

	// Agent API.
	authed.GET("/api/agents", a.listAgents)
	authed.POST("/api/agents/tools/:name", a.callTool)
	authed.GET("/api/agents/sessions", a.listSessions)
	authed.GET("/api/agents/sessions/:id", a.getSession)

	// Agent WebSocket endpoints (auth via ?token= query param).
	r.GET("/api/agents/run", a.runAgentWS)
	r.GET("/api/agents/qa", a.runQAWS)

	// Admin: reload apps from disk.
	admin.POST("/admin/reload-apps", a.reloadApps)

	// MCP endpoint: per-user tool serving.
	// Uses a single StreamableHTTPHandler with per-session user resolution.
	mcpHandler := a.buildMCPHandler()
	mcpGin := func(c *gin.Context) {
		mcpHandler.ServeHTTP(c.Writer, c.Request)
	}
	r.Any("/mcp", mcpGin)
	r.Any("/mcp/*path", mcpGin)

	return r
}
