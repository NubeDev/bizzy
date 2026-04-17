// Package database initialises a GORM database (SQLite by default) and handles
// one-time migration of legacy JSON data files.
package database

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/NubeDev/bizzy/pkg/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// allModels is the list of models that GORM will auto-migrate.
var allModels = []any{
	&models.Workspace{},
	&models.User{},
	&models.AppInstall{},
	&models.Session{},
	&models.StoreApp{},
	&models.AppShare{},
	&models.AppReview{},
	&models.WorkflowRun{},
	&models.ProviderConfig{},
}

// Open creates (or opens) a SQLite database inside dataDir, runs auto-migration,
// and imports any legacy JSON files that exist alongside it.
func Open(dataDir string) (*gorm.DB, error) {
	dbPath := filepath.Join(dataDir, "bizzy.db")

	db, err := gorm.Open(sqlite.Open(dbPath+"?_journal_mode=WAL&_busy_timeout=5000"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("database: open %s: %w", dbPath, err)
	}

	// Enable WAL mode and foreign keys for SQLite.
	db.Exec("PRAGMA foreign_keys = ON")

	// Auto-migrate all models.
	if err := db.AutoMigrate(allModels...); err != nil {
		return nil, fmt.Errorf("database: migrate: %w", err)
	}

	// Seed default provider config if missing.
	var count int64
	db.Model(&models.ProviderConfig{}).Count(&count)
	if count == 0 {
		cfg := models.DefaultProviderConfig()
		db.Create(&cfg)
	}

	// Import legacy JSON files.
	importLegacyJSON(db, dataDir)

	return db, nil
}

// importLegacyJSON reads each legacy JSON collection file. If the corresponding
// SQL table is empty and the JSON file exists, it bulk-inserts the records and
// renames the JSON file to .json.bak.
func importLegacyJSON(db *gorm.DB, dataDir string) {
	type migration struct {
		file  string
		model any   // pointer to slice, e.g. &[]models.User{}
		label string
	}

	migrations := []migration{
		{"workspaces.json", &[]models.Workspace{}, "workspaces"},
		{"users.json", &[]models.User{}, "users"},
		{"app_installs.json", &[]models.AppInstall{}, "app_installs"},
		{"sessions.json", &[]models.Session{}, "sessions"},
		{"store_apps.json", &[]models.StoreApp{}, "store_apps"},
		{"app_shares.json", &[]models.AppShare{}, "app_shares"},
		{"app_reviews.json", &[]models.AppReview{}, "app_reviews"},
		{"workflow_runs.json", &[]models.WorkflowRun{}, "workflow_runs"},
	}

	for _, m := range migrations {
		importCollection(db, dataDir, m.file, m.model, m.label)
	}

	// Provider config is special — single object, not a collection.
	importProviderConfig(db, dataDir)
}

// importCollection imports a single JSON collection file into the database.
func importCollection(db *gorm.DB, dataDir, fileName string, dest any, label string) {
	filePath := filepath.Join(dataDir, fileName)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return // file doesn't exist, nothing to import
	}
	if len(data) == 0 {
		return
	}

	if err := json.Unmarshal(data, dest); err != nil {
		log.Printf("[db-migrate] failed to parse %s: %v", fileName, err)
		return
	}

	// Check if table already has data — only import into empty tables.
	var count int64
	db.Table(label).Count(&count)
	if count > 0 {
		return // already imported
	}

	result := db.CreateInBatches(dest, 100)
	if result.Error != nil {
		log.Printf("[db-migrate] failed to import %s: %v", fileName, result.Error)
		return
	}

	log.Printf("[db-migrate] imported %d records from %s", result.RowsAffected, fileName)

	// Rename the JSON file to .bak so we don't re-import next time.
	bakPath := filePath + ".bak"
	if err := os.Rename(filePath, bakPath); err != nil {
		log.Printf("[db-migrate] warning: could not rename %s to .bak: %v", fileName, err)
	}
}

func importProviderConfig(db *gorm.DB, dataDir string) {
	filePath := filepath.Join(dataDir, "provider_config.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return
	}
	if len(data) == 0 {
		return
	}

	var cfg models.ProviderConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Printf("[db-migrate] failed to parse provider_config.json: %v", err)
		return
	}

	// Overwrite the default config with the imported one.
	cfg.ID = "default"
	db.Save(&cfg)

	log.Printf("[db-migrate] imported provider_config.json")
	bakPath := filePath + ".bak"
	if err := os.Rename(filePath, bakPath); err != nil {
		log.Printf("[db-migrate] warning: could not rename provider_config.json to .bak: %v", err)
	}
}
