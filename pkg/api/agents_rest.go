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
	App            string `json:"app,omitempty"`             // App name — stored for session lookup on next page load
	SessionID      string `json:"session_id,omitempty"`      // If set, resumes this session (multi-turn)
	Provider       string `json:"provider,omitempty"`        // "claude" (default), "codex", "copilot"
	Model          string `json:"model,omitempty"`           // model override for the chosen provider
	ThinkingBudget string `json:"thinking_budget,omitempty"` // "low", "medium", "high", or token count
}

// runAgentResponse is the synchronous response.
type runAgentResponse struct {
	SessionID       string  `json:"session_id"`
	ClaudeSessionID string  `json:"claude_session_id,omitempty"` // For multi-turn resume
	Provider        string  `json:"provider"`
	Text            string  `json:"text"`
	DurationMS      int     `json:"duration_ms"`
	CostUSD         float64 `json:"cost_usd"`
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

	// Always generate a fresh session ID for the audit record.
	sessionID := models.GenerateID("ses-")
	mcpURL := a.AgentSvc.MCPURL()

	// Conversation ID stays stable across turns for non-Claude providers.
	// First call: same as sessionID. Resume calls: reuse the original.
	conversationID := sessionID

	cfg := airunner.RunConfig{
		Prompt:         prompt,
		SystemPrompt:   systemPrompt,
		MCPURL:         mcpURL,
		MCPToken:       user.Token,
		AllowedTools:   "mcp__nube__*",
		Model:          model,
		ThinkingBudget: req.ThinkingBudget,
	}

	// Handle resume based on provider type.
	if req.SessionID != "" {
		if provider == airunner.ProviderClaude {
			// Claude: pass the session ID for --resume (Claude CLI handles state)
			cfg.ResumeID = req.SessionID
		} else {
			// Non-Claude: load stored conversation history (server manages state)
			conversationID = req.SessionID
			if history, err := a.AgentSvc.LoadChatHistory(req.SessionID); err == nil && len(history) > 0 {
				cfg.History = history
			}
		}
	}

	// Collect events (the caller gets the final result, not a stream).
	var events []airunner.Event
	result := runner.Run(c.Request.Context(), cfg, sessionID, func(ev airunner.Event) {
		events = append(events, ev)
	})

	// For non-Claude providers, save the accumulated conversation history.
	if provider != airunner.ProviderClaude {
		var fullHistory []airunner.HistoryMessage
		if len(cfg.History) > 0 {
			// Resume: extend existing history
			fullHistory = append(fullHistory, cfg.History...)
		} else if systemPrompt != "" {
			// New session: start with system prompt
			fullHistory = append(fullHistory, airunner.HistoryMessage{Role: "system", Content: systemPrompt})
		}
		fullHistory = append(fullHistory, airunner.HistoryMessage{Role: "user", Content: prompt})
		if result.Text != "" {
			fullHistory = append(fullHistory, airunner.HistoryMessage{Role: "assistant", Content: result.Text})
		}
		a.AgentSvc.SaveChatHistory(conversationID, req.App, string(provider), user.ID, fullHistory)
	}

	// Persist audit session (always a new record).
	// Use App name as the agent if no explicit agent was set — enables session lookup by app.
	agent := req.Agent
	if agent == "" {
		agent = req.App
	}
	a.AgentSvc.SaveSession(services.SessionParams{
		ID:        sessionID,
		Agent:     agent,
		Prompt:    req.Prompt,
		UserID:    user.ID,
		JobStatus: "done",
		Result:    &result,
	})

	// Return the conversation ID so the client can resume.
	// For Claude: client uses claude_session_id instead (preferred by the hook).
	// For Ollama/others: client uses session_id (= conversationID, stable across turns).
	c.JSON(http.StatusOK, runAgentResponse{
		SessionID:       conversationID,
		ClaudeSessionID: result.ClaudeSessionID,
		Provider:        result.Provider,
		Text:            result.Text,
		DurationMS:      result.DurationMS,
		CostUSD:         result.CostUSD,
	})
}

// getLatestAppSession returns the resume session ID for the most recent
// conversation in a given app. The frontend calls this on page load so
// usePromptRunner can resume where the user left off.
//
//	GET /api/agents/sessions/app/:name
func (a *API) getLatestAppSession(c *gin.Context) {
	user := auth.GetUser(c)
	name := c.Param("name")

	resumeID, err := a.AgentSvc.LatestSessionForApp(name, user.ID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"session_id": resumeID})
}

// listProviders returns which AI providers are available on this server.
//
//	GET /api/agents/providers
func (a *API) listProviders(c *gin.Context) {
	c.JSON(http.StatusOK, a.Runners.Available())
}
