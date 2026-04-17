package models

// ProviderConfig holds the global (admin-managed) configuration for all AI providers.
// Stored as a single JSON file, not a collection — there's only one config.
type ProviderConfig struct {
	Providers map[string]ProviderSettings `json:"providers"`
}

// ProviderSettings holds the configuration for a single provider.
type ProviderSettings struct {
	Enabled bool   `json:"enabled"`
	APIKey  string `json:"api_key,omitempty"`  // OpenAI, Anthropic, Gemini
	Host    string `json:"host,omitempty"`     // Ollama (default http://localhost:11434)
}

// DefaultProviderConfig returns a config with sensible defaults.
func DefaultProviderConfig() ProviderConfig {
	return ProviderConfig{
		Providers: map[string]ProviderSettings{
			"claude":    {Enabled: true},
			"ollama":    {Enabled: true, Host: "http://localhost:11434"},
			"openai":    {Enabled: false},
			"anthropic": {Enabled: false},
			"gemini":    {Enabled: false},
		},
	}
}
