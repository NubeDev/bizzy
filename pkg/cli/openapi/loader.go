// Package openapi maps an OpenAPI spec to cobra commands.
package openapi

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Spec is a minimal OpenAPI 3.0 parse — only the fields we need for CLI generation.
type Spec struct {
	Info  SpecInfo            `yaml:"info"`
	Paths map[string]PathItem `yaml:"paths"`
}

type SpecInfo struct {
	Title   string `yaml:"title"`
	Version string `yaml:"version"`
}

type PathItem map[string]Operation // keyed by HTTP method (get, post, etc.)

type Operation struct {
	OperationID string      `yaml:"operationId"`
	Summary     string      `yaml:"summary"`
	Parameters  []Parameter `yaml:"parameters"`
	RequestBody *ReqBody    `yaml:"requestBody"`
	CLI         *CLIConfig  `yaml:"x-cli"`
	Security    any         `yaml:"security"` // nil means use global, [] means no auth
}

type Parameter struct {
	Name        string `yaml:"name"`
	In          string `yaml:"in"` // path, query, header
	Required    bool   `yaml:"required"`
	Description string `yaml:"description"`
	Schema      Schema `yaml:"schema"`
}

type ReqBody struct {
	Required bool               `yaml:"required"`
	Content  map[string]Content `yaml:"content"`
}

type Content struct {
	Schema Schema `yaml:"schema"`
}

type Schema struct {
	Type       string            `yaml:"type"`
	Required   []string          `yaml:"required"`
	Properties map[string]Schema `yaml:"properties"`
	Enum       []string          `yaml:"enum"`
	Default    any               `yaml:"default"`
	Desc       string            `yaml:"description"`
	Items      *Schema           `yaml:"items"`
	Additional *Schema           `yaml:"additionalProperties"`
}

// CLIConfig is the x-cli extension on each operation.
type CLIConfig struct {
	Command string `yaml:"command"` // e.g. "apps list", "users me"
}

// CLIOp is a parsed operation ready for command registration.
type CLIOp struct {
	Command     string // e.g. "apps list"
	Method      string // GET, POST, etc.
	Path        string // /apps/{id}
	Summary     string
	OperationID string
	PathParams  []Parameter
	QueryParams []Parameter
	BodyFields  []BodyField
	NoAuth      bool
}

// BodyField is a field from the request body schema.
type BodyField struct {
	Name     string
	Type     string // string, boolean, number, object
	Required bool
	Desc     string
	Enum     []string
	Default  any
}

// LoadSpec parses an OpenAPI YAML file and extracts CLI operations.
func LoadSpec(path string) ([]CLIOp, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read spec: %w", err)
	}
	return ParseSpec(data)
}

// ParseSpec parses OpenAPI YAML bytes and extracts CLI operations.
func ParseSpec(data []byte) ([]CLIOp, error) {
	var spec Spec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("parse spec: %w", err)
	}

	var ops []CLIOp
	for path, item := range spec.Paths {
		for method, op := range item {
			if op.CLI == nil || op.CLI.Command == "" {
				continue
			}

			cliOp := CLIOp{
				Command:     op.CLI.Command,
				Method:      strings.ToUpper(method),
				Path:        path,
				Summary:     op.Summary,
				OperationID: op.OperationID,
			}

			// Check if no auth required.
			if op.Security != nil {
				if arr, ok := op.Security.([]any); ok && len(arr) == 0 {
					cliOp.NoAuth = true
				}
			}

			// Separate path and query params.
			for _, p := range op.Parameters {
				switch p.In {
				case "path":
					cliOp.PathParams = append(cliOp.PathParams, p)
				case "query":
					cliOp.QueryParams = append(cliOp.QueryParams, p)
				}
			}

			// Extract body fields.
			if op.RequestBody != nil {
				if content, ok := op.RequestBody.Content["application/json"]; ok {
					reqSet := make(map[string]bool)
					for _, r := range content.Schema.Required {
						reqSet[r] = true
					}
					for name, prop := range content.Schema.Properties {
						bf := BodyField{
							Name:     name,
							Type:     prop.Type,
							Required: reqSet[name],
							Desc:     prop.Desc,
							Enum:     prop.Enum,
							Default:  prop.Default,
						}
						cliOp.BodyFields = append(cliOp.BodyFields, bf)
					}
					// Sort for stable flag ordering.
					sort.Slice(cliOp.BodyFields, func(i, j int) bool {
						return cliOp.BodyFields[i].Name < cliOp.BodyFields[j].Name
					})
				}
			}

			ops = append(ops, cliOp)
		}
	}

	// Sort by command name for stable ordering.
	sort.Slice(ops, func(i, j int) bool {
		return ops[i].Command < ops[j].Command
	})

	return ops, nil
}
