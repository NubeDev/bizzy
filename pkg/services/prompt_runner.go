package services

import (
	"context"
	"fmt"

	"github.com/NubeDev/bizzy/pkg/airunner"
	"github.com/NubeDev/bizzy/pkg/models"
)

// AgentPromptRunner bridges AI prompt execution to the AgentService.
// It resolves the user, picks a provider/model, builds system prompts,
// and runs a single-turn AI call. Satisfies the PromptRunner interface.
type AgentPromptRunner struct {
	AgentSvc *AgentService
}

// NewPromptRunner creates a PromptRunner backed by the AgentService.
func NewPromptRunner(agentSvc *AgentService) *AgentPromptRunner {
	return &AgentPromptRunner{AgentSvc: agentSvc}
}

func (pr *AgentPromptRunner) RunPrompt(ctx context.Context, userID, prompt string) (string, error) {
	user, err := pr.AgentSvc.GetUser(userID)
	if err != nil {
		return "", err
	}

	provider, model := pr.AgentSvc.ResolveProvider("", "", user)

	runner, err := pr.AgentSvc.GetRunner(provider)
	if err != nil {
		return "", fmt.Errorf("provider %s: %w", provider, err)
	}

	systemPrompt := pr.AgentSvc.BuildSystemPrompt(user.ID)
	prompt = pr.AgentSvc.EnrichPrompt(user.ID, prompt)

	sessionID := models.GenerateID("ses-")
	mcpURL := pr.AgentSvc.MCPURL()

	result := runner.Run(ctx, airunner.RunConfig{
		Prompt:       prompt,
		SystemPrompt: systemPrompt,
		MCPURL:       mcpURL,
		MCPToken:     user.Token,
		AllowedTools: "mcp__nube__*",
		Model:        model,
	}, sessionID, func(ev airunner.Event) {})

	if result.Text == "" {
		return "", fmt.Errorf("AI returned empty response")
	}
	return result.Text, nil
}
