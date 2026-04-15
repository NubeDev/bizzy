// Package claude provides a runner that spawns the Claude Code CLI
// and parses its stream-json output into typed events.
package claude

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RunConfig configures a Claude Code invocation.
type RunConfig struct {
	Prompt       string
	MCPURL       string // e.g. http://localhost:8090/mcp
	MCPToken     string // Bearer token for MCP auth
	AllowedTools string // Tool pattern, e.g. "mcp__nube__*"
}

// Event is a parsed event from Claude's stream-json output.
type Event struct {
	Type      string  `json:"type"`                 // "connected", "tool_call", "text", "error", "done"
	SessionID string  `json:"session_id"`            // always set
	Model     string  `json:"model,omitempty"`       // set on "connected"
	Name      string  `json:"name,omitempty"`        // tool name on "tool_call"
	Content   string  `json:"content,omitempty"`     // text on "text"
	Error     string  `json:"error,omitempty"`       // message on "error"
	DurationMS int    `json:"duration_ms,omitempty"` // set on "done"
	CostUSD   float64 `json:"cost_usd,omitempty"`   // set on "done"
}

// RunResult contains the aggregated output after a run completes.
type RunResult struct {
	Text       string
	DurationMS int
	CostUSD    float64
}

// Run spawns claude and sends parsed events to the callback.
// It blocks until the process exits. The callback is called
// sequentially from one goroutine.
func Run(cfg RunConfig, sessionID string, onEvent func(Event)) RunResult {
	var result RunResult

	claudePath, err := exec.LookPath("claude")
	if err != nil {
		onEvent(Event{Type: "error", SessionID: sessionID, Error: "claude CLI not found in PATH"})
		return result
	}

	mcpFile, cleanup, err := writeMCPConfig(cfg.MCPURL, cfg.MCPToken)
	if err != nil {
		onEvent(Event{Type: "error", SessionID: sessionID, Error: "mcp config: " + err.Error()})
		return result
	}
	defer cleanup()

	args := []string{
		"-p",
		"--output-format", "stream-json",
		"--verbose",
		"--allowedTools", cfg.AllowedTools, "ToolSearch",
		"--mcp-config", mcpFile,
	}

	cmd := exec.Command(claudePath, args...)
	cmd.Stdin = strings.NewReader(cfg.Prompt)
	cmd.Stderr = nil

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		onEvent(Event{Type: "error", SessionID: sessionID, Error: "stdout pipe: " + err.Error()})
		return result
	}

	if err := cmd.Start(); err != nil {
		onEvent(Event{Type: "error", SessionID: sessionID, Error: "start: " + err.Error()})
		return result
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var raw map[string]any
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}

		for _, ev := range parseRawEvent(raw, sessionID) {
			// Accumulate result data.
			result.Text += ev.Content
			if ev.Type == "done" {
				result.DurationMS = ev.DurationMS
				result.CostUSD = ev.CostUSD
			}
			onEvent(ev)
		}
	}

	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			onEvent(Event{
				Type:      "error",
				SessionID: sessionID,
				Error:     fmt.Sprintf("claude exited with code %d", exitErr.ExitCode()),
			})
		}
	}

	return result
}

// parseRawEvent converts a raw stream-json line into typed events.
func parseRawEvent(raw map[string]any, sessionID string) []Event {
	eventType, _ := raw["type"].(string)

	switch eventType {
	case "system":
		if sub, _ := raw["subtype"].(string); sub == "init" {
			model, _ := raw["model"].(string)
			return []Event{{Type: "connected", SessionID: sessionID, Model: model}}
		}

	case "assistant":
		msg, _ := raw["message"].(map[string]any)
		if msg == nil {
			return nil
		}
		content, _ := msg["content"].([]any)
		var events []Event
		for _, block := range content {
			b, ok := block.(map[string]any)
			if !ok {
				continue
			}
			switch b["type"] {
			case "text":
				text, _ := b["text"].(string)
				events = append(events, Event{Type: "text", SessionID: sessionID, Content: text})
			case "tool_use":
				name, _ := b["name"].(string)
				events = append(events, Event{Type: "tool_call", SessionID: sessionID, Name: name})
			}
		}
		return events

	case "result":
		return []Event{{
			Type:       "done",
			SessionID:  sessionID,
			DurationMS: intFromAny(raw["duration_ms"]),
			CostUSD:    floatFromAny(raw["total_cost_usd"]),
		}}
	}

	return nil
}

func writeMCPConfig(mcpURL, token string) (string, func(), error) {
	cfg := map[string]any{
		"mcpServers": map[string]any{
			"nube": map[string]any{
				"type": "http",
				"url":  mcpURL,
				"headers": map[string]string{
					"Authorization": "Bearer " + token,
				},
			},
		},
	}

	data, _ := json.MarshalIndent(cfg, "", "  ")

	tmpDir, err := os.MkdirTemp("", "nube-agent-*")
	if err != nil {
		return "", func() {}, err
	}
	tmpFile := filepath.Join(tmpDir, ".mcp.json")

	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		os.RemoveAll(tmpDir)
		return "", func() {}, err
	}

	return tmpFile, func() { os.RemoveAll(tmpDir) }, nil
}

func intFromAny(v any) int {
	if n, ok := v.(float64); ok {
		return int(n)
	}
	return 0
}

func floatFromAny(v any) float64 {
	if n, ok := v.(float64); ok {
		return n
	}
	return 0
}
