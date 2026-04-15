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
	"github.com/NubeDev/bizzy/pkg/jsondb"
	"github.com/NubeDev/bizzy/pkg/models"
)

func main() {
	dataDir := os.Getenv("NUBE_DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}
	appsDir := filepath.Join(dataDir, "apps")
	os.MkdirAll(appsDir, 0755)

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
	storeApps, err := jsondb.NewCollection[models.StoreApp](filepath.Join(dataDir, "store_apps.json"))
	if err != nil {
		log.Fatalf("failed to load store_apps: %v", err)
	}
	appShares, err := jsondb.NewCollection[models.AppShare](filepath.Join(dataDir, "app_shares.json"))
	if err != nil {
		log.Fatalf("failed to load app_shares: %v", err)
	}
	appReviews, err := jsondb.NewCollection[models.AppReview](filepath.Join(dataDir, "app_reviews.json"))
	if err != nil {
		log.Fatalf("failed to load app_reviews: %v", err)
	}

	// Migrate: store apps with inline content but no disk files → write to disk.
	migrateStoreAppsToDisk(storeApps, appsDir)

	// Load app registry from disk.
	registry, err := apps.NewRegistry(appsDir)
	if err != nil {
		log.Fatalf("failed to load apps: %v", err)
	}

	// Sync: disk apps without a store_apps.json record → auto-create one.
	syncDiskAppsToStore(registry, storeApps)

	// Build MCP factory.
	mcpFactory := apps.NewMCPFactory(registry)

	a := &api.API{
		Workspaces:  workspaces,
		Users:       users,
		AppInstalls: appInstalls,
		Sessions:    sessions,
		AppRegistry: registry,
		MCPFactory:  mcpFactory,
		Runners:     airunner.NewRegistry(),
		StoreApps:   storeApps,
		AppShares:   appShares,
		AppReviews:  appReviews,
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

// migrateStoreAppsToDisk writes disk files for any store app that has inline
// content but no directory on disk yet.
func migrateStoreAppsToDisk(storeApps *jsondb.Collection[models.StoreApp], appsDir string) {
	all := storeApps.All()
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

// syncDiskAppsToStore creates store_apps.json records for any disk app that
// doesn't have one yet (e.g. system apps shipped with the code).
func syncDiskAppsToStore(registry *apps.Registry, storeApps *jsondb.Collection[models.StoreApp]) {
	synced := 0
	for _, app := range registry.List() {
		_, found := storeApps.FindOne(func(sa models.StoreApp) bool {
			return sa.Name == app.Name
		})
		if found {
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

		storeApps.Create(sa)
		synced++
		log.Printf("[sync] created store record for disk app: %s", app.Name)
	}
	if synced > 0 {
		log.Printf("[sync] synced %d disk apps to store_apps.json", synced)
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
