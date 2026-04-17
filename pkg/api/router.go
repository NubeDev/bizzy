// Package api provides the REST API router and handlers for the multi-tenant server.
package api

import (
	"github.com/NubeDev/bizzy/pkg/airunner"
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
	Runners        *airunner.Registry                          // AI providers (claude, ollama, openai, etc.)
	Jobs           *airunner.JobStore                          // In-memory async job store
	ProviderConfig *jsondb.ConfigFile[models.ProviderConfig]   // Global provider settings (admin)

	// Store collections.
	StoreApps  *jsondb.Collection[models.StoreApp]
	AppShares  *jsondb.Collection[models.AppShare]
	AppReviews *jsondb.Collection[models.AppReview]
}

// SetupRouter creates a gin router with all routes mounted.
func (a *API) SetupRouter() *gin.Engine {
	r := gin.Default()

	// CORS middleware — allows cross-origin requests from the frontend dev server.
	r.Use(CORSMiddleware())

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
	authed.GET("/api/agents/providers", a.listProviders)
	authed.POST("/api/agents/run/sync", a.runAgentREST)
	authed.GET("/api/agents/jobs", a.listJobs)
	authed.POST("/api/agents/jobs", a.submitJob)
	authed.GET("/api/agents/jobs/:id", a.pollJob)
	authed.DELETE("/api/agents/jobs/:id", a.cancelJob)

	// Provider test (any authenticated user).
	authed.POST("/api/agents/providers/:name/test", a.testProvider)

	// User preferences.
	authed.GET("/users/me/preferences", a.getUserPreferences)
	authed.PUT("/users/me/preferences", a.updateUserPreferences)

	// Agent WebSocket endpoints (auth via ?token= query param).
	r.GET("/api/agents/run", a.runAgentWS)
	r.GET("/api/agents/qa", a.runQAWS)

	// Admin: provider config + reload.
	admin.GET("/api/settings/providers", a.getProviderConfig)
	admin.PUT("/api/settings/providers", a.updateProviderConfig)
	admin.POST("/admin/reload-apps", a.reloadApps)

	// MCP endpoint: per-user tool serving.
	// Uses a single StreamableHTTPHandler with per-session user resolution.
	mcpHandler := a.buildMCPHandler()
	mcpGin := func(c *gin.Context) {
		mcpHandler.ServeHTTP(c.Writer, c.Request)
	}
	r.Any("/mcp", mcpGin)
	r.Any("/mcp/*path", mcpGin)

	// --- App Store ---

	store := authed.Group("/api/store")
	store.GET("/apps", a.listStoreApps)
	store.GET("/apps/:id", a.getStoreApp)
	store.GET("/categories", a.listCategories)
	store.GET("/apps/:id/reviews", a.listStoreAppReviews)

	store.POST("/apps/:id/install", a.installStoreApp)
	store.POST("/apps/:id/reviews", a.submitReview)
	store.PUT("/apps/:id/reviews", a.submitReview)
	store.DELETE("/apps/:id/reviews", a.deleteReview)

	myApps := authed.Group("/api/my/apps")
	myApps.GET("", a.listMyStoreApps)
	myApps.POST("", a.createStoreApp)
	myApps.GET("/:id", a.getMyStoreApp)
	myApps.PUT("/:id", a.updateStoreApp)
	myApps.DELETE("/:id", a.deleteStoreApp)
	myApps.POST("/:id/publish", a.publishStoreApp)
	myApps.PATCH("/:id/visibility", a.setStoreAppVisibility)

	myApps.POST("/:id/share", a.shareStoreApp)
	myApps.POST("/:id/share-link", a.shareStoreAppLink)
	myApps.GET("/:id/shares", a.listStoreAppShares)
	myApps.DELETE("/:id/shares/:shareId", a.deleteStoreAppShare)

	myApps.POST("/:id/tools", a.addStoreTool)
	myApps.PUT("/:id/tools/:name", a.updateStoreTool)
	myApps.DELETE("/:id/tools/:name", a.deleteStoreTool)

	myApps.POST("/:id/prompts", a.addStorePrompt)
	myApps.PUT("/:id/prompts/:name", a.updateStorePrompt)
	myApps.DELETE("/:id/prompts/:name", a.deleteStorePrompt)

	return r
}
