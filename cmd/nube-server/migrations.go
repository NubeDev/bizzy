package main

import (
	"log"

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
