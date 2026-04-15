package api

import (
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/NubeDev/bizzy/pkg/auth"
	"github.com/NubeDev/bizzy/pkg/models"
	"github.com/gin-gonic/gin"
)

var validStoreName = regexp.MustCompile(`^[a-z][a-z0-9-]{1,48}[a-z0-9]$`)

// --- Store: public catalog ---

func (a *API) listStoreApps(c *gin.Context) {
	q := strings.ToLower(c.Query("q"))
	category := c.Query("category")
	sortBy := c.DefaultQuery("sort", "popular")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}

	all := a.StoreApps.All()
	var results []models.StoreApp

	for _, app := range all {
		if category != "" && app.Category != category {
			continue
		}
		if q != "" && !matchesQuery(app, q) {
			continue
		}
		results = append(results, app)
	}

	sortStoreApps(results, sortBy)

	total := len(results)
	start := (page - 1) * limit
	if start >= total {
		c.JSON(http.StatusOK, gin.H{"apps": []any{}, "total": total, "page": page, "limit": limit})
		return
	}
	end := start + limit
	if end > total {
		end = total
	}

	type appSummary struct {
		ID           string     `json:"id"`
		Name         string     `json:"name"`
		DisplayName  string     `json:"displayName"`
		Description  string     `json:"description"`
		Version      string     `json:"version"`
		Icon         string     `json:"icon"`
		Color        string     `json:"color"`
		Category     string     `json:"category"`
		Tags         []string   `json:"tags"`
		AuthorName   string     `json:"authorName"`
		InstallCount int        `json:"installCount"`
		AvgRating    float64    `json:"avgRating"`
		ReviewCount  int        `json:"reviewCount"`
		ToolCount    int        `json:"toolCount"`
		PromptCount  int        `json:"promptCount"`
		PublishedAt  *time.Time `json:"publishedAt,omitempty"`
	}

	summaries := make([]appSummary, 0, end-start)
	for _, app := range results[start:end] {
		summaries = append(summaries, appSummary{
			ID:           app.ID,
			Name:         app.Name,
			DisplayName:  app.DisplayName,
			Description:  app.Description,
			Version:      app.Version,
			Icon:         app.Icon,
			Color:        app.Color,
			Category:     app.Category,
			Tags:         app.Tags,
			AuthorName:   app.AuthorName,
			InstallCount: app.InstallCount,
			AvgRating:    app.AvgRating,
			ReviewCount:  app.ReviewCount,
			ToolCount:    len(app.Tools),
			PromptCount:  len(app.Prompts),
			PublishedAt:  app.PublishedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"apps":  summaries,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

func (a *API) getStoreApp(c *gin.Context) {
	id := c.Param("id")
	app, ok := a.StoreApps.Get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "app not found"})
		return
	}

	user := auth.GetUser(c)
	switch app.Visibility {
	case models.VisibilityPublic, models.VisibilityUnlisted:
		// allow
	case models.VisibilityPrivate:
		if app.AuthorID != user.ID {
			c.JSON(http.StatusNotFound, gin.H{"error": "app not found"})
			return
		}
	case models.VisibilityShared:
		if app.AuthorID != user.ID {
			invite := c.Query("invite")
			hasAccess := false
			if invite != "" {
				_, found := a.AppShares.FindOne(func(s models.AppShare) bool {
					return s.AppID == id && s.Token == invite && (s.ExpiresAt == nil || s.ExpiresAt.After(time.Now()))
				})
				hasAccess = found
			}
			if !hasAccess {
				_, found := a.AppShares.FindOne(func(s models.AppShare) bool {
					return s.AppID == id && s.InviteeID == user.ID && (s.ExpiresAt == nil || s.ExpiresAt.After(time.Now()))
				})
				hasAccess = found
			}
			if !hasAccess {
				c.JSON(http.StatusNotFound, gin.H{"error": "app not found"})
				return
			}
		}
	}

	installed := false
	installID := ""
	installs := a.AppInstalls.FindFunc(func(ai models.AppInstall) bool {
		return ai.UserID == user.ID && ai.AppName == app.Name
	})
	if len(installs) > 0 {
		installed = true
		installID = installs[0].ID
	}

	c.JSON(http.StatusOK, gin.H{
		"app":       app,
		"installed": installed,
		"installId": installID,
	})
}

