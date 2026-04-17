package api

import (
	"net/http"
	"time"

	"github.com/NubeDev/bizzy/pkg/auth"
	"github.com/NubeDev/bizzy/pkg/models"
	"github.com/gin-gonic/gin"
)

// listApps returns all available apps from the registry.
func (a *API) listApps(c *gin.Context) {
	c.JSON(http.StatusOK, a.AppRegistry.List())
}

// getApp returns details for a specific app.
func (a *API) getApp(c *gin.Context) {
	app, ok := a.AppRegistry.Get(c.Param("id"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "app not found"})
		return
	}
	// Include prompts in the response.
	prompts := a.AppRegistry.GetPrompts(app.Name)
	c.JSON(http.StatusOK, gin.H{
		"app":     app,
		"prompts": prompts,
	})
}

// installApp installs an app for the current user.
func (a *API) installApp(c *gin.Context) {
	appName := c.Param("id")
	user := auth.GetUser(c)

	app, ok := a.AppRegistry.Get(appName)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "app not found"})
		return
	}

	// Check if already installed.
	var existing models.AppInstall
	result := a.DB.Where("user_id = ? AND app_name = ?", user.ID, appName).First(&existing)
	if result.Error == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "app already installed", "installId": existing.ID})
		return
	}

	// Parse user-provided settings.
	var req struct {
		Settings map[string]string `json:"settings"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		// Allow empty body for apps with no settings.
		req.Settings = make(map[string]string)
	}

	// Validate required settings.
	settings := make(map[string]string)
	secrets := make(map[string]string)
	for _, def := range app.Settings {
		val := req.Settings[def.Key]
		if val == "" {
			val = def.Default
		}
		if val == "" && def.Required {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing required setting: " + def.Key})
			return
		}
		if def.Type == "secret" {
			secrets[def.Key] = val
		} else {
			settings[def.Key] = val
		}
	}

	install := models.AppInstall{
		ID:          models.GenerateID("inst-"),
		AppName:     appName,
		AppVersion:  app.Version,
		WorkspaceID: user.WorkspaceID,
		UserID:      user.ID,
		Enabled:     true,
		Settings:    settings,
		Secrets:     secrets,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := a.DB.Create(&install).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, install)
}

// listInstalls returns the current user's installed apps.
func (a *API) listInstalls(c *gin.Context) {
	user := auth.GetUser(c)
	var installs []models.AppInstall
	a.DB.Where("user_id = ?", user.ID).Find(&installs)
	// Check for stale versions.
	for i, inst := range installs {
		app, ok := a.AppRegistry.Get(inst.AppName)
		if ok && app.Version != inst.AppVersion {
			installs[i].Stale = true
		}
	}
	c.JSON(http.StatusOK, installs)
}

// updateInstall enables/disables or updates settings for an install.
func (a *API) updateInstall(c *gin.Context) {
	user := auth.GetUser(c)
	var install models.AppInstall
	if err := a.DB.First(&install, "id = ? AND user_id = ?", c.Param("id"), user.ID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "install not found"})
		return
	}

	var req struct {
		Enabled  *bool             `json:"enabled"`
		Settings map[string]string `json:"settings"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Enabled != nil {
		install.Enabled = *req.Enabled
	}
	if req.Settings != nil {
		app, ok := a.AppRegistry.Get(install.AppName)
		if ok {
			for _, def := range app.Settings {
				if val, exists := req.Settings[def.Key]; exists {
					if def.Type == "secret" {
						install.Secrets[def.Key] = val
					} else {
						install.Settings[def.Key] = val
					}
				}
			}
		}
		// Update version to current on settings change.
		if ok {
			install.AppVersion = app.Version
			install.Stale = false
		}
	}
	install.UpdatedAt = time.Now().UTC()

	if err := a.DB.Save(&install).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, install)
}

// uninstallApp removes an app install.
func (a *API) uninstallApp(c *gin.Context) {
	user := auth.GetUser(c)
	var install models.AppInstall
	if err := a.DB.First(&install, "id = ? AND user_id = ?", c.Param("id"), user.ID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "install not found"})
		return
	}
	if err := a.DB.Delete(&install).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": install.ID})
}

// reloadApps rescans the apps directory (admin only).
func (a *API) reloadApps(c *gin.Context) {
	if err := a.AppRegistry.Reload(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"reloaded": len(a.AppRegistry.List()),
		"apps":     a.AppRegistry.List(),
	})
}
