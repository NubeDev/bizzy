// Package webhook provides an inbound webhook adapter for the command bus.
// It receives HTTP POSTs with command text or structured commands and
// dispatches them through the router.
//
// Two auth modes:
//   - HMAC-SHA256 signature via X-Signature-256 header (for programmatic callers)
//   - Bearer token via Authorization header (uses the same user tokens as the REST API)
//
// Two input modes:
//   - Text: {"text": "run workflow/weekly-report --site Sydney"}
//   - Structured: {"verb": "run", "target": "workflow/weekly-report", "params": {"site": "Sydney"}}
package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/NubeDev/bizzy/pkg/command"
	"github.com/NubeDev/bizzy/pkg/models"
	"gorm.io/gorm"
)

// WebhookLog records each inbound webhook for audit/debugging.
type WebhookLog struct {
	ID        string    `json:"id" gorm:"primaryKey"`
	CommandID string    `json:"command_id"`
	Source    string    `json:"source"`     // IP or caller identity
	UserID    string    `json:"user_id"`
	Text      string    `json:"text"`
	Status    string    `json:"status"`     // "accepted", "rejected", "error"
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Adapter receives HTTP POST webhooks and dispatches commands.
type Adapter struct {
	secret string  // HMAC-SHA256 signing secret (optional)
	db     *gorm.DB
	router *command.Router
}

// New creates a webhook adapter.
// If secret is non-empty, HMAC-SHA256 signature verification is enabled.
// If db is non-nil, inbound webhooks are logged to the webhook_logs table.
func New(secret string, db *gorm.DB) *Adapter {
	return &Adapter{secret: secret, db: db}
}

func (w *Adapter) Name() string { return "webhook" }

func (w *Adapter) Start(ctx context.Context, router *command.Router) error {
	w.router = router
	if w.db != nil {
		w.db.AutoMigrate(&WebhookLog{})
	}
	return nil
}

func (w *Adapter) Stop() error { return nil }

// BuildReply reconstructs a reply channel from stored ReplyInfo.
// Webhooks reply via a callback URL if one was provided.
func (w *Adapter) BuildReply(info command.ReplyInfo) (command.ReplyChannel, error) {
	if len(info.Address) == 0 {
		return &noopReply{}, nil
	}
	var addr command.WebhookAddress
	if err := json.Unmarshal(info.Address, &addr); err != nil {
		return nil, fmt.Errorf("invalid webhook address: %w", err)
	}
	if addr.CallbackURL == "" {
		return &noopReply{}, nil
	}
	return &callbackReply{url: addr.CallbackURL}, nil
}

// webhookRequest supports both text and structured command input.
type webhookRequest struct {
	// Text mode: parse this string as a command.
	Text string `json:"text,omitempty"`

	// Structured mode: explicit verb + target + params.
	Verb   string         `json:"verb,omitempty"`
	Target string         `json:"target,omitempty"`
	Params map[string]any `json:"params,omitempty"`

	// Caller identity. Required unless using Bearer token auth.
	UserID string `json:"user_id,omitempty"`
}

// Handler returns an http.HandlerFunc to mount on the gin router.
func (w *Adapter) Handler() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		// --- Auth ---
		userID, authErr := w.authenticate(r)
		if authErr != nil {
			w.logWebhook("", r.RemoteAddr, "", "", "rejected", authErr.Error())
			http.Error(rw, authErr.Error(), http.StatusUnauthorized)
			return
		}

		// --- Parse body ---
		var req webhookRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.logWebhook("", r.RemoteAddr, userID, "", "error", "invalid JSON: "+err.Error())
			http.Error(rw, "invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}

		// UserID from body overrides token-derived userID only if token auth wasn't used.
		if req.UserID != "" && userID == "" {
			userID = req.UserID
		}
		if userID == "" {
			w.logWebhook("", r.RemoteAddr, "", req.Text, "error", "user_id required")
			http.Error(rw, "user_id is required (in body or via Bearer token)", http.StatusBadRequest)
			return
		}

		// --- Build command ---
		replyTo := command.ReplyInfo{Channel: "webhook"}
		if callbackURL := r.Header.Get("X-Callback-URL"); callbackURL != "" {
			addrJSON, _ := json.Marshal(command.WebhookAddress{CallbackURL: callbackURL})
			replyTo.Address = addrJSON
		}

		var cmd command.Command
		var err error

		if req.Text != "" {
			// Text mode.
			cmd, err = w.router.Parser().Parse(req.Text, userID, replyTo,
				command.ParseConfig{BareTextBehaviour: "reject"})
			if err != nil {
				w.logWebhook("", r.RemoteAddr, userID, req.Text, "error", err.Error())
				http.Error(rw, err.Error(), http.StatusBadRequest)
				return
			}
		} else if req.Verb != "" {
			// Structured mode.
			cmd = command.Command{
				ID:       command.NewID(),
				Verb:     command.Verb(req.Verb),
				UserID:   userID,
				ReplyTo:  replyTo,
				Params:   req.Params,
				IssuedAt: time.Now().UTC(),
			}
			if req.Target != "" {
				kind, name := splitTarget(req.Target)
				cmd.Target = command.Target{Kind: kind, Name: name}
			}
			if cmd.Params == nil {
				cmd.Params = make(map[string]any)
			}
		} else {
			w.logWebhook("", r.RemoteAddr, userID, "", "error", "text or verb required")
			http.Error(rw, "either 'text' or 'verb' is required", http.StatusBadRequest)
			return
		}

		// --- Dispatch ---
		w.router.Execute(r.Context(), cmd)
		w.logWebhook(cmd.ID, r.RemoteAddr, userID, req.Text, "accepted", "")

		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusAccepted)
		json.NewEncoder(rw).Encode(map[string]any{
			"command_id": cmd.ID,
			"verb":       cmd.Verb,
			"target":     cmd.Target,
			"status":     "accepted",
		})
	}
}

