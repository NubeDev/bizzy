package models

import "time"

// PluginStatus represents the runtime state of a plugin.
type PluginStatus string

const (
	PluginStatusActive   PluginStatus = "active"
	PluginStatusCrashed  PluginStatus = "crashed"
	PluginStatusDisabled PluginStatus = "disabled"
)

// Plugin is the persistent record for a registered plugin.
// The full manifest JSON is the source of truth; top-level fields
// are denormalised for easy querying and admin display.
type Plugin struct {
	Name           string       `json:"name" gorm:"primaryKey"`
	Version        string       `json:"version"`
	Description    string       `json:"description"`
	Services       string       `json:"services"`        // JSON array, e.g. ["tools","handler"]
	Manifest       string       `json:"manifest"`        // full JSON manifest
	Status         PluginStatus `json:"status" gorm:"default:active"`
	RegisteredAt   time.Time    `json:"registered_at"`
	LastHeartbeat  *time.Time   `json:"last_heartbeat"`
	HealthFailures int          `json:"health_failures" gorm:"default:0"`
}
