package airunner

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// CopilotRunner drives the GitHub Copilot CLI (gh copilot).
//
// Install: gh extension install github/gh-copilot
// Requires: authenticated gh session (gh auth login).
//
// We use "gh copilot suggest -t shell" for shell-oriented prompts and
// fall back to a generic explain/suggest flow. The CLI writes its
// response to stdout.
type CopilotRunner struct{}

func (r *CopilotRunner) Name() Provider { return ProviderCopilot }

func (r *CopilotRunner) Available() bool {
	// Check that gh exists and the copilot extension is installed.
	ghPath, err := exec.LookPath("gh")
	if err != nil {
		return false
	}
	out, err := exec.Command(ghPath, "extension", "list").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "copilot")
}

func (r *CopilotRunner) Run(cfg RunConfig, sessionID string, onEvent func(Event)) RunResult {
	var result RunResult

	ghPath, err := exec.LookPath("gh")
	if err != nil {
		onEvent(Event{Type: "error", Provider: string(ProviderCopilot), SessionID: sessionID,
			Error: "gh CLI not found in PATH — install from https://cli.github.com"})
		return result
	}

	// gh copilot suggest -t shell "prompt"
	args := []string{"copilot", "suggest", "-t", "shell", cfg.Prompt}

	cmd := exec.Command(ghPath, args...)
	if cfg.WorkDir != "" {
		cmd.Dir = cfg.WorkDir
	}
	cmd.Stderr = nil

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		onEvent(Event{Type: "error", Provider: string(ProviderCopilot), SessionID: sessionID,
			Error: "stdout pipe: " + err.Error()})
		return result
	}

	start := time.Now()

	if err := cmd.Start(); err != nil {
		onEvent(Event{Type: "error", Provider: string(ProviderCopilot), SessionID: sessionID,
			Error: "start: " + err.Error()})
		return result
	}

	onEvent(Event{Type: "connected", Provider: string(ProviderCopilot), SessionID: sessionID, Model: "copilot"})

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	var textBuf strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		textBuf.WriteString(line)
		textBuf.WriteString("\n")
		onEvent(Event{
			Type:      "text",
			Provider:  string(ProviderCopilot),
			SessionID: sessionID,
			Content:   line + "\n",
		})
	}

	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			onEvent(Event{
				Type:      "error",
				Provider:  string(ProviderCopilot),
				SessionID: sessionID,
				Error:     fmt.Sprintf("gh copilot exited with code %d", exitErr.ExitCode()),
			})
		}
	}

	elapsed := time.Since(start)
	result.Text = textBuf.String()
	result.DurationMS = int(elapsed.Milliseconds())

	onEvent(Event{
		Type:       "done",
		Provider:   string(ProviderCopilot),
		SessionID:  sessionID,
		DurationMS: result.DurationMS,
	})

	return result
}
