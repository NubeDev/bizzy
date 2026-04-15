// Package airunner provides a unified interface for running AI CLI tools
// (Claude Code, OpenAI Codex, GitHub Copilot) and streaming their output
// as typed events.
package airunner

// Provider identifies which AI CLI backend to use.
type Provider string

const (
	ProviderClaude  Provider = "claude"
	ProviderCodex   Provider = "codex"
	ProviderCopilot Provider = "copilot"
)

// RunConfig is the provider-agnostic configuration for a run.
type RunConfig struct {
	Prompt       string `json:"prompt"`
	MCPURL       string `json:"mcp_url,omitempty"`       // MCP server endpoint (Claude-specific)
	MCPToken     string `json:"mcp_token,omitempty"`      // Bearer token for MCP auth
	AllowedTools string `json:"allowed_tools,omitempty"`  // Tool pattern filter (Claude-specific)
	Model        string `json:"model,omitempty"`          // Model override (e.g. "o4-mini", "gpt-4.1")
	WorkDir      string `json:"work_dir,omitempty"`       // Working directory for the CLI process
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

// RunResult contains the aggregated output after a run completes.
type RunResult struct {
	Text       string  `json:"text"`
	DurationMS int     `json:"duration_ms"`
	CostUSD    float64 `json:"cost_usd"`
}

// Runner is the interface every AI CLI backend must implement.
type Runner interface {
	// Name returns the provider identifier.
	Name() Provider
	// Available reports whether the CLI binary is installed and reachable.
	Available() bool
	// Run executes a prompt and streams events to onEvent. It blocks
	// until the process exits and returns the aggregated result.
	Run(cfg RunConfig, sessionID string, onEvent func(Event)) RunResult
}
