package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

// --- App CRUD ---

// createAppRequest is the body for POST /api/apps.
type createAppRequest struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description"`
	Author      string   `json:"author"`
	Tags        []string `json:"tags"`
	Version     string   `json:"version"`
}

var validAppName = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,48}[a-z0-9]$`)

// createApp creates a new app directory with app.yaml.
//
//	POST /api/apps
func (a *API) createApp(c *gin.Context) {
	var req createAppRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	if !validAppName.MatchString(req.Name) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid name: must be lowercase, 3-50 chars, letters/numbers/hyphens only",
		})
		return
	}

	// Check for duplicates.
	if _, exists := a.AppRegistry.Get(req.Name); exists {
		c.JSON(http.StatusConflict, gin.H{"error": "app already exists: " + req.Name})
		return
	}

	if req.Version == "" {
		req.Version = "1.0.0"
	}
	if req.Author == "" {
		req.Author = "NubeIO"
	}

	// Create directory structure.
	appDir := filepath.Join(a.AppRegistry.AppsDir(), req.Name)
	for _, sub := range []string{"tools", "prompts"} {
		if err := os.MkdirAll(filepath.Join(appDir, sub), 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "create dir: " + err.Error()})
			return
		}
	}

	// Write app.yaml.
	appYAML := map[string]any{
		"name":        req.Name,
		"version":     req.Version,
		"description": req.Description,
		"author":      req.Author,
		"permissions": map[string]any{
			"allowedHosts":    []string{},
			"defaultToolClass": "read-only",
		},
		"settings": []any{},
		"tags":     req.Tags,
	}

	if err := writeYAML(filepath.Join(appDir, "app.yaml"), appYAML); err != nil {
		os.RemoveAll(appDir)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "write app.yaml: " + err.Error()})
		return
	}

	// Reload registry.
	a.AppRegistry.Reload()

	app, _ := a.AppRegistry.Get(req.Name)
	c.JSON(http.StatusCreated, app)
}

// updateApp updates an app's metadata.
//
//	PUT /api/apps/:id
func (a *API) updateApp(c *gin.Context) {
	name := c.Param("id")

	app, ok := a.AppRegistry.Get(name)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "app not found"})
		return
	}

	var req struct {
		Description string   `json:"description"`
		Version     string   `json:"version"`
		Author      string   `json:"author"`
		Tags        []string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Read existing app.yaml, update fields, write back.
	appPath := filepath.Join(app.Dir, "app.yaml")
	data, err := os.ReadFile(appPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "read app.yaml: " + err.Error()})
		return
	}

	var appMap map[string]any
	yaml.Unmarshal(data, &appMap)

	if req.Description != "" {
		appMap["description"] = req.Description
	}
	if req.Version != "" {
		appMap["version"] = req.Version
	}
	if req.Author != "" {
		appMap["author"] = req.Author
	}
	if req.Tags != nil {
		appMap["tags"] = req.Tags
	}

	if err := writeYAML(appPath, appMap); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "write app.yaml: " + err.Error()})
		return
	}

	a.AppRegistry.Reload()
	updated, _ := a.AppRegistry.Get(name)
	c.JSON(http.StatusOK, updated)
}

// deleteApp removes an app and all its files.
//
//	DELETE /api/apps/:id
func (a *API) deleteApp(c *gin.Context) {
	name := c.Param("id")

	app, ok := a.AppRegistry.Get(name)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "app not found"})
		return
	}

	if err := os.RemoveAll(app.Dir); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete: " + err.Error()})
		return
	}

	a.AppRegistry.Reload()
	c.JSON(http.StatusOK, gin.H{"deleted": name})
}

// --- Tool CRUD ---

// createToolRequest is the body for POST /api/apps/:id/tools.
type createToolRequest struct {
	Name        string         `json:"name" binding:"required"`
	Description string         `json:"description"`
	Mode        string         `json:"mode"`
	ToolClass   string         `json:"toolClass"`
	Params      map[string]any `json:"params"`
	Script      string         `json:"script" binding:"required"`
}

// createTool creates a new JS tool (.json + .js) in an app.
//
//	POST /api/apps/:id/tools
func (a *API) createTool(c *gin.Context) {
	appName := c.Param("id")

	app, ok := a.AppRegistry.Get(appName)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "app not found"})
		return
	}

	var req createToolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	toolsDir := filepath.Join(app.Dir, "tools")
	os.MkdirAll(toolsDir, 0755)

	jsPath := filepath.Join(toolsDir, req.Name+".js")
	jsonPath := filepath.Join(toolsDir, req.Name+".json")

	// Check for duplicates.
	if fileExistsCRUD(jsPath) {
		c.JSON(http.StatusConflict, gin.H{"error": "tool already exists: " + req.Name})
		return
	}

	// Write manifest JSON.
	manifest := map[string]any{
		"name":        req.Name,
		"description": req.Description,
		"toolClass":   req.ToolClass,
		"params":      req.Params,
	}
	if req.Mode != "" {
		manifest["mode"] = req.Mode
	}

	if err := writeJSON(jsonPath, manifest); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "write json: " + err.Error()})
		return
	}

	// Write JS script.
	if err := os.WriteFile(jsPath, []byte(req.Script), 0644); err != nil {
		os.Remove(jsonPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "write js: " + err.Error()})
		return
	}

	a.AppRegistry.Reload()
	c.JSON(http.StatusCreated, gin.H{
		"name": appName + "." + req.Name,
		"mode": req.Mode,
	})
}

// updateTool updates an existing tool's manifest and/or script.
//
//	PUT /api/apps/:id/tools/:name
func (a *API) updateTool(c *gin.Context) {
	appName := c.Param("id")
	toolName := c.Param("name")

	app, ok := a.AppRegistry.Get(appName)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "app not found"})
		return
	}

	toolsDir := filepath.Join(app.Dir, "tools")
	jsPath := filepath.Join(toolsDir, toolName+".js")
	jsonPath := filepath.Join(toolsDir, toolName+".json")

	if !fileExistsCRUD(jsPath) {
		c.JSON(http.StatusNotFound, gin.H{"error": "tool not found: " + toolName})
		return
	}

	var req struct {
		Description string         `json:"description"`
		Mode        string         `json:"mode"`
		ToolClass   string         `json:"toolClass"`
		Params      map[string]any `json:"params"`
		Script      string         `json:"script"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update manifest if any fields provided.
	if req.Description != "" || req.Mode != "" || req.ToolClass != "" || req.Params != nil {
		data, _ := os.ReadFile(jsonPath)
		var manifest map[string]any
		if err := decodeJSON(data, &manifest); err != nil {
			manifest = map[string]any{"name": toolName}
		}

		if req.Description != "" {
			manifest["description"] = req.Description
		}
		if req.Mode != "" {
			manifest["mode"] = req.Mode
		}
		if req.ToolClass != "" {
			manifest["toolClass"] = req.ToolClass
		}
		if req.Params != nil {
			manifest["params"] = req.Params
		}

		writeJSON(jsonPath, manifest)
	}

	// Update script if provided.
	if req.Script != "" {
		os.WriteFile(jsPath, []byte(req.Script), 0644)
	}

	a.AppRegistry.Reload()
	c.JSON(http.StatusOK, gin.H{"updated": appName + "." + toolName})
}

