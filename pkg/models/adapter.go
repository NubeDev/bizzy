package models

import "time"

// AdapterConfig stores configuration for a command bus adapter.
// Managed via the admin REST API; enables runtime enable/disable without restarts.
type AdapterConfig struct {
	Name      string    `json:"name" gorm:"primaryKey"`
	Type      string    `json:"type"`
	Enabled   bool      `json:"enabled" gorm:"default:false"`
	Config    string    `json:"config"`    // JSON config (tokens, schedules, etc.)
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
