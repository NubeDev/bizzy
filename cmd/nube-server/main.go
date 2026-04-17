// nube-server is the multi-tenant central server for NubeIO developer tools.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/NubeDev/bizzy/pkg/airunner"
	"github.com/NubeDev/bizzy/pkg/api"
	"github.com/NubeDev/bizzy/pkg/apps"
	"github.com/NubeDev/bizzy/pkg/database"
	"github.com/NubeDev/bizzy/pkg/memory"
	"github.com/NubeDev/bizzy/pkg/models"
	"github.com/NubeDev/bizzy/pkg/services"
	"github.com/NubeDev/bizzy/pkg/workflow"
	"gorm.io/gorm"
)

func main() {
	dataDir := os.Getenv("NUBE_DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}
	// Resolve to absolute path so the server works regardless of working directory.
	dataDir, _ = filepath.Abs(dataDir)
	appsDir := filepath.Join(dataDir, "apps")
	os.MkdirAll(appsDir, 0755)

	addr := os.Getenv("NUBE_ADDR")
	if addr == "" {
		addr = ":8090"
	}

	// Open database (SQLite). Auto-migrates schema and imports legacy JSON files.
	db, err := database.Open(dataDir)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}

	// Migrate: backfill provider="claude" on existing sessions that have no provider set.
	migrateSessionProvider(db)

	// Migrate: store apps with inline content but no disk files → write to disk.
	migrateStoreAppsToDisk(db, appsDir)

	// Load app registry from disk.
	registry, err := apps.NewRegistry(appsDir)
	if err != nil {
		log.Fatalf("failed to load apps: %v", err)
	}

	// Sync: disk apps without a store_apps record → auto-create one.
	syncDiskAppsToStore(registry, db)

	// Build MCP factory.
	mcpFactory := apps.NewMCPFactory(registry)

	// Load workflow definitions from app directories.
	wfStore := workflow.NewStore()
	for _, app := range registry.List() {
		if err := wfStore.LoadFromAppDir(app.Name, app.Dir); err != nil {
			log.Printf("[workflows] failed to load workflows for %s: %v", app.Name, err)
		}
	}
	wfCount := 0
	for _, wfs := range wfStore.ListAll() {
		wfCount += len(wfs)
	}

	memStore := memory.NewStore(dataDir)

	runners := airunner.NewRegistry()

	// Create reusable application services.
	agentSvc := &services.AgentService{
		DB:          db,
		Memory:      memStore,
		MCPFactory:  mcpFactory,
		Runners:     runners,
		Jobs:        airunner.NewJobStore(),
		AppRegistry: registry,
	}

	toolSvc := &services.ToolService{
		DB:          db,
		AppRegistry: registry,
	}

	a := &api.API{
		DB:            db,
		AppRegistry:   registry,
		MCPFactory:    mcpFactory,
		Runners:       runners,
		Jobs:          agentSvc.Jobs,
		Memory:        memStore,
		WorkflowStore: wfStore,
		AgentSvc:      agentSvc,
		ToolSvc:       toolSvc,
	}

	// Wire up the workflow engine (uses services, not the API directly).
	a.Workflows = workflow.NewRunner(
		db,
		wfStore,
		api.NewWorkflowToolCaller(toolSvc),
		api.NewWorkflowPromptRunner(agentSvc),
	)

	// Apply saved provider config to runners (host overrides, etc.).
	a.ApplyProviderConfig(a.ProviderConfigGet())

	router := a.SetupRouter()

	var userCount int64
	db.Model(&models.User{}).Count(&userCount)

	fmt.Fprintf(os.Stderr, "[nube-server] listening on %s\n", addr)
	fmt.Fprintf(os.Stderr, "[nube-server] apps: %d loaded from %s\n", len(registry.List()), appsDir)
	fmt.Fprintf(os.Stderr, "[nube-server] workflows: %d loaded\n", wfCount)
	fmt.Fprintf(os.Stderr, "[nube-server] database: SQLite (bizzy.db)\n")
	if userCount == 0 {
		fmt.Fprintf(os.Stderr, "[nube-server] no users found — POST /bootstrap to create the first admin\n")
	}

	if err := router.Run(addr); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

// migrateStoreAppsToDisk writes disk files for any store app that has inline
// content but no directory on disk yet.
func migrateStoreAppsToDisk(db *gorm.DB, appsDir string) {
	var all []models.StoreApp
	db.Find(&all)
	migrated := 0
	for _, sa := range all {
		appDir := filepath.Join(appsDir, sa.Name)
		if _, err := os.Stat(filepath.Join(appDir, "app.yaml")); err == nil {
			continue
		}
		if err := api.WriteStoreAppToDisk(sa, appsDir); err != nil {
			log.Printf("[migrate] failed to migrate store app %s: %v", sa.Name, err)
			continue
		}
		migrated++
	}
	if migrated > 0 {
		log.Printf("[migrate] migrated %d store apps to disk", migrated)
	}
}

// syncDiskAppsToStore creates store_apps records for any disk app that
// doesn't have one yet (e.g. system apps shipped with the code).
func syncDiskAppsToStore(registry *apps.Registry, db *gorm.DB) {
	synced := 0
	for _, app := range registry.List() {
		var existing models.StoreApp
		if err := db.Where("name = ?", app.Name).First(&existing).Error; err == nil {
			continue
		}

		now := time.Now().UTC()
		sa := models.StoreApp{
			ID:          models.GenerateID("app-"),
			Name:        app.Name,
			DisplayName: app.Name,
			Description: app.Description,
			Version:     app.Version,
			Category:    categoryFromTags(app.Tags),
			Tags:        app.Tags,
			AuthorID:    "system",
			AuthorName:  app.Author,
			Visibility:  models.VisibilityPublic,
			Permissions: models.Permissions{
				AllowedHosts:     app.Permissions.AllowedHosts,
				DefaultToolClass: app.Permissions.DefaultToolClass,
			},
			Settings: convertSettings(app.Settings),
			Tools:    []models.StoreTool{},
			Prompts:  []models.StorePrompt{},
			CreatedAt: now,
			UpdatedAt: now,
			PublishedAt: &now,
		}
		if sa.Tags == nil {
			sa.Tags = []string{}
		}
		if sa.Permissions.AllowedHosts == nil {
			sa.Permissions.AllowedHosts = []string{}
		}
		if sa.Settings == nil {
			sa.Settings = []models.SettingDef{}
		}

		// Count tools/prompts from registry for the store record.
		tools := registry.GetTools(app.Name)
		for _, t := range tools {
			sa.Tools = append(sa.Tools, models.StoreTool{
				Name:        t.Name,
				Description: t.Description,
				ToolClass:   t.ToolClass,
				Mode:        t.Mode,
			})
		}
		prompts := registry.GetPrompts(app.Name)
		for _, p := range prompts {
			sp := models.StorePrompt{
				Name:        p.Name,
				Description: p.Description,
				Body:        p.Body,
			}
			for _, a := range p.Arguments {
				sp.Arguments = append(sp.Arguments, models.PromptArgument{
					Name:        a.Name,
					Description: a.Description,
					Required:    a.Required,
				})
			}
			sa.Prompts = append(sa.Prompts, sp)
		}

		db.Create(&sa)
		synced++
		log.Printf("[sync] created store record for disk app: %s", app.Name)
	}
	if synced > 0 {
		log.Printf("[sync] synced %d disk apps to store", synced)
	}
}

func categoryFromTags(tags []string) string {
	for _, t := range tags {
		switch t {
		case "iot-devices", "analytics", "devops", "marketing", "design", "utilities", "integrations", "automation":
			return t
		}
	}
	return "utilities"
}

// migrateSessionProvider backfills provider="claude" on sessions that predate
// the multi-provider fields (Phase 1a).
func migrateSessionProvider(db *gorm.DB) {
	result := db.Model(&models.Session{}).Where("provider = '' OR provider IS NULL").Update("provider", "claude")
	if result.RowsAffected > 0 {
		log.Printf("[migrate] backfilled provider=claude on %d sessions", result.RowsAffected)
	}
}

func convertSettings(defs []apps.SettingDef) []models.SettingDef {
	out := make([]models.SettingDef, len(defs))
	for i, d := range defs {
		out[i] = models.SettingDef{
			Key:      d.Key,
			Label:    d.Label,
			Type:     d.Type,
			Required: d.Required,
			Default:  d.Default,
		}
	}
	return out
}
