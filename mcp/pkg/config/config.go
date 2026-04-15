// Package config provides configuration for the OpenAPI-MCP server.
package config

import (
	"fmt"
	"os"
	"strings"
)

// Config holds all configuration for an OpenAPI-MCP server instance.
type Config struct {
	// Name is the MCP server name advertised to clients.
	Name string `yaml:"name" json:"name"`
	// Version is the MCP server version.
	Version string `yaml:"version" json:"version"`
	// Spec is the path (or URL) to the OpenAPI spec file.
	Spec string `yaml:"spec" json:"spec"`
	// BaseURL is the upstream API base URL.
	BaseURL string `yaml:"baseUrl" json:"baseUrl"`
	// Token is the bearer token for upstream API auth.
	Token string `yaml:"token" json:"token"`
	// APIKey is the API key for upstream API auth.
	APIKey string `yaml:"apiKey" json:"apiKey"`
	// APIKeyHeader is the header name for the API key.
	APIKeyHeader string `yaml:"apiKeyHeader" json:"apiKeyHeader"`
	// Transport is the MCP transport: "stdio" or "http".
	Transport string `yaml:"transport" json:"transport"`
	// Addr is the HTTP listen address (e.g. ":8080").
	Addr string `yaml:"addr" json:"addr"`
	// Plugins is the directory containing JS plugins.
	Plugins string `yaml:"plugins" json:"plugins"`
	// LogHTTP enables HTTP request/response logging.
	LogHTTP bool `yaml:"logHttp" json:"logHttp"`
	// Tags filters operations by OpenAPI tag.
	Tags []string `yaml:"tags" json:"tags"`
	// ReadOnly if true, only registers GET operations.
	ReadOnly bool `yaml:"readOnly" json:"readOnly"`
	// ConfirmDangerous requires confirmation for PUT/POST/DELETE.
	ConfirmDangerous bool `yaml:"confirmDangerous" json:"confirmDangerous"`
	// DefaultParams are injected into every tool call when the param exists in the operation schema.
	DefaultParams map[string]string `yaml:"defaultParams" json:"defaultParams"`
}

// envPrefix is the prefix for environment variable overrides.
const envPrefix = "NUBE_MCP_"

// envMap maps env var suffixes to Config field setters.
var envMap = []struct {
	suffix string
	apply  func(*Config, string)
}{
	{"NAME", func(c *Config, v string) { c.Name = v }},
	{"VERSION", func(c *Config, v string) { c.Version = v }},
	{"SPEC", func(c *Config, v string) { c.Spec = v }},
	{"BASE_URL", func(c *Config, v string) { c.BaseURL = v }},
	{"TOKEN", func(c *Config, v string) { c.Token = v }},
	{"API_KEY", func(c *Config, v string) { c.APIKey = v }},
	{"API_KEY_HEADER", func(c *Config, v string) { c.APIKeyHeader = v }},
	{"TRANSPORT", func(c *Config, v string) { c.Transport = v }},
	{"ADDR", func(c *Config, v string) { c.Addr = v }},
	{"PLUGINS", func(c *Config, v string) { c.Plugins = v }},
	{"LOG_HTTP", func(c *Config, v string) { c.LogHTTP = v == "1" || v == "true" }},
	{"TAGS", func(c *Config, v string) { c.Tags = strings.Split(v, ",") }},
	{"READ_ONLY", func(c *Config, v string) { c.ReadOnly = v == "1" || v == "true" }},
	{"CONFIRM_DANGEROUS", func(c *Config, v string) { c.ConfirmDangerous = v == "1" || v == "true" }},
}

// ApplyEnv overrides Config fields from environment variables.
// Environment variables use the prefix NUBE_MCP_ (e.g. NUBE_MCP_SPEC, NUBE_MCP_BASE_URL).
func (c *Config) ApplyEnv() {
	for _, e := range envMap {
		if v := os.Getenv(envPrefix + e.suffix); v != "" {
			e.apply(c, v)
		}
	}
	// Also honor the legacy env vars used by the core library.
	if v := os.Getenv("OPENAPI_BASE_URL"); v != "" && c.BaseURL == "" {
		c.BaseURL = v
	}
	if v := os.Getenv("BEARER_TOKEN"); v != "" && c.Token == "" {
		c.Token = v
	}
	if v := os.Getenv("API_KEY"); v != "" && c.APIKey == "" {
		c.APIKey = v
	}
}

// SetAuthEnv propagates Config auth fields to the environment variables
// expected by the core openapi2mcp library.
func (c *Config) SetAuthEnv() {
	if c.Token != "" {
		os.Setenv("BEARER_TOKEN", c.Token)
	}
	if c.APIKey != "" {
		os.Setenv("API_KEY", c.APIKey)
	}
	if c.APIKeyHeader != "" {
		os.Setenv("API_KEY_HEADER", c.APIKeyHeader)
	}
	if c.BaseURL != "" {
		os.Setenv("OPENAPI_BASE_URL", c.BaseURL)
	}
	if c.LogHTTP {
		os.Setenv("MCP_LOG_HTTP", "1")
	}
}

// Defaults fills in zero-value fields with sensible defaults.
func (c *Config) Defaults() {
	if c.Transport == "" {
		c.Transport = "stdio"
	}
	if c.Addr == "" {
		c.Addr = ":8080"
	}
	if c.Name == "" {
		c.Name = "openapi-mcp"
	}
	if c.Version == "" {
		c.Version = "0.1.0"
	}
}

// Validate checks that required fields are set.
func (c *Config) Validate() error {
	if c.Spec == "" {
		return fmt.Errorf("spec path is required (set via --spec flag or NUBE_MCP_SPEC env)")
	}
	return nil
}
