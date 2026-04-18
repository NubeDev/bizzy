package bus

import (
	"encoding/json"
	"time"
)

// Topic constants for the event bus.
const (
	// Command lifecycle.
	TopicCommandReceived  = "command.received"
	TopicCommandAccepted  = "command.accepted"
	TopicCommandCompleted = "command.completed"
	TopicCommandFailed    = "command.failed"

	// Job lifecycle.
	TopicJobStarted   = "job.started"
	TopicJobProgress  = "job.progress"
	TopicJobCompleted = "job.completed"
	TopicJobFailed    = "job.failed"
	TopicJobCancelled = "job.cancelled"

	// Tool lifecycle.
	TopicToolCalled    = "tool.called"
	TopicToolCompleted = "tool.completed"
	TopicToolFailed    = "tool.failed"

	// Plugin lifecycle.
	TopicPluginRegistered   = "extension.register"
	TopicPluginDeregistered = "extension.deregister"
)

// Event is the envelope for all bus messages.
type Event struct {
	Topic     string    `json:"topic"`
	Data      EventData `json:"data"`
	Timestamp time.Time `json:"timestamp"`
}

// EventData carries enough context to route a reply.
type EventData struct {
	CommandID  string          `json:"command_id,omitempty"`
	UserID     string          `json:"user_id"`
	TargetKind string          `json:"target_kind,omitempty"`
	TargetName string          `json:"target_name,omitempty"`
	TargetID   string          `json:"target_id,omitempty"`
	ReplyTo    json.RawMessage `json:"reply_to,omitempty"`
	Status     string          `json:"status,omitempty"`
	Output     any             `json:"output,omitempty"`
	Error      string          `json:"error,omitempty"`
}
