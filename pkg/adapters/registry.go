// Package adapters provides the adapter interface and registry for the
// command bus. Each adapter converts between an external protocol (Slack,
// email, webhook, cron) and the Command/Event system.
package adapters

import (
	"fmt"
	"log"
	"sync"

	"github.com/NubeDev/bizzy/pkg/command"
)

// Registry maps channel names to adapters and reconstructs ReplyChannels
// from stored ReplyInfo.
type Registry struct {
	mu       sync.RWMutex
	adapters map[string]command.Adapter
}

// NewRegistry creates an empty adapter registry.
func NewRegistry() *Registry {
	return &Registry{
		adapters: make(map[string]command.Adapter),
	}
}

// Register adds an adapter to the registry.
func (r *Registry) Register(name string, adapter command.Adapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[name] = adapter
}

// BuildReply reconstructs a live ReplyChannel from stored ReplyInfo.
// If the adapter was disabled or deregistered after a command started,
// this returns an error so the reply router can log the failure.
func (r *Registry) BuildReply(info command.ReplyInfo) (command.ReplyChannel, error) {
	if info.Channel == "" {
		return nil, fmt.Errorf("reply info has no channel")
	}

	r.mu.RLock()
	adapter, ok := r.adapters[info.Channel]
	r.mu.RUnlock()

	if !ok {
		log.Printf("[adapters] reply dropped: adapter %q not registered", info.Channel)
		return nil, fmt.Errorf("adapter %q not registered", info.Channel)
	}

	return adapter.BuildReply(info)
}

// Get returns a registered adapter by name.
func (r *Registry) Get(name string) (command.Adapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.adapters[name]
	return a, ok
}

// List returns all registered adapter names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.adapters))
	for name := range r.adapters {
		names = append(names, name)
	}
	return names
}
