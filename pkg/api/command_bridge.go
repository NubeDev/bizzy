package api

import (
	"github.com/NubeDev/bizzy/pkg/airunner"
	"github.com/NubeDev/bizzy/pkg/command"
	"github.com/NubeDev/bizzy/pkg/models"
	"github.com/NubeDev/bizzy/pkg/services"
)

// CommandAgentBridge implements command.AgentExecutor by wiring to the
// existing AgentService and JobStore. This keeps the command package
// decoupled from the API layer.
type CommandAgentBridge struct {
	AgentSvc *services.AgentService
	Jobs     *airunner.JobStore
}

// RunJob starts an async AI job and returns the job ID.
func (b *CommandAgentBridge) RunJob(userID, prompt, provider, model string) string {
	user, err := b.AgentSvc.GetUser(userID)
	if err != nil {
		return ""
	}

	resolvedProvider, resolvedModel := b.AgentSvc.ResolveProvider(provider, model, user)
	runner, err := b.AgentSvc.GetRunner(resolvedProvider)
	if err != nil {
		return ""
	}

	systemPrompt := b.AgentSvc.BuildSystemPrompt(userID)
	enriched := b.AgentSvc.EnrichPrompt(userID, prompt)

	jobID := models.GenerateID("job-")
	sessionID := models.GenerateID("ses-")
	mcpURL := b.AgentSvc.MCPURL()

	job := b.Jobs.Submit(
		jobID,
		string(resolvedProvider),
		resolvedModel,
		runner,
		airunner.RunConfig{
			Prompt:       enriched,
			SystemPrompt: systemPrompt,
			MCPURL:       mcpURL,
			MCPToken:     user.Token,
			AllowedTools: "mcp__nube__*",
			Model:        resolvedModel,
		},
		sessionID,
		userID,
	)

	// Persist session when done.
	go func() {
		ch := job.Subscribe()
		for range ch {
		}
		result := job.Result()
		if result != nil {
			b.AgentSvc.SaveSession(services.SessionParams{
				ID:        sessionID,
				Prompt:    prompt,
				UserID:    userID,
				JobStatus: string(job.Status),
				Result:    result,
			})
		}
	}()

	return jobID
}

// CommandToolLister implements command.ToolLister by wiring to ToolService.
type CommandToolLister struct {
	ToolSvc *services.ToolService
}

func (l *CommandToolLister) ListTools(userID string) []command.ToolInfo {
	tools := l.ToolSvc.ListTools(userID)
	out := make([]command.ToolInfo, len(tools))
	for i, t := range tools {
		out[i] = command.ToolInfo{Name: t.Name, AppName: t.AppName, Desc: t.Desc}
	}
	return out
}

func (l *CommandToolLister) ListPrompts(userID string) []command.PromptInfo {
	prompts := l.ToolSvc.ListPrompts(userID)
	out := make([]command.PromptInfo, len(prompts))
	for i, p := range prompts {
		out[i] = command.PromptInfo{Name: p.Name, AppName: p.AppName, Desc: p.Desc}
	}
	return out
}
