package apps

import (
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/NubeDev/bizzy/pkg/models"
	"gorm.io/gorm"
)

// Registry holds all available apps loaded from disk and the database.
type Registry struct {
	mu       sync.RWMutex
	db       *gorm.DB
	appsDirs []string
	apps     map[string]*App           // keyed by app name
	prompts  map[string][]Prompt       // keyed by app name
	tools    map[string][]ToolManifest // keyed by app name
}

// NewRegistry scans the apps directories and loads store apps from the database.
// The first directory is the "primary" used for system app CRUD.
func NewRegistry(db *gorm.DB, appsDirs ...string) (*Registry, error) {
	r := &Registry{
		db:       db,
		appsDirs: appsDirs,
		apps:     make(map[string]*App),
		prompts:  make(map[string][]Prompt),
		tools:    make(map[string][]ToolManifest),
	}
	if err := r.scan(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Registry) scan() error {
	for _, dir := range r.appsDirs {
		r.scanDir(dir)
	}
	r.loadStoreApps()
	return nil
}

func (r *Registry) scanDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("[apps] no apps directory at %s — skipping", dir)
			return
		}
		log.Printf("[apps] error reading %s: %v", dir, err)
		return
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		appDir := dir + "/" + e.Name()
		app, err := LoadApp(appDir)
		if err != nil {
			log.Printf("[apps] skipping %s: %v", e.Name(), err)
			continue
		}
		// First directory wins on name collisions.
		if _, exists := r.apps[app.Name]; exists {
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
}

// loadStoreApps queries the database for store apps and merges them into the
// registry. Disk apps take priority — only store apps whose name doesn't
// already exist in the registry are added.
func (r *Registry) loadStoreApps() {
	if r.db == nil {
		return
	}

	var storeApps []models.StoreApp
	if err := r.db.Find(&storeApps).Error; err != nil {
		log.Printf("[apps] failed to load store apps from DB: %v", err)
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	added := 0
	for _, sa := range storeApps {
		// Disk apps win on name collisions.
		if _, exists := r.apps[sa.Name]; exists {
			continue
		}

		// Build an App from the StoreApp.
		app := &App{
			Name:        sa.Name,
			Version:     sa.Version,
			Description: sa.Description,
			Author:      sa.AuthorName,
			Permissions: Permissions{
				AllowedHosts:     sa.Permissions.AllowedHosts,
				DefaultToolClass: sa.Permissions.DefaultToolClass,
				Secrets:          sa.Permissions.Secrets,
			},
			HasTools:   len(sa.Tools) > 0,
			HasPrompts: len(sa.Prompts) > 0,
		}
		// Convert settings.
		for _, s := range sa.Settings {
			app.Settings = append(app.Settings, SettingDef{
				Key:      s.Key,
				Label:    s.Label,
				Type:     s.Type,
				Required: s.Required,
				Default:  s.Default,
			})
		}

		r.apps[sa.Name] = app

		// Convert tools.
		if len(sa.Tools) > 0 {
			var manifests []ToolManifest
			for _, st := range sa.Tools {
				m := ToolManifest{
					Name:        st.Name,
					Description: st.Description,
					ToolClass:   st.ToolClass,
					Mode:        st.Mode,
					Script:      st.Script,
					AppName:     sa.Name,
					Params:      make(map[string]ToolParamDef),
				}
				if m.ToolClass == "" {
					m.ToolClass = sa.Permissions.DefaultToolClass
				}
				for pName, p := range st.Params {
					m.Params[pName] = ToolParamDef{
						Type:        p.Type,
						Required:    p.Required,
						Description: p.Description,
						Options:     p.Options,
					}
				}
				manifests = append(manifests, m)
			}
			r.tools[sa.Name] = manifests
		}

		// Convert prompts.
		if len(sa.Prompts) > 0 {
			var prompts []Prompt
			for _, sp := range sa.Prompts {
				p := Prompt{
					Name:        sp.Name,
					Description: sp.Description,
					Body:        sp.Body,
					AppName:     sa.Name,
				}
				for _, arg := range sp.Arguments {
					p.Arguments = append(p.Arguments, PromptArgument{
						Name:        arg.Name,
						Description: arg.Description,
						Required:    arg.Required,
					})
				}
				prompts = append(prompts, p)
			}
			r.prompts[sa.Name] = prompts
		}

		added++
		log.Printf("[apps] loaded from DB: %s v%s (prompts=%d jstools=%d)",
			sa.Name, sa.Version, len(sa.Prompts), len(sa.Tools))
	}

	if added > 0 {
		log.Printf("[apps] loaded %d store apps from DB", added)
	}
}

// Reload re-scans the apps directories. Safe for concurrent use.
func (r *Registry) Reload() error {
	newApps := make(map[string]*App)
	newPrompts := make(map[string][]Prompt)
	newTools := make(map[string][]ToolManifest)

	for _, dir := range r.appsDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("read apps dir %s: %w", dir, err)
		}

		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			appDir := dir + "/" + e.Name()
			app, err := LoadApp(appDir)
			if err != nil {
				log.Printf("[apps] reload: skipping %s: %v", e.Name(), err)
				continue
			}
			// First directory wins.
			if _, exists := newApps[app.Name]; exists {
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
	}

	r.mu.Lock()
	r.apps = newApps
	r.prompts = newPrompts
	r.tools = newTools
	r.mu.Unlock()

	// Layer DB apps on top (disk apps win on collision).
	r.loadStoreApps()

	log.Printf("[apps] reloaded: %d apps", len(r.apps))
	return nil
}

// AppsDir returns the primary directory (first) where system apps are stored.
func (r *Registry) AppsDir() string {
	if len(r.appsDirs) == 0 {
		return ""
	}
	return r.appsDirs[0]
}

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
