// Package gmail provides a Gmail adapter for the command bus.
// It polls for new emails matching a label/query, parses commands from
// the subject or body, verifies sender identity (domain whitelist + DKIM/SPF),
// and replies via SMTP.
package gmail

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/mail"
	"strings"
	"time"

	"github.com/NubeDev/bizzy/pkg/command"
	"github.com/NubeDev/bizzy/pkg/models"
	gmail "google.golang.org/api/gmail/v1"
	"gorm.io/gorm"
)

// Config holds Gmail adapter configuration.
type Config struct {
	// Gmail API service (pre-authenticated with OAuth2).
	GmailService *gmail.Service

	// SMTP settings for sending replies.
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	FromAddress  string

	// Polling config.
	PollInterval time.Duration // default: 2 minutes
	Query        string        // Gmail search query, e.g. "is:unread label:bizzy"
	MarkRead     bool          // mark processed emails as read

	// Security.
	AllowedDomains []string // e.g. ["nube-io.com"] — reject unknown senders

	DB *gorm.DB
}

// GmailLog records processed emails for audit.
type GmailLog struct {
	ID        string    `json:"id" gorm:"primaryKey"`
	MessageID string    `json:"message_id"`
	From      string    `json:"from"`
	Subject   string    `json:"subject"`
	CommandID string    `json:"command_id"`
	Status    string    `json:"status"` // "processed", "rejected", "error"
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Adapter polls Gmail and dispatches commands parsed from emails.
type Adapter struct {
	gmailSvc       *gmail.Service
	smtpHost       string
	smtpPort       int
	smtpUser       string
	smtpPassword   string
	fromAddress    string
	pollInterval   time.Duration
	query          string
	markRead       bool
	allowedDomains []string
	db             *gorm.DB
	router         *command.Router
	cancel         context.CancelFunc
}

// New creates a Gmail adapter.
func New(cfg Config) *Adapter {
	interval := cfg.PollInterval
	if interval == 0 {
		interval = 2 * time.Minute
	}
	query := cfg.Query
	if query == "" {
		query = "is:unread label:bizzy"
	}

	return &Adapter{
		gmailSvc:       cfg.GmailService,
		smtpHost:       cfg.SMTPHost,
		smtpPort:       cfg.SMTPPort,
		smtpUser:       cfg.SMTPUser,
		smtpPassword:   cfg.SMTPPassword,
		fromAddress:    cfg.FromAddress,
		pollInterval:   interval,
		query:          query,
		markRead:       cfg.MarkRead,
		allowedDomains: cfg.AllowedDomains,
		db:             cfg.DB,
	}
}

func (g *Adapter) Name() string { return "email" }

func (g *Adapter) Start(ctx context.Context, router *command.Router) error {
	g.router = router

	if g.db != nil {
		g.db.AutoMigrate(&GmailLog{})
	}

	if g.gmailSvc == nil {
		return fmt.Errorf("gmail service not configured")
	}

	ctx, g.cancel = context.WithCancel(ctx)

	// Poll loop.
	go func() {
		// Initial poll.
		g.poll(ctx)

		ticker := time.NewTicker(g.pollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				g.poll(ctx)
			}
		}
	}()

	log.Printf("[gmail] started polling every %s with query: %s", g.pollInterval, g.query)
	return nil
}

func (g *Adapter) Stop() error {
	if g.cancel != nil {
		g.cancel()
	}
	return nil
}

// BuildReply reconstructs an email reply channel from stored ReplyInfo.
func (g *Adapter) BuildReply(info command.ReplyInfo) (command.ReplyChannel, error) {
	if len(info.Address) == 0 {
		return nil, fmt.Errorf("email reply info has no address")
	}
	var addr command.EmailAddress
	if err := json.Unmarshal(info.Address, &addr); err != nil {
		return nil, fmt.Errorf("invalid email address: %w", err)
	}
	if addr.To == "" {
		return nil, fmt.Errorf("email address missing 'to'")
	}
	return &EmailReply{
		smtpHost:     g.smtpHost,
		smtpPort:     g.smtpPort,
		smtpUser:     g.smtpUser,
		smtpPassword: g.smtpPassword,
		from:         g.fromAddress,
		to:           addr.To,
		subject:      addr.Subject,
	}, nil
}

// poll fetches new emails and processes them as commands.
func (g *Adapter) poll(ctx context.Context) {
	msgs, err := g.gmailSvc.Users.Messages.List("me").Q(g.query).MaxResults(10).Do()
	if err != nil {
		log.Printf("[gmail] poll failed: %v", err)
		return
	}

	for _, m := range msgs.Messages {
		if ctx.Err() != nil {
			return
		}

		full, err := g.gmailSvc.Users.Messages.Get("me", m.Id).Format("full").Do()
		if err != nil {
			log.Printf("[gmail] fetch message %s failed: %v", m.Id, err)
			continue
		}

		g.processMessage(ctx, full)
	}
}

