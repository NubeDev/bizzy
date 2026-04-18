package api

// ZIP-based upload endpoints for apps and plugins.
//
// App ZIP format:
//
//	app.yaml               — name, displayName, description, version, category, tags, icon, color
//	tools/
//	  <tool>.json          — { name, description, toolClass, mode, params }
//	  <tool>.js            — script body
//	prompts/
//	  <prompt>.md          — YAML front-matter (name, description, arguments) + markdown body
//
// Plugin ZIP format:
//
//	plugin.yaml            — full plugin manifest (name, version, services, tools, prompts, …)

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/NubeDev/bizzy/pkg/auth"
	"github.com/NubeDev/bizzy/pkg/models"
	"github.com/NubeDev/bizzy/pkg/plugin"
	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

// ──────────────────────────────────────────────────────────────────────────────
// App upload
// ──────────────────────────────────────────────────────────────────────────────

// uploadStoreApp accepts a multipart ZIP, parses it, and creates (or replaces)
// a StoreApp record in the database.
//
//	POST /api/my/apps/upload
func (a *API) uploadStoreApp(c *gin.Context) {
	user := auth.GetUser(c)

	zr, err := openUploadZip(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// --- Parse zip contents ---
	files, err := zipContents(zr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Locate app.yaml.
	appYAML, ok := files["app.yaml"]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "zip must contain app.yaml at root (or inside a single top-level directory)"})
		return
	}

	var appMeta struct {
		Name        string              `yaml:"name"`
		DisplayName string              `yaml:"displayName"`
		Description string              `yaml:"description"`
		LongDesc    string              `yaml:"longDescription"`
		Version     string              `yaml:"version"`
		Icon        string              `yaml:"icon"`
		Color       string              `yaml:"color"`
		Category    string              `yaml:"category"`
		Tags        []string            `yaml:"tags"`
		Permissions models.Permissions  `yaml:"permissions"`
		Settings    []models.SettingDef `yaml:"settings"`
	}
	if err := yaml.Unmarshal(appYAML, &appMeta); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "parse app.yaml: " + err.Error()})
		return
	}
	if appMeta.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "app.yaml: name is required"})
		return
	}
	if !validStoreName.MatchString(appMeta.Name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "app.yaml: name must be lowercase, 3-50 chars, letters/numbers/hyphens only"})
		return
	}

	// Defaults.
	if appMeta.Version == "" {
		appMeta.Version = "1.0.0"
	}
	if appMeta.DisplayName == "" {
		appMeta.DisplayName = appMeta.Name
	}
	if appMeta.Tags == nil {
		appMeta.Tags = []string{}
	}
	if appMeta.Settings == nil {
		appMeta.Settings = []models.SettingDef{}
	}

	// --- Parse tools ---
	tools, err := parseZipTools(files)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tools: " + err.Error()})
		return
	}

	// --- Parse prompts ---
	prompts, err := parseZipPrompts(files)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "prompts: " + err.Error()})
		return
	}

	now := time.Now().UTC()
	overwrite := c.Query("overwrite") == "true"

	// Check for existing app.
	var existing models.StoreApp
	exists := a.DB.Where("name = ?", appMeta.Name).First(&existing).Error == nil

	if exists && !overwrite {
		c.JSON(http.StatusConflict, gin.H{
			"error": "app already exists: " + appMeta.Name + " (add ?overwrite=true to replace)",
		})
		return
	}

	if exists && overwrite {
		existing.DisplayName = appMeta.DisplayName
		existing.Description = appMeta.Description
		existing.LongDesc = appMeta.LongDesc
		existing.Version = appMeta.Version
		existing.Icon = appMeta.Icon
		existing.Color = appMeta.Color
		existing.Category = appMeta.Category
		existing.Tags = appMeta.Tags
		existing.Permissions = appMeta.Permissions
		existing.Settings = appMeta.Settings
		existing.Tools = tools
		existing.Prompts = prompts
		existing.UpdatedAt = now

		if err := a.DB.Save(&existing).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		a.AppRegistry.Reload()
		a.MCPFactory.Rebuild()
		c.JSON(http.StatusOK, existing)
		return
	}

	// Create new.
	app := models.StoreApp{
		ID:          models.GenerateID("app-"),
		Name:        appMeta.Name,
		DisplayName: appMeta.DisplayName,
		Description: appMeta.Description,
		LongDesc:    appMeta.LongDesc,
		Version:     appMeta.Version,
		Icon:        appMeta.Icon,
		Color:       appMeta.Color,
		Category:    appMeta.Category,
		Tags:        appMeta.Tags,
		AuthorID:    user.ID,
		AuthorName:  user.Name,
		WorkspaceID: user.WorkspaceID,
		Visibility:  models.VisibilityPrivate,
		Permissions: appMeta.Permissions,
		Settings:    appMeta.Settings,
		Tools:       tools,
		Prompts:     prompts,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if len(app.Permissions.AllowedHosts) == 0 {
		app.Permissions.AllowedHosts = []string{}
	}
	if app.Permissions.DefaultToolClass == "" {
		app.Permissions.DefaultToolClass = "read-only"
	}

	if err := a.DB.Create(&app).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Auto-install for the uploader.
	a.DB.Create(&models.AppInstall{
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

	a.AppRegistry.Reload()
	a.MCPFactory.Rebuild()
	c.JSON(http.StatusCreated, app)
}

// ──────────────────────────────────────────────────────────────────────────────
// Plugin upload
// ──────────────────────────────────────────────────────────────────────────────

// uploadPlugin accepts a multipart ZIP containing a plugin.yaml manifest and
// pre-registers it as a Plugin DB record.
//
// The plugin process itself still needs to start and connect via NATS before it
// is active — this just stores the manifest so admins can manage and inspect it.
//
//	POST /api/plugins/upload
func (a *API) uploadPlugin(c *gin.Context) {
	zr, err := openUploadZip(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	files, err := zipContents(zr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	manifestData, ok := files["plugin.yaml"]
	if !ok {
		// Also accept plugin.json for convenience.
		manifestData, ok = files["plugin.json"]
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "zip must contain plugin.yaml (or plugin.json)"})
			return
		}
	}

	m, err := plugin.ParseManifest(manifestData)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid manifest: " + err.Error()})
		return
	}

	manifestJSON, err := plugin.MarshalManifest(m)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "marshal manifest: " + err.Error()})
		return
	}

	servicesJSON, _ := json.Marshal(m.Services)

	now := time.Now().UTC()
	overwrite := c.Query("overwrite") == "true"

	var existing models.Plugin
	exists := a.DB.First(&existing, "name = ?", m.Name).Error == nil

	if exists && !overwrite {
		c.JSON(http.StatusConflict, gin.H{
			"error": "plugin already exists: " + m.Name + " (add ?overwrite=true to replace)",
		})
		return
	}

	record := models.Plugin{
		Name:         m.Name,
		Version:      m.Version,
		Description:  m.Description,
		Services:     string(servicesJSON),
		Manifest:     string(manifestJSON),
		Status:       models.PluginStatusActive,
		RegisteredAt: now,
	}

	var dbErr error
	if exists {
		dbErr = a.DB.Save(&record).Error
	} else {
		dbErr = a.DB.Create(&record).Error
	}
	if dbErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": dbErr.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"name":     m.Name,
		"version":  m.Version,
		"services": m.Services,
		"status":   "registered",
		"note":     "plugin manifest stored; start the plugin process to activate it via NATS",
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// Shared helpers
// ──────────────────────────────────────────────────────────────────────────────