func (a *API) listCategories(c *gin.Context) {
	c.JSON(http.StatusOK, models.Categories)
}

// --- Store: install ---

func (a *API) installStoreApp(c *gin.Context) {
	id := c.Param("id")
	user := auth.GetUser(c)

	app, ok := a.StoreApps.Get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "app not found"})
		return
	}

	existing := a.AppInstalls.FindFunc(func(ai models.AppInstall) bool {
		return ai.UserID == user.ID && ai.AppName == app.Name
	})
	if len(existing) > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "app already installed", "installId": existing[0].ID})
		return
	}

	var req struct {
		Settings map[string]string `json:"settings"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Settings = make(map[string]string)
	}

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
		AppName:     app.Name,
		AppVersion:  app.Version,
		WorkspaceID: user.WorkspaceID,
		UserID:      user.ID,
		Enabled:     true,
		Settings:    settings,
		Secrets:     secrets,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := a.AppInstalls.Create(install); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	app.InstallCount++
	app.ActiveInstalls++
	a.StoreApps.Update(app)

	c.JSON(http.StatusCreated, install)
}

// --- Reviews ---

func (a *API) listStoreAppReviews(c *gin.Context) {
	appID := c.Param("id")
	reviews := a.AppReviews.FindFunc(func(r models.AppReview) bool {
		return r.AppID == appID
	})
	c.JSON(http.StatusOK, reviews)
}

func (a *API) submitReview(c *gin.Context) {
	appID := c.Param("id")
	user := auth.GetUser(c)

	app, ok := a.StoreApps.Get(appID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "app not found"})
		return
	}
	if app.AuthorID == user.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot review your own app"})
		return
	}

	var req struct {
		Rating  int    `json:"rating" binding:"required"`
		Comment string `json:"comment"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Rating < 1 || req.Rating > 5 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rating must be 1-5"})
		return
	}

	existing, found := a.AppReviews.FindOne(func(r models.AppReview) bool {
		return r.AppID == appID && r.UserID == user.ID
	})

	now := time.Now().UTC()
	if found {
		existing.Rating = req.Rating
		existing.Comment = req.Comment
		existing.UpdatedAt = now
		a.AppReviews.Update(existing)
	} else {
		review := models.AppReview{
			ID:        models.GenerateID("rev-"),
			AppID:     appID,
			UserID:    user.ID,
			UserName:  user.Name,
			Rating:    req.Rating,
			Comment:   req.Comment,
			CreatedAt: now,
			UpdatedAt: now,
		}
		a.AppReviews.Create(review)
	}

	a.recalcRating(appID)

	if found {
		c.JSON(http.StatusOK, existing)
	} else {
		c.JSON(http.StatusCreated, gin.H{"status": "review submitted"})
	}
}

func (a *API) deleteReview(c *gin.Context) {
	appID := c.Param("id")
	user := auth.GetUser(c)
	review, found := a.AppReviews.FindOne(func(r models.AppReview) bool {
		return r.AppID == appID && r.UserID == user.ID
	})
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "review not found"})
		return
	}
	a.AppReviews.Delete(review.ID)
	a.recalcRating(appID)
	c.JSON(http.StatusOK, gin.H{"deleted": review.ID})
}

func (a *API) recalcRating(appID string) {
	reviews := a.AppReviews.FindFunc(func(r models.AppReview) bool {
		return r.AppID == appID
	})
	app, ok := a.StoreApps.Get(appID)
	if !ok {
		return
	}
	app.ReviewCount = len(reviews)
	if len(reviews) == 0 {
		app.AvgRating = 0
	} else {
		total := 0
		for _, r := range reviews {
			total += r.Rating
		}
		app.AvgRating = float64(total) / float64(len(reviews))
	}
	a.StoreApps.Update(app)
}

// --- My Apps: author CRUD ---

func (a *API) listMyStoreApps(c *gin.Context) {
	user := auth.GetUser(c)
	apps := a.StoreApps.FindFunc(func(app models.StoreApp) bool {
		return app.AuthorID == user.ID
	})
	c.JSON(http.StatusOK, apps)
}

