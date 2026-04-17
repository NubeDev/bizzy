package api

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/NubeDev/bizzy/pkg/airunner"
	"github.com/NubeDev/bizzy/pkg/auth"
	"github.com/NubeDev/bizzy/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// --- WebSocket: GET /api/agents/run?token=<token> ---

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// wsRequest is the first message the client sends after connecting.
type wsRequest struct {
	Prompt    string `json:"prompt"`
	SessionID string `json:"session_id,omitempty"` // If set, resumes this Claude session (multi-turn)
	Agent     string `json:"agent"`
	Provider  string `json:"provider,omitempty"` // "claude" (default), "codex", "copilot"
	Model     string `json:"model,omitempty"`    // model override for the chosen provider
}

// runAgentWS upgrades to WebSocket and runs a Claude session.
//
// Protocol:
//  1. Client connects: ws://host/api/agents/run?token=<bearer-token>
//  2. Server sends session event:  {"type":"session","session_id":"ses-..."}
//  3. Client sends run request:    {"prompt":"...","agent":"..."}
//  4. Server streams events:       {"type":"connected|tool_call|text|done","session_id":"ses-..."}
//  5. Server closes the connection after the "done" event.
func (a *API) runAgentWS(c *gin.Context) {
	log.Printf("[agents-ws] new connection from %s", c.Request.RemoteAddr)

	// Auth via query param (browsers can't set headers on WS upgrade).
	// Dev mode: if no token, fall back to first user.
	var user models.User
	token := c.Query("token")
	if token != "" {
		u, ok := a.Users.FindOne(func(u models.User) bool {
			return u.Token == token
		})
		if !ok {
			log.Printf("[agents-ws] invalid token")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		user = u
	} else {
		all := a.Users.All()
		if len(all) == 0 {
			log.Printf("[agents-ws] no users exist")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "no users — POST /bootstrap first"})
			return
		}
		user = all[0]
		log.Printf("[agents-ws] dev mode: using user %s (%s)", user.ID, user.Name)
	}

	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[agents-ws] upgrade failed: %v", err)
		return
	}
	defer conn.Close()
	log.Printf("[agents-ws] upgraded to websocket")

	// Create session and send ID to client immediately.
	sessionID := models.GenerateID("ses-")
	if err := conn.WriteJSON(airunner.Event{Type: "session", SessionID: sessionID}); err != nil {
		log.Printf("[agents-ws] failed to send session event: %v", err)
		return
	}
	log.Printf("[agents-ws] sent session %s, waiting for prompt...", sessionID)

	// Wait for the run request.
	var req wsRequest
	if err := conn.ReadJSON(&req); err != nil {
		log.Printf("[agents-ws] failed to read request: %v", err)
		sendWSError(conn, sessionID, "invalid request: "+err.Error())
		return
	}
	log.Printf("[agents-ws] received prompt (%d chars), agent=%q, provider=%q, resume=%q", len(req.Prompt), req.Agent, req.Provider, req.SessionID)
	// Log the full prompt so we can see exactly what Claude receives.
	if len(req.Prompt) <= 2000 {
		log.Printf("[agents-ws] prompt text:\n---\n%s\n---", req.Prompt)
	} else {
		log.Printf("[agents-ws] prompt text (truncated):\n---\n%s\n...[%d more chars]\n---", req.Prompt[:2000], len(req.Prompt)-2000)
	}

	if req.Prompt == "" {
		sendWSError(conn, sessionID, "prompt is required")
		return
	}
	if req.Agent != "" {
		if _, exists := a.AppRegistry.Get(req.Agent); !exists {
			sendWSError(conn, sessionID, "agent not found: "+req.Agent)
			return
		}
	}

	mcpURL := "http://localhost" + os.Getenv("NUBE_ADDR") + "/mcp"

	// Prepend memory (server + user) and app context to the prompt.
	prompt := req.Prompt
	if a.Memory != nil {
		if prefix := a.Memory.BuildPromptPrefix(user.ID); prefix != "" {
			prompt = prefix + prompt
		}
	}
	// Inject installed app/tool descriptions so the AI knows what tools are available.
	if a.MCPFactory != nil {
		installs := a.AppInstalls.FindFunc(func(ai models.AppInstall) bool {
			return ai.UserID == user.ID && ai.Enabled
		})
		if appCtx := a.MCPFactory.BuildAppContext(installs); appCtx != "" {
			prompt = appCtx + prompt
		}
	}

	provider, model := resolveProvider(req.Provider, req.Model, user)
	log.Printf("[agents-ws] using provider: %s", provider)

	runner, runnerErr := a.Runners.Get(provider)
	if runnerErr != nil {
		log.Printf("[agents-ws] provider error: %v", runnerErr)
		sendWSError(conn, sessionID, runnerErr.Error())
		return
	}
	if !runner.Available() {
		log.Printf("[agents-ws] provider %s not available", provider)
		sendWSError(conn, sessionID, string(provider)+" is not available")
		return
	}
	log.Printf("[agents-ws] provider %s available, starting run...", provider)

	if req.SessionID != "" {
		log.Printf("[agents-ws] RESUMING session %s", req.SessionID)
	} else {
		log.Printf("[agents-ws] starting NEW session with MCP=%s", mcpURL)
	}

	// Submit as a job — all runs go through the job store so they're cancellable.
	jobID := models.GenerateID("job-")
	job := a.Jobs.Submit(
		jobID,
		string(provider),
		model,
		runner,
		airunner.RunConfig{
			Prompt:       prompt,
			ResumeID:     req.SessionID,
			MCPURL:       mcpURL,
			MCPToken:     user.Token,
			AllowedTools: "mcp__nube__*",
			Model:        model,
		},
		sessionID,
		user.ID,
	)

	// Send job_id to client so they can cancel from elsewhere.
	conn.WriteJSON(airunner.Event{
		Type:      "job",
		SessionID: sessionID,
		Content:   jobID,
	})

	// Subscribe to real-time events and stream to WS.
	events := job.Subscribe()
	defer job.Unsubscribe(events)

	eventCount := 0
	for ev := range events {
		eventCount++
		switch ev.Type {
		case "text":
			// skip logging streaming text chunks
		case "tool_call":
			log.Printf("[agents-ws] event #%d: tool_call name=%s", eventCount, ev.Name)
		default:
			log.Printf("[agents-ws] event #%d: type=%s", eventCount, ev.Type)
		}
		if err := conn.WriteJSON(ev); err != nil {
			log.Printf("[agents-ws] ws write error: %v", err)
			// WS broken — cancel the job.
			a.Jobs.Cancel(jobID)
			break
		}
	}

	// Wait for the result (job may still be finishing).
	for job.Result() == nil {
		time.Sleep(50 * time.Millisecond)
	}
	result := job.Result()

	// If the provider returned a session ID for resume (Claude), send it to the client.
	if result.ClaudeSessionID != "" {
		log.Printf("[agents-ws] claude session ID: %s", result.ClaudeSessionID)
		conn.WriteJSON(airunner.Event{
			Type:      "session_id",
			Provider:  string(provider),
			SessionID: sessionID,
			Content:   result.ClaudeSessionID,
		})
	}
	log.Printf("[agents-ws] %s finished: %d events, %dms, $%.4f", provider, eventCount, result.DurationMS, result.CostUSD)

	// Persist session to disk.
	a.Sessions.Create(models.Session{
		ID:              sessionID,
		Provider:        result.Provider,
		Model:           result.Model,
		ClaudeSessionID: result.ClaudeSessionID,
		Agent:           req.Agent,
		Prompt:          req.Prompt,
		Result:          result.Text,
		Status:          string(job.Status),
		DurationMS:      result.DurationMS,
		CostUSD:         result.CostUSD,
		InputTokens:     result.InputTokens,
		OutputTokens:    result.OutputTokens,
		ToolCalls:       result.ToolCalls,
		ToolCallLog:     convertToolCallLog(result.ToolCallLog),
		UserID:          user.ID,
		CreatedAt:       time.Now().UTC(),
	})

	conn.WriteMessage(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
	)
}

