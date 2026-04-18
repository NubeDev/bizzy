package plugin

import "github.com/NubeDev/bizzy/pkg/apps"

// MCPBridge adapts the plugin Registry to the apps.PluginToolSource interface
// so MCPFactory can register plugin tools without importing this package.
type MCPBridge struct {
	reg *Registry
}

// NewMCPBridge creates a bridge between the plugin registry and MCPFactory.
func NewMCPBridge(reg *Registry) *MCPBridge {
	return &MCPBridge{reg: reg}
}

// ActivePluginTools returns all tools from active plugins in the format MCPFactory expects.
func (b *MCPBridge) ActivePluginTools() []apps.PluginToolEntry {
	tools := b.reg.ActiveTools()
	out := make([]apps.PluginToolEntry, len(tools))
	for i, t := range tools {
		out[i] = apps.PluginToolEntry{
			FullName:    t.FullName,
			PluginName:  t.PluginName,
			Name:        t.Spec.Name,
			Description: t.Spec.Description,
			Parameters:  t.Spec.Parameters,
		}
	}
	return out
}

// ActivePluginPrompts returns all prompts from active plugins.
func (b *MCPBridge) ActivePluginPrompts() []apps.PluginPromptEntry {
	prompts := b.reg.ActivePrompts()
	out := make([]apps.PluginPromptEntry, len(prompts))
	for i, p := range prompts {
		args := make([]apps.PluginPromptArg, len(p.Spec.Arguments))
		for j, a := range p.Spec.Arguments {
			args[j] = apps.PluginPromptArg{
				Name:        a.Name,
				Description: a.Description,
				Required:    a.Required,
			}
		}
		out[i] = apps.PluginPromptEntry{
			FullName:    p.FullName,
			PluginName:  p.PluginName,
			Name:        p.Spec.Name,
			Description: p.Spec.Description,
			Template:    p.Spec.Template,
			Arguments:   args,
		}
	}
	return out
}

// CallTool dispatches a tool call to a plugin via NATS.
func (b *MCPBridge) CallTool(pluginName, toolName string, params map[string]any) (any, error) {
	return b.reg.Proxy().Call(pluginName, toolName, params, ToolCallCtx{})
}

// ---------------------------------------------------------------------------
// PluginQuerySource implementation — used by JSRuntime's plugins.* host API.
// ---------------------------------------------------------------------------

// PluginExists reports whether a named plugin is active.
func (b *MCPBridge) PluginExists(name string) bool {
	p, ok := b.reg.GetPlugin(name)
	return ok && p.Status == "active"
}

// PluginInfo returns metadata about a plugin, or nil if not found.
func (b *MCPBridge) PluginInfo(name string) *apps.PluginInfoResult {
	p, ok := b.reg.GetPlugin(name)
	if !ok {
		return nil
	}
	services := make([]string, len(p.Manifest.Services))
	for i, s := range p.Manifest.Services {
		services[i] = string(s)
	}
	tools := make([]string, len(p.Manifest.Tools))
	for i, t := range p.Manifest.Tools {
		tools[i] = t.Name
	}
	return &apps.PluginInfoResult{
		Name:     p.Manifest.Name,
		Version:  p.Manifest.Version,
		Status:   p.Status,
		Services: services,
		Tools:    tools,
	}
}

// PluginList returns the names of active plugins, optionally filtered by service type.
func (b *MCPBridge) PluginList(serviceFilter string) []string {
	plugins := b.reg.ActivePluginsByService(serviceFilter)
	names := make([]string, len(plugins))
	for i, p := range plugins {
		names[i] = p.Manifest.Name
	}
	return names
}

// CallPluginTool dispatches a tool call to a plugin over NATS.
func (b *MCPBridge) CallPluginTool(pluginName, toolName string, params map[string]any) (any, error) {
	return b.reg.Proxy().Call(pluginName, toolName, params, ToolCallCtx{})
}
