package airunner

import (
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

func (r *ClaudeRunner) Run(cfg RunConfig, sessionID string, onEvent func(Event)) RunResult {
	res := claude.Run(claude.RunConfig{
		Prompt:       cfg.Prompt,
		MCPURL:       cfg.MCPURL,
		MCPToken:     cfg.MCPToken,
		AllowedTools: cfg.AllowedTools,
	}, sessionID, func(ev claude.Event) {
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
		Text:       res.Text,
		DurationMS: res.DurationMS,
		CostUSD:    res.CostUSD,
	}
}
