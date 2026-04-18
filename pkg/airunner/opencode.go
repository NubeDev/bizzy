package airunner

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// OpenCodeRunner drives the OpenCode CLI (https://opencode.ai).
//
// OpenCode is an open-source coding agent CLI with MCP support,
// local model support (Ollama, LMStudio), and free built-in models.
//
// Install: curl -fsSL https://opencode.ai/install | bash
// Binary:  ~/.opencode/bin/opencode
type OpenCodeRunner struct{}

func (r *OpenCodeRunner) Name() Provider { return ProviderOpenCode }

func (r *OpenCodeRunner) Available() bool {
	_, err := findOpenCode()
	return err == nil
}

func (r *OpenCodeRunner) Run(ctx context.Context, cfg RunConfig, sessionID string, onEvent func(Event)) RunResult {
	var result RunResult
	result.Provider = string(ProviderOpenCode)

	ocPath, err := findOpenCode()
	if err != nil {
		onEvent(Event{Type: "error", Provider: string(ProviderOpenCode), SessionID: sessionID,
			Error: "opencode CLI not found — install with: curl -fsSL https://opencode.ai/install | bash"})
		return result
	}

	// Write a temporary MCP config so OpenCode can reach the bizzy server.
	var mcpCleanup func()
	var mcpConfigDir string
	if cfg.MCPURL != "" && cfg.MCPToken != "" {
		mcpConfigDir, mcpCleanup, err = writeOpenCodeMCPConfig(cfg.MCPURL, cfg.MCPToken)
		if err != nil {
			onEvent(Event{Type: "error", Provider: string(ProviderOpenCode), SessionID: sessionID,
				Error: "mcp config: " + err.Error()})
			return result
		}
		defer mcpCleanup()
	}

	// Build the command: opencode run --format json [--model provider/model] "prompt"
	args := []string{
		"run",
		"--format", "json",
		"--dangerously-skip-permissions",
	}

	if cfg.Model != "" {
		args = append(args, "--model", cfg.Model)
	}

	// Continue a previous session if resuming.
	if cfg.ResumeID != "" {
		args = append(args, "--session", cfg.ResumeID, "--continue")
	}

	args = append(args, cfg.Prompt)

	cmd := exec.CommandContext(ctx, ocPath, args...)

	// Set working directory.
	if cfg.WorkDir != "" {
		cmd.Dir = cfg.WorkDir
	}

	// If we wrote an MCP config, point OpenCode at that directory
	// so it picks up the nube MCP server.
	if mcpConfigDir != "" {
		env := os.Environ()
		env = append(env, "OPENCODE_MCP_CONFIG="+filepath.Join(mcpConfigDir, "mcp.json"))
		cmd.Env = env
	}

	// Prepend system prompt to the user prompt if provided.
	if cfg.SystemPrompt != "" {
		cmd.Stdin = strings.NewReader(cfg.SystemPrompt + "\n\n---\n\n" + cfg.Prompt)
	}

	cmd.Stderr = nil

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		onEvent(Event{Type: "error", Provider: string(ProviderOpenCode), SessionID: sessionID,
			Error: "stdout pipe: " + err.Error()})
		return result
	}

	start := time.Now()

	if err := cmd.Start(); err != nil {
		onEvent(Event{Type: "error", Provider: string(ProviderOpenCode), SessionID: sessionID,
			Error: "start: " + err.Error()})
		return result
	}

	model := cfg.Model
	if model == "" {
		model = "opencode"
	}
	onEvent(Event{Type: "connected", Provider: string(ProviderOpenCode), SessionID: sessionID, Model: model})

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	var textBuf strings.Builder
	var toolCalls int
	var toolCallLog []ToolCallEntry

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Try to parse as JSON event.
		var raw map[string]any
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			// Not JSON — treat as plain text output.
			textBuf.WriteString(line)
			textBuf.WriteString("\n")
			onEvent(Event{
				Type:      "text",
				Provider:  string(ProviderOpenCode),
				SessionID: sessionID,
				Content:   line + "\n",
			})
			continue
		}

		// Parse OpenCode JSON events.
		events, tc := parseOpenCodeEvent(raw, sessionID)
		toolCalls += tc
		for _, ev := range events {
			if ev.Type == "text" {
				textBuf.WriteString(ev.Content)
			}
			if ev.Type == "tool_call" {
				toolCallLog = append(toolCallLog, ToolCallEntry{
					Name:   ev.Name,
					Status: "ok",
				})
			}
			onEvent(ev)
		}
	}

	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			onEvent(Event{
				Type:      "error",
				Provider:  string(ProviderOpenCode),
				SessionID: sessionID,
				Error:     fmt.Sprintf("opencode exited with code %d", exitErr.ExitCode()),
			})
		}
	}

	elapsed := time.Since(start)
	result.Text = textBuf.String()
	result.Model = model
	result.DurationMS = int(elapsed.Milliseconds())
	result.ToolCalls = toolCalls
	result.ToolCallLog = toolCallLog

	onEvent(Event{
		Type:       "done",
		Provider:   string(ProviderOpenCode),
		SessionID:  sessionID,
		DurationMS: result.DurationMS,
	})

	return result
}

