package models

import "time"

// ToolCallEntry records a single tool invocation within a session.
type ToolCallEntry struct {
	Name        string `json:"name"`                   // e.g. "rubix.query_nodes"
	DurationMS  int    `json:"duration_ms,omitempty"`
	Status      string `json:"status"`                 // "ok", "error"
	Error       string `json:"error,omitempty"`
	InputBytes  int    `json:"input_bytes,omitempty"`
	OutputBytes int    `json:"output_bytes,omitempty"`
}

// Session records a single agent run.
type Session struct {
	ID              string          `json:"id"`
	Provider        string          `json:"provider"`                    // "claude", "ollama", "openai", "anthropic", "gemini"
	Model           string          `json:"model,omitempty"`             // e.g. "claude-sonnet-4-20250514", "gemma4", "gpt-4.1"
	ClaudeSessionID string          `json:"claude_session_id,omitempty"` // Claude CLI session ID for --resume
	Agent           string          `json:"agent,omitempty"`
	Prompt          string          `json:"prompt"`
	Result          string          `json:"result,omitempty"`
	Status          string          `json:"status"`
	DurationMS      int             `json:"duration_ms"`
	CostUSD         float64         `json:"cost_usd"`
	InputTokens     int             `json:"input_tokens,omitempty"`
	OutputTokens    int             `json:"output_tokens,omitempty"`
	ToolCalls       int             `json:"tool_calls,omitempty"`
	ToolCallLog     []ToolCallEntry `json:"tool_call_log,omitempty"`
	UserID          string          `json:"user_id"`
	CreatedAt       time.Time       `json:"created_at"`
}

func (s Session) GetID() string { return s.ID }
