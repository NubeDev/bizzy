package api

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/NubeDev/bizzy/pkg/models"
	"gopkg.in/yaml.v3"
)

// diskApp is the app.yaml structure written to disk for store apps.
type diskApp struct {
	Name        string           `yaml:"name"`
	Version     string           `yaml:"version"`
	Description string           `yaml:"description"`
	Author      string           `yaml:"author"`
	Permissions diskPermissions  `yaml:"permissions"`
	Settings    []models.SettingDef `yaml:"settings"`
	Tags        []string         `yaml:"tags"`
}

type diskPermissions struct {
	AllowedHosts     []string `yaml:"allowedHosts"`
	DefaultToolClass string   `yaml:"defaultToolClass"`
}

// diskToolManifest is the tool.json written alongside the .js file.
type diskToolManifest struct {
	Name        string                       `json:"name"`
	Description string                       `json:"description"`
	ToolClass   string                       `json:"toolClass"`
	Mode        string                       `json:"mode,omitempty"`
	Params      map[string]models.ToolParam  `json:"params"`
}

// WriteStoreAppToDisk writes the full disk structure for a store app:
// {storeAppsDir}/{name}/app.yaml, tools/*.json + tools/*.js, prompts/*.md
func WriteStoreAppToDisk(sa models.StoreApp, storeAppsDir string) error {
	appDir := filepath.Join(storeAppsDir, sa.Name)

	// Ensure directories exist.
	for _, sub := range []string{"", "tools", "prompts"} {
		if err := os.MkdirAll(filepath.Join(appDir, sub), 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", sub, err)
		}
	}

	// Write app.yaml.
	if err := writeAppYAML(sa, appDir); err != nil {
		return err
	}

	// Write tools.
	for _, t := range sa.Tools {
		if err := writeToolDisk(t, appDir); err != nil {
			return fmt.Errorf("write tool %s: %w", t.Name, err)
		}
	}

	// Write prompts.
	for _, p := range sa.Prompts {
		if err := writePromptDisk(p, appDir); err != nil {
			return fmt.Errorf("write prompt %s: %w", p.Name, err)
		}
	}

	return nil
}

func writeAppYAML(sa models.StoreApp, appDir string) error {
	app := diskApp{
		Name:        sa.Name,
		Version:     sa.Version,
		Description: sa.Description,
		Author:      sa.AuthorName,
		Permissions: diskPermissions{
			AllowedHosts:     sa.Permissions.AllowedHosts,
			DefaultToolClass: sa.Permissions.DefaultToolClass,
		},
		Settings: sa.Settings,
		Tags:     sa.Tags,
	}
	if app.Permissions.AllowedHosts == nil {
		app.Permissions.AllowedHosts = []string{}
	}
	if app.Settings == nil {
		app.Settings = []models.SettingDef{}
	}
	if app.Tags == nil {
		app.Tags = []string{}
	}

	data, err := yaml.Marshal(app)
	if err != nil {
		return fmt.Errorf("marshal app.yaml: %w", err)
	}
	return os.WriteFile(filepath.Join(appDir, "app.yaml"), data, 0644)
}

func writeToolDisk(t models.StoreTool, appDir string) error {
	toolsDir := filepath.Join(appDir, "tools")

	// Write the .json manifest.
	manifest := diskToolManifest{
		Name:        t.Name,
		Description: t.Description,
		ToolClass:   t.ToolClass,
		Mode:        t.Mode,
		Params:      t.Params,
	}
	if manifest.Params == nil {
		manifest.Params = make(map[string]models.ToolParam)
	}
	jsonData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal tool json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(toolsDir, t.Name+".json"), jsonData, 0644); err != nil {
		return err
	}

	// Write the .js script.
	script := t.Script
	if script == "" {
		script = "function handle(params) {\n  return { result: \"not implemented\" };\n}\n"
	}
	return os.WriteFile(filepath.Join(toolsDir, t.Name+".js"), []byte(script), 0644)
}

func writePromptDisk(p models.StorePrompt, appDir string) error {
	promptsDir := filepath.Join(appDir, "prompts")

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString("name: " + p.Name + "\n")
	sb.WriteString("description: " + p.Description + "\n")
	if len(p.Arguments) > 0 {
		sb.WriteString("arguments:\n")
		for _, a := range p.Arguments {
			sb.WriteString("  - name: " + a.Name + "\n")
			sb.WriteString("    description: " + a.Description + "\n")
			if a.Required {
				sb.WriteString("    required: true\n")
			}
		}
	}
	sb.WriteString("---\n\n")
	sb.WriteString(p.Body)
	if !strings.HasSuffix(p.Body, "\n") {
		sb.WriteString("\n")
	}

	return os.WriteFile(filepath.Join(promptsDir, p.Name+".md"), []byte(sb.String()), 0644)
}

// deleteToolDisk removes the .json and .js files for a tool.
func deleteToolDisk(toolName, appDir string) {
	toolsDir := filepath.Join(appDir, "tools")
	os.Remove(filepath.Join(toolsDir, toolName+".json"))
	os.Remove(filepath.Join(toolsDir, toolName+".js"))
}

// deletePromptDisk removes the .md file for a prompt.
func deletePromptDisk(promptName, appDir string) {
	os.Remove(filepath.Join(appDir, "prompts", promptName+".md"))
}