// deleteTool removes a tool's .js and .json files.
//
//	DELETE /api/apps/:id/tools/:name
func (a *API) deleteTool(c *gin.Context) {
	appName := c.Param("id")
	toolName := c.Param("name")

	app, ok := a.AppRegistry.Get(appName)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "app not found"})
		return
	}

	toolsDir := filepath.Join(app.Dir, "tools")
	jsPath := filepath.Join(toolsDir, toolName+".js")
	jsonPath := filepath.Join(toolsDir, toolName+".json")

	if !fileExistsCRUD(jsPath) {
		c.JSON(http.StatusNotFound, gin.H{"error": "tool not found: " + toolName})
		return
	}

	os.Remove(jsPath)
	os.Remove(jsonPath)

	a.AppRegistry.Reload()
	c.JSON(http.StatusOK, gin.H{"deleted": appName + "." + toolName})
}

// --- Prompt CRUD ---

// createPromptRequest is the body for POST /api/apps/:id/prompts.
type createPromptRequest struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description"`
	Arguments   []promptArg `json:"arguments"`
	Body        string   `json:"body" binding:"required"`
}

type promptArg struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// createPrompt creates a new prompt markdown file in an app.
//
//	POST /api/apps/:id/prompts
func (a *API) createPrompt(c *gin.Context) {
	appName := c.Param("id")

	app, ok := a.AppRegistry.Get(appName)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "app not found"})
		return
	}

	var req createPromptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	promptsDir := filepath.Join(app.Dir, "prompts")
	os.MkdirAll(promptsDir, 0755)

	fileName := strings.ReplaceAll(req.Name, "_", "-") + ".md"
	promptPath := filepath.Join(promptsDir, fileName)

	if fileExistsCRUD(promptPath) {
		c.JSON(http.StatusConflict, gin.H{"error": "prompt already exists: " + req.Name})
		return
	}

	// Build markdown with YAML frontmatter.
	var md strings.Builder
	md.WriteString("---\n")
	md.WriteString(fmt.Sprintf("name: %s\n", req.Name))
	md.WriteString(fmt.Sprintf("description: %s\n", req.Description))
	if len(req.Arguments) > 0 {
		md.WriteString("arguments:\n")
		for _, arg := range req.Arguments {
			md.WriteString(fmt.Sprintf("  - name: %s\n", arg.Name))
			md.WriteString(fmt.Sprintf("    description: %s\n", arg.Description))
			md.WriteString(fmt.Sprintf("    required: %v\n", arg.Required))
		}
	}
	md.WriteString("---\n\n")
	md.WriteString(req.Body)
	md.WriteString("\n")

	if err := os.WriteFile(promptPath, []byte(md.String()), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "write prompt: " + err.Error()})
		return
	}

	a.AppRegistry.Reload()
	c.JSON(http.StatusCreated, gin.H{"name": appName + "." + req.Name})
}

