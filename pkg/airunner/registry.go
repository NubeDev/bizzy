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
	r.Register(&CodexRunner{})
	r.Register(&CopilotRunner{})
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

// Available returns the list of providers whose CLI binary is installed.
func (r *Registry) Available() []ProviderInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []ProviderInfo
	for _, runner := range r.runners {
		out = append(out, ProviderInfo{
			Provider:  runner.Name(),
			Available: runner.Available(),
		})
	}
	return out
}

// ProviderInfo describes a registered provider and its availability.
type ProviderInfo struct {
	Provider  Provider `json:"provider"`
	Available bool     `json:"available"`
}
