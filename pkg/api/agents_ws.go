package api

import (
	"log"
	"net/http"
	"os"
	"time"

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
	Prompt string `json:"prompt"`
	Agent  string `json:"agent"`
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
	// Auth via query param (browsers can't set headers on WS upgrade).
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token query parameter required"})
		return
	}

	user, ok := a.Users.FindOne(func(u models.User) bool {
		return u.Token == token
	})
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[agents] ws upgrade: %v", err)
		return
	}
	defer conn.Close()

	// Create session and send ID to client immediately.
	sessionID := models.GenerateID("ses-")
	conn.WriteJSON(claude.Event{Type: "session", SessionID: sessionID})

	// Wait for the run request.
	var req wsRequest
	if err := conn.ReadJSON(&req); err != nil {
		sendWSError(conn, sessionID, "invalid request: "+err.Error())
		return
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

	result := claude.Run(claude.RunConfig{
		Prompt:       req.Prompt,
		MCPURL:       mcpURL,
		MCPToken:     user.Token,
		AllowedTools: "mcp__nube__*",
	}, sessionID, func(ev claude.Event) {
		if err := conn.WriteJSON(ev); err != nil {
			log.Printf("[agents] ws write: %v", err)
		}
	})

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
