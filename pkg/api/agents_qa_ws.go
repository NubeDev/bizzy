package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/NubeDev/bizzy/pkg/airunner"
	"github.com/NubeDev/bizzy/pkg/claude"
	"github.com/NubeDev/bizzy/pkg/models"
	"github.com/NubeDev/bizzy/pkg/services"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// qaStartRequest is the first message the client sends to start a QA flow.
type qaStartRequest struct {
	Flow string `json:"flow"` // e.g. "nube-marketing.marketing_plan_qa"
}

// qaAnswer is a message from the client answering a question.
type qaAnswer struct {
	Answer any `json:"answer"` // string, number, []string, etc.
}

// qaEvent is sent from server to client during a QA flow.
type qaEvent struct {
	Type      string `json:"type"`       // "session", "question", "generating", "text", "tool_call", "done", "error"
	SessionID string `json:"session_id"`

	// Question fields (type="question").
	Field       string `json:"field,omitempty"`
	Label       string `json:"label,omitempty"`
	Input       string `json:"input,omitempty"` // "text", "textarea", "select", "multi_select", "number"
	Required    bool   `json:"required,omitempty"`
	Default     any    `json:"default,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
	Options     any    `json:"options,omitempty"`
	MinLength   int    `json:"min_length,omitempty"`
	MaxLength   int    `json:"max_length,omitempty"`

	// Progress fields.
	Message string `json:"message,omitempty"`

	// Text/done/error fields (reuse claude.Event shape).
	Content    string  `json:"content,omitempty"`
	Model      string  `json:"model,omitempty"`
	Name       string  `json:"name,omitempty"`
	Error      string  `json:"error,omitempty"`
	DurationMS int     `json:"duration_ms,omitempty"`
	CostUSD    float64 `json:"cost_usd,omitempty"`

	// Skill generator output (sent with "generating" event when present).
	CreateApp    map[string]any `json:"create_app,omitempty"`
	CreatePrompt map[string]any `json:"create_prompt,omitempty"`
	CreateTool   map[string]any `json:"create_tool,omitempty"`
}

// runQAWS handles the conversational QA WebSocket flow.
//
// Protocol:
//  1. Client connects: ws://host/api/agents/qa?token=<bearer-token>
//  2. Server sends:    {"type":"session","session_id":"ses-..."}
//  3. Client sends:    {"flow":"nube-marketing.marketing_plan_qa"}
//  4. Server sends:    {"type":"question","field":"product","label":"What product?","input":"text",...}
//  5. Client sends:    {"answer":"Rubix"}
//  6. Repeat 4-5 until all questions answered.
//  7. Server sends:    {"type":"generating","message":"Building your marketing plan..."}
//  8. Server streams:  {"type":"text","content":"..."} (Claude response)
//  9. Server sends:    {"type":"done","duration_ms":...,"cost_usd":...}
// 10. Server closes connection.
func (a *API) runQAWS(c *gin.Context) {
	token := c.Query("token")

	var user models.User
	var ok bool

	if token == "" || token == "dev" {
		// Dev mode: use the first user (same as REST auth middleware).
		all := a.Users.All()
		if len(all) == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "no users exist"})
			return
		}
		user = all[0]
		ok = true
	} else {
		user, ok = a.Users.FindOne(func(u models.User) bool {
			return u.Token == token
		})
	}
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[qa] ws upgrade: %v", err)
		return
	}
	defer conn.Close()

	sessionID := models.GenerateID("ses-")
	conn.WriteJSON(qaEvent{Type: "session", SessionID: sessionID})

	// Read the flow request.
	var req qaStartRequest
	if err := conn.ReadJSON(&req); err != nil {
		sendQAError(conn, sessionID, "invalid request: "+err.Error())
		return
	}
	if req.Flow == "" {
		sendQAError(conn, sessionID, "flow is required")
		return
	}

	// Resolve the JS tool.
	resolved, err := a.ToolSvc.ResolveTool(user.ID, req.Flow)
	if err != nil {
		sendQAError(conn, sessionID, err.Error())
		return
	}
	runtime := resolved.Runtime
	manifest := resolved.Manifest

	// Q&A loop: call JS tool → get question or prompt → repeat.
	answers := make(map[string]any)

	for {
		// Build params: accumulated answers + no _submit flag.
		params := map[string]any{"_answers": answers}

		result, err := runtime.Execute(manifest.ScriptPath, params)
		if err != nil {
			sendQAError(conn, sessionID, "tool error: "+err.Error())
			return
		}

		resultType, _ := result["type"].(string)

		switch resultType {
		case "question":
			// Send question to client.
			ev := qaEvent{
				Type:      "question",
				SessionID: sessionID,
			}
			mapToQAEvent(result, &ev)
			conn.WriteJSON(ev)

			// Wait for answer.
			var ans qaAnswer
			if err := conn.ReadJSON(&ans); err != nil {
				sendQAError(conn, sessionID, "read answer: "+err.Error())
				return
			}

			field, _ := result["field"].(string)
			answers[field] = ans.Answer

		case "prompt":
			// All questions answered — run through Claude.
			prompt, _ := result["prompt"].(string)
			title, _ := result["title"].(string)
			if title == "" {
				title = manifest.Name
			}

			// Send generating event with optional skill generator output.
			genEvent := qaEvent{
				Type:      "generating",
				SessionID: sessionID,
				Message:   "Generating " + title + "...",
			}
			if v, ok := result["create_app"].(map[string]any); ok {
				genEvent.CreateApp = v
			}
			if v, ok := result["create_prompt"].(map[string]any); ok {
				genEvent.CreatePrompt = v
			}
			if v, ok := result["create_tool"].(map[string]any); ok {
				genEvent.CreateTool = v
			}
			conn.WriteJSON(genEvent)

			mcpURL := a.AgentSvc.MCPURL()
			claudeResult := claude.Run(c.Request.Context(), claude.RunConfig{
				Prompt:       prompt,
				MCPURL:       mcpURL,
				MCPToken:     user.Token,
				AllowedTools: "mcp__nube__*",
			}, sessionID, func(ev claude.Event) {
				// Forward Claude events as QA events.
				conn.WriteJSON(qaEvent{
					Type:       ev.Type,
					SessionID:  sessionID,
					Content:    ev.Content,
					Model:      ev.Model,
					Name:       ev.Name,
					Error:      ev.Error,
					DurationMS: ev.DurationMS,
					CostUSD:    ev.CostUSD,
				})
			})

			// Build the full prompt with answers for the session record.
			answersJSON, _ := json.Marshal(answers)

			a.AgentSvc.SaveSession(services.SessionParams{
				ID:        sessionID,
				Agent:     manifest.AppName,
				Prompt:    string(answersJSON),
				UserID:    user.ID,
				JobStatus: "done",
				Result: &airunner.RunResult{
					Text:            claudeResult.Text,
					Provider:        "claude",
					ClaudeSessionID: claudeResult.ClaudeSessionID,
					DurationMS:      claudeResult.DurationMS,
					CostUSD:         claudeResult.CostUSD,
				},
			})

			conn.WriteMessage(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			)
			return

		case "result":
			// Tool returned a final result (no Claude streaming needed).
			// Send it directly to the client as a "text" event with the JSON,
			// then close cleanly.
			resultJSON, _ := json.Marshal(result)
			conn.WriteJSON(qaEvent{
				Type:      "text",
				SessionID: sessionID,
				Content:   string(resultJSON),
			})
			conn.WriteJSON(qaEvent{
				Type:      "done",
				SessionID: sessionID,
			})
			conn.WriteMessage(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			)
			return

		default:
			sendQAError(conn, sessionID, "unexpected tool response type: "+resultType)
			return
		}
	}
}

// mapToQAEvent copies question fields from a JS tool result to a qaEvent.
func mapToQAEvent(result map[string]any, ev *qaEvent) {
	if v, ok := result["field"].(string); ok {
		ev.Field = v
	}
	if v, ok := result["label"].(string); ok {
		ev.Label = v
	}
	if v, ok := result["input"].(string); ok {
		ev.Input = v
	}
	if v, ok := result["required"].(bool); ok {
		ev.Required = v
	}
	if v, ok := result["default"]; ok {
		ev.Default = v
	}
	if v, ok := result["placeholder"].(string); ok {
		ev.Placeholder = v
	}
	if v, ok := result["options"]; ok {
		ev.Options = v
	}
	if v, ok := result["min_length"].(float64); ok {
		ev.MinLength = int(v)
	}
	if v, ok := result["max_length"].(float64); ok {
		ev.MaxLength = int(v)
	}
}

func sendQAError(conn *websocket.Conn, sessionID, msg string) {
	conn.WriteJSON(qaEvent{Type: "error", SessionID: sessionID, Error: msg})
}
