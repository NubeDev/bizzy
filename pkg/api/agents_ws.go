package api

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/NubeDev/bizzy/pkg/airunner"
	"github.com/NubeDev/bizzy/pkg/auth"
	"github.com/NubeDev/bizzy/pkg/claude"
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
	Prompt   string `json:"prompt"`
	Agent    string `json:"agent"`
	Provider string `json:"provider,omitempty"` // "claude" (default), "codex", "copilot"
	Model    string `json:"model,omitempty"`    // model override for the chosen provider
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
	if err := conn.WriteJSON(claude.Event{Type: "session", SessionID: sessionID}); err != nil {
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
	log.Printf("[agents-ws] received prompt (%d chars), agent=%q, provider=%q", len(req.Prompt), req.Agent, req.Provider)

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

	provider := airunner.Provider(req.Provider)
	if provider == "" {
		provider = airunner.ProviderClaude
	}
	log.Printf("[agents-ws] using provider: %s", provider)

	runner, runnerErr := a.Runners.Get(provider)
	if runnerErr != nil {
		log.Printf("[agents-ws] provider error: %v", runnerErr)
		sendWSError(conn, sessionID, runnerErr.Error())
		return
	}
	if !runner.Available() {
		log.Printf("[agents-ws] provider %s not available", provider)
		sendWSError(conn, sessionID, string(provider)+" CLI is not installed or not in PATH")
		return
	}
	log.Printf("[agents-ws] provider %s available, starting run...", provider)

	var result claude.RunResult
	eventCount := 0

	if provider == airunner.ProviderClaude {
		result = claude.Run(claude.RunConfig{
			Prompt:       req.Prompt,
			MCPURL:       mcpURL,
			MCPToken:     user.Token,
			AllowedTools: "mcp__nube__*",
		}, sessionID, func(ev claude.Event) {
			eventCount++
			if ev.Type != "text" {
				log.Printf("[agents-ws] event #%d: type=%s", eventCount, ev.Type)
			}
			if err := conn.WriteJSON(ev); err != nil {
				log.Printf("[agents-ws] ws write error: %v", err)
			}
		})
		log.Printf("[agents-ws] claude finished: %d events, %dms, $%.4f", eventCount, result.DurationMS, result.CostUSD)
	} else {
		// Use the airunner abstraction for codex / copilot.
		aiResult := runner.Run(airunner.RunConfig{
			Prompt:       req.Prompt,
			MCPURL:       mcpURL,
			MCPToken:     user.Token,
			AllowedTools: "mcp__nube__*",
			Model:        req.Model,
		}, sessionID, func(ev airunner.Event) {
			if err := conn.WriteJSON(ev); err != nil {
				log.Printf("[agents] ws write: %v", err)
			}
		})
		result = claude.RunResult{
			Text:       aiResult.Text,
			DurationMS: aiResult.DurationMS,
			CostUSD:    aiResult.CostUSD,
		}
	}

	// Persist session to disk.
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

	conn.WriteMessage(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
	)
}

func sendWSError(conn *websocket.Conn, sessionID, msg string) {
	conn.WriteJSON(claude.Event{Type: "error", SessionID: sessionID, Error: msg})
}

// --- Session history REST endpoints ---

// sessionSummary is the list view (omits full result text).
type sessionSummary struct {
	ID         string    `json:"id"`
	Agent      string    `json:"agent,omitempty"`
	Prompt     string    `json:"prompt"`
	Status     string    `json:"status"`
	DurationMS int       `json:"duration_ms"`
	CostUSD    float64   `json:"cost_usd"`
	UserID     string    `json:"user_id"`
	CreatedAt  time.Time `json:"created_at"`
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
			ID:         s.ID,
			Agent:      s.Agent,
			Prompt:     s.Prompt,
			Status:     s.Status,
			DurationMS: s.DurationMS,
			CostUSD:    s.CostUSD,
			UserID:     s.UserID,
			CreatedAt:  s.CreatedAt,
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
