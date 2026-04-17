package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/NubeDev/bizzy/pkg/claude"
	"github.com/NubeDev/bizzy/pkg/models"
	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
)

// NewAskCmd creates the "nube ask" command that streams AI responses.
//
// Default: connects to the server via WebSocket (any provider, session persistence).
// --direct: bypasses server, shells out to the provider CLI directly.
func NewAskCmd() *cobra.Command {
	var provider, model, sessionID string
	var direct bool

	cmd := &cobra.Command{
		Use:   "ask <prompt>",
		Short: "Ask an AI provider a question with access to your Nube tools",
		Long: `Send a prompt to an AI provider with your Nube MCP server connected.

The AI will have access to all your installed apps and tools.

By default, connects to the Nube server via WebSocket for streaming.
Use --session to resume a previous conversation.
Use --direct to bypass the server and call the provider CLI directly.

Examples:
  nube ask "write a marketing plan for Rubix"
  nube ask --provider ollama --model gemma3 "check devices"
  nube ask --session ses-abc123 "what about level 12?"
  nube ask --direct "quick question"`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prompt := strings.Join(args, " ")

			cfg, err := LoadConfig()
			if err != nil {
				return err
			}

			if direct {
				return askDirect(cfg, prompt, provider, model)
			}

			// If --session is set, look up the claude_session_id for resume.
			resumeID := ""
			if sessionID != "" {
				rid, err := resolveResumeID(cfg, sessionID)
				if err != nil {
					return err
				}
				resumeID = rid
			}

			return askWS(cfg, prompt, provider, model, resumeID)
		},
	}

	cmd.Flags().StringVar(&provider, "provider", "", "AI provider: claude, ollama, openai, anthropic, gemini (default: server default)")
	cmd.Flags().StringVar(&model, "model", "", "Model override (e.g. gemma3, gpt-4.1)")
	cmd.Flags().StringVar(&sessionID, "session", "", "Resume a previous session by ID (e.g. ses-abc123)")
	cmd.Flags().BoolVar(&direct, "direct", false, "Bypass server, call provider CLI directly")

	return cmd
}

// resolveResumeID fetches a session and returns the provider-specific resume ID.
func resolveResumeID(cfg *Config, sessionID string) (string, error) {
	if cfg.Server == "" {
		return "", fmt.Errorf("not logged in — run: nube login <server-url> <token>")
	}

	client := &Client{Server: cfg.Server, Token: cfg.Token}

	status, data, err := client.Do("GET", "/api/agents/sessions/"+sessionID, nil)
	if err != nil {
		return "", fmt.Errorf("fetch session: %v", err)
	}
	if status == 404 {
		return "", fmt.Errorf("session not found: %s\nRun 'nube sessions list' to see available sessions", sessionID)
	}
	if status != 200 {
		return "", fmt.Errorf("fetch session failed (HTTP %d)", status)
	}

	var session struct {
		ClaudeSessionID string `json:"claude_session_id"`
		Provider        string `json:"provider"`
	}
	json.Unmarshal(data, &session)

	if session.ClaudeSessionID == "" {
		if session.Provider != "" && session.Provider != "claude" {
			return "", fmt.Errorf("session %s used provider %q — only Claude sessions support resume currently", sessionID, session.Provider)
		}
		return "", fmt.Errorf("session %s has no resume ID (it may be too old or from a direct run)", sessionID)
	}

	fmt.Fprintf(os.Stderr, "\033[2mResuming session %s...\033[0m\n", sessionID)
	return session.ClaudeSessionID, nil
}

