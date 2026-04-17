package api

import (
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/NubeDev/bizzy/pkg/auth"
	"github.com/NubeDev/bizzy/pkg/models"
	"github.com/NubeDev/bizzy/pkg/revision"
	"github.com/gin-gonic/gin"
)

// listRevisions returns revision history for an entity.
// GET /api/my/apps/:id/revisions/:type/:entityName
func (a *API) listRevisions(c *gin.Context) {
	if a.Revisions == nil {
		c.JSON(http.StatusOK, []any{})
		return
	}
	user := auth.GetUser(c)
	id := c.Param("id")

	var app models.StoreApp
	if err := a.DB.First(&app, "id = ? AND author_id = ?", id, user.ID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "app not found"})
		return
	}

	entityID := revision.EntityKey(id, c.Param("entityName"))
	revs, err := a.Revisions.List(c.Param("type"), entityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, revs)
}

// revertRevision restores an entity to a previous revision.
// POST /api/my/apps/:id/revisions/:type/:entityName/revert/:rev
func (a *API) revertRevision(c *gin.Context) {
	if a.Revisions == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "revision history not enabled"})
		return
	}
	user := auth.GetUser(c)
	id := c.Param("id")
	entityType := c.Param("type")
	entityName := c.Param("entityName")
	revNum, err := strconv.Atoi(c.Param("rev"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid revision number"})
		return
	}

	var app models.StoreApp
	if err := a.DB.First(&app, "id = ? AND author_id = ?", id, user.ID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "app not found"})
		return
	}

	entityID := revision.EntityKey(id, entityName)

	switch entityType {
	case "tool":
		var oldTool models.StoreTool
		if err := a.Revisions.GetData(entityType, entityID, revNum, &oldTool); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "revision not found"})
			return
		}
		// Snapshot current before reverting.
		for _, t := range app.Tools {
			if t.Name == entityName {
				_ = a.Revisions.Save("tool", entityID, user.ID, "before revert to r"+strconv.Itoa(revNum), t)
				break
			}
		}
		found := false
		for i, t := range app.Tools {
			if t.Name == entityName {
				oldTool.Name = entityName
				app.Tools[i] = oldTool
				found = true
				break
			}
		}
		if !found {
			c.JSON(http.StatusNotFound, gin.H{"error": "tool not found"})
			return
		}
		app.UpdatedAt = time.Now().UTC()
		a.DB.Save(&app)
		writeToolDisk(oldTool, filepath.Join(a.AppRegistry.AppsDir(), app.Name))
		a.AppRegistry.Reload()
		a.MCPFactory.Rebuild()
		c.JSON(http.StatusOK, oldTool)

	case "prompt":
		var oldPrompt models.StorePrompt
		if err := a.Revisions.GetData(entityType, entityID, revNum, &oldPrompt); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "revision not found"})
			return
		}
		for _, p := range app.Prompts {
			if p.Name == entityName {
				_ = a.Revisions.Save("prompt", entityID, user.ID, "before revert to r"+strconv.Itoa(revNum), p)
				break
			}
		}
		found := false
		for i, p := range app.Prompts {
			if p.Name == entityName {
				oldPrompt.Name = entityName
				app.Prompts[i] = oldPrompt
				found = true
				break
			}
		}
		if !found {
			c.JSON(http.StatusNotFound, gin.H{"error": "prompt not found"})
			return
		}
		app.UpdatedAt = time.Now().UTC()
		a.DB.Save(&app)
		writePromptDisk(oldPrompt, filepath.Join(a.AppRegistry.AppsDir(), app.Name))
		a.AppRegistry.Reload()
		a.MCPFactory.Rebuild()
		c.JSON(http.StatusOK, oldPrompt)

	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported entity type: " + entityType})
	}
}
