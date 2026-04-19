// Package bootstrap provides the built-in "bizzy-dev" reference prompts as
// reusable structured data.  Any consumer (REST API, MCP, CLI, disk writer)
// can call List() and Get() without depending on the app registry or disk files.
package bootstrap

import "fmt"

// Prompt is a single bootstrap reference prompt.
type Prompt struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
	Body        string           `json:"body"`
}

// PromptArgument describes one substitution variable in a prompt template.
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// AppName is the well-known app name for the bootstrap prompts.
const AppName = "bizzy-dev"

// List returns all bootstrap prompts.
func List() []Prompt {
	return []Prompt{
		// Platform reference
		promptOverview,
		promptAPIGuide,
		promptPluginSystem,
		promptAppDevelopment,
		promptServerTesting,
		promptNewApp,
		promptFlowEngine,
		// Builder / AI prompts
		promptToolNaming,
		promptUIReference,
		promptAppBuilder,
		promptWorkshop,
		promptToolEditor,
	}
}

// Get returns a single bootstrap prompt by name, or an error if not found.
func Get(name string) (*Prompt, error) {
	for _, p := range List() {
		if p.Name == name {
			cp := p
			return &cp, nil
		}
	}
	return nil, fmt.Errorf("bootstrap prompt not found: %s", name)
}

// Names returns just the prompt names (useful for summaries / preambles).
func Names() []string {
	all := List()
	names := make([]string, len(all))
	for i, p := range all {
		names[i] = p.Name
	}
	return names
}
