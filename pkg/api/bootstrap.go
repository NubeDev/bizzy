package api

import (
	"net/http"
	"time"

	"github.com/NubeDev/bizzy/pkg/models"
	"github.com/gin-gonic/gin"
)

// bootstrap creates the first workspace and admin user.
// Only works when no users exist — prevents re-bootstrap.
func (a *API) bootstrap(c *gin.Context) {
	if a.Users.Count() > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "server already bootstrapped — users exist"})
		return
	}

	var req struct {
		WorkspaceName string `json:"workspaceName" binding:"required"`
		AdminName     string `json:"adminName" binding:"required"`
		AdminEmail    string `json:"adminEmail" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ws := models.Workspace{
		ID:        models.GenerateID("ws-"),
		Name:      req.WorkspaceName,
		CreatedAt: time.Now().UTC(),
	}
	if err := a.Workspaces.Create(ws); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	admin := models.User{
		ID:          models.GenerateID("usr-"),
		WorkspaceID: ws.ID,
		Name:        req.AdminName,
		Email:       req.AdminEmail,
		Role:        models.RoleAdmin,
		Token:       models.GenerateToken(),
		CreatedAt:   time.Now().UTC(),
	}
	if err := a.Users.Create(admin); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"workspace": ws,
		"admin":     admin,
		"message":   "Save the admin token — it won't be shown again in production.",
	})
}
