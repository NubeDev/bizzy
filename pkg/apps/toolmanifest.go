package apps

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ToolManifest is parsed from a tool.json file alongside a .js script.
type ToolManifest struct {
	Name        string                    `json:"name"`
	Description string                    `json:"description"`
	ToolClass   string                    `json:"toolClass"` // read-only, read-write, destructive
	Mode        string                    `json:"mode,omitempty"` // "" (default) or "qa"
	QAPrompt    string                    `json:"qa_prompt,omitempty"` // custom MCP prompt body for QA tools (overrides auto-generation)
	Params      map[string]ToolParamDef   `json:"params"`

	// Set by the loader.
	ScriptPath string `json:"-"`
	AppName    string `json:"-"`
}

// ToolParamDef describes a single parameter for a JS tool.
type ToolParamDef struct {
	Type        string `json:"type"` // string, number, boolean
	Required    bool   `json:"required"`
	Description string `json:"description"`
	Default     any    `json:"default,omitempty"`
	Order       int    `json:"order,omitempty"` // display/prompt order for QA params (0 = unset, sorted last)
}

// LoadToolManifests loads all tool.json + .js pairs from an app's tools/ directory.
func LoadToolManifests(app *App) ([]ToolManifest, error) {
	toolsDir := filepath.Join(app.Dir, "tools")
	entries, err := os.ReadDir(toolsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read tools dir: %w", err)
	}

	var manifests []ToolManifest
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}

		// Look for matching .js file.
		baseName := strings.TrimSuffix(e.Name(), ".json")
		jsPath := filepath.Join(toolsDir, baseName+".js")
		if !fileExists(jsPath) {
			continue // no matching script
		}

		data, err := os.ReadFile(filepath.Join(toolsDir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}

		var m ToolManifest
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("parse %s: %w", e.Name(), err)
		}
		if m.Name == "" {
			m.Name = baseName
		}
		if m.ToolClass == "" {
			m.ToolClass = app.Permissions.DefaultToolClass
		}
		m.ScriptPath = jsPath
		m.AppName = app.Name
		manifests = append(manifests, m)
	}
	return manifests, nil
}
