// Package models defines the core data types for the multi-tenant server.
package models

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// --- Workspace ---

type Workspace struct {
	ID        string    `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

// --- User ---

type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

type User struct {
	ID          string           `json:"id" gorm:"primaryKey"`
	WorkspaceID string           `json:"workspaceId" gorm:"index"`
	Name        string           `json:"name"`
	Email       string           `json:"email"`
	Role        Role             `json:"role"`
	Token       string           `json:"token" gorm:"index"`
	Preferences *UserPreferences `json:"preferences,omitempty" gorm:"serializer:json"`
	CreatedAt   time.Time        `json:"createdAt"`
}

// UserPreferences holds per-user AI defaults.
type UserPreferences struct {
	DefaultProvider string `json:"default_provider,omitempty"` // "claude", "ollama", etc.
	DefaultModel    string `json:"default_model,omitempty"`    // "gemma3", "gpt-4.1", etc.
}

// GenerateToken creates a random 32-byte hex token.
func GenerateToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// GenerateID creates a short random ID with the given prefix (e.g. "ws-", "usr-").
func GenerateID(prefix string) string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return prefix + hex.EncodeToString(b)
}
