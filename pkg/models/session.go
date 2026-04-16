package models

import "time"

// Session records a single agent run.
type Session struct {
	ID               string    `json:"id"`
	ClaudeSessionID  string    `json:"claude_session_id,omitempty"` // Claude CLI session ID for --resume
	Agent            string    `json:"agent,omitempty"`
	Prompt           string    `json:"prompt"`
	Result           string    `json:"result,omitempty"`
	Status           string    `json:"status"`
	DurationMS       int       `json:"duration_ms"`
	CostUSD          float64   `json:"cost_usd"`
	UserID           string    `json:"user_id"`
	CreatedAt        time.Time `json:"created_at"`
}

func (s Session) GetID() string { return s.ID }
