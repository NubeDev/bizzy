// Package command defines the unified command syntax and routing layer.
// Every interaction — from Slack, CLI, email, webhook, or cron — parses
// into the same Command struct and flows through the same router.
package command

import (
	"context"
	"encoding/json"
	"time"

	"github.com/NubeDev/bizzy/pkg/models"
	"github.com/NubeDev/bizzy/pkg/version"
)

// Verb describes what action to take.
type Verb string

const (
	VerbRun     Verb = "run"
	VerbAsk     Verb = "ask"
	VerbStatus  Verb = "status"
	VerbCancel  Verb = "cancel"
	VerbRestart Verb = "restart"
	VerbList    Verb = "list"
	VerbApprove Verb = "approve"
	VerbReject  Verb = "reject"
	VerbHelp    Verb = "help"
)

// Target identifies what the command acts on.
type Target struct {
	Kind string `json:"kind"` // "workflow", "tool", "job", "prompt", "session"
	Name string `json:"name"` // "weekly-report", "rubix.query_nodes", "wf-abc123"
}

// ReplyInfo is serialisable routing data — stored in the DB alongside the
// command/run so replies survive server restarts.
type ReplyInfo struct {
	Channel string          `json:"channel"` // "slack", "email", "http", "webhook", "cron"
	Address json.RawMessage `json:"address,omitempty"`
}

// Typed address structs — one per channel. Deserialized from ReplyInfo.Address.

// SlackAddress routes replies to a Slack channel/thread.
type SlackAddress struct {
	ChannelID string `json:"channel_id"`
	ThreadTS  string `json:"thread_ts"`
}

// EmailAddress routes replies via email.
type EmailAddress struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
}

// WebhookAddress routes replies to a callback URL.
type WebhookAddress struct {
	CallbackURL string `json:"callback_url,omitempty"`
}

// Command is the universal unit of intent. Every adapter parses external
// input into this struct before dispatching to the router.
type Command struct {
	ID            string         `json:"id"`
	SyntaxVersion string         `json:"syntax_version"`
	Verb          Verb           `json:"verb"`
	Target        Target         `json:"target"`
	Params        map[string]any `json:"params,omitempty"`
	UserID        string         `json:"user_id"`
	ReplyTo       ReplyInfo      `json:"reply_to"`
	IssuedAt      time.Time      `json:"issued_at"`
}

// Result is returned by executor dispatch methods.
type Result struct {
	ID      string `json:"id,omitempty"`
	Message string `json:"message,omitempty"`
	Output  any    `json:"output,omitempty"`
	Async   bool   `json:"async,omitempty"`
}

// ReplyMessage is what gets sent back through a ReplyChannel.
type ReplyMessage struct {
	Text string `json:"text"`
}

// ReplyChannel is the live sender, reconstructed from ReplyInfo at send time.
type ReplyChannel interface {
	Send(ctx context.Context, msg ReplyMessage) error
}

// AdapterRegistry maps channel names to factories that rebuild ReplyChannel
// from stored ReplyInfo.
type AdapterRegistry interface {
	BuildReply(info ReplyInfo) (ReplyChannel, error)
	Register(name string, adapter Adapter)
}

// Adapter is the interface for inbound/outbound channel adapters.
type Adapter interface {
	Name() string
	Start(ctx context.Context, router *Router) error
	Stop() error
	BuildReply(info ReplyInfo) (ReplyChannel, error)
}

// CommandEvent wraps a command for bus publishing.
type CommandEvent struct {
	Command Command `json:"command"`
}

// CommandResultEvent wraps a command and its result/error for bus publishing.
type CommandResultEvent struct {
	Command Command `json:"command"`
	Result  Result  `json:"result,omitempty"`
	Error   string  `json:"error,omitempty"`
}

// NewID generates a unique command ID.
func NewID() string {
	return models.GenerateID("cmd-")
}

// NewCommand creates a Command with the current syntax version pre-filled.
func NewCommand() Command {
	return Command{
		ID:            NewID(),
		SyntaxVersion: version.CommandSyntax,
		IssuedAt:      time.Now().UTC(),
		Params:        make(map[string]any),
	}
}
