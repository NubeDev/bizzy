package airunner

import (
	"context"
	"os/exec"
	"time"

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
	var toolCallLog []ToolCallEntry
	var pendingTool *ToolCallEntry
	var toolStartTime time.Time

	res := claude.Run(ctx, claude.RunConfig{
		Prompt:       cfg.Prompt,
		ResumeID:     cfg.ResumeID,
		MCPURL:       cfg.MCPURL,
		MCPToken:     cfg.MCPToken,
		AllowedTools: cfg.AllowedTools,
	}, sessionID, func(ev claude.Event) {
		if ev.Type == "tool_call" {
			// Finish the previous tool call if one was pending.
			if pendingTool != nil {
				pendingTool.DurationMS = int(time.Since(toolStartTime).Milliseconds())
				pendingTool.Status = "ok"
				toolCallLog = append(toolCallLog, *pendingTool)
			}
			toolCalls++
			pendingTool = &ToolCallEntry{Name: ev.Name}
			toolStartTime = time.Now()
		} else if ev.Type == "error" && pendingTool != nil {
			pendingTool.DurationMS = int(time.Since(toolStartTime).Milliseconds())
			pendingTool.Status = "error"
			pendingTool.Error = ev.Error
			toolCallLog = append(toolCallLog, *pendingTool)
			pendingTool = nil
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

	// Finish the last pending tool call.
	if pendingTool != nil {
		pendingTool.DurationMS = int(time.Since(toolStartTime).Milliseconds())
		pendingTool.Status = "ok"
		toolCallLog = append(toolCallLog, *pendingTool)
	}

	return RunResult{
		Text:            res.Text,
		Provider:        string(ProviderClaude),
		ClaudeSessionID: res.ClaudeSessionID,
		DurationMS:      res.DurationMS,
		CostUSD:         res.CostUSD,
		ToolCalls:       toolCalls,
		ToolCallLog:     toolCallLog,
	}
}
