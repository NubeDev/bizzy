package api

import (
	"net/http"

	"github.com/NubeDev/bizzy/pkg/auth"
	"github.com/NubeDev/bizzy/pkg/command"
	"github.com/gin-gonic/gin"
)

// commandRequest is the JSON body for POST /api/command.
type commandRequest struct {
	Text   string         `json:"text,omitempty"`   // text command to parse
	Verb   string         `json:"verb,omitempty"`   // structured: verb
	Target string         `json:"target,omitempty"` // structured: "kind/name"
	Params map[string]any `json:"params,omitempty"` // structured: params
}

// handleCommand processes a command from the REST API.
//
//	POST /api/command
func (a *API) handleCommand(c *gin.Context) {
	if a.CmdRouter == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "command bus not configured"})
		return
	}

	user := auth.GetUser(c)
	var req commandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	replyTo := command.ReplyInfo{Channel: "http"}

	var cmd command.Command
	var err error

	if req.Text != "" {
		// Parse from text.
		cmd, err = a.CmdRouter.Parser().Parse(req.Text, user.ID, replyTo,
			command.ParseConfig{BareTextBehaviour: "ask"})
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	} else if req.Verb != "" {
		// Structured command.
		cmd = command.Command{
			ID:      command.NewID(),
			Verb:    command.Verb(req.Verb),
			UserID:  user.ID,
			ReplyTo: replyTo,
			Params:  req.Params,
		}
		if req.Target != "" {
			parts := splitTarget(req.Target)
			cmd.Target = command.Target{Kind: parts[0], Name: parts[1]}
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "either 'text' or 'verb' is required"})
		return
	}

	// Execute synchronously for HTTP — the reply router handles async notifications.
	a.CmdRouter.Execute(c.Request.Context(), cmd)

	c.JSON(http.StatusAccepted, gin.H{
		"command_id": cmd.ID,
		"verb":       cmd.Verb,
		"target":     cmd.Target,
		"status":     "accepted",
	})
}

// handleCommandHelp returns available commands.
//
//	GET /api/command/help
func (a *API) handleCommandHelp(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"verbs": []string{"run", "ask", "status", "cancel", "restart", "list", "approve", "reject", "help"},
		"target_kinds": []string{"workflow", "tool", "job", "prompt"},
		"syntax": "[verb] [kind/name] [--param value ...]",
		"examples": []string{
			"run workflow/weekly-report --site Sydney",
			"run tool/rubix.query_nodes --floor 3",
			"ask \"check which devices are offline\"",
			"status workflow/wf-abc123",
			"cancel wf-abc123",
			"list workflows --status running",
			"approve wf-abc123",
		},
	})
}

func splitTarget(s string) [2]string {
	for i, ch := range s {
		if ch == '/' {
			return [2]string{s[:i], s[i+1:]}
		}
	}
	return [2]string{"", s}
}
