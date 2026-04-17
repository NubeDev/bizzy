package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/NubeDev/bizzy/pkg/bus"
	"github.com/NubeDev/bizzy/pkg/command"
	"github.com/NubeDev/bizzy/pkg/models"
	"github.com/nats-io/nats.go"
	"gorm.io/gorm"
)

// Notifier subscribes to bus events and fans out notifications to channels
// configured in the user's NotifyPrefs. This is separate from the ReplyRouter
// which only sends replies to the originating channel.
//
// Example: a user starts a workflow from the CLI but wants a Slack notification
// when it completes. Their NotifyPrefs.OnWorkflowDone includes "slack".
type Notifier struct {
	bus      *bus.Bus
	db       *gorm.DB
	adapters command.AdapterRegistry
}

// NewNotifier creates a notification fan-out subscriber.
func NewNotifier(b *bus.Bus, db *gorm.DB, adapters command.AdapterRegistry) *Notifier {
	return &Notifier{bus: b, db: db, adapters: adapters}
}

// Start subscribes to lifecycle events and sends notifications based on user prefs.
func (n *Notifier) Start() error {
	// Workflow completed.
	if _, err := n.bus.SubscribeDurable(bus.TopicWorkflowCompleted, "notifier-wf-done", func(msg *nats.Msg) {
		n.handleEvent(msg, "on_workflow_done", func(ev eventEnvelope) string {
			return fmt.Sprintf("Workflow %s completed", ev.TargetName)
		})
	}); err != nil {
		return fmt.Errorf("subscribe workflow.completed: %w", err)
	}

	// Workflow failed.
	if _, err := n.bus.SubscribeDurable(bus.TopicWorkflowFailed, "notifier-wf-failed", func(msg *nats.Msg) {
		n.handleEvent(msg, "on_workflow_failed", func(ev eventEnvelope) string {
			return fmt.Sprintf("Workflow %s failed: %s", ev.TargetName, ev.Error)
		})
	}); err != nil {
		return fmt.Errorf("subscribe workflow.failed: %w", err)
	}

	// Workflow approval needed.
	if _, err := n.bus.SubscribeDurable(bus.TopicWorkflowWaitingApproval, "notifier-wf-approval", func(msg *nats.Msg) {
		n.handleEvent(msg, "on_approval_needed", func(ev eventEnvelope) string {
			return fmt.Sprintf("Approval needed for %s (%s)\nReply: 'approve' or 'reject [feedback]'",
				ev.TargetName, ev.TargetID)
		})
	}); err != nil {
		return fmt.Errorf("subscribe workflow.waiting_approval: %w", err)
	}

	// Job completed.
	if _, err := n.bus.SubscribeDurable("job.completed", "notifier-job-done", func(msg *nats.Msg) {
		n.handleEvent(msg, "on_job_done", func(ev eventEnvelope) string {
			return fmt.Sprintf("Job %s completed", ev.TargetID)
		})
	}); err != nil {
		return fmt.Errorf("subscribe job.completed: %w", err)
	}

	// Job failed.
	if _, err := n.bus.SubscribeDurable("job.failed", "notifier-job-failed", func(msg *nats.Msg) {
		n.handleEvent(msg, "on_job_failed", func(ev eventEnvelope) string {
			return fmt.Sprintf("Job %s failed: %s", ev.TargetID, ev.Error)
		})
	}); err != nil {
		return fmt.Errorf("subscribe job.failed: %w", err)
	}

	log.Println("[notifier] started")
	return nil
}

// eventEnvelope is a superset struct that can unmarshal any event type.
type eventEnvelope struct {
	UserID     string `json:"user_id"`
	TargetKind string `json:"target_kind"`
	TargetName string `json:"target_name"`
	TargetID   string `json:"target_id"`
	Error      string `json:"error"`

	// Workflow events use these field names.
	RunID    string `json:"run_id"`
	Workflow string `json:"workflow"`

	// Job events use these field names.
	JobID string `json:"job_id"`
}

func (e eventEnvelope) resolveUserID() string {
	return e.UserID
}

func (e eventEnvelope) resolveName() string {
	if e.TargetName != "" {
		return e.TargetName
	}
	if e.Workflow != "" {
		return e.Workflow
	}
	return e.TargetID
}

func (e eventEnvelope) resolveID() string {
	if e.TargetID != "" {
		return e.TargetID
	}
	if e.RunID != "" {
		return e.RunID
	}
	return e.JobID
}

func (n *Notifier) handleEvent(msg *nats.Msg, prefField string, formatMsg func(eventEnvelope) string) {
	defer msg.Ack()

	var ev eventEnvelope
	if err := json.Unmarshal(msg.Data, &ev); err != nil {
		return
	}

	// Fill in resolved fields.
	ev.TargetName = ev.resolveName()
	ev.TargetID = ev.resolveID()

	userID := ev.resolveUserID()
	if userID == "" {
		return
	}

	// Load user prefs.
	var prefs models.NotifyPrefs
	if err := n.db.Where("user_id = ?", userID).First(&prefs).Error; err != nil {
		return // no prefs configured — skip extra notifications
	}

	// Get the channel list for this event type.
	channels := n.getChannels(prefs, prefField)
	if len(channels) == 0 {
		return
	}

	text := formatMsg(ev)

	// Fan out to each configured channel.
	for _, channel := range channels {
		replyInfo := command.ReplyInfo{Channel: channel}

		ch, err := n.adapters.BuildReply(replyInfo)
		if err != nil {
			log.Printf("[notifier] channel %s unavailable for user %s: %v", channel, userID, err)
			continue
		}

		if err := ch.Send(context.Background(), command.ReplyMessage{Text: text}); err != nil {
			log.Printf("[notifier] send failed to %s for user %s: %v", channel, userID, err)
		}
	}
}

func (n *Notifier) getChannels(prefs models.NotifyPrefs, field string) []string {
	switch field {
	case "on_workflow_done":
		return prefs.OnWorkflowDone
	case "on_workflow_failed":
		return prefs.OnWorkflowFailed
	case "on_job_done":
		return prefs.OnJobDone
	case "on_job_failed":
		return prefs.OnJobFailed
	case "on_approval_needed":
		return prefs.OnApprovalNeeded
	default:
		return nil
	}
}
