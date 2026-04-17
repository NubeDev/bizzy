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
	if err := a.DB.Create(&ws).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, ws)
}

func (a *API) listWorkspaces(c *gin.Context) {
	user := auth.GetUser(c)
	if user.Role == models.RoleAdmin {
		var all []models.Workspace
		a.DB.Find(&all)
		c.JSON(http.StatusOK, all)
		return
	}
	var ws models.Workspace
	if err := a.DB.First(&ws, "id = ?", user.WorkspaceID).Error; err != nil {
		c.JSON(http.StatusOK, []models.Workspace{})
		return
	}
	c.JSON(http.StatusOK, []models.Workspace{ws})
}

func (a *API) getWorkspace(c *gin.Context) {
	var ws models.Workspace
	if err := a.DB.First(&ws, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workspace not found"})
		return
	}
	c.JSON(http.StatusOK, ws)
}

func (a *API) deleteWorkspace(c *gin.Context) {
	id := c.Param("id")
	var userCount int64
	a.DB.Model(&models.User{}).Where("workspace_id = ?", id).Count(&userCount)
	if userCount > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "workspace still has users — delete them first"})
		return
	}
	result := a.DB.Delete(&models.Workspace{}, "id = ?", id)
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "workspace not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": id})
}

func (a *API) createUser(c *gin.Context) {
	wsID := c.Param("id")
	var ws models.Workspace
	if err := a.DB.First(&ws, "id = ?", wsID).Error; err != nil {
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
	if err := a.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, user)
}

func (a *API) listWorkspaceUsers(c *gin.Context) {
	wsID := c.Param("id")
	var users []models.User
	a.DB.Where("workspace_id = ?", wsID).Find(&users)
	c.JSON(http.StatusOK, users)
}
