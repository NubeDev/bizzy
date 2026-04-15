// nube-server is the multi-tenant central server for NubeIO developer tools.
// Stage 1: REST API for workspace/user management + auth.
// Stage 2: App loader, install flow, per-user MCP tool scoping.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/NubeDev/bizzy/pkg/api"
	"github.com/NubeDev/bizzy/pkg/apps"
	"github.com/NubeDev/bizzy/pkg/jsondb"
	"github.com/NubeDev/bizzy/pkg/models"
)

func main() {
	dataDir := os.Getenv("NUBE_DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}
	appsDir := os.Getenv("NUBE_APPS_DIR")
	if appsDir == "" {
		appsDir = "./apps"
	}
	addr := os.Getenv("NUBE_ADDR")
	if addr == "" {
		addr = ":8090"
	}

	// Load JSON DB collections.
	workspaces, err := jsondb.NewCollection[models.Workspace](filepath.Join(dataDir, "workspaces.json"))
	if err != nil {
		log.Fatalf("failed to load workspaces: %v", err)
	}
	users, err := jsondb.NewCollection[models.User](filepath.Join(dataDir, "users.json"))
	if err != nil {
		log.Fatalf("failed to load users: %v", err)
	}
	appInstalls, err := jsondb.NewCollection[models.AppInstall](filepath.Join(dataDir, "app_installs.json"))
	if err != nil {
		log.Fatalf("failed to load app_installs: %v", err)
	}
	sessions, err := jsondb.NewCollection[models.Session](filepath.Join(dataDir, "sessions.json"))
	if err != nil {
		log.Fatalf("failed to load sessions: %v", err)
	}

	// Load app registry.
	registry, err := apps.NewRegistry(appsDir)
	if err != nil {
		log.Fatalf("failed to load apps: %v", err)
	}

	// Build MCP factory.
	mcpFactory := apps.NewMCPFactory(registry)

	a := &api.API{
		Workspaces:  workspaces,
		Users:       users,
		AppInstalls: appInstalls,
		Sessions:    sessions,
		AppRegistry: registry,
		MCPFactory:  mcpFactory,
	}

	router := a.SetupRouter()

	fmt.Fprintf(os.Stderr, "[nube-server] listening on %s\n", addr)
	fmt.Fprintf(os.Stderr, "[nube-server] apps: %d loaded from %s\n", len(registry.List()), appsDir)
	if users.Count() == 0 {
		fmt.Fprintf(os.Stderr, "[nube-server] no users found — POST /bootstrap to create the first admin\n")
	}

	if err := router.Run(addr); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
