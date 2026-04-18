package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/NubeDev/bizzy/pkg/airunner"
	"github.com/NubeDev/bizzy/pkg/auth"
	"github.com/NubeDev/bizzy/pkg/models"
	"github.com/gin-gonic/gin"
)

// --- Global provider config (admin-only) ---

// getProviderConfig returns the global provider configuration.
//
//	GET /api/settings/providers
func (a *API) getProviderConfig(c *gin.Context) {
	cfg := a.ProviderConfigGet()

	// Build response with availability info merged in.
	type providerView struct {
		Provider  string   `json:"provider"`
		Enabled   bool     `json:"enabled"`
		Available bool     `json:"available"`
		Type      string   `json:"type"` // "cli" or "api"
		Host      string   `json:"host,omitempty"`
		HasAPIKey bool     `json:"has_api_key,omitempty"`
		Models    []string `json:"models,omitempty"`
	}

	// Get live availability from the registry.
	available := a.Runners.Available()
	availMap := make(map[string]airunner.ProviderInfo)
	for _, p := range available {
		availMap[string(p.Provider)] = p
	}

	var out []providerView
	for name, settings := range cfg.Providers {
		info := availMap[name]
		out = append(out, providerView{
			Provider:  name,
			Enabled:   settings.Enabled,
			Available: info.Available,
			Type:      info.Type,
			Host:      settings.Host,
			HasAPIKey: settings.APIKey != "",
			Models:    info.Models,
		})
	}

	c.JSON(http.StatusOK, out)
}

// updateProviderConfig updates the global provider configuration.
//
//	PUT /api/settings/providers
func (a *API) updateProviderConfig(c *gin.Context) {
	var req models.ProviderConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Merge with existing config: only update fields that are present.
	existing := a.ProviderConfigGet()
	if req.Providers != nil {
		for name, settings := range req.Providers {
			existing.Providers[name] = settings
		}
	}

	if err := a.ProviderConfigSet(existing); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save: " + err.Error()})
		return
	}

	// Apply config to runners.
	a.ApplyProviderConfig(existing)

	c.JSON(http.StatusOK, existing)
}

// applyProviderConfig pushes the config values into the runner registry.
func (a *API) ApplyProviderConfig(cfg models.ProviderConfig) {
	for name, settings := range cfg.Providers {
		runner, err := a.Runners.Get(airunner.Provider(name))
		if err != nil {
			continue
		}
		// Apply host to Ollama runner.
		if configurable, ok := runner.(airunner.Configurable); ok {
			configurable.Configure(settings.Host, settings.APIKey)
		}
	}
}

// --- User preferences ---

// getUserPreferences returns the current user's AI preferences.
//
//	GET /users/me/preferences
func (a *API) getUserPreferences(c *gin.Context) {
	user := auth.GetUser(c)
	prefs := user.Preferences
	if prefs == nil {
		prefs = &models.UserPreferences{}
	}
	c.JSON(http.StatusOK, prefs)
}

// updateUserPreferences sets the current user's AI preferences.
//
//	PUT /users/me/preferences
func (a *API) updateUserPreferences(c *gin.Context) {
	user := auth.GetUser(c)

	var prefs models.UserPreferences
	if err := c.ShouldBindJSON(&prefs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate provider exists if set.
	if prefs.DefaultProvider != "" {
		_, err := a.Runners.Get(airunner.Provider(prefs.DefaultProvider))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown provider: " + prefs.DefaultProvider})
			return
		}
	}

	user.Preferences = &prefs
	if err := a.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, prefs)
}

// --- Provider test ---

// testProvider tests connectivity to a specific provider.
//
//	POST /api/agents/providers/:name/test
func (a *API) testProvider(c *gin.Context) {
	name := c.Param("name")

	runner, err := a.Runners.Get(airunner.Provider(name))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	type testResult struct {
		Provider  string   `json:"provider"`
		Available bool     `json:"available"`
		Models    []string `json:"models,omitempty"`
		Error     string   `json:"error,omitempty"`
		LatencyMS int      `json:"latency_ms"`
	}

	start := time.Now()
	available := runner.Available()
	latency := int(time.Since(start).Milliseconds())

	result := testResult{
		Provider:  name,
		Available: available,
		LatencyMS: latency,
	}

	if !available {
		result.Error = providerUnavailableReason(name, runner)
	}

	// Get models if available.
	if lister, ok := runner.(airunner.ModelLister); ok && available {
		if models, err := lister.InstalledModels(); err == nil {
			result.Models = models
		}
	}

	c.JSON(http.StatusOK, result)
}

// providerUnavailableReason returns a human-readable reason why a provider isn't available.
func providerUnavailableReason(name string, runner airunner.Runner) string {
	switch name {
	case "claude":
		if _, err := exec.LookPath("claude"); err != nil {
			return "claude CLI not found in PATH — install from https://claude.ai/download"
		}
	case "ollama":
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost:11434/api/tags", nil)
		if _, err := http.DefaultClient.Do(req); err != nil {
			return "ollama not reachable — is it running? (ollama serve)"
		}
	case "opencode":
		if _, err := exec.LookPath("opencode"); err != nil {
			// Check default install location.
			home, _ := os.UserHomeDir()
			if home != "" {
				if _, err := os.Stat(home + "/.opencode/bin/opencode"); err == nil {
					return "" // Found at default location, it's available.
				}
			}
			return "opencode CLI not found — install with: curl -fsSL https://opencode.ai/install | bash"
		}
	case "openai":
		return "OPENAI_API_KEY not configured"
	case "anthropic":
		return "ANTHROPIC_API_KEY not configured"
	case "gemini":
		return "GEMINI_API_KEY not configured"
	}
	return fmt.Sprintf("%s is not available", name)
}
