package airunner

import (
	"context"
	"os/exec"

	"github.com/NubeDev/bizzy/pkg/claude"
)

// ClaudeRunner wraps the existing pkg/claude runner.
type ClaudeRunner struct{}

func (r *ClaudeRunner) Name() Provider { return ProviderClaude }

func (r *ClaudeRunner) Available() bool {
	_, err := exec.LookPath("claude")
	return err == nil
}

func (r *ClaudeRunner) Run(ctx context.Context, cfg RunConfig, sessionID string, onEvent func(Event)) RunResult {
	var toolCalls int
	res := claude.Run(ctx, claude.RunConfig{
		Prompt:       cfg.Prompt,
		ResumeID:     cfg.ResumeID,
		MCPURL:       cfg.MCPURL,
		MCPToken:     cfg.MCPToken,
		AllowedTools: cfg.AllowedTools,
	}, sessionID, func(ev claude.Event) {
		if ev.Type == "tool_call" {
			toolCalls++
		}
		// Convert claude.Event → airunner.Event
		onEvent(Event{
			Type:       ev.Type,
			Provider:   string(ProviderClaude),
			SessionID:  ev.SessionID,
			Model:      ev.Model,
			Name:       ev.Name,
			Content:    ev.Content,
			Error:      ev.Error,
			DurationMS: ev.DurationMS,
			CostUSD:    ev.CostUSD,
		})
	})

	return RunResult{
		Text:            res.Text,
		Provider:        string(ProviderClaude),
		ClaudeSessionID: res.ClaudeSessionID,
		DurationMS:      res.DurationMS,
		CostUSD:         res.CostUSD,
		ToolCalls:       toolCalls,
	}
}