func (a *API) createStoreApp(c *gin.Context) {
	user := auth.GetUser(c)

	var req struct {
		Name        string `json:"name" binding:"required"`
		DisplayName string `json:"displayName"`
		Description string `json:"description"`
		Category    string `json:"category"`
		Icon        string `json:"icon"`
		Color       string `json:"color"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	if !validStoreName.MatchString(req.Name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid name: must be lowercase, 3-50 chars, letters/numbers/hyphens only"})
		return
	}
	_, found := a.StoreApps.FindOne(func(app models.StoreApp) bool { return app.Name == req.Name })
	if found {
		c.JSON(http.StatusConflict, gin.H{"error": "store app already exists: " + req.Name})
		return
	}
	if req.DisplayName == "" {
		req.DisplayName = req.Name
	}

	now := time.Now().UTC()
	app := models.StoreApp{
		ID:          models.GenerateID("app-"),
		Name:        req.Name,
		DisplayName: req.DisplayName,
		Description: req.Description,
		Version:     "1.0.0",
		Icon:        req.Icon,
		Color:       req.Color,
		Category:    req.Category,
		Tags:        []string{},
		AuthorID:    user.ID,
		AuthorName:  user.Name,
		WorkspaceID: user.WorkspaceID,
		Visibility:  models.VisibilityPrivate,
		Permissions: models.Permissions{AllowedHosts: []string{}, DefaultToolClass: "read-only"},
		Settings:    []models.SettingDef{},
		Tools:       []models.StoreTool{},
		Prompts:     []models.StorePrompt{},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := a.StoreApps.Create(app); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Write disk files and reload registry.
	if err := WriteStoreAppToDisk(app, a.AppRegistry.AppsDir()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write app to disk: " + err.Error()})
		return
	}
	a.AppRegistry.Reload()
	a.MCPFactory.Rebuild()

	// Auto-install for the author so tools are immediately testable via MCP.
	a.AppInstalls.Create(models.AppInstall{
		ID:          models.GenerateID("inst-"),
		AppName:     app.Name,
		AppVersion:  app.Version,
		WorkspaceID: user.WorkspaceID,
		UserID:      user.ID,
		Enabled:     true,
		Settings:    map[string]string{},
		Secrets:     map[string]string{},
		CreatedAt:   now,
		UpdatedAt:   now,
	})

	c.JSON(http.StatusCreated, app)
}

func (a *API) getMyStoreApp(c *gin.Context) {
	user := auth.GetUser(c)
	id := c.Param("id")
	app, ok := a.StoreApps.Get(id)
	if !ok || app.AuthorID != user.ID {
		c.JSON(http.StatusNotFound, gin.H{"error": "app not found"})
		return
	}
	c.JSON(http.StatusOK, app)
}

func (a *API) updateStoreApp(c *gin.Context) {
	user := auth.GetUser(c)
	id := c.Param("id")
	app, ok := a.StoreApps.Get(id)
	if !ok || app.AuthorID != user.ID {
		c.JSON(http.StatusNotFound, gin.H{"error": "app not found"})
		return
	}

	var req struct {
		DisplayName *string             `json:"displayName"`
		Description *string             `json:"description"`
		LongDesc    *string             `json:"longDescription"`
		Version     *string             `json:"version"`
		Icon        *string             `json:"icon"`
		Color       *string             `json:"color"`
		Category    *string             `json:"category"`
		Tags        []string            `json:"tags"`
		Permissions *models.Permissions `json:"permissions"`
		Settings    []models.SettingDef `json:"settings"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.DisplayName != nil { app.DisplayName = *req.DisplayName }
	if req.Description != nil { app.Description = *req.Description }
	if req.LongDesc != nil { app.LongDesc = *req.LongDesc }
	if req.Version != nil { app.Version = *req.Version }
	if req.Icon != nil { app.Icon = *req.Icon }
	if req.Color != nil { app.Color = *req.Color }
	if req.Category != nil { app.Category = *req.Category }
	if req.Tags != nil { app.Tags = req.Tags }
	if req.Permissions != nil { app.Permissions = *req.Permissions }
	if req.Settings != nil { app.Settings = req.Settings }
	app.UpdatedAt = time.Now().UTC()

	if err := a.StoreApps.Update(app); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Re-write disk files and reload.
	WriteStoreAppToDisk(app, a.AppRegistry.AppsDir())
	a.AppRegistry.Reload()
	a.MCPFactory.Rebuild()

	c.JSON(http.StatusOK, app)
}

func (a *API) deleteStoreApp(c *gin.Context) {
	user := auth.GetUser(c)
	id := c.Param("id")
	app, ok := a.StoreApps.Get(id)
	if !ok || app.AuthorID != user.ID {
		c.JSON(http.StatusNotFound, gin.H{"error": "app not found"})
		return
	}
	if err := a.StoreApps.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	reviews := a.AppReviews.FindFunc(func(r models.AppReview) bool { return r.AppID == id })
	for _, r := range reviews {
		a.AppReviews.Delete(r.ID)
	}

	// Remove disk directory and reload.
	os.RemoveAll(filepath.Join(a.AppRegistry.AppsDir(), app.Name))
	a.AppRegistry.Reload()
	a.MCPFactory.Rebuild()

	c.JSON(http.StatusOK, gin.H{"deleted": id})
}

func (a *API) publishStoreApp(c *gin.Context) {
	user := auth.GetUser(c)
	id := c.Param("id")
	app, ok := a.StoreApps.Get(id)
	if !ok || app.AuthorID != user.ID {
		c.JSON(http.StatusNotFound, gin.H{"error": "app not found"})
		return
	}

	var errors []string
	if app.DisplayName == "" { errors = append(errors, "displayName is required") }
	if len(app.Description) < 20 { errors = append(errors, "description must be at least 20 characters") }
	if app.Category == "" || !models.ValidCategory(app.Category) { errors = append(errors, "valid category is required") }
	if len(app.Tools) == 0 && len(app.Prompts) == 0 { errors = append(errors, "at least one tool or prompt is required") }
	for _, host := range app.Permissions.AllowedHosts {
		if isPrivateHost(host) {
			errors = append(errors, "allowedHosts cannot contain localhost or private IPs: "+host)
		}
	}
	if len(errors) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"errors": errors})
		return
	}

	now := time.Now().UTC()
	app.Visibility = models.VisibilityPublic
	if app.PublishedAt == nil { app.PublishedAt = &now }
	app.UpdatedAt = now
	a.StoreApps.Update(app)
	c.JSON(http.StatusOK, app)
}

