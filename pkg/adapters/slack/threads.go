package slack

import (
	"time"

	"gorm.io/gorm"
)

// SlackThread maps a Slack thread to an active workflow/job run.
// Persisted in SQLite so it survives restarts.
type SlackThread struct {
	ThreadTS  string    `json:"thread_ts" gorm:"primaryKey"`
	RunID     string    `json:"run_id"`
	RunType   string    `json:"run_type"` // "workflow" or "job"
	ChannelID string    `json:"channel_id"`
	CreatedAt time.Time `json:"created_at"`
}

// ThreadStore manages thread-to-run mappings in the database.
type ThreadStore struct {
	db *gorm.DB
}

// NewThreadStore creates a thread store.
func NewThreadStore(db *gorm.DB) *ThreadStore {
	return &ThreadStore{db: db}
}

// Migrate creates the slack_threads table if it doesn't exist.
func (s *ThreadStore) Migrate() {
	if s.db != nil {
		s.db.AutoMigrate(&SlackThread{})
	}
}

// Set records a thread → run mapping.
func (s *ThreadStore) Set(threadTS, channelID, runID, runType string) {
	if s.db == nil {
		return
	}
	entry := SlackThread{
		ThreadTS:  threadTS,
		RunID:     runID,
		RunType:   runType,
		ChannelID: channelID,
		CreatedAt: time.Now().UTC(),
	}
	s.db.Save(&entry)
}

// Lookup returns the run ID for a thread, if one exists.
func (s *ThreadStore) Lookup(threadTS string) (string, bool) {
	if s.db == nil {
		return "", false
	}
	var entry SlackThread
	if err := s.db.Where("thread_ts = ?", threadTS).First(&entry).Error; err != nil {
		return "", false
	}
	return entry.RunID, true
}

// Delete removes a thread mapping.
func (s *ThreadStore) Delete(threadTS string) {
	if s.db != nil {
		s.db.Delete(&SlackThread{}, "thread_ts = ?", threadTS)
	}
}

// LookupByRun returns the thread TS for a given run ID.
func (s *ThreadStore) LookupByRun(runID string) (string, string, bool) {
	if s.db == nil {
		return "", "", false
	}
	var entry SlackThread
	if err := s.db.Where("run_id = ?", runID).First(&entry).Error; err != nil {
		return "", "", false
	}
	return entry.ThreadTS, entry.ChannelID, true
}
