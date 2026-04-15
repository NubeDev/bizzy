package api

import (
	"net/http"
	"time"

	"github.com/NubeDev/bizzy/pkg/auth"
	"github.com/NubeDev/bizzy/pkg/models"
	"github.com/gin-gonic/gin"
)

func (a *API) createWorkspace(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ws := models.Workspace{
		ID:        models.GenerateID("ws-"),
		Name:      req.Name,
		CreatedAt: time.Now().UTC(),
	}
	if err := a.Workspaces.Create(ws); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, ws)
}

func (a *API) listWorkspaces(c *gin.Context) {
	user := auth.GetUser(c)
	// Admins see all workspaces, users see only their own.
	if user.Role == models.RoleAdmin {
		c.JSON(http.StatusOK, a.Workspaces.All())
		return
	}
	ws, ok := a.Workspaces.Get(user.WorkspaceID)
	if !ok {
		c.JSON(http.StatusOK, []models.Workspace{})
		return
	}
	c.JSON(http.StatusOK, []models.Workspace{ws})
}

func (a *API) getWorkspace(c *gin.Context) {
	ws, ok := a.Workspaces.Get(c.Param("id"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "workspace not found"})
		return
	}
	c.JSON(http.StatusOK, ws)
}

func (a *API) deleteWorkspace(c *gin.Context) {
	id := c.Param("id")
	// Check no users remain in this workspace.
	users := a.Users.FindFunc(func(u models.User) bool {
		return u.WorkspaceID == id
	})
	if len(users) > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "workspace still has users — delete them first"})
		return
	}
	if err := a.Workspaces.Delete(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workspace not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": id})
}

func (a *API) createUser(c *gin.Context) {
	wsID := c.Param("id")
	if _, ok := a.Workspaces.Get(wsID); !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "workspace not found"})
		return
	}

	var req struct {
		Name  string      `json:"name" binding:"required"`
		Email string      `json:"email" binding:"required"`
		Role  models.Role `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Role == "" {
		req.Role = models.RoleUser
	}
	if req.Role != models.RoleAdmin && req.Role != models.RoleUser {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role must be 'admin' or 'user'"})
		return
	}

	user := models.User{
		ID:          models.GenerateID("usr-"),
		WorkspaceID: wsID,
		Name:        req.Name,
		Email:       req.Email,
		Role:        req.Role,
		Token:       models.GenerateToken(),
		CreatedAt:   time.Now().UTC(),
	}
	if err := a.Users.Create(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, user)
}

func (a *API) listWorkspaceUsers(c *gin.Context) {
	wsID := c.Param("id")
	users := a.Users.FindFunc(func(u models.User) bool {
		return u.WorkspaceID == wsID
	})
	c.JSON(http.StatusOK, users)
}
