package airunner

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// CodexRunner drives the OpenAI Codex CLI (https://github.com/openai/codex).
//
// Install: npm install -g @openai/codex
// Requires: OPENAI_API_KEY environment variable.
//
// The CLI is invoked in quiet + full-auto mode so it runs non-interactively
// and writes its response to stdout.
type CodexRunner struct{}

func (r *CodexRunner) Name() Provider { return ProviderCodex }

func (r *CodexRunner) Available() bool {
	_, err := exec.LookPath("codex")
	return err == nil
}

func (r *CodexRunner) Run(ctx context.Context, cfg RunConfig, sessionID string, onEvent func(Event)) RunResult {
	var result RunResult

	codexPath, err := exec.LookPath("codex")
	if err != nil {
		onEvent(Event{Type: "error", Provider: string(ProviderCodex), SessionID: sessionID,
			Error: "codex CLI not found in PATH — install with: npm install -g @openai/codex"})
		return result
	}

	args := []string{
		"--quiet",
		"--full-auto",
	}
	if cfg.Model != "" {
		args = append(args, "--model", cfg.Model)
	}
	args = append(args, cfg.Prompt)

	cmd := exec.CommandContext(ctx, codexPath, args...)
	if cfg.WorkDir != "" {
		cmd.Dir = cfg.WorkDir
	}
	cmd.Stderr = nil

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		onEvent(Event{Type: "error", Provider: string(ProviderCodex), SessionID: sessionID,
			Error: "stdout pipe: " + err.Error()})
		return result
	}

	start := time.Now()

	if err := cmd.Start(); err != nil {
		onEvent(Event{Type: "error", Provider: string(ProviderCodex), SessionID: sessionID,
			Error: "start: " + err.Error()})
		return result
	}

	// Codex doesn't report the model in its output; use the requested one or a default.
	model := cfg.Model
	if model == "" {
		model = "codex"
	}
	onEvent(Event{Type: "connected", Provider: string(ProviderCodex), SessionID: sessionID, Model: model})

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	var textBuf strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		textBuf.WriteString(line)
		textBuf.WriteString("\n")
		onEvent(Event{
			Type:      "text",
			Provider:  string(ProviderCodex),
			SessionID: sessionID,
			Content:   line + "\n",
		})
	}

	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			onEvent(Event{
				Type:      "error",
				Provider:  string(ProviderCodex),
				SessionID: sessionID,
				Error:     fmt.Sprintf("codex exited with code %d", exitErr.ExitCode()),
			})
		}
	}

	elapsed := time.Since(start)
	result.Text = textBuf.String()
	result.Provider = string(ProviderCodex)
	result.Model = model
	result.DurationMS = int(elapsed.Milliseconds())

	onEvent(Event{
		Type:       "done",
		Provider:   string(ProviderCodex),
		SessionID:  sessionID,
		DurationMS: result.DurationMS,
	})

	return result
}
