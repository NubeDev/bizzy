package api

import (
	"net/http"
	"os"
	"time"

	"github.com/NubeDev/bizzy/pkg/airunner"
	"github.com/NubeDev/bizzy/pkg/auth"
	"github.com/NubeDev/bizzy/pkg/models"
	"github.com/gin-gonic/gin"
)

// runAgentRequest is the JSON body for POST /api/agents/run.
type runAgentRequest struct {
	Prompt   string `json:"prompt"   binding:"required"`
	Agent    string `json:"agent,omitempty"`
	Provider string `json:"provider,omitempty"` // "claude" (default), "codex", "copilot"
	Model    string `json:"model,omitempty"`    // model override for the chosen provider
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

	provider := airunner.Provider(req.Provider)
	if provider == "" {
		provider = airunner.ProviderClaude
	}

	runner, err := a.Runners.Get(provider)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !runner.Available() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": string(provider) + " CLI is not installed or not in PATH",
		})
		return
	}

	if req.Agent != "" {
		if _, exists := a.AppRegistry.Get(req.Agent); !exists {
			c.JSON(http.StatusBadRequest, gin.H{"error": "agent not found: " + req.Agent})
			return
		}
	}

	sessionID := models.GenerateID("ses-")
	mcpURL := "http://localhost" + os.Getenv("NUBE_ADDR") + "/mcp"

	// Collect events (the caller gets the final result, not a stream).
	var events []airunner.Event
	result := runner.Run(airunner.RunConfig{
		Prompt:       req.Prompt,
		MCPURL:       mcpURL,
		MCPToken:     user.Token,
		AllowedTools: "mcp__nube__*",
		Model:        req.Model,
	}, sessionID, func(ev airunner.Event) {
		events = append(events, ev)
	})

	// Persist session.
	a.Sessions.Create(models.Session{
		ID:         sessionID,
		Agent:      req.Agent,
		Prompt:     req.Prompt,
		Result:     result.Text,
		Status:     "done",
		DurationMS: result.DurationMS,
		CostUSD:    result.CostUSD,
		UserID:     user.ID,
		CreatedAt:  time.Now().UTC(),
	})

	c.JSON(http.StatusOK, runAgentResponse{
		SessionID:  sessionID,
		Provider:   string(provider),
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