func (a *API) setStoreAppVisibility(c *gin.Context) {
	user := auth.GetUser(c)
	id := c.Param("id")
	app, ok := a.StoreApps.Get(id)
	if !ok || app.AuthorID != user.ID {
		c.JSON(http.StatusNotFound, gin.H{"error": "app not found"})
		return
	}
	var req struct {
		Visibility models.Visibility `json:"visibility" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	app.Visibility = req.Visibility
	app.UpdatedAt = time.Now().UTC()
	a.StoreApps.Update(app)
	c.JSON(http.StatusOK, app)
}

// --- Tool CRUD within a store app ---

func (a *API) addStoreTool(c *gin.Context) {
	user := auth.GetUser(c); id := c.Param("id")
	app, ok := a.StoreApps.Get(id)
	if !ok || app.AuthorID != user.ID { c.JSON(http.StatusNotFound, gin.H{"error": "app not found"}); return }
	var tool models.StoreTool
	if err := c.ShouldBindJSON(&tool); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return }
	for _, t := range app.Tools { if t.Name == tool.Name { c.JSON(http.StatusConflict, gin.H{"error": "tool already exists: " + tool.Name}); return } }
	app.Tools = append(app.Tools, tool)
	app.UpdatedAt = time.Now().UTC()
	a.StoreApps.Update(app)
	writeToolDisk(tool, filepath.Join(a.AppRegistry.AppsDir(), app.Name))
	a.AppRegistry.Reload()
	a.MCPFactory.Rebuild()
	c.JSON(http.StatusCreated, tool)
}

func (a *API) updateStoreTool(c *gin.Context) {
	user := auth.GetUser(c); id := c.Param("id"); toolName := c.Param("name")
	app, ok := a.StoreApps.Get(id)
	if !ok || app.AuthorID != user.ID { c.JSON(http.StatusNotFound, gin.H{"error": "app not found"}); return }
	var tool models.StoreTool
	if err := c.ShouldBindJSON(&tool); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return }
	found := false
	for i, t := range app.Tools { if t.Name == toolName { tool.Name = toolName; app.Tools[i] = tool; found = true; break } }
	if !found { c.JSON(http.StatusNotFound, gin.H{"error": "tool not found: " + toolName}); return }
	app.UpdatedAt = time.Now().UTC()
	a.StoreApps.Update(app)
	writeToolDisk(tool, filepath.Join(a.AppRegistry.AppsDir(), app.Name))
	a.AppRegistry.Reload()
	a.MCPFactory.Rebuild()
	c.JSON(http.StatusOK, tool)
}

func (a *API) deleteStoreTool(c *gin.Context) {
	user := auth.GetUser(c); id := c.Param("id"); toolName := c.Param("name")
	app, ok := a.StoreApps.Get(id)
	if !ok || app.AuthorID != user.ID { c.JSON(http.StatusNotFound, gin.H{"error": "app not found"}); return }
	found := false
	for i, t := range app.Tools { if t.Name == toolName { app.Tools = append(app.Tools[:i], app.Tools[i+1:]...); found = true; break } }
	if !found { c.JSON(http.StatusNotFound, gin.H{"error": "tool not found: " + toolName}); return }
	app.UpdatedAt = time.Now().UTC()
	a.StoreApps.Update(app)
	deleteToolDisk(toolName, filepath.Join(a.AppRegistry.AppsDir(), app.Name))
	a.AppRegistry.Reload()
	a.MCPFactory.Rebuild()
	c.JSON(http.StatusOK, gin.H{"deleted": toolName})
}

// --- Prompt CRUD within a store app ---

func (a *API) addStorePrompt(c *gin.Context) {
	user := auth.GetUser(c); id := c.Param("id")
	app, ok := a.StoreApps.Get(id)
	if !ok || app.AuthorID != user.ID { c.JSON(http.StatusNotFound, gin.H{"error": "app not found"}); return }
	var prompt models.StorePrompt
	if err := c.ShouldBindJSON(&prompt); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return }
	for _, p := range app.Prompts { if p.Name == prompt.Name { c.JSON(http.StatusConflict, gin.H{"error": "prompt already exists: " + prompt.Name}); return } }
	app.Prompts = append(app.Prompts, prompt)
	app.UpdatedAt = time.Now().UTC()
	a.StoreApps.Update(app)
	writePromptDisk(prompt, filepath.Join(a.AppRegistry.AppsDir(), app.Name))
	a.AppRegistry.Reload()
	a.MCPFactory.Rebuild()
	c.JSON(http.StatusCreated, prompt)
}

func (a *API) updateStorePrompt(c *gin.Context) {
	user := auth.GetUser(c); id := c.Param("id"); promptName := c.Param("name")
	app, ok := a.StoreApps.Get(id)
	if !ok || app.AuthorID != user.ID { c.JSON(http.StatusNotFound, gin.H{"error": "app not found"}); return }
	var prompt models.StorePrompt
	if err := c.ShouldBindJSON(&prompt); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return }
	found := false
	for i, p := range app.Prompts { if p.Name == promptName { prompt.Name = promptName; app.Prompts[i] = prompt; found = true; break } }
	if !found { c.JSON(http.StatusNotFound, gin.H{"error": "prompt not found: " + promptName}); return }
	app.UpdatedAt = time.Now().UTC()
	a.StoreApps.Update(app)
	writePromptDisk(prompt, filepath.Join(a.AppRegistry.AppsDir(), app.Name))
	a.AppRegistry.Reload()
	a.MCPFactory.Rebuild()
	c.JSON(http.StatusOK, prompt)
}

func (a *API) deleteStorePrompt(c *gin.Context) {
	user := auth.GetUser(c); id := c.Param("id"); promptName := c.Param("name")
	app, ok := a.StoreApps.Get(id)
	if !ok || app.AuthorID != user.ID { c.JSON(http.StatusNotFound, gin.H{"error": "app not found"}); return }
	found := false
	for i, p := range app.Prompts { if p.Name == promptName { app.Prompts = append(app.Prompts[:i], app.Prompts[i+1:]...); found = true; break } }
	if !found { c.JSON(http.StatusNotFound, gin.H{"error": "prompt not found: " + promptName}); return }
	app.UpdatedAt = time.Now().UTC()
	a.StoreApps.Update(app)
	deletePromptDisk(promptName, filepath.Join(a.AppRegistry.AppsDir(), app.Name))
	a.AppRegistry.Reload()
	a.MCPFactory.Rebuild()
	c.JSON(http.StatusOK, gin.H{"deleted": promptName})
}

// --- Sharing ---

func (a *API) shareStoreApp(c *gin.Context) {
	user := auth.GetUser(c); id := c.Param("id")
	app, ok := a.StoreApps.Get(id)
	if !ok || app.AuthorID != user.ID { c.JSON(http.StatusNotFound, gin.H{"error": "app not found"}); return }
	var req struct { UserID string `json:"userId" binding:"required"` }
	if err := c.ShouldBindJSON(&req); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": "userId is required"}); return }
	if _, exists := a.Users.Get(req.UserID); !exists { c.JSON(http.StatusNotFound, gin.H{"error": "user not found: " + req.UserID}); return }
	_, found := a.AppShares.FindOne(func(s models.AppShare) bool { return s.AppID == id && s.InviteeID == req.UserID })
	if found { c.JSON(http.StatusConflict, gin.H{"error": "already shared with this user"}); return }
	share := models.AppShare{ID: models.GenerateID("share-"), AppID: id, InvitedBy: user.ID, InviteeID: req.UserID, CreatedAt: time.Now().UTC()}
	if err := a.AppShares.Create(share); err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return }
	c.JSON(http.StatusCreated, share)
}

func (a *API) shareStoreAppLink(c *gin.Context) {
	user := auth.GetUser(c); id := c.Param("id")
	app, ok := a.StoreApps.Get(id)
	if !ok || app.AuthorID != user.ID { c.JSON(http.StatusNotFound, gin.H{"error": "app not found"}); return }
	token := models.GenerateToken()
	share := models.AppShare{ID: models.GenerateID("share-"), AppID: id, InvitedBy: user.ID, Token: token, CreatedAt: time.Now().UTC()}
	if err := a.AppShares.Create(share); err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return }
	c.JSON(http.StatusCreated, gin.H{"share": share, "link": "/api/store/apps/" + app.ID + "?invite=" + token})
}

func (a *API) listStoreAppShares(c *gin.Context) {
	user := auth.GetUser(c); id := c.Param("id")
	app, ok := a.StoreApps.Get(id)
	if !ok || app.AuthorID != user.ID { c.JSON(http.StatusNotFound, gin.H{"error": "app not found"}); return }
	shares := a.AppShares.FindFunc(func(s models.AppShare) bool { return s.AppID == id })
	c.JSON(http.StatusOK, shares)
}

func (a *API) deleteStoreAppShare(c *gin.Context) {
	user := auth.GetUser(c); id := c.Param("id"); shareID := c.Param("shareId")
	app, ok := a.StoreApps.Get(id)
	if !ok || app.AuthorID != user.ID { c.JSON(http.StatusNotFound, gin.H{"error": "app not found"}); return }
	share, ok := a.AppShares.Get(shareID)
	if !ok || share.AppID != id { c.JSON(http.StatusNotFound, gin.H{"error": "share not found"}); return }
	a.AppShares.Delete(shareID)
	c.JSON(http.StatusOK, gin.H{"deleted": shareID})
}

// --- Helpers ---

func matchesQuery(app models.StoreApp, q string) bool {
	if strings.Contains(strings.ToLower(app.Name), q) { return true }
	if strings.Contains(strings.ToLower(app.DisplayName), q) { return true }
	if strings.Contains(strings.ToLower(app.Description), q) { return true }
	for _, tag := range app.Tags { if strings.Contains(strings.ToLower(tag), q) { return true } }
	return false
}

func isPrivateHost(host string) bool {
	h := host
	if idx := strings.LastIndex(h, ":"); idx != -1 { h = h[:idx] }
	h = strings.TrimSpace(strings.ToLower(h))
	if h == "localhost" || h == "127.0.0.1" || h == "::1" || h == "0.0.0.0" { return true }
	ip := net.ParseIP(h)
	if ip == nil { return false }
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified()
}

func sortStoreApps(apps []models.StoreApp, sortBy string) {
	switch sortBy {
	case "recent":
		sort.Slice(apps, func(i, j int) bool {
			if apps[i].PublishedAt == nil { return false }
			if apps[j].PublishedAt == nil { return true }
			return apps[i].PublishedAt.After(*apps[j].PublishedAt)
		})
	case "rating":
		sort.Slice(apps, func(i, j int) bool { return apps[i].AvgRating > apps[j].AvgRating })
	case "name":
		sort.Slice(apps, func(i, j int) bool { return apps[i].DisplayName < apps[j].DisplayName })
	default:
		sort.Slice(apps, func(i, j int) bool { return apps[i].InstallCount > apps[j].InstallCount })
	}
}
