package api

import (
	"net/http"

	"github.com/NubeDev/bizzy/pkg/auth"
	"github.com/gin-gonic/gin"
)

// listMyTools returns all MCP tools available to the current user (based on installed apps).
func (a *API) listMyTools(c *gin.Context) {
	user := auth.GetUser(c)
	c.JSON(http.StatusOK, a.ToolSvc.ListTools(user.ID))
}

// listMyPrompts returns all MCP prompts available to the current user.
func (a *API) listMyPrompts(c *gin.Context) {
	user := auth.GetUser(c)
	c.JSON(http.StatusOK, a.ToolSvc.ListPrompts(user.ID))
}

// getPrompt renders a prompt with the given arguments.
func (a *API) getPrompt(c *gin.Context) {
	user := auth.GetUser(c)
	promptName := c.Param("name")

	// Collect arguments from query params.
	args := make(map[string]string)
	for key, values := range c.Request.URL.Query() {
		if len(values) > 0 {
			args[key] = values[0]
		}
	}

	result, err := a.ToolSvc.GetPrompt(user.ID, promptName, args)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}
