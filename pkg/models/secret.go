package models

import "time"

// SecretScope controls who can access a secret.
type SecretScope string

const (
	// SecretScopeGlobal is visible to all users.
	SecretScopeGlobal SecretScope = "global"
	// SecretScopeUser is visible only to the owning user.
	SecretScopeUser SecretScope = "user"
)

// Secret stores an encrypted key-value pair scoped to an owner (plugin or app)
// and optionally to a specific user.
//
// Resolution order: user-scoped secrets override global secrets with the same
// owner + key.
type Secret struct {
	// Composite primary key: (scope, scope_id, owner_type, owner_name, key)
	Scope     SecretScope `json:"scope"      gorm:"primaryKey;size:10"`
	ScopeID   string      `json:"scope_id"   gorm:"primaryKey;size:64"`  // "" for global, user_id for user
	OwnerType string      `json:"owner_type" gorm:"primaryKey;size:10"`  // "plugin" or "app"
	OwnerName string      `json:"owner_name" gorm:"primaryKey;size:64"`  // e.g. "github", "rubix"
	Key       string      `json:"key"        gorm:"primaryKey;size:128"` // e.g. "GITHUB_TOKEN"

	// Value is AES-256-GCM encrypted. Never returned via API.
	Value []byte `json:"-" gorm:"type:blob"`

	UpdatedAt time.Time `json:"updated_at"`
}

// SecretEntry is the API-safe view — value is never exposed.
type SecretEntry struct {
	Scope     SecretScope `json:"scope"`
	ScopeID   string      `json:"scope_id,omitempty"`
	OwnerType string      `json:"owner_type"`
	OwnerName string      `json:"owner_name"`
	Key       string      `json:"key"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// ToEntry strips the encrypted value for API responses.
func (s *Secret) ToEntry() SecretEntry {
	return SecretEntry{
		Scope:     s.Scope,
		ScopeID:   s.ScopeID,
		OwnerType: s.OwnerType,
		OwnerName: s.OwnerName,
		Key:       s.Key,
		UpdatedAt: s.UpdatedAt,
	}
}
