// Package revision provides generic entity revision history.
// It stores snapshots of any JSON-serializable entity before updates,
// supports listing history, and reverting to a previous version.
package revision

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/NubeDev/bizzy/pkg/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MaxRevisions is the default number of revisions kept per entity.
const MaxRevisions = 10

// Store handles revision CRUD.
type Store struct {
	db *gorm.DB
}

// NewStore creates a revision store.
func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

// Save creates a new revision snapshot for an entity.
// data is the entity state BEFORE the update (so we can revert to it).
// Auto-prunes old revisions beyond MaxRevisions.
func (s *Store) Save(entityType, entityID, authorID, changeSummary string, data any) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("revision: marshal: %w", err)
	}

	var maxRev int
	s.db.Model(&models.Revision{}).
		Where("entity_type = ? AND entity_id = ?", entityType, entityID).
		Select("COALESCE(MAX(revision), 0)").
		Scan(&maxRev)

	rev := models.Revision{
		ID:            uuid.New().String(),
		EntityType:    entityType,
		EntityID:      entityID,
		Revision:      maxRev + 1,
		Data:          jsonData,
		ChangeSummary: changeSummary,
		AuthorID:      authorID,
		CreatedAt:     time.Now().UTC(),
	}

	if err := s.db.Create(&rev).Error; err != nil {
		return fmt.Errorf("revision: create: %w", err)
	}

	s.prune(entityType, entityID)
	return nil
}

// List returns revisions for an entity, newest first.
func (s *Store) List(entityType, entityID string) ([]models.Revision, error) {
	var revisions []models.Revision
	err := s.db.
		Where("entity_type = ? AND entity_id = ?", entityType, entityID).
		Order("revision DESC").
		Find(&revisions).Error
	return revisions, err
}

// Get returns a specific revision by number.
func (s *Store) Get(entityType, entityID string, revisionNum int) (*models.Revision, error) {
	var rev models.Revision
	err := s.db.
		Where("entity_type = ? AND entity_id = ? AND revision = ?", entityType, entityID, revisionNum).
		First(&rev).Error
	if err != nil {
		return nil, err
	}
	return &rev, nil
}

// GetData fetches a revision and unmarshals its data into dest.
func (s *Store) GetData(entityType, entityID string, revisionNum int, dest any) error {
	rev, err := s.Get(entityType, entityID, revisionNum)
	if err != nil {
		return err
	}
	return json.Unmarshal(rev.Data, dest)
}

// prune removes old revisions beyond MaxRevisions.
func (s *Store) prune(entityType, entityID string) {
	var count int64
	s.db.Model(&models.Revision{}).
		Where("entity_type = ? AND entity_id = ?", entityType, entityID).
		Count(&count)

	if count <= MaxRevisions {
		return
	}

	var cutoff int
	s.db.Model(&models.Revision{}).
		Where("entity_type = ? AND entity_id = ?", entityType, entityID).
		Order("revision DESC").
		Offset(MaxRevisions).
		Limit(1).
		Select("revision").
		Scan(&cutoff)

	s.db.
		Where("entity_type = ? AND entity_id = ? AND revision <= ?", entityType, entityID, cutoff).
		Delete(&models.Revision{})
}

// EntityKey builds a composite entity ID, e.g. EntityKey("app123", "check_weather").
func EntityKey(parts ...string) string {
	key := parts[0]
	for _, p := range parts[1:] {
		key += ":" + p
	}
	return key
}
