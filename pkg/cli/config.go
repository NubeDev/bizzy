// Package cli provides the nube CLI framework.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds the CLI configuration persisted to ~/.nube/config.json.
type Config struct {
	Server string `json:"server"`
	Token  string `json:"token"`
}

// ConfigPath returns the path to the config file.
func ConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".nube", "config.json")
}

// LoadConfig reads the config from disk. Returns empty config if not found.
func LoadConfig() (*Config, error) {
	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}

// Save writes the config to disk.
func (c *Config) Save() error {
	dir := filepath.Dir(ConfigPath())
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(c, "", "  ")
	return os.WriteFile(ConfigPath(), data, 0600)
}

// Validate checks that server and token are set.
func (c *Config) Validate() error {
	if c.Server == "" {
		return fmt.Errorf("not logged in — run: nube login")
	}
	if c.Token == "" {
		return fmt.Errorf("no token — run: nube login")
	}
	return nil
}