// processMessage parses a single email and dispatches it as a command.
func (g *Adapter) processMessage(ctx context.Context, msg *gmail.Message) {
	from := getHeader(msg, "From")
	subject := getHeader(msg, "Subject")
	body := extractBody(msg)

	// Extract email address from "Name <email>" format.
	fromAddr := extractEmail(from)

	// Security: verify sender domain.
	if !g.isAllowed(fromAddr) {
		g.logEmail(msg.Id, from, subject, "", "rejected", "domain not allowed")
		return
	}

	// Security: check DKIM/SPF (Gmail provides this in Authentication-Results header).
	authResults := getHeader(msg, "Authentication-Results")
	if !checkAuthResults(authResults) {
		g.logEmail(msg.Id, from, subject, "", "rejected", "DKIM/SPF check failed")
		return
	}

	// Parse command from body (or subject as fallback).
	text := strings.TrimSpace(body)
	if text == "" {
		text = strings.TrimSpace(subject)
	}
	if text == "" {
		return
	}

	// Resolve user by email.
	userID := g.resolveUserByEmail(fromAddr)
	if userID == "" {
		g.logEmail(msg.Id, from, subject, "", "rejected", "no user found for "+fromAddr)
		return
	}

	// Build reply info.
	addrJSON, _ := json.Marshal(command.EmailAddress{
		To:      fromAddr,
		Subject: "Re: " + subject,
	})
	replyTo := command.ReplyInfo{
		Channel: "email",
		Address: addrJSON,
	}

	// Parse command — ignore unrecognised text (no AI fallback for email).
	cmd, err := g.router.Parser().Parse(text, userID, replyTo,
		command.ParseConfig{BareTextBehaviour: "ignore"})
	if err != nil {
		g.logEmail(msg.Id, from, subject, "", "error", err.Error())
		return
	}

	// Dispatch.
	g.router.Execute(ctx, cmd)
	g.logEmail(msg.Id, from, subject, cmd.ID, "processed", "")

	// Mark as read if configured.
	if g.markRead {
		g.gmailSvc.Users.Messages.Modify("me", msg.Id, &gmail.ModifyMessageRequest{
			RemoveLabelIds: []string{"UNREAD"},
		}).Do()
	}
}

// isAllowed checks if the sender's domain is in the whitelist.
func (g *Adapter) isAllowed(email string) bool {
	if len(g.allowedDomains) == 0 {
		return true // no whitelist = allow all
	}
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return false
	}
	domain := strings.ToLower(parts[1])
	for _, d := range g.allowedDomains {
		if strings.ToLower(d) == domain {
			return true
		}
	}
	return false
}

// resolveUserByEmail looks up a bizzy user by their email address.
func (g *Adapter) resolveUserByEmail(email string) string {
	if g.db == nil {
		return ""
	}
	var user models.User
	if err := g.db.Where("email = ?", email).First(&user).Error; err != nil {
		return ""
	}
	return user.ID
}

func (g *Adapter) logEmail(messageID, from, subject, cmdID, status, errMsg string) {
	if g.db == nil {
		return
	}
	entry := GmailLog{
		ID:        models.GenerateID("em-"),
		MessageID: messageID,
		From:      from,
		Subject:   subject,
		CommandID: cmdID,
		Status:    status,
		Error:     errMsg,
		CreatedAt: time.Now().UTC(),
	}
	g.db.Create(&entry)
}

// --- helpers ---

func getHeader(msg *gmail.Message, name string) string {
	for _, h := range msg.Payload.Headers {
		if strings.EqualFold(h.Name, name) {
			return h.Value
		}
	}
	return ""
}

func extractBody(msg *gmail.Message) string {
	// Try plain text part first.
	if msg.Payload.MimeType == "text/plain" && msg.Payload.Body != nil {
		data, err := base64.URLEncoding.DecodeString(msg.Payload.Body.Data)
		if err == nil {
			return string(data)
		}
	}

	// Search parts for text/plain.
	for _, part := range msg.Payload.Parts {
		if part.MimeType == "text/plain" && part.Body != nil {
			data, err := base64.URLEncoding.DecodeString(part.Body.Data)
			if err == nil {
				return string(data)
			}
		}
	}

	return ""
}

func extractEmail(from string) string {
	addr, err := mail.ParseAddress(from)
	if err != nil {
		// Fallback: try as plain email.
		return strings.TrimSpace(from)
	}
	return addr.Address
}

// checkAuthResults parses the Authentication-Results header for DKIM/SPF pass.
func checkAuthResults(header string) bool {
	if header == "" {
		return true // no header = can't verify, allow (conservative)
	}
	lower := strings.ToLower(header)
	dkimPass := strings.Contains(lower, "dkim=pass")
	spfPass := strings.Contains(lower, "spf=pass")
	// Require at least one to pass.
	return dkimPass || spfPass
}
