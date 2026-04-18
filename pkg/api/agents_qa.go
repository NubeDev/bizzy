package api

import (
	"net/http"

	"github.com/NubeDev/bizzy/pkg/auth"
	"github.com/gin-gonic/gin"
)

// callTool executes a JS tool directly via REST.
//
//	POST /api/agents/tools/:name
//	Body: {"product": "Rubix", "_submit": true, ...}
func (a *API) callTool(c *gin.Context) {
	user := auth.GetUser(c)
	toolName := c.Param("name")

	var params map[string]any
	if err := c.ShouldBindJSON(&params); err != nil {
		params = make(map[string]any)
	}

	result, err := a.ToolSvc.CallTool(c.Request.Context(), user.ID, toolName, params)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}
