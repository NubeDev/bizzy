// Package cron provides a scheduled command adapter for the command bus.
// Commands are stored in the database and reloaded periodically so new/changed
// entries take effect without a server restart.
package cron

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/NubeDev/bizzy/pkg/command"
	"gorm.io/gorm"
)

// CronCommand is a database record for a scheduled command.
type CronCommand struct {
	ID        string    `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"uniqueIndex"`
	Schedule  string    `json:"schedule"`  // cron expression e.g. "0 9 * * *"
	Command   string    `json:"command"`   // command text e.g. "run workflow/weekly-report"
	UserID    string    `json:"user_id"`   // run as this user
	Enabled   bool      `json:"enabled" gorm:"default:true"`
	CreatedAt time.Time `json:"created_at"`
}

// Adapter fires commands on a cron schedule. It uses a simple tick-based
// approach with minute-granularity matching instead of requiring gocron.
type Adapter struct {
	db      *gorm.DB
	router  *command.Router
	mu      sync.RWMutex
	entries []CronCommand
	cancel  context.CancelFunc
}

// New creates a cron adapter.
func New(db *gorm.DB) *Adapter {
	return &Adapter{db: db}
}

func (a *Adapter) Name() string { return "cron" }

func (a *Adapter) Start(ctx context.Context, router *command.Router) error {
	a.router = router

	// Auto-migrate the cron_commands table.
	a.db.AutoMigrate(&CronCommand{})

	// Initial load.
	a.reload()

	ctx, a.cancel = context.WithCancel(ctx)

	// Reload from DB every 60s.
	go func() {
		reloadTicker := time.NewTicker(60 * time.Second)
		// Check cron entries every minute.
		cronTicker := time.NewTicker(1 * time.Minute)
		defer reloadTicker.Stop()
		defer cronTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-reloadTicker.C:
				a.reload()
			case t := <-cronTicker.C:
				a.tick(ctx, t)
			}
		}
	}()

	log.Printf("[cron] started with %d entries", len(a.entries))
	return nil
}

func (a *Adapter) Stop() error {
	if a.cancel != nil {
		a.cancel()
	}
	return nil
}

func (a *Adapter) BuildReply(info command.ReplyInfo) (command.ReplyChannel, error) {
	return &logReply{}, nil
}

func (a *Adapter) reload() {
	var entries []CronCommand
	a.db.Where("enabled = ?", true).Find(&entries)
	a.mu.Lock()
	a.entries = entries
	a.mu.Unlock()
}

// tick checks all entries against the current time and fires matching ones.
func (a *Adapter) tick(ctx context.Context, now time.Time) {
	a.mu.RLock()
	entries := make([]CronCommand, len(a.entries))
	copy(entries, a.entries)
	a.mu.RUnlock()

	for _, entry := range entries {
		if !matchesCron(entry.Schedule, now) {
			continue
		}

		replyTo := command.ReplyInfo{Channel: "cron"}

		cmd, err := a.router.Parser().Parse(entry.Command, entry.UserID, replyTo,
			command.ParseConfig{BareTextBehaviour: "reject"})
		if err != nil {
			log.Printf("[cron] parse error for %q: %v", entry.Name, err)
			continue
		}

		log.Printf("[cron] firing %q: %s", entry.Name, entry.Command)
		a.router.Execute(ctx, cmd)
	}
}

// matchesCron is a simple cron expression matcher (minute hour dom month dow).
// Supports * and specific values. Does not support ranges or step values.
func matchesCron(expr string, t time.Time) bool {
	var minute, hour, dom, month, dow string
	_, err := fmt.Sscanf(expr, "%s %s %s %s %s", &minute, &hour, &dom, &month, &dow)
	if err != nil {
		return false
	}

	return fieldMatches(minute, t.Minute()) &&
		fieldMatches(hour, t.Hour()) &&
		fieldMatches(dom, t.Day()) &&
		fieldMatches(month, int(t.Month())) &&
		fieldMatches(dow, int(t.Weekday()))
}

func fieldMatches(field string, value int) bool {
	if field == "*" {
		return true
	}
	var v int
	if _, err := fmt.Sscanf(field, "%d", &v); err == nil {
		return v == value
	}
	return false
}

// logReply logs cron command results instead of sending them somewhere.
type logReply struct{}

func (l *logReply) Send(ctx context.Context, msg command.ReplyMessage) error {
	log.Printf("[cron] result: %s", msg.Text)
	return nil
}

// --- REST helpers for managing cron entries ---

// ListEntries returns all cron command entries.
func ListEntries(db *gorm.DB) []CronCommand {
	var entries []CronCommand
	db.Find(&entries)
	return entries
}

// CreateEntry creates a new cron command entry.
func CreateEntry(db *gorm.DB, entry CronCommand) error {
	if entry.ID == "" {
		entry.ID = generateID()
	}
	entry.CreatedAt = time.Now().UTC()
	return db.Create(&entry).Error
}

// DeleteEntry deletes a cron command entry by ID.
func DeleteEntry(db *gorm.DB, id string) error {
	return db.Delete(&CronCommand{}, "id = ?", id).Error
}

// ToggleEntry enables or disables a cron entry.
func ToggleEntry(db *gorm.DB, id string, enabled bool) error {
	return db.Model(&CronCommand{}).Where("id = ?", id).Update("enabled", enabled).Error
}

func generateID() string {
	return fmt.Sprintf("cron-%d", time.Now().UnixNano())
}
