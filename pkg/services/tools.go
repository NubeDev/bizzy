package services

import (
	"fmt"
	"strings"
	"time"

	"github.com/NubeDev/bizzy/pkg/apps"
	"github.com/NubeDev/bizzy/pkg/models"
	"gorm.io/gorm"
)

// ToolService handles tool resolution, execution, and listing.
// It extracts business logic that was previously in agents_qa.go, tools.go,
// and workflows.go into a single reusable layer.
type ToolService struct {
	DB          *gorm.DB
	AppRegistry *apps.Registry
}

// ResolvedTool contains a prepared runtime and manifest ready for execution.
type ResolvedTool struct {
	Runtime  *apps.JSRuntime
	Manifest *apps.ToolManifest
}

// userEnabledInstalls returns the user's enabled app installs.
func (s *ToolService) userEnabledInstalls(userID string) []models.AppInstall {
	var installs []models.AppInstall
	s.DB.Where("user_id = ? AND enabled = ?", userID, true).Find(&installs)
	return installs
}

// ResolveTool finds a JS tool by namespaced name (e.g. "rubix.query_nodes")
// and returns a configured runtime ready for execution.
func (s *ToolService) ResolveTool(userID, toolName string) (*ResolvedTool, error) {
	installs := s.userEnabledInstalls(userID)

	for _, inst := range installs {
		app, ok := s.AppRegistry.Get(inst.AppName)
		if !ok {
			continue
		}

		for _, manifest := range s.AppRegistry.GetTools(inst.AppName) {
			fullName := inst.AppName + "." + manifest.Name
			if fullName != toolName {
				continue
			}

			secrets := make(map[string]string)
			config := make(map[string]string)
			for _, def := range app.Settings {
				val := inst.GetSetting(def.Key)
				if val == "" && def.Default != "" {
					val = def.Default
				}
				if def.Type == "secret" {
					secrets[def.Key] = val
				} else {
					config[def.Key] = val
				}
			}

			timeout := 5 * time.Second
			if app.Timeout != "" {
				if d, err := time.ParseDuration(app.Timeout); err == nil {
					timeout = d
				}
			}

			m := manifest // copy for pointer
			return &ResolvedTool{
				Runtime:  apps.NewJSRuntime(app, secrets, config, timeout),
				Manifest: &m,
			}, nil
		}
	}

	return nil, fmt.Errorf("tool not found: %s", toolName)
}

// CallTool resolves and executes a JS tool in one step.
func (s *ToolService) CallTool(userID, toolName string, params map[string]any) (map[string]any, error) {
	resolved, err := s.ResolveTool(userID, toolName)
	if err != nil {
		return nil, err
	}
	return resolved.Runtime.Execute(resolved.Manifest.ScriptPath, params)
}

// ToolInfo describes a tool available to a user.
type ToolInfo struct {
	Name    string      `json:"name"`
	AppName string      `json:"appName"`
	Type    string      `json:"type"` // "openapi" or "js"
	Mode    string      `json:"mode,omitempty"`
	Prompt  string      `json:"prompt,omitempty"`
	Desc    string      `json:"description"`
	Params  []ParamInfo `json:"params,omitempty"`
}

// ParamInfo describes a tool parameter.
type ParamInfo struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Description string   `json:"description"`
	Options     []string `json:"options,omitempty"`
}

// ListTools returns all MCP tools available to a user based on their installed apps.
func (s *ToolService) ListTools(userID string) []ToolInfo {
	installs := s.userEnabledInstalls(userID)

	tools := make([]ToolInfo, 0)
	for _, inst := range installs {
		app, ok := s.AppRegistry.Get(inst.AppName)
		if !ok {
			continue
		}
		if app.HasOpenAPI {
			tools = append(tools, ToolInfo{
				Name:    inst.AppName + ".*",
				AppName: inst.AppName,
				Type:    "openapi",
				Desc:    "OpenAPI-generated tools from " + inst.AppName,
			})
		}
		for _, m := range s.AppRegistry.GetTools(inst.AppName) {
			ti := ToolInfo{
				Name:    inst.AppName + "." + m.Name,
				AppName: inst.AppName,
				Type:    "js",
				Mode:    m.Mode,
				Prompt:  m.Prompt,
				Desc:    m.Description,
			}
			for pName, pDef := range m.Params {
				if pName == "_submit" || pName == "_answers" {
					continue
				}
				ti.Params = append(ti.Params, ParamInfo{
					Name:        pName,
					Type:        pDef.Type,
					Required:    pDef.Required,
					Description: pDef.Description,
					Options:     pDef.Options,
				})
			}
			tools = append(tools, ti)
		}
	}
	return tools
}

// PromptInfo describes a prompt available to a user.
type PromptInfo struct {
	Name    string         `json:"name"`
	AppName string         `json:"appName"`
	Desc    string         `json:"description"`
	Args    []PromptArgInfo `json:"arguments,omitempty"`
}

// PromptArgInfo describes a prompt argument.
type PromptArgInfo struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
}

// ListPrompts returns all MCP prompts available to a user.
func (s *ToolService) ListPrompts(userID string) []PromptInfo {
	installs := s.userEnabledInstalls(userID)

	prompts := make([]PromptInfo, 0)
	for _, inst := range installs {
		for _, p := range s.AppRegistry.GetPrompts(inst.AppName) {
			pi := PromptInfo{
				Name:    inst.AppName + "." + p.Name,
				AppName: inst.AppName,
				Desc:    p.Description,
			}
			for _, a := range p.Arguments {
				pi.Args = append(pi.Args, PromptArgInfo{
					Name:     a.Name,
					Required: a.Required,
				})
			}
			prompts = append(prompts, pi)
		}
	}
	return prompts
}

// RenderedPrompt is the result of rendering a prompt with arguments.
type RenderedPrompt struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Rendered    string `json:"rendered"`
}

// GetPrompt finds and renders a prompt with the given arguments.
func (s *ToolService) GetPrompt(userID, promptName string, args map[string]string) (*RenderedPrompt, error) {
	installs := s.userEnabledInstalls(userID)

	for _, inst := range installs {
		for _, p := range s.AppRegistry.GetPrompts(inst.AppName) {
			fullName := inst.AppName + "." + p.Name
			if fullName != promptName {
				continue
			}
			body := p.Body
			for _, arg := range p.Arguments {
				if val, ok := args[arg.Name]; ok && val != "" {
					body = strings.ReplaceAll(body, "{{"+arg.Name+"}}", val)
				}
			}
			return &RenderedPrompt{
				Name:        fullName,
				Description: p.Description,
				Rendered:    body,
			}, nil
		}
	}

	return nil, fmt.Errorf("prompt not found — is the app installed?")
}