func sendWSError(conn *websocket.Conn, sessionID, msg string) {
	conn.WriteJSON(airunner.Event{Type: "error", SessionID: sessionID, Error: msg})
}

// convertToolCallLog converts airunner.ToolCallEntry to models.ToolCallEntry.
func convertToolCallLog(entries []airunner.ToolCallEntry) []models.ToolCallEntry {
	if len(entries) == 0 {
		return nil
	}
	out := make([]models.ToolCallEntry, len(entries))
	for i, e := range entries {
		out[i] = models.ToolCallEntry{
			Name:        e.Name,
			DurationMS:  e.DurationMS,
			Status:      e.Status,
			Error:       e.Error,
			InputBytes:  e.InputBytes,
			OutputBytes: e.OutputBytes,
		}
	}
	return out
}

// --- Session history REST endpoints ---

// sessionSummary is the list view (omits full result text).
type sessionSummary struct {
	ID              string    `json:"id"`
	Provider        string    `json:"provider"`
	Model           string    `json:"model,omitempty"`
	ClaudeSessionID string    `json:"claude_session_id,omitempty"`
	Agent           string    `json:"agent,omitempty"`
	Prompt          string    `json:"prompt"`
	Status          string    `json:"status"`
	DurationMS      int       `json:"duration_ms"`
	CostUSD         float64   `json:"cost_usd"`
	InputTokens     int       `json:"input_tokens,omitempty"`
	OutputTokens    int       `json:"output_tokens,omitempty"`
	ToolCalls       int       `json:"tool_calls,omitempty"`
	UserID          string    `json:"user_id"`
	CreatedAt       time.Time `json:"created_at"`
}

// listSessions returns session history for the authenticated user.
func (a *API) listSessions(c *gin.Context) {
	user := auth.GetUser(c)

	all := a.Sessions.FindFunc(func(s models.Session) bool {
		return s.UserID == user.ID
	})

	out := make([]sessionSummary, 0, len(all))
	for _, s := range all {
		out = append(out, sessionSummary{
			ID:              s.ID,
			Provider:        s.Provider,
			Model:           s.Model,
			ClaudeSessionID: s.ClaudeSessionID,
			Agent:           s.Agent,
			Prompt:          s.Prompt,
			Status:          s.Status,
			DurationMS:      s.DurationMS,
			CostUSD:         s.CostUSD,
			InputTokens:     s.InputTokens,
			OutputTokens:    s.OutputTokens,
			ToolCalls:       s.ToolCalls,
			UserID:          s.UserID,
			CreatedAt:       s.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, out)
}

// getSession returns a single session with the full result text.
func (a *API) getSession(c *gin.Context) {
	user := auth.GetUser(c)
	id := c.Param("id")

	s, ok := a.Sessions.Get(id)
	if !ok || s.UserID != user.ID {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	c.JSON(http.StatusOK, s)
}
