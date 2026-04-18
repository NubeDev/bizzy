package airunner

import (
	"fmt"
	"sync"
)

// Registry holds the available AI CLI runners.
type Registry struct {
	mu      sync.RWMutex
	runners map[Provider]Runner
}

// NewRegistry creates a registry pre-loaded with all built-in runners.
func NewRegistry() *Registry {
	r := &Registry{runners: make(map[Provider]Runner)}
	r.Register(&ClaudeRunner{})
	r.Register(&OllamaRunner{})
	r.Register(&CodexRunner{})
	r.Register(&CopilotRunner{})
	r.Register(&OpenCodeRunner{})
	return r
}

// Register adds or replaces a runner.
func (r *Registry) Register(runner Runner) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runners[runner.Name()] = runner
}

// Get returns the runner for the given provider.
func (r *Registry) Get(provider Provider) (Runner, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	runner, ok := r.runners[provider]
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}
	return runner, nil
}

// Available returns the list of providers and their availability/models.
func (r *Registry) Available() []ProviderInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []ProviderInfo
	for _, runner := range r.runners {
		info := ProviderInfo{
			Provider:  runner.Name(),
			Available: runner.Available(),
			Type:      "cli",
		}
		// API-based providers.
		switch runner.Name() {
		case ProviderOllama, ProviderOpenAI, ProviderAnthropic, ProviderGemini:
			info.Type = "api"
		}
		// If the runner can list models, include them.
		if lister, ok := runner.(ModelLister); ok && info.Available {
			if models, err := lister.InstalledModels(); err == nil {
				info.Models = models
			}
		}
		out = append(out, info)
	}
	return out
}

// ModelLister is an optional interface runners can implement to report
// which models are installed/available.
type ModelLister interface {
	InstalledModels() ([]string, error)
}

// Configurable is an optional interface runners can implement to accept
// runtime configuration (API keys, host URLs) from the admin config.
type Configurable interface {
	Configure(host, apiKey string)
}

// ProviderInfo describes a registered provider and its availability.
type ProviderInfo struct {
	Provider  Provider `json:"provider"`
	Available bool     `json:"available"`
	Type      string   `json:"type"`             // "cli" or "api"
	Models    []string `json:"models,omitempty"`
}
