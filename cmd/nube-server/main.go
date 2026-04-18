// nube-server is the multi-tenant central server for NubeIO developer tools.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/NubeDev/bizzy/pkg/airunner"
	"github.com/NubeDev/bizzy/pkg/api"
	"github.com/NubeDev/bizzy/pkg/apps"
	"github.com/NubeDev/bizzy/pkg/database"
	"github.com/NubeDev/bizzy/pkg/memory"
	"github.com/NubeDev/bizzy/pkg/models"
	"github.com/NubeDev/bizzy/pkg/revision"
	"github.com/NubeDev/bizzy/pkg/secrets"
	"github.com/NubeDev/bizzy/pkg/services"
	"github.com/NubeDev/bizzy/pkg/workflow"
)

func main() {
	ctx := context.Background()

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

	// Generate the bizzy-dev bootstrap app (recreated every startup).
	generateBootstrapApp(appsDir)

	// Load app registry from disk + database.
	registry, err := apps.NewRegistry(db, appsDir)
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

	// Encrypted secrets store.
	secretKey, err := secrets.LoadOrCreateKey(dataDir)
	if err != nil {
		log.Fatalf("failed to load secret key: %v", err)
	}
	secretStore := secrets.NewStore(db, secretKey)

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
		Secrets:       secretStore,
		Revisions:     revision.NewStore(db),
	}

	// Wire up the workflow engine (uses services, not the API directly).
	a.Workflows = workflow.NewRunner(
		db,
		wfStore,
		api.NewWorkflowToolCaller(toolSvc),
		api.NewWorkflowPromptRunner(agentSvc),
	)

	// --- Command Bus & Event Bus ---
	cleanup, err := setupCommandBus(ctx, a, agentSvc, toolSvc, db, dataDir)
	if err != nil {
		log.Printf("[command-bus] failed to start NATS bus: %v (command bus disabled)", err)
	} else {
		defer cleanup()
	}

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
