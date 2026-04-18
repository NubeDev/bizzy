// Package api provides the REST API router and handlers for the multi-tenant server.
package api

import (
	"net/http"

	"github.com/NubeDev/bizzy/pkg/airunner"
	"github.com/NubeDev/bizzy/pkg/apps"
	"github.com/NubeDev/bizzy/pkg/auth"
	"github.com/NubeDev/bizzy/pkg/command"
	"github.com/NubeDev/bizzy/pkg/memory"
	"github.com/NubeDev/bizzy/pkg/models"
	"github.com/NubeDev/bizzy/pkg/plugin"
	"github.com/NubeDev/bizzy/pkg/revision"
	"github.com/NubeDev/bizzy/pkg/secrets"
	"github.com/NubeDev/bizzy/pkg/services"
	"github.com/NubeDev/bizzy/pkg/version"
	"github.com/NubeDev/bizzy/pkg/workflow"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// API holds the shared state for all API handlers.
type API struct {
	DB          *gorm.DB
	AppRegistry *apps.Registry
	MCPFactory  *apps.MCPFactory
	Runners     *airunner.Registry // AI providers (claude, ollama, openai, etc.)
	Jobs        *airunner.JobStore // In-memory async job store

	Memory *memory.Store // Server + per-user memory

	// Workflow engine.
	Workflows     *workflow.Runner
	WorkflowStore *workflow.Store

	// Reusable application services (decoupled from HTTP).
	AgentSvc *services.AgentService
	ToolSvc  *services.ToolService

	// Plugin system (optional — nil if NATS bus is not enabled).
	PluginRegistry *plugin.Registry

	// Command bus (optional — nil if not configured).
	CmdRouter      *command.Router
	WebhookHandler func(http.ResponseWriter, *http.Request) // set by main if webhook adapter is enabled

	// Revision history for undo/revert.
	Revisions *revision.Store

	// Encrypted secrets store (optional — nil if not configured).
	Secrets *secrets.Store
}

// ProviderConfigGet loads the provider config from the database.
func (a *API) ProviderConfigGet() models.ProviderConfig {
	var cfg models.ProviderConfig
	a.DB.First(&cfg, "id = ?", "default")
	return cfg
}

// ProviderConfigSet saves the provider config to the database.
func (a *API) ProviderConfigSet(cfg models.ProviderConfig) error {
	cfg.ID = "default"
	return a.DB.Save(&cfg).Error
}

// SetupRouter creates a gin router with all routes mounted.
func (a *API) SetupRouter() *gin.Engine {
	r := gin.Default()

	// CORS middleware — allows cross-origin requests from the frontend dev server.
	r.Use(CORSMiddleware())

	// Health check (no auth).
	r.GET("/health", func(c *gin.Context) {
		var userCount int64
		a.DB.Model(&models.User{}).Count(&userCount)
		c.JSON(200, gin.H{
			"status":   "ok",
			"versions": version.All(),
			"users":    userCount,
			"apps":     len(a.AppRegistry.List()),
		})
	})

	// Bootstrap endpoint: create the first workspace + admin user without auth.
	// Only works when no users exist yet.
	r.POST("/bootstrap", a.bootstrap)

	// All other routes require auth.
	authed := r.Group("/", auth.Middleware(a.DB))

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

	// App Workshop: test a tool script in a sandboxed runtime.
	authed.POST("/api/apps/test-tool", a.testTool)

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

	// Bootstrap prompts — built-in reference docs, no app install required.
	authed.GET("/api/bootstrap/prompts", a.listBootstrapPrompts)
	authed.GET("/api/bootstrap/prompts/:name", a.getBootstrapPrompt)

	// Memory API.
	authed.GET("/api/memory/me", a.getMyMemory)
	authed.PUT("/api/memory/me", a.setMyMemory)
	authed.POST("/api/memory/me", a.appendMyMemory)
	admin.GET("/api/memory/server", a.getServerMemory)
	admin.PUT("/api/memory/server", a.setServerMemory)

	// Agent API.
	authed.GET("/api/agents", a.listAgents)
	authed.POST("/api/agents/tools/:name", a.callTool)
	authed.GET("/api/agents/sessions", a.listSessions)
	authed.GET("/api/agents/sessions/app/:name", a.getLatestAppSession)
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

	// --- Command Bus ---
	if a.CmdRouter != nil {
		authed.POST("/api/command", a.handleCommand)
		authed.GET("/api/command/help", a.handleCommandHelp)
		authed.GET("/api/events/stream", a.handleEventStream)

		// Webhook inbound endpoint.
		if a.WebhookHandler != nil {
			r.POST("/hooks/command", gin.WrapF(a.WebhookHandler))
		}

		// Adapter management (admin).
		adm := admin.Group("/api/adapters")
		adm.GET("", a.listAdapters)

		// Cron management (admin).
		cronRoutes := admin.Group("/api/cron")
		cronRoutes.GET("", a.listCronEntries)
		cronRoutes.POST("", a.createCronEntry)
		cronRoutes.DELETE("/:id", a.deleteCronEntry)
		cronRoutes.PATCH("/:id", a.toggleCronEntry)

		// Notification preferences (per-user).
		authed.GET("/api/notifications/prefs", a.getNotifyPrefs)
		authed.PUT("/api/notifications/prefs", a.updateNotifyPrefs)

		// Webhook logs (admin).
		admin.GET("/api/webhooks/logs", a.listWebhookLogs)
	}

	// --- Plugins ---
	// Upload always available (DB only); other routes require the NATS plugin system.
	admin.POST("/api/plugins/upload", a.uploadPlugin)
	if a.PluginRegistry != nil {
		plugins := admin.Group("/api/plugins")
		plugins.GET("", a.listPlugins)
		plugins.GET("/:name", a.getPlugin)
		plugins.DELETE("/:name", a.deletePlugin)
		plugins.POST("/:name/disable", a.disablePlugin)
		plugins.POST("/:name/enable", a.enablePlugin)
	}

	// --- Secrets ---
	if a.Secrets != nil {
		// Global secrets (admin only).
		sec := admin.Group("/api/secrets")
		sec.GET("", a.listAllSecrets)
		sec.GET("/:ownerType/:ownerName", a.listGlobalSecrets)
		sec.PUT("/:ownerType/:ownerName/:key", a.setGlobalSecret)
		sec.DELETE("/:ownerType/:ownerName/:key", a.deleteGlobalSecret)

		// User secrets (per-user, any authenticated user).
		userSec := authed.Group("/api/secrets/me")
		userSec.GET("/:ownerType/:ownerName", a.listUserSecrets)
		userSec.PUT("/:ownerType/:ownerName/:key", a.setUserSecret)
		userSec.DELETE("/:ownerType/:ownerName/:key", a.deleteUserSecret)
	}

	// --- Workflows ---
	wf := authed.Group("/api/workflows")
	wf.POST("/run", a.runWorkflow)
	wf.GET("/definitions", a.listWorkflowDefs)
	wf.GET("", a.listWorkflowRuns)
	wf.GET("/:id", a.getWorkflowRun)
	wf.POST("/:id/approve", a.approveWorkflow)
	wf.POST("/:id/cancel", a.cancelWorkflow)

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
	myApps.GET("/:id/chat", a.getBuilderChat)
	myApps.DELETE("/:id/chat", a.deleteBuilderChat)
	myApps.POST("/:id/publish", a.publishStoreApp)
	myApps.PATCH("/:id/visibility", a.setStoreAppVisibility)

	myApps.POST("/upload", a.uploadStoreApp)

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

	// Revision history.
	myApps.GET("/:id/revisions/:type/:entityName", a.listRevisions)
	myApps.POST("/:id/revisions/:type/:entityName/revert/:rev", a.revertRevision)

	return r
}
