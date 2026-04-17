package main

import (
	"log"
	"time"

	"github.com/NubeDev/bizzy/pkg/apps"
	"github.com/NubeDev/bizzy/pkg/models"
	"gorm.io/gorm"
)

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
			Settings:    convertSettings(app.Settings),
			Tools:       []models.StoreTool{},
			Prompts:     []models.StorePrompt{},
			CreatedAt:   now,
			UpdatedAt:   now,
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
