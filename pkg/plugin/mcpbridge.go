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
