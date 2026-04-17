package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

// namePattern is the allowed format for plugin and tool names.
var namePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,62}$`)

// LoadManifestFile reads a plugin.yaml from disk and returns a validated Manifest.
func LoadManifestFile(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	return ParseManifest(data)
}

// ParseManifest parses YAML (or JSON) bytes into a validated Manifest.
// JSON is valid YAML, so this handles both formats.
func ParseManifest(data []byte) (*Manifest, error) {
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if err := ValidateManifest(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

// ManifestFromJSON parses a JSON-encoded manifest (e.g. from a NATS message).
func ManifestFromJSON(data []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest json: %w", err)
	}
	if err := ValidateManifest(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

// ValidateManifest checks that all required fields are present and consistent.
func ValidateManifest(m *Manifest) error {
	if m.Name == "" {
		return fmt.Errorf("manifest: name is required")
	}
	if !namePattern.MatchString(m.Name) {
		return fmt.Errorf("manifest: name %q must match %s", m.Name, namePattern.String())
	}
	if m.Version == "" {
		return fmt.Errorf("manifest: version is required")
	}
	if len(m.Services) == 0 {
		return fmt.Errorf("manifest: at least one service is required")
	}
	for _, s := range m.Services {
		if !ValidServiceTypes[s] {
			return fmt.Errorf("manifest: unknown service type %q", s)
		}
	}

	// Cross-validate: service declarations must have matching config.
	if m.HasService(ServiceTools) && len(m.Tools) == 0 {
		return fmt.Errorf("manifest: services includes %q but no tools defined", ServiceTools)
	}
	if m.HasService(ServicePrompts) && len(m.Prompts) == 0 {
		return fmt.Errorf("manifest: services includes %q but no prompts defined", ServicePrompts)
	}
	if m.HasService(ServiceWorkflows) && len(m.Workflows) == 0 {
		return fmt.Errorf("manifest: services includes %q but no workflows defined", ServiceWorkflows)
	}
	if m.HasService(ServiceAdapter) && m.Adapter == nil {
		return fmt.Errorf("manifest: services includes %q but no adapter config defined", ServiceAdapter)
	}

	// Validate tool names.
	for i, t := range m.Tools {
		if t.Name == "" {
			return fmt.Errorf("manifest: tool[%d] name is required", i)
		}
		if !namePattern.MatchString(t.Name) {
			return fmt.Errorf("manifest: tool %q name must match %s", t.Name, namePattern.String())
		}
	}

	// Validate prompt names.
	for i, p := range m.Prompts {
		if p.Name == "" {
			return fmt.Errorf("manifest: prompt[%d] name is required", i)
		}
		if p.Template == "" {
			return fmt.Errorf("manifest: prompt %q template is required", p.Name)
		}
	}

	return nil
}

// MarshalManifest encodes a Manifest to JSON for NATS transmission or DB storage.
func MarshalManifest(m *Manifest) ([]byte, error) {
	return json.Marshal(m)
}