// parseOpenCodeEvent converts an OpenCode JSON event into bizzy events.
//
// OpenCode --format json emits newline-delimited JSON with these event types:
//
//	{"type":"step_start",  "sessionID":"...", "part":{"type":"step-start", ...}}
//	{"type":"text",        "sessionID":"...", "part":{"type":"text", "text":"...", "time":{...}}}
//	{"type":"tool_call",   "sessionID":"...", "part":{"type":"tool-call", "tool":"...", ...}}
//	{"type":"tool_result", "sessionID":"...", "part":{"type":"tool-result", ...}}
//	{"type":"step_finish", "sessionID":"...", "part":{"type":"step-finish", "tokens":{...}, "cost":0}}
func parseOpenCodeEvent(raw map[string]any, sessionID string) ([]Event, int) {
	eventType, _ := raw["type"].(string)

	// Extract the "part" sub-object — most event data lives here.
	part, _ := raw["part"].(map[string]any)

	switch eventType {
	case "text":
		var text string
		if part != nil {
			text, _ = part["text"].(string)
		}
		if text == "" {
			text, _ = raw["text"].(string)
		}
		if text == "" {
			return nil, 0
		}
		return []Event{{
			Type:      "text",
			Provider:  string(ProviderOpenCode),
			SessionID: sessionID,
			Content:   text,
		}}, 0

	case "tool_use", "tool_call":
		var name string
		if part != nil {
			name, _ = part["tool"].(string)
			if name == "" {
				name, _ = part["name"].(string)
			}
		}
		if name == "" {
			name, _ = raw["tool"].(string)
		}
		return []Event{{
			Type:      "tool_call",
			Provider:  string(ProviderOpenCode),
			SessionID: sessionID,
			Name:      name,
		}}, 1

	case "tool_result":
		return nil, 0

	case "step_start":
		return nil, 0

	case "step_finish":
		// Only emit "done" when the step finishes with reason "stop" (final response).
		// Steps that finish with reason "tool-calls" are intermediate — more steps follow.
		var reason string
		if part != nil {
			reason, _ = part["reason"].(string)
		}
		if reason == "stop" || reason == "end_turn" {
			var cost float64
			if part != nil {
				cost, _ = part["cost"].(float64)
			}
			return []Event{{
				Type:      "done",
				Provider:  string(ProviderOpenCode),
				SessionID: sessionID,
				CostUSD:   cost,
			}}, 0
		}
		return nil, 0

	case "error":
		errMsg, _ := raw["error"].(string)
		if errMsg == "" && part != nil {
			errMsg, _ = part["error"].(string)
		}
		if errMsg == "" {
			errMsg, _ = raw["message"].(string)
		}
		return []Event{{
			Type:      "error",
			Provider:  string(ProviderOpenCode),
			SessionID: sessionID,
			Error:     errMsg,
		}}, 0
	}

	return nil, 0
}

// writeOpenCodeMCPConfig writes a temporary MCP config file for OpenCode.
// Returns the directory path, a cleanup function, and any error.
func writeOpenCodeMCPConfig(mcpURL, token string) (string, func(), error) {
	cfg := map[string]any{
		"nube": map[string]any{
			"type": "streamable-http",
			"url":  mcpURL,
			"headers": map[string]string{
				"Authorization": "Bearer " + token,
			},
		},
	}

	data, _ := json.MarshalIndent(cfg, "", "  ")

	tmpDir, err := os.MkdirTemp("", "nube-opencode-*")
	if err != nil {
		return "", func() {}, err
	}

	tmpFile := filepath.Join(tmpDir, "mcp.json")
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		os.RemoveAll(tmpDir)
		return "", func() {}, err
	}

	return tmpDir, func() { os.RemoveAll(tmpDir) }, nil
}

// findOpenCode locates the opencode binary.
// Checks PATH first, then the default install location.
func findOpenCode() (string, error) {
	if p, err := exec.LookPath("opencode"); err == nil {
		return p, nil
	}
	// Check default install location.
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("opencode not found")
	}
	p := filepath.Join(home, ".opencode", "bin", "opencode")
	if _, err := os.Stat(p); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("opencode not found")
}