// openUploadZip reads the "file" form field and returns a zip.Reader.
func openUploadZip(c *gin.Context) (*zip.Reader, error) {
	fh, err := c.FormFile("file")
	if err != nil {
		return nil, fmt.Errorf("file field required (multipart/form-data)")
	}
	f, err := fh.Open()
	if err != nil {
		return nil, fmt.Errorf("open upload: %w", err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("read upload: %w", err)
	}
	return zip.NewReader(bytes.NewReader(data), int64(len(data)))
}

// zipContents reads all files in a zip into a map[relPath]contents.
// If the zip has a single top-level directory, it is stripped automatically.
func zipContents(zr *zip.Reader) (map[string][]byte, error) {
	// Detect strip prefix: if all files share a common top-level dir, strip it.
	prefix := detectStripPrefix(zr)

	out := make(map[string][]byte, len(zr.File))
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rel := filepath.ToSlash(f.Name)
		if prefix != "" {
			if !strings.HasPrefix(rel, prefix) {
				continue
			}
			rel = strings.TrimPrefix(rel, prefix)
		}
		if rel == "" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", f.Name, err)
		}
		out[rel] = data
	}
	return out, nil
}

// detectStripPrefix returns the common top-level directory prefix (e.g. "myapp/")
// if every file in the zip shares one, otherwise returns "".
func detectStripPrefix(zr *zip.Reader) string {
	var prefix string
	for _, f := range zr.File {
		rel := filepath.ToSlash(f.Name)
		parts := strings.SplitN(rel, "/", 2)
		if len(parts) < 2 {
			return "" // file at root level — no strip
		}
		if prefix == "" {
			prefix = parts[0] + "/"
		} else if parts[0]+"/" != prefix {
			return "" // inconsistent top-level dirs
		}
	}
	return prefix
}