// askWS connects to the server via WebSocket and streams events.
func askWS(cfg *Config, prompt, provider, model, resumeID string) error {
	if cfg.Server == "" {
		return fmt.Errorf("not logged in — run: nube login <server-url> <token>\n(or use --direct to bypass the server)")
	}

	// Build WS URL: http://... → ws://..., https://... → wss://...
	serverURL := strings.TrimRight(cfg.Server, "/")
	wsURL := strings.Replace(serverURL, "https://", "wss://", 1)
	wsURL = strings.Replace(wsURL, "http://", "ws://", 1)
	wsURL += "/api/agents/run?token=" + url.QueryEscape(cfg.Token)

	// Try to connect with a timeout.
	dialer := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
	}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("server not reachable at %s — start with 'make server' or use --direct\n(%v)", serverURL, err)
	}
	defer conn.Close()

	// 1. Read session event.
	var sessionEvent struct {
		Type      string `json:"type"`
		SessionID string `json:"session_id"`
	}
	if err := conn.ReadJSON(&sessionEvent); err != nil {
		return fmt.Errorf("read session event: %v", err)
	}

	// 2. Send run request.
	req := map[string]string{
		"prompt": prompt,
	}
	if provider != "" {
		req["provider"] = provider
	}
	if model != "" {
		req["model"] = model
	}
	if resumeID != "" {
		req["session_id"] = resumeID
	}
	if err := conn.WriteJSON(req); err != nil {
		return fmt.Errorf("send request: %v", err)
	}

	// 3. Stream events.
	var lastWasText bool
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				break
			}
			// Connection closed after done event is normal.
			if lastWasText {
				break
			}
			return fmt.Errorf("read event: %v", err)
		}

		var ev struct {
			Type       string  `json:"type"`
			Model      string  `json:"model,omitempty"`
			Name       string  `json:"name,omitempty"`
			Content    string  `json:"content,omitempty"`
			Error      string  `json:"error,omitempty"`
			DurationMS int     `json:"duration_ms,omitempty"`
			CostUSD    float64 `json:"cost_usd,omitempty"`
		}
		if err := json.Unmarshal(msg, &ev); err != nil {
			continue
		}

		switch ev.Type {
		case "session", "session_id":
			// internal protocol events, skip
		case "connected":
			fmt.Fprintf(os.Stderr, "\033[2m⚡ Connected — model: %s\033[0m\n\n", ev.Model)
		case "tool_call":
			if lastWasText {
				fmt.Println()
				lastWasText = false
			}
			fmt.Fprintf(os.Stderr, "\033[36m⚙ calling %s\033[0m\n", ev.Name)
		case "text":
			fmt.Print(ev.Content)
			lastWasText = true
		case "error":
			fmt.Fprintf(os.Stderr, "\033[31m✗ %s\033[0m\n", ev.Error)
		case "done":
			if lastWasText {
				fmt.Println()
			}
			fmt.Fprintf(os.Stderr, "\n\033[2m— done (%dms, $%.4f)\033[0m\n", ev.DurationMS, ev.CostUSD)
			return nil
		}
	}

	return nil
}

// askDirect bypasses the server and calls the provider CLI directly.
// Currently only Claude is supported in direct mode.
func askDirect(cfg *Config, prompt, provider, model string) error {
	if provider == "" || provider == "claude" {
		return askDirectClaude(cfg, prompt)
	}
	return fmt.Errorf("--direct mode only supports claude provider (got %q)\nfor other providers, start the server and use: nube ask --provider %s", provider, provider)
}

func askDirectClaude(cfg *Config, prompt string) error {
	mcpURL := ""
	mcpToken := ""
	allowedTools := ""
	if cfg.Server != "" {
		mcpURL = strings.TrimRight(cfg.Server, "/") + "/mcp"
		mcpToken = cfg.Token
		allowedTools = "mcp__nube__*"
	}

	sessionID := models.GenerateID("ses-")
	var lastWasText bool

	claude.Run(context.Background(), claude.RunConfig{
		Prompt:       prompt,
		MCPURL:       mcpURL,
		MCPToken:     mcpToken,
		AllowedTools: allowedTools,
	}, sessionID, func(ev claude.Event) {
		switch ev.Type {
		case "connected":
			fmt.Fprintf(os.Stderr, "\033[2m⚡ Connected — model: %s (direct)\033[0m\n\n", ev.Model)
		case "tool_call":
			if lastWasText {
				fmt.Println()
				lastWasText = false
			}
			fmt.Fprintf(os.Stderr, "\033[36m⚙ calling %s\033[0m\n", ev.Name)
		case "text":
			fmt.Print(ev.Content)
			lastWasText = true
		case "error":
			fmt.Fprintf(os.Stderr, "\033[31m✗ %s\033[0m\n", ev.Error)
		case "done":
			if lastWasText {
				fmt.Println()
			}
			fmt.Fprintf(os.Stderr, "\n\033[2m— done (%dms, $%.4f)\033[0m\n", ev.DurationMS, ev.CostUSD)
		}
	})

	return nil
}
