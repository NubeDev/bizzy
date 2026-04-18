package flow

import (
	"fmt"
	"time"

	"github.com/NubeDev/bizzy/pkg/models"
	"gorm.io/gorm"
)

// Store provides CRUD operations for FlowDef and FlowRun.
type Store struct {
	db *gorm.DB
}

// NewStore creates a flow store and auto-migrates the tables.
func NewStore(db *gorm.DB) *Store {
	db.AutoMigrate(&FlowDef{}, &FlowRun{})
	return &Store{db: db}
}

// --- FlowDef CRUD ---

// CreateFlow persists a new flow definition.
func (s *Store) CreateFlow(def *FlowDef) error {
	def.ID = models.GenerateID("flow-")
	def.Version = 1
	def.CreatedAt = time.Now()
	def.UpdatedAt = def.CreatedAt
	return s.db.Create(def).Error
}

// GetFlow retrieves a flow definition by ID.
func (s *Store) GetFlow(id string) (*FlowDef, error) {
	var def FlowDef
	if err := s.db.First(&def, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &def, nil
}

// GetFlowByName retrieves a flow definition by name.
func (s *Store) GetFlowByName(name string) (*FlowDef, error) {
	var def FlowDef
	if err := s.db.First(&def, "name = ?", name).Error; err != nil {
		return nil, err
	}
	return &def, nil
}

// ListFlows returns all flow definitions for a user.
func (s *Store) ListFlows(userID string) ([]FlowDef, error) {
	var defs []FlowDef
	q := s.db.Order("updated_at DESC")
	if userID != "" {
		q = q.Where("user_id = ?", userID)
	}
	if err := q.Find(&defs).Error; err != nil {
		return nil, err
	}
	return defs, nil
}

// UpdateFlow updates a flow definition, incrementing the version.
func (s *Store) UpdateFlow(def *FlowDef) error {
	def.Version++
	def.UpdatedAt = time.Now()
	return s.db.Save(def).Error
}

// DeleteFlow removes a flow definition.
func (s *Store) DeleteFlow(id string) error {
	return s.db.Delete(&FlowDef{}, "id = ?", id).Error
}

// DuplicateFlow clones a flow with a new name.
func (s *Store) DuplicateFlow(id string) (*FlowDef, error) {
	src, err := s.GetFlow(id)
	if err != nil {
		return nil, err
	}
	dup := *src
	dup.ID = models.GenerateID("flow-")
	dup.Name = fmt.Sprintf("%s (copy)", src.Name)
	dup.Version = 1
	dup.CreatedAt = time.Now()
	dup.UpdatedAt = dup.CreatedAt
	if err := s.db.Create(&dup).Error; err != nil {
		return nil, err
	}
	return &dup, nil
}

// --- FlowRun CRUD ---

// CreateRun persists a new flow run.
func (s *Store) CreateRun(run *FlowRun) error {
	run.ID = models.GenerateID("frun-")
	run.CreatedAt = time.Now()
	return s.db.Create(run).Error
}

// GetRun retrieves a flow run by ID.
func (s *Store) GetRun(id string) (*FlowRun, error) {
	var run FlowRun
	if err := s.db.First(&run, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &run, nil
}

// ListRuns returns runs for a flow, ordered by creation time desc.
func (s *Store) ListRuns(flowID string) ([]FlowRun, error) {
	var runs []FlowRun
	q := s.db.Order("created_at DESC").Limit(100)
	if flowID != "" {
		q = q.Where("flow_id = ?", flowID)
	}
	if err := q.Find(&runs).Error; err != nil {
		return nil, err
	}
	return runs, nil
}

// ListRunsByStatus returns runs matching the given status.
func (s *Store) ListRunsByStatus(status FlowRunStatus) ([]FlowRun, error) {
	var runs []FlowRun
	if err := s.db.Where("status = ?", status).Find(&runs).Error; err != nil {
		return nil, err
	}
	return runs, nil
}

// SaveRun persists the current state of a run.
func (s *Store) SaveRun(run *FlowRun) error {
	return s.db.Save(run).Error
}

// CountActiveRuns returns the number of non-terminal runs for a user.
func (s *Store) CountActiveRuns(userID string) (int64, error) {
	var count int64
	err := s.db.Model(&FlowRun{}).
		Where("user_id = ? AND status IN ?", userID, []FlowRunStatus{FlowRunPending, FlowRunRunning, FlowRunWaitingApproval}).
		Count(&count).Error
	return count, err
}
