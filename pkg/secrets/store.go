// Package secrets provides an encrypted key-value store for plugin and app
// secrets. Secrets are scoped globally or per-user, with user-scoped secrets
// overriding global ones.
package secrets

import (
	"fmt"
	"time"

	"github.com/NubeDev/bizzy/pkg/models"
	"gorm.io/gorm"
)

// Store manages encrypted secrets in the database.
type Store struct {
	db  *gorm.DB
	key masterKey
}

// NewStore creates a secret store. Call LoadOrCreateKey first to get the key.
func NewStore(db *gorm.DB, key masterKey) *Store {
	return &Store{db: db, key: key}
}

// Set creates or updates a secret. The value is encrypted before storage.
func (s *Store) Set(scope models.SecretScope, scopeID, ownerType, ownerName, key, value string) error {
	enc, err := encrypt(s.key, []byte(value))
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}

	secret := models.Secret{
		Scope:     scope,
		ScopeID:   scopeID,
		OwnerType: ownerType,
		OwnerName: ownerName,
		Key:       key,
		Value:     enc,
		UpdatedAt: time.Now(),
	}

	// Upsert: create or update.
	result := s.db.Save(&secret)
	return result.Error
}

// SetGlobal is a convenience for setting a global secret.
func (s *Store) SetGlobal(ownerType, ownerName, key, value string) error {
	return s.Set(models.SecretScopeGlobal, "", ownerType, ownerName, key, value)
}

// SetUser is a convenience for setting a user-scoped secret.
func (s *Store) SetUser(userID, ownerType, ownerName, key, value string) error {
	return s.Set(models.SecretScopeUser, userID, ownerType, ownerName, key, value)
}

// Get retrieves and decrypts a specific secret.
func (s *Store) Get(scope models.SecretScope, scopeID, ownerType, ownerName, key string) (string, error) {
	var secret models.Secret
	err := s.db.Where(
		"scope = ? AND scope_id = ? AND owner_type = ? AND owner_name = ? AND key = ?",
		scope, scopeID, ownerType, ownerName, key,
	).First(&secret).Error
	if err != nil {
		return "", err
	}

	plain, err := decrypt(s.key, secret.Value)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plain), nil
}

// Resolve returns the effective value for a secret, checking user-scoped first
// then falling back to global. This is the primary lookup method for plugins.
func (s *Store) Resolve(userID, ownerType, ownerName, key string) (string, error) {
	// Try user-scoped first.
	if userID != "" {
		val, err := s.Get(models.SecretScopeUser, userID, ownerType, ownerName, key)
		if err == nil {
			return val, nil
		}
	}
	// Fall back to global.
	return s.Get(models.SecretScopeGlobal, "", ownerType, ownerName, key)
}

// ResolveAll returns all effective secrets for an owner, with user-scoped
// values overriding global ones. Returns a map of key→plaintext.
func (s *Store) ResolveAll(userID, ownerType, ownerName string) (map[string]string, error) {
	result := make(map[string]string)

	// Load global secrets.
	var globals []models.Secret
	s.db.Where("scope = ? AND owner_type = ? AND owner_name = ?",
		models.SecretScopeGlobal, ownerType, ownerName,
	).Find(&globals)

	for _, sec := range globals {
		plain, err := decrypt(s.key, sec.Value)
		if err != nil {
			continue
		}
		result[sec.Key] = string(plain)
	}

	// Overlay user secrets.
	if userID != "" {
		var userSecrets []models.Secret
		s.db.Where("scope = ? AND scope_id = ? AND owner_type = ? AND owner_name = ?",
			models.SecretScopeUser, userID, ownerType, ownerName,
		).Find(&userSecrets)

		for _, sec := range userSecrets {
			plain, err := decrypt(s.key, sec.Value)
			if err != nil {
				continue
			}
			result[sec.Key] = string(plain)
		}
	}

	return result, nil
}

// Delete removes a specific secret.
func (s *Store) Delete(scope models.SecretScope, scopeID, ownerType, ownerName, key string) error {
	return s.db.Where(
		"scope = ? AND scope_id = ? AND owner_type = ? AND owner_name = ? AND key = ?",
		scope, scopeID, ownerType, ownerName, key,
	).Delete(&models.Secret{}).Error
}

// List returns all secret entries (without values) for an owner.
// If userID is provided, returns both global and user-scoped entries.
func (s *Store) List(userID, ownerType, ownerName string) []models.SecretEntry {
	var secrets []models.Secret

	if userID != "" {
		s.db.Where(
			"(scope = ? OR (scope = ? AND scope_id = ?)) AND owner_type = ? AND owner_name = ?",
			models.SecretScopeGlobal, models.SecretScopeUser, userID, ownerType, ownerName,
		).Find(&secrets)
	} else {
		s.db.Where(
			"scope = ? AND owner_type = ? AND owner_name = ?",
			models.SecretScopeGlobal, ownerType, ownerName,
		).Find(&secrets)
	}

	entries := make([]models.SecretEntry, len(secrets))
	for i, sec := range secrets {
		entries[i] = sec.ToEntry()
	}
	return entries
}

// ListAll returns all secret entries (without values) across all owners.
func (s *Store) ListAll() []models.SecretEntry {
	var secrets []models.Secret
	s.db.Find(&secrets)

	entries := make([]models.SecretEntry, len(secrets))
	for i, sec := range secrets {
		entries[i] = sec.ToEntry()
	}
	return entries
}
