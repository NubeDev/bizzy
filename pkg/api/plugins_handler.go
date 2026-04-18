package api

import (
	"net/http"
	"time"

	"github.com/NubeDev/bizzy/pkg/plugin"
	"github.com/NubeDev/bizzy/pkg/version"
	"github.com/gin-gonic/gin"
)

// --- Plugin REST Handlers ---
// These give admins visibility into the plugin system and control over
// plugin lifecycle. Plugins themselves never call these — they only speak NATS.

// pluginSummary is the JSON shape returned by list/detail endpoints.
type pluginSummary struct {
	Name           string     `json:"name"`
	Version        string     `json:"version"`
	Description    string     `json:"description,omitempty"`
	Services       []string   `json:"services"`
	Status         string     `json:"status"`
	RegisteredAt   time.Time  `json:"registered_at"`
	LastHeartbeat  *time.Time `json:"last_heartbeat,omitempty"`
	HealthFailures int        `json:"health_failures"`
	ToolCount      int        `json:"tool_count"`
	PromptCount    int        `json:"prompt_count"`
}

func (a *API) listPlugins(c *gin.Context) {
	if a.PluginRegistry == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "plugin system not enabled"})
		return
	}

	serviceFilter := c.Query("service")
	all := a.PluginRegistry.AllPlugins()

	out := make([]pluginSummary, 0, len(all))
	for _, p := range all {
		if serviceFilter != "" && !p.Manifest.HasService(plugin.ServiceType(serviceFilter)) {
			continue
		}
		out = append(out, toPluginSummary(p))
	}
	c.JSON(http.StatusOK, gin.H{
		"api_version": version.PluginProtocol,
		"plugins":     out,
	})
}

func (a *API) getPlugin(c *gin.Context) {
	if a.PluginRegistry == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "plugin system not enabled"})
		return
	}

	name := c.Param("name")
	p, ok := a.PluginRegistry.GetPlugin(name)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "plugin not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"api_version": version.PluginProtocol,
		"plugin":      toPluginSummary(p),
		"manifest":    p.Manifest,
	})
}

func (a *API) deletePlugin(c *gin.Context) {
	if a.PluginRegistry == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "plugin system not enabled"})
		return
	}

	name := c.Param("name")
	if err := a.PluginRegistry.ForceUnload(name); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"api_version": version.PluginProtocol,
		"status":      "unloaded",
	})
}

func (a *API) disablePlugin(c *gin.Context) {
	if a.PluginRegistry == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "plugin system not enabled"})
		return
	}

	name := c.Param("name")
	if err := a.PluginRegistry.DisablePlugin(name); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"api_version": version.PluginProtocol,
		"status":      "disabled",
	})
}

func (a *API) enablePlugin(c *gin.Context) {
	if a.PluginRegistry == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "plugin system not enabled"})
		return
	}

	name := c.Param("name")
	if err := a.PluginRegistry.EnablePlugin(name); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"api_version": version.PluginProtocol,
		"status":      "enabled",
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func toPluginSummary(p plugin.PluginState) pluginSummary {
	services := make([]string, len(p.Manifest.Services))
	for i, s := range p.Manifest.Services {
		services[i] = string(s)
	}

	var lastHB *time.Time
	if !p.LastHeartbeat.IsZero() {
		t := p.LastHeartbeat
		lastHB = &t
	}

	return pluginSummary{
		Name:           p.Manifest.Name,
		Version:        p.Manifest.Version,
		Description:    p.Manifest.Description,
		Services:       services,
		Status:         p.Status,
		RegisteredAt:   p.RegisteredAt,
		LastHeartbeat:  lastHB,
		HealthFailures: p.HealthFailures,
		ToolCount:      len(p.Manifest.Tools),
		PromptCount:    len(p.Manifest.Prompts),
	}
}