// deletePrompt removes a prompt file.
//
//	DELETE /api/apps/:id/prompts/:name
func (a *API) deletePrompt(c *gin.Context) {
	appName := c.Param("id")
	promptName := c.Param("name")

	app, ok := a.AppRegistry.Get(appName)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "app not found"})
		return
	}

	// Try both naming conventions.
	promptsDir := filepath.Join(app.Dir, "prompts")
	candidates := []string{
		filepath.Join(promptsDir, promptName+".md"),
		filepath.Join(promptsDir, strings.ReplaceAll(promptName, "_", "-")+".md"),
	}

	for _, path := range candidates {
		if fileExistsCRUD(path) {
			os.Remove(path)
			a.AppRegistry.Reload()
			c.JSON(http.StatusOK, gin.H{"deleted": appName + "." + promptName})
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "prompt not found: " + promptName})
}

// --- List tools/prompts for an app ---

// listAppTools returns all tools for a specific app.
//
//	GET /api/apps/:id/tools
func (a *API) listAppTools(c *gin.Context) {
	appName := c.Param("id")

	if _, ok := a.AppRegistry.Get(appName); !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "app not found"})
		return
	}

	tools := a.AppRegistry.GetTools(appName)
	c.JSON(http.StatusOK, tools)
}

// listAppPrompts returns all prompts for a specific app.
//
//	GET /api/apps/:id/prompts
func (a *API) listAppPrompts(c *gin.Context) {
	appName := c.Param("id")

	if _, ok := a.AppRegistry.Get(appName); !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "app not found"})
		return
	}

	prompts := a.AppRegistry.GetPrompts(appName)
	c.JSON(http.StatusOK, prompts)
}

// --- Helpers ---

func writeYAML(path string, data any) error {
	out, err := yaml.Marshal(data)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0644)
}

func writeJSON(path string, data any) error {
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0644)
}

func decodeJSON(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func fileExistsCRUD(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
