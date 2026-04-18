package api

import (
	"net/http"

	"github.com/NubeDev/bizzy/pkg/auth"
	"github.com/gin-gonic/gin"
)

// listAgents returns all agents available to the authenticated user.
// Each installed+enabled app becomes an agent.
func (a *API) listAgents(c *gin.Context) {
	user := auth.GetUser(c)
	c.JSON(http.StatusOK, a.AgentSvc.ListAgents(user.ID))
}
