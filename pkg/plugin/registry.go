package plugin

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/NubeDev/bizzy/pkg/models"
	"github.com/NubeDev/bizzy/pkg/version"
	"github.com/nats-io/nats.go"
	"gorm.io/gorm"
)

// RegistryConfig holds the dependencies for a plugin registry.
type RegistryConfig struct {
	NC            *nats.Conn
	DB            *gorm.DB
	HealthCfg     HealthConfig
	ToolTimeout   time.Duration // default timeout for plugin tool calls
}

// Registry manages plugin registration, lifecycle, and tool routing.
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]*PluginState

	nc    *nats.Conn
	db    *gorm.DB
	proxy *Proxy
	health *HealthMonitor

	regSub   *nats.Subscription
	deregSub *nats.Subscription
}

// NewRegistry creates a plugin registry. Call Start() to begin listening.
func NewRegistry(cfg RegistryConfig) *Registry {
	r := &Registry{
		plugins: make(map[string]*PluginState),
		nc:      cfg.NC,
		db:      cfg.DB,
		proxy:   NewProxy(cfg.NC, cfg.ToolTimeout),
	}
	r.health = NewHealthMonitor(cfg.NC, r.plugins, &r.mu, cfg.HealthCfg, r.onPluginCrashed)
	return r
}

// Start subscribes to NATS registration subjects, reloads persisted plugins
// from the database, and begins health monitoring.
func (r *Registry) Start() error {
	// Reload previously-active plugins from DB.
	r.reloadFromDB()

	// Subscribe to registration (request/reply so plugin gets an immediate ack).
	var err error
	r.regSub, err = r.nc.Subscribe(SubjectRegister, r.handleRegister)
	if err != nil {
		return fmt.Errorf("subscribe %s: %w", SubjectRegister, err)
	}

	r.deregSub, err = r.nc.Subscribe(SubjectDeregister, r.handleDeregister)
	if err != nil {
		return fmt.Errorf("subscribe %s: %w", SubjectDeregister, err)
	}

	if err := r.health.Start(); err != nil {
		return fmt.Errorf("start health monitor: %w", err)
	}

	log.Printf("[plugins] registry started (%d plugins loaded from db)", len(r.plugins))
	return nil
}

// Stop unsubscribes from NATS and stops the health monitor.
func (r *Registry) Stop() {
	if r.regSub != nil {
		r.regSub.Unsubscribe()
	}
	if r.deregSub != nil {
		r.deregSub.Unsubscribe()
	}
	r.health.Stop()
}

// Proxy returns the tool call proxy for executing plugin tools.
func (r *Registry) Proxy() *Proxy {
	return r.proxy
}

// ---------------------------------------------------------------------------
// Queries — thread-safe accessors used by MCPFactory, REST API, etc.
// ---------------------------------------------------------------------------

// ActivePlugins returns all plugins with status "active".
func (r *Registry) ActivePlugins() []PluginState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]PluginState, 0, len(r.plugins))
	for _, p := range r.plugins {
		if p.Status == "active" {
			out = append(out, *p)
		}
	}
	return out
}

// AllPlugins returns every registered plugin regardless of status.
func (r *Registry) AllPlugins() []PluginState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]PluginState, 0, len(r.plugins))
	for _, p := range r.plugins {
		out = append(out, *p)
	}
	return out
}

// GetPlugin returns a single plugin by name.
func (r *Registry) GetPlugin(name string) (PluginState, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.plugins[name]
	if !ok {
		return PluginState{}, false
	}
	return *p, true
}

// ActivePluginsByService returns active plugins that declare the given service type.
// If serviceFilter is empty, all active plugins are returned.
func (r *Registry) ActivePluginsByService(serviceFilter string) []PluginState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]PluginState, 0, len(r.plugins))
	for _, p := range r.plugins {
		if p.Status != "active" {
			continue
		}
		if serviceFilter != "" && !p.Manifest.HasService(ServiceType(serviceFilter)) {
			continue
		}
		out = append(out, *p)
	}
	return out
}

// ActiveTools returns all tools from active plugins, namespaced as
// "plugin.<pluginName>.<toolName>".
func (r *Registry) ActiveTools() []NamespacedTool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []NamespacedTool
	for _, p := range r.plugins {
		if p.Status != "active" {
			continue
		}
		for _, t := range p.Manifest.Tools {
			out = append(out, NamespacedTool{
				FullName:   "plugin." + p.Manifest.Name + "." + t.Name,
				PluginName: p.Manifest.Name,
				Spec:       t,
			})
		}
	}
	return out
}

// NamespacedTool pairs a tool spec with its fully-qualified name.
type NamespacedTool struct {
	FullName   string
	PluginName string
	Spec       ToolSpec
}

// ActivePrompts returns all prompts from active plugins, namespaced.
func (r *Registry) ActivePrompts() []NamespacedPrompt {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []NamespacedPrompt
	for _, p := range r.plugins {
		if p.Status != "active" {
			continue
		}
		for _, pr := range p.Manifest.Prompts {
			out = append(out, NamespacedPrompt{
				FullName:   "plugin." + p.Manifest.Name + "." + pr.Name,
				PluginName: p.Manifest.Name,
				Spec:       pr,
			})
		}
	}
	return out
}

// NamespacedPrompt pairs a prompt spec with its fully-qualified name.
type NamespacedPrompt struct {
	FullName   string
	PluginName string
	Spec       PromptSpec
}

// ---------------------------------------------------------------------------
// Admin operations — called by REST API handlers.
// ---------------------------------------------------------------------------