// authenticate checks HMAC signature or Bearer token.
// Returns the user ID (from token) or empty string (HMAC — user_id comes from body).
func (w *Adapter) authenticate(r *http.Request) (string, error) {
	// Try Bearer token first (resolves user from DB).
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		token := strings.TrimPrefix(auth, "Bearer ")
		if w.db != nil {
			var user models.User
			if err := w.db.Where("token = ?", token).First(&user).Error; err == nil {
				return user.ID, nil
			}
		}
		return "", fmt.Errorf("invalid bearer token")
	}

	// Try HMAC signature.
	if w.secret != "" {
		if w.verifySignature(r) {
			return "", nil // user_id must come from body
		}
		return "", fmt.Errorf("invalid signature")
	}

	// No auth configured — allow (user_id must come from body).
	return "", nil
}

func (w *Adapter) verifySignature(r *http.Request) bool {
	sig := r.Header.Get("X-Signature-256")
	if sig == "" {
		return false
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return false
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	mac := hmac.New(sha256.New, []byte(w.secret))
	mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(sig))
}

func (w *Adapter) logWebhook(cmdID, source, userID, text, status, errMsg string) {
	if w.db == nil {
		return
	}
	entry := WebhookLog{
		ID:        models.GenerateID("wh-"),
		CommandID: cmdID,
		Source:    source,
		UserID:    userID,
		Text:      text,
		Status:    status,
		Error:     errMsg,
		CreatedAt: time.Now().UTC(),
	}
	if err := w.db.Create(&entry).Error; err != nil {
		log.Printf("[webhook] failed to log: %v", err)
	}
}

func splitTarget(s string) (string, string) {
	if i := strings.IndexByte(s, '/'); i >= 0 {
		return s[:i], s[i+1:]
	}
	return "", s
}

// --- reply channels ---

type noopReply struct{}

func (n *noopReply) Send(ctx context.Context, msg command.ReplyMessage) error {
	return nil
}

type callbackReply struct {
	url string
}

func (c *callbackReply) Send(ctx context.Context, msg command.ReplyMessage) error {
	body, _ := json.Marshal(msg)
	req, err := http.NewRequestWithContext(ctx, "POST", c.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
