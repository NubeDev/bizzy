package api

import (
	"net/http"
	"time"

	"github.com/NubeDev/bizzy/pkg/auth"
	"github.com/NubeDev/bizzy/pkg/models"
	"github.com/gin-gonic/gin"
)

// getNotifyPrefs returns the notification preferences for the current user.
//
//	GET /api/notifications/prefs
func (a *API) getNotifyPrefs(c *gin.Context) {
	user := auth.GetUser(c)

	var prefs models.NotifyPrefs
	if err := a.DB.Where("user_id = ?", user.ID).First(&prefs).Error; err != nil {
		// Return empty defaults if none exist.
		c.JSON(http.StatusOK, models.NotifyPrefs{UserID: user.ID})
		return
	}
	c.JSON(http.StatusOK, prefs)
}

// updateNotifyPrefs creates or updates the notification preferences for the current user.
//
//	PUT /api/notifications/prefs
func (a *API) updateNotifyPrefs(c *gin.Context) {
	user := auth.GetUser(c)

	var req models.NotifyPrefs
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.UserID = user.ID
	req.UpdatedAt = time.Now().UTC()

	// Upsert.
	var existing models.NotifyPrefs
	if err := a.DB.Where("user_id = ?", user.ID).First(&existing).Error; err != nil {
		// Create.
		if err := a.DB.Create(&req).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else {
		// Update.
		if err := a.DB.Save(&req).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, req)
}

// listWebhookLogs returns recent webhook logs (admin only).
//
//	GET /api/webhooks/logs
func (a *API) listWebhookLogs(c *gin.Context) {
	type WebhookLog struct {
		ID        string    `json:"id"`
		CommandID string    `json:"command_id"`
		Source    string    `json:"source"`
		UserID    string    `json:"user_id"`
		Text      string    `json:"text"`
		Status    string    `json:"status"`
		Error     string    `json:"error,omitempty"`
		CreatedAt time.Time `json:"created_at"`
	}

	var logs []WebhookLog
	a.DB.Order("created_at DESC").Limit(100).Find(&logs)
	c.JSON(http.StatusOK, logs)
}