// DisablePlugin marks a plugin as disabled and removes its tools.
func (r *Registry) DisablePlugin(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.plugins[name]
	if !ok {
		return fmt.Errorf("plugin %q not found", name)
	}
	p.Status = "disabled"
	r.persistPlugin(p)
	log.Printf("[plugins] %s: disabled", name)
	return nil
}

// EnablePlugin re-enables a disabled plugin and restores its tools.
func (r *Registry) EnablePlugin(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.plugins[name]
	if !ok {
		return fmt.Errorf("plugin %q not found", name)
	}
	p.Status = "active"
	p.HealthFailures = 0
	p.LastHeartbeat = time.Now()
	r.persistPlugin(p)
	log.Printf("[plugins] %s: re-enabled", name)
	return nil
}

// ForceUnload removes a plugin entirely.
func (r *Registry) ForceUnload(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.plugins[name]; !ok {
		return fmt.Errorf("plugin %q not found", name)
	}
	delete(r.plugins, name)
	r.db.Delete(&models.Plugin{}, "name = ?", name)
	log.Printf("[plugins] %s: force unloaded", name)
	return nil
}

// ---------------------------------------------------------------------------
// NATS handlers
// ---------------------------------------------------------------------------

func (r *Registry) handleRegister(msg *nats.Msg) {
	var req RegisterRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		r.reply(msg, RegisterResponse{
			APIVersion: version.PluginProtocol,
			Status:     "error",
			Error:      "invalid json: " + err.Error(),
		})
		return
	}

	m := &req.Manifest
	if err := ValidateManifest(m); err != nil {
		r.reply(msg, RegisterResponse{
			APIVersion: version.PluginProtocol,
			Status:     "error",
			Error:      err.Error(),
		})
		return
	}

	r.mu.Lock()
	_, isReload := r.plugins[m.Name]

	now := time.Now()
	state := &PluginState{
		Manifest:      *m,
		Status:        "active",
		RegisteredAt:  now,
		LastHeartbeat: now, // grace: treat registration as a heartbeat
	}
	r.plugins[m.Name] = state
	r.persistPlugin(state)
	r.mu.Unlock()

	action := "registered"
	if isReload {
		action = "reloaded"
	}
	log.Printf("[plugins] %s: %s (v%s, services=%v, tools=%d)",
		m.Name, action, m.Version, m.Services, len(m.Tools))

	r.reply(msg, RegisterResponse{
		APIVersion:      version.PluginProtocol,
		Status:          "ok",
		ToolsRegistered: len(m.Tools),
		Reloaded:        isReload,
	})
}

func (r *Registry) handleDeregister(msg *nats.Msg) {
	var req DeregisterRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		return
	}

	r.mu.Lock()
	if _, ok := r.plugins[req.Name]; ok {
		delete(r.plugins, req.Name)
		r.db.Delete(&models.Plugin{}, "name = ?", req.Name)
		log.Printf("[plugins] %s: deregistered (clean shutdown)", req.Name)
	}
	r.mu.Unlock()
}

// ---------------------------------------------------------------------------
// Crash callback — called by HealthMonitor while holding mu.
// ---------------------------------------------------------------------------

func (r *Registry) onPluginCrashed(name string) {
	// Persist the crashed status. The health monitor already set the state
	// under lock, so we just need to write to DB.
	if p, ok := r.plugins[name]; ok {
		r.persistPlugin(p)
	}
}

// ---------------------------------------------------------------------------
// Persistence helpers
// ---------------------------------------------------------------------------

func (r *Registry) persistPlugin(p *PluginState) {
	manifestJSON, _ := MarshalManifest(&p.Manifest)
	servicesJSON, _ := json.Marshal(p.Manifest.Services)

	var lastHB *time.Time
	if !p.LastHeartbeat.IsZero() {
		t := p.LastHeartbeat
		lastHB = &t
	}

	record := models.Plugin{
		Name:           p.Manifest.Name,
		Version:        p.Manifest.Version,
		Description:    p.Manifest.Description,
		Services:       string(servicesJSON),
		Manifest:       string(manifestJSON),
		Status:         models.PluginStatus(p.Status),
		RegisteredAt:   p.RegisteredAt,
		LastHeartbeat:  lastHB,
		HealthFailures: p.HealthFailures,
	}
	r.db.Save(&record)
}

func (r *Registry) reloadFromDB() {
	var records []models.Plugin
	r.db.Where("status IN ?", []string{"active", "disabled"}).Find(&records)

	now := time.Now()
	for _, rec := range records {
		var m Manifest
		if err := json.Unmarshal([]byte(rec.Manifest), &m); err != nil {
			log.Printf("[plugins] reload: skipping %s (bad manifest): %v", rec.Name, err)
			continue
		}

		state := &PluginState{
			Manifest:       m,
			Status:         string(rec.Status),
			RegisteredAt:   rec.RegisteredAt,
			LastHeartbeat:  now, // grace period: don't immediately mark as stale
			HealthFailures: 0,  // reset on restart
		}
		r.plugins[m.Name] = state
	}
}

func (r *Registry) reply(msg *nats.Msg, resp RegisterResponse) {
	if msg.Reply == "" {
		// Fire-and-forget publish — no reply subject. Publish ack on the
		// well-known subject instead.
		data, _ := json.Marshal(resp)
		// We need the plugin name. Try to extract it from the request.
		var req RegisterRequest
		json.Unmarshal(msg.Data, &req)
		if req.Name != "" {
			r.nc.Publish(SubjectRegisteredPrefix+req.Name, data)
		}
		return
	}
	data, _ := json.Marshal(resp)
	msg.Respond(data)
}
