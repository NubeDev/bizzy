package api

import (
	"net/http"

	"github.com/NubeDev/bizzy/pkg/models"
	"github.com/gin-gonic/gin"
)

// --- Global secrets (admin only) ---

// PUT /api/secrets/:ownerType/:ownerName/:key
func (a *API) setGlobalSecret(c *gin.Context) {
	ownerType := c.Param("ownerType")
	ownerName := c.Param("ownerName")
	key := c.Param("key")

	var body struct {
		Value string `json:"value" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := a.Secrets.SetGlobal(ownerType, ownerName, key, body.Value); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// DELETE /api/secrets/:ownerType/:ownerName/:key
func (a *API) deleteGlobalSecret(c *gin.Context) {
	ownerType := c.Param("ownerType")
	ownerName := c.Param("ownerName")
	key := c.Param("key")

	if err := a.Secrets.Delete(models.SecretScopeGlobal, "", ownerType, ownerName, key); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// GET /api/secrets/:ownerType/:ownerName
func (a *API) listGlobalSecrets(c *gin.Context) {
	ownerType := c.Param("ownerType")
	ownerName := c.Param("ownerName")
	entries := a.Secrets.List("", ownerType, ownerName)
	c.JSON(http.StatusOK, entries)
}

// GET /api/secrets
func (a *API) listAllSecrets(c *gin.Context) {
	entries := a.Secrets.ListAll()
	c.JSON(http.StatusOK, entries)
}

// --- User secrets (per-user) ---

// PUT /api/secrets/me/:ownerType/:ownerName/:key
func (a *API) setUserSecret(c *gin.Context) {
	userID := c.GetString("user_id")
	ownerType := c.Param("ownerType")
	ownerName := c.Param("ownerName")
	key := c.Param("key")

	var body struct {
		Value string `json:"value" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := a.Secrets.SetUser(userID, ownerType, ownerName, key, body.Value); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// DELETE /api/secrets/me/:ownerType/:ownerName/:key
func (a *API) deleteUserSecret(c *gin.Context) {
	userID := c.GetString("user_id")
	ownerType := c.Param("ownerType")
	ownerName := c.Param("ownerName")
	key := c.Param("key")

	if err := a.Secrets.Delete(models.SecretScopeUser, userID, ownerType, ownerName, key); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// GET /api/secrets/me/:ownerType/:ownerName
func (a *API) listUserSecrets(c *gin.Context) {
	userID := c.GetString("user_id")
	ownerType := c.Param("ownerType")
	ownerName := c.Param("ownerName")
	entries := a.Secrets.List(userID, ownerType, ownerName)
	c.JSON(http.StatusOK, entries)
}
