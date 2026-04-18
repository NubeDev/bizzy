// Package airunner provides a unified interface for running AI providers
// (Claude Code CLI, Ollama, OpenAI, Anthropic, Gemini, Codex, Copilot)
// and streaming their output as typed events.
package airunner

import "context"

// Provider identifies which AI backend to use.
type Provider string

const (
	ProviderClaude    Provider = "claude"
	ProviderOllama    Provider = "ollama"
	ProviderOpenAI    Provider = "openai"
	ProviderAnthropic Provider = "anthropic"
	ProviderGemini    Provider = "gemini"
	ProviderCodex     Provider = "codex"
	ProviderCopilot   Provider = "copilot"
)

// HistoryMessage is a single message in a conversation history,
// used for multi-turn resume with stateless providers (Ollama, OpenAI, etc.).
type HistoryMessage struct {
	Role    string `json:"role"`    // "system", "user", "assistant", "tool"
	Content string `json:"content"`
}

// RunConfig is the provider-agnostic configuration for a run.
type RunConfig struct {
	Prompt       string `json:"prompt"`
	SystemPrompt string `json:"system_prompt,omitempty"`  // System-level context (memory, app descriptions)
	ResumeID     string `json:"resume_id,omitempty"`      // Resume a previous session (Claude: --resume)
	MCPURL       string `json:"mcp_url,omitempty"`        // MCP server endpoint
	MCPToken     string `json:"mcp_token,omitempty"`      // Bearer token for MCP auth
	AllowedTools string `json:"allowed_tools,omitempty"`  // Tool pattern filter
	Model          string `json:"model,omitempty"`            // Model override (e.g. "o4-mini", "gpt-4.1")
	ThinkingBudget string `json:"thinking_budget,omitempty"` // Thinking level: "low", "medium", "high", or token count
	WorkDir        string `json:"work_dir,omitempty"`        // Working directory for the CLI process
	History        []HistoryMessage `json:"-"`               // Pre-loaded conversation history for stateless providers
}

// Event is a normalised event emitted by any provider.
type Event struct {
	Type       string  `json:"type"`                  // "connected", "tool_call", "text", "error", "done"
	Provider   string  `json:"provider"`              // "claude", "codex", "copilot"
	SessionID  string  `json:"session_id"`
	Model      string  `json:"model,omitempty"`
	Name       string  `json:"name,omitempty"`        // tool name on "tool_call"
	Content    string  `json:"content,omitempty"`     // text on "text"
	Error      string  `json:"error,omitempty"`
	DurationMS int     `json:"duration_ms,omitempty"`
	CostUSD    float64 `json:"cost_usd,omitempty"`
}

// ToolCallEntry records a single tool invocation within a run.
type ToolCallEntry struct {
	Name        string `json:"name"`
	DurationMS  int    `json:"duration_ms,omitempty"`
	Status      string `json:"status"`
	Error       string `json:"error,omitempty"`
	InputBytes  int    `json:"input_bytes,omitempty"`
	OutputBytes int    `json:"output_bytes,omitempty"`
}

// RunResult contains the aggregated output after a run completes.
type RunResult struct {
	Text            string          `json:"text"`
	Provider        string          `json:"provider"`
	Model           string          `json:"model,omitempty"`
	ClaudeSessionID string          `json:"claude_session_id,omitempty"` // Claude-specific, used for --resume
	DurationMS      int             `json:"duration_ms"`
	CostUSD         float64         `json:"cost_usd"`
	InputTokens     int             `json:"input_tokens,omitempty"`
	OutputTokens    int             `json:"output_tokens,omitempty"`
	ToolCalls       int             `json:"tool_calls,omitempty"`
	ToolCallLog     []ToolCallEntry `json:"tool_call_log,omitempty"`
}

// Runner is the interface every AI backend must implement.
type Runner interface {
	// Name returns the provider identifier.
	Name() Provider
	// Available reports whether the backend is installed and reachable.
	Available() bool
	// Run executes a prompt and streams events to onEvent. It blocks
	// until the process exits or the context is cancelled, and returns
	// the aggregated result.
	Run(ctx context.Context, cfg RunConfig, sessionID string, onEvent func(Event)) RunResult
}
