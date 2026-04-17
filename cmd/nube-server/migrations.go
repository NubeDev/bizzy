package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/NubeDev/bizzy/pkg/api"
	"github.com/NubeDev/bizzy/pkg/apps"
	"github.com/NubeDev/bizzy/pkg/models"
	"gorm.io/gorm"
)

// migrateSessionProvider backfills provider="claude" on sessions that predate
// the multi-provider fields (Phase 1a).
func migrateSessionProvider(db *gorm.DB) {
	result := db.Model(&models.Session{}).Where("provider = '' OR provider IS NULL").Update("provider", "claude")
	if result.RowsAffected > 0 {
		log.Printf("[migrate] backfilled provider=claude on %d sessions", result.RowsAffected)
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
