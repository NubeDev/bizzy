package models

// ProviderConfig holds the global (admin-managed) configuration for all AI providers.
// Stored as a single row in the configs table.
type ProviderConfig struct {
	ID        string                         `json:"-" gorm:"primaryKey"`
	Providers map[string]ProviderSettings    `json:"providers" gorm:"serializer:json"`
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
		ID: "default",
		Providers: map[string]ProviderSettings{
			"claude":    {Enabled: true},
			"ollama":    {Enabled: true, Host: "http://localhost:11434"},
			"opencode":  {Enabled: true},
			"openai":    {Enabled: false},
			"anthropic": {Enabled: false},
			"gemini":    {Enabled: false},
		},
	}
}