// parseZipTools extracts StoreTool entries from tools/*.json + tools/*.js pairs.
func parseZipTools(files map[string][]byte) ([]models.StoreTool, error) {
	var tools []models.StoreTool

	for path, data := range files {
		if !strings.HasPrefix(path, "tools/") || !strings.HasSuffix(path, ".json") {
			continue
		}
		base := strings.TrimPrefix(path, "tools/")
		name := strings.TrimSuffix(base, ".json")

		var manifest struct {
			Name        string                     `json:"name"`
			Description string                     `json:"description"`
			ToolClass   string                     `json:"toolClass"`
			Mode        string                     `json:"mode"`
			Params      map[string]models.ToolParam `json:"params"`
		}
		if err := json.Unmarshal(data, &manifest); err != nil {
			return nil, fmt.Errorf("parse tools/%s.json: %w", name, err)
		}
		if manifest.Name == "" {
			manifest.Name = name
		}
		if manifest.Params == nil {
			manifest.Params = map[string]models.ToolParam{}
		}

		// Read matching .js file.
		jsKey := "tools/" + name + ".js"
		script := string(files[jsKey]) // empty string if not present

		tools = append(tools, models.StoreTool{
			Name:        manifest.Name,
			Description: manifest.Description,
			ToolClass:   manifest.ToolClass,
			Mode:        manifest.Mode,
			Params:      manifest.Params,
			Script:      script,
		})
	}

	if tools == nil {
		tools = []models.StoreTool{}
	}
	return tools, nil
}

// parseZipPrompts extracts StorePrompt entries from prompts/*.md files.
// Each file may have optional YAML front-matter (name, description, arguments).
func parseZipPrompts(files map[string][]byte) ([]models.StorePrompt, error) {
	var prompts []models.StorePrompt

	for path, data := range files {
		if !strings.HasPrefix(path, "prompts/") || !strings.HasSuffix(path, ".md") {
			continue
		}
		base := strings.TrimPrefix(path, "prompts/")
		fileName := strings.TrimSuffix(base, ".md")

		var meta struct {
			Name        string                  `yaml:"name"`
			Description string                  `yaml:"description"`
			Arguments   []models.PromptArgument `yaml:"arguments"`
		}
		body := string(data)

		// Parse YAML front-matter if present.
		if stripped, fm, ok := parseFrontMatter(data); ok {
			if err := yaml.Unmarshal(fm, &meta); err != nil {
				return nil, fmt.Errorf("parse prompts/%s.md front-matter: %w", fileName, err)
			}
			body = stripped
		}

		if meta.Name == "" {
			meta.Name = fileName
		}

		prompts = append(prompts, models.StorePrompt{
			Name:        meta.Name,
			Description: meta.Description,
			Arguments:   meta.Arguments,
			Body:        strings.TrimSpace(body),
		})
	}

	if prompts == nil {
		prompts = []models.StorePrompt{}
	}
	return prompts, nil
}

// parseFrontMatter splits a markdown file into (body, frontMatter, found).
// Front-matter is delimited by leading and trailing "---" lines.
func parseFrontMatter(data []byte) (body string, fm []byte, ok bool) {
	s := string(data)
	if !strings.HasPrefix(s, "---\n") {
		return "", nil, false
	}
	rest := s[4:]
	end := strings.Index(rest, "\n---\n")
	if end == -1 {
		// Trailing --- at EOF.
		end = strings.Index(rest, "\n---")
		if end == -1 {
			return "", nil, false
		}
		return "", []byte(rest[:end]), true
	}
	return rest[end+5:], []byte(rest[:end]), true
}
