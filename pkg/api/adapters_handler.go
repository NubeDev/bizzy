package api

import (
	"net/http"

	"github.com/NubeDev/bizzy/pkg/adapters/cron"
	"github.com/NubeDev/bizzy/pkg/models"
	"github.com/gin-gonic/gin"
)

// listAdapters returns all adapter configs from the database.
//
//	GET /api/adapters
func (a *API) listAdapters(c *gin.Context) {
	var configs []models.AdapterConfig
	a.DB.Find(&configs)
	c.JSON(http.StatusOK, configs)
}

// --- Cron CRUD ---

// listCronEntries returns all scheduled command entries.
//
//	GET /api/cron
func (a *API) listCronEntries(c *gin.Context) {
	entries := cron.ListEntries(a.DB)
	c.JSON(http.StatusOK, entries)
}

// createCronEntry creates a new scheduled command.
//
//	POST /api/cron
func (a *API) createCronEntry(c *gin.Context) {
	var entry cron.CronCommand
	if err := c.ShouldBindJSON(&entry); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if entry.Schedule == "" || entry.Command == "" || entry.UserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "schedule, command, and user_id are required"})
		return
	}
	if err := cron.CreateEntry(a.DB, entry); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"status": "created"})
}

// deleteCronEntry removes a scheduled command.
//
//	DELETE /api/cron/:id
func (a *API) deleteCronEntry(c *gin.Context) {
	id := c.Param("id")
	if err := cron.DeleteEntry(a.DB, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// toggleCronEntry enables or disables a scheduled command.
//
//	PATCH /api/cron/:id  { "enabled": true/false }
func (a *API) toggleCronEntry(c *gin.Context) {
	id := c.Param("id")
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := cron.ToggleEntry(a.DB, id, body.Enabled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated", "enabled": body.Enabled})
}
