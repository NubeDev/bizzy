package api

import (
	"net/http"

	"github.com/NubeDev/bizzy/pkg/airunner"
	"github.com/NubeDev/bizzy/pkg/auth"
	"github.com/NubeDev/bizzy/pkg/models"
	"github.com/NubeDev/bizzy/pkg/services"
	"github.com/gin-gonic/gin"
)

// runAgentRequest is the JSON body for POST /api/agents/run.
type runAgentRequest struct {
	Prompt         string `json:"prompt"   binding:"required"`
	Agent          string `json:"agent,omitempty"`
	Provider       string `json:"provider,omitempty"`        // "claude" (default), "codex", "copilot"
	Model          string `json:"model,omitempty"`           // model override for the chosen provider
	ThinkingBudget string `json:"thinking_budget,omitempty"` // "low", "medium", "high", or token count
}

// runAgentResponse is the synchronous response.
type runAgentResponse struct {
	SessionID  string  `json:"session_id"`
	Provider   string  `json:"provider"`
	Text       string  `json:"text"`
	DurationMS int     `json:"duration_ms"`
	CostUSD    float64 `json:"cost_usd"`
}

// runAgentREST executes a prompt synchronously via the chosen AI provider
// and returns the full result.
//
//	POST /api/agents/run/sync
//	Body: {"prompt":"...", "provider":"codex", "model":"o4-mini"}
func (a *API) runAgentREST(c *gin.Context) {
	user := auth.GetUser(c)

	var req runAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	provider, model := a.AgentSvc.ResolveProvider(req.Provider, req.Model, user)

	runner, err := a.AgentSvc.GetRunner(provider)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	systemPrompt := a.AgentSvc.BuildSystemPrompt(user.ID)
	prompt := a.AgentSvc.EnrichPrompt(user.ID, req.Prompt)

	if req.Agent != "" {
		if _, exists := a.AppRegistry.Get(req.Agent); !exists {
			c.JSON(http.StatusBadRequest, gin.H{"error": "agent not found: " + req.Agent})
			return
		}
	}

	sessionID := models.GenerateID("ses-")
	mcpURL := a.AgentSvc.MCPURL()

	// Collect events (the caller gets the final result, not a stream).
	var events []airunner.Event
	result := runner.Run(c.Request.Context(), airunner.RunConfig{
		Prompt:         prompt,
		SystemPrompt:   systemPrompt,
		MCPURL:         mcpURL,
		MCPToken:       user.Token,
		AllowedTools:   "mcp__nube__*",
		Model:          model,
		ThinkingBudget: req.ThinkingBudget,
	}, sessionID, func(ev airunner.Event) {
		events = append(events, ev)
	})

	// Persist session.
	a.AgentSvc.SaveSession(services.SessionParams{
		ID:        sessionID,
		Agent:     req.Agent,
		Prompt:    req.Prompt,
		UserID:    user.ID,
		JobStatus: "done",
		Result:    &result,
	})

	c.JSON(http.StatusOK, runAgentResponse{
		SessionID:  sessionID,
		Provider:   result.Provider,
		Text:       result.Text,
		DurationMS: result.DurationMS,
		CostUSD:    result.CostUSD,
	})
}

// listProviders returns which AI providers are available on this server.
//
//	GET /api/agents/providers
func (a *API) listProviders(c *gin.Context) {
	c.JSON(http.StatusOK, a.Runners.Available())
}
