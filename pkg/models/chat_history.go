package models

import "time"

// ChatMessage is a single message in a conversation history.
type ChatMessage struct {
	Role    string `json:"role"`    // "system", "user", "assistant", "tool"
	Content string `json:"content"`
}

// ChatHistory stores the full conversation history for a session,
// enabling multi-turn resume for stateless providers (Ollama, OpenAI, etc).
// Claude doesn't need this — it uses --resume via its CLI.
type ChatHistory struct {
	SessionID string        `json:"session_id" gorm:"primaryKey"`
	AppName   string        `json:"app_name" gorm:"index"` // which app this conversation belongs to
	Messages  []ChatMessage `json:"messages" gorm:"serializer:json"`
	Provider  string        `json:"provider"`
	UserID    string        `json:"user_id" gorm:"index"`
	UpdatedAt time.Time     `json:"updated_at"`
}
