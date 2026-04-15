package apps

import (
	"fmt"
	"log"
	"os"
	"sync"
)

// Registry holds all available apps loaded from disk.
type Registry struct {
	mu      sync.RWMutex
	appsDir string
	apps    map[string]*App            // keyed by app name
	prompts map[string][]Prompt        // keyed by app name
	tools   map[string][]ToolManifest  // keyed by app name
}

// NewRegistry scans the apps directory and loads all valid apps.
func NewRegistry(appsDir string) (*Registry, error) {
	r := &Registry{
		appsDir: appsDir,
		apps:    make(map[string]*App),
		prompts: make(map[string][]Prompt),
		tools:   make(map[string][]ToolManifest),
	}
	if err := r.scan(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Registry) scan() error {
	entries, err := os.ReadDir(r.appsDir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("[apps] no apps directory at %s — starting with zero apps", r.appsDir)
			return nil
		}
		return fmt.Errorf("read apps dir: %w", err)
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := r.appsDir + "/" + e.Name()
		app, err := LoadApp(dir)
		if err != nil {
			log.Printf("[apps] skipping %s: %v", e.Name(), err)
			continue
		}
		r.apps[app.Name] = app

		// Load prompts.
		if app.HasPrompts {
			prompts, err := LoadPrompts(app)
			if err != nil {
				log.Printf("[apps] %s: failed to load prompts: %v", app.Name, err)
			} else {
				r.prompts[app.Name] = prompts
			}
		}

		// Load JS tool manifests.
		if app.HasTools {
			tools, err := LoadToolManifests(app)
			if err != nil {
				log.Printf("[apps] %s: failed to load tools: %v", app.Name, err)
			} else {
				r.tools[app.Name] = tools
			}
		}

		log.Printf("[apps] loaded: %s v%s (openapi=%v prompts=%d jstools=%d)",
			app.Name, app.Version, app.HasOpenAPI, len(r.prompts[app.Name]), len(r.tools[app.Name]))
	}
	return nil
}

// Reload re-scans the apps directory. Safe for concurrent use.
func (r *Registry) Reload() error {
	newApps := make(map[string]*App)
	newPrompts := make(map[string][]Prompt)
	newTools := make(map[string][]ToolManifest)

	entries, err := os.ReadDir(r.appsDir)
	if err != nil {
		return fmt.Errorf("read apps dir: %w", err)
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := r.appsDir + "/" + e.Name()
		app, err := LoadApp(dir)
		if err != nil {
			log.Printf("[apps] reload: skipping %s: %v", e.Name(), err)
			continue
		}
		newApps[app.Name] = app
		if app.HasPrompts {
			prompts, err := LoadPrompts(app)
			if err != nil {
				log.Printf("[apps] reload: %s: prompts: %v", app.Name, err)
			} else {
				newPrompts[app.Name] = prompts
			}
		}
		if app.HasTools {
			tools, err := LoadToolManifests(app)
			if err != nil {
				log.Printf("[apps] reload: %s: tools: %v", app.Name, err)
			} else {
				newTools[app.Name] = tools
			}
		}
	}

	r.mu.Lock()
	r.apps = newApps
	r.prompts = newPrompts
	r.tools = newTools
	r.mu.Unlock()

	log.Printf("[apps] reloaded: %d apps", len(newApps))
	return nil
}

// AppsDir returns the root directory where apps are stored.
func (r *Registry) AppsDir() string { return r.appsDir }

// List returns all available apps.
func (r *Registry) List() []*App {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*App, 0, len(r.apps))
	for _, app := range r.apps {
		out = append(out, app)
	}
	return out
}

// Get returns an app by name.
func (r *Registry) Get(name string) (*App, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	app, ok := r.apps[name]
	return app, ok
}

// GetPrompts returns prompts for an app.
func (r *Registry) GetPrompts(appName string) []Prompt {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.prompts[appName]
}

// GetTools returns JS tool manifests for an app.
func (r *Registry) GetTools(appName string) []ToolManifest {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tools[appName]
}
