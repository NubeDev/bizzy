// Package models defines the core data types for the multi-tenant server.
package models

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// --- Workspace ---

type Workspace struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

func (w Workspace) GetID() string { return w.ID }

// --- User ---

type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

type User struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspaceId"`
	Name        string    `json:"name"`
	Email       string    `json:"email"`
	Role        Role      `json:"role"`
	Token       string    `json:"token"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (u User) GetID() string { return u.ID }

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
