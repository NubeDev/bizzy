// Package apps handles app scanning, loading, and per-user tool scoping.
package apps

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// App represents a parsed app.yaml — the metadata for an installable app.
type App struct {
	Name          string          `yaml:"name" json:"name"`
	Version       string          `yaml:"version" json:"version"`
	Description   string          `yaml:"description" json:"description"`
	Author        string          `yaml:"author" json:"author"`
	Permissions   Permissions     `yaml:"permissions" json:"permissions"`
	Settings      []SettingDef    `yaml:"settings" json:"settings"`
	Tags          []string        `yaml:"tags" json:"tags"`
	Timeout       string          `yaml:"timeout" json:"timeout"`
	OpenAPIRemote *OpenAPIRemote  `yaml:"openapi" json:"openapi,omitempty"`
	Preamble      string          `yaml:"preamble" json:"preamble,omitempty"`

	// Populated by the loader, not from YAML.
	Dir         string `yaml:"-" json:"dir"`
	HasOpenAPI  bool   `yaml:"-" json:"hasOpenAPI"`
	HasPrompts  bool   `yaml:"-" json:"hasPrompts"`
	HasTools    bool   `yaml:"-" json:"hasTools"`
	PromptFiles []string `yaml:"-" json:"promptFiles,omitempty"`
}

// OpenAPIRemote configures fetching an OpenAPI spec from a live server at runtime.
// The URL supports {{setting_key}} placeholders resolved from the user's install settings.
type OpenAPIRemote struct {
	URL         string   `yaml:"url" json:"url"`
	IncludeTags []string `yaml:"includeTags" json:"includeTags,omitempty"`
	ExcludeTags []string `yaml:"excludeTags" json:"excludeTags,omitempty"`
}

// Permissions declares what an app is allowed to do.
type Permissions struct {
	AllowedHosts     []string `yaml:"allowedHosts" json:"allowedHosts"`
	DefaultToolClass string   `yaml:"defaultToolClass" json:"defaultToolClass"`
	Secrets          []string `yaml:"secrets" json:"secrets"`
}

// SettingDef describes a user-configurable setting.
type SettingDef struct {
	Key      string `yaml:"key" json:"key"`
	Label    string `yaml:"label" json:"label"`
	Type     string `yaml:"type" json:"type"` // url, string, secret, number, bool
	Required bool   `yaml:"required" json:"required"`
	Default  string `yaml:"default" json:"default"`
}

// Prompt represents a parsed markdown prompt file with frontmatter.
type Prompt struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Arguments   []PromptArgument  `json:"arguments,omitempty"`
	Body        string            `json:"body"`
	AppName     string            `json:"appName"`
}

// PromptArgument is a single argument in a prompt.
type PromptArgument struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description"`
	Required    bool   `yaml:"required" json:"required"`
}

// LoadApp parses an app.yaml from the given directory.
func LoadApp(dir string) (*App, error) {
	yamlPath := filepath.Join(dir, "app.yaml")
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return nil, fmt.Errorf("read app.yaml: %w", err)
	}
	var app App
	if err := yaml.Unmarshal(data, &app); err != nil {
		return nil, fmt.Errorf("parse app.yaml: %w", err)
	}
	if app.Name == "" {
		return nil, fmt.Errorf("app.yaml in %s: name is required", dir)
	}
	if app.Version == "" {
		app.Version = "0.0.0"
	}

	app.Dir = dir
	app.HasOpenAPI = app.OpenAPIRemote != nil || fileExists(filepath.Join(dir, "openapi.yaml")) || fileExists(filepath.Join(dir, "openapi.json"))
	app.HasPrompts = dirHasFiles(filepath.Join(dir, "prompts"), ".md")
	app.HasTools = dirHasFiles(filepath.Join(dir, "tools"), ".js")

	// Collect prompt file names.
	if app.HasPrompts {
		entries, _ := os.ReadDir(filepath.Join(dir, "prompts"))
		for _, e := range entries {
			if !e.IsDir() && filepath.Ext(e.Name()) == ".md" {
				app.PromptFiles = append(app.PromptFiles, e.Name())
			}
		}
	}

	return &app, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func dirHasFiles(dir, ext string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ext {
			return true
		}
	}
	return false
}
