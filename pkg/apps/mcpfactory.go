package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	openapi2mcp "github.com/NubeIO/openapi-mcp"
	"github.com/NubeDev/bizzy/pkg/models"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPFactory builds per-user MCP servers with only the tools from their installed apps.
type MCPFactory struct {
	registry *Registry
	// Cached parsed OpenAPI docs per app name.
	specCache map[string]*specData
}

type specData struct {
	doc *openapi3.T
	ops []openapi2mcp.OpenAPIOperation
}

// NewMCPFactory creates a factory that builds per-user MCP servers.
func NewMCPFactory(registry *Registry) *MCPFactory {
	f := &MCPFactory{
		registry:  registry,
		specCache: make(map[string]*specData),
	}
	f.preloadSpecs()
	return f
}

func (f *MCPFactory) preloadSpecs() {
	for _, app := range f.registry.List() {
		if !app.HasOpenAPI {
			continue
		}
		specPath := app.Dir + "/openapi.yaml"
		if !fileExists(specPath) {
			specPath = app.Dir + "/openapi.json"
		}
		doc, err := openapi2mcp.LoadOpenAPISpec(specPath)
		if err != nil {
			log.Printf("[mcpfactory] %s: failed to load openapi spec: %v", app.Name, err)
			continue
		}
		ops := openapi2mcp.ExtractOpenAPIOperations(doc)
		f.specCache[app.Name] = &specData{doc: doc, ops: ops}
		log.Printf("[mcpfactory] %s: loaded %d operations from OpenAPI spec", app.Name, len(ops))
	}
}

// Rebuild re-caches specs after a registry reload.
func (f *MCPFactory) Rebuild() {
	f.specCache = make(map[string]*specData)
	f.preloadSpecs()
}

// BuildServer creates an MCP server with tools scoped to the user's installed apps.
// The registry now contains both system apps and store apps, so no store fallback is needed.
func (f *MCPFactory) BuildServer(installs []models.AppInstall) *mcp.Server {
	impl := &mcp.Implementation{Name: "nube-server", Version: "0.1.0"}
	srv := mcp.NewServer(impl, nil)

	for _, install := range installs {
		if !install.Enabled {
			continue
		}

		app, ok := f.registry.Get(install.AppName)
		if !ok {
			continue
		}

		if app.HasOpenAPI {
			f.registerOpenAPITools(srv, app, install)
		}
		if app.HasTools {
			f.registerJSTools(srv, app, install)
		}
		f.registerPrompts(srv, app)
	}

	return srv
}

func (f *MCPFactory) registerOpenAPITools(srv *mcp.Server, app *App, install models.AppInstall) {
	sd, ok := f.specCache[app.Name]
	if !ok {
		return
	}

	// Determine base URL from user's settings.
	baseURL := ""
	for _, def := range app.Settings {
		if def.Type == "url" {
			baseURL = install.GetSetting(def.Key)
			break
		}
	}

	// Fallback to spec's servers.
	if baseURL == "" && len(sd.doc.Servers) > 0 {
		baseURL = sd.doc.Servers[0].URL
	}
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	// Build a request handler that injects the user's auth token.
	token := ""
	for _, def := range app.Settings {
		if def.Type == "secret" {
			token = install.GetSetting(def.Key)
			break
		}
	}

	opts := &openapi2mcp.ToolGenOptions{
		NameFormat: func(name string) string {
			return app.Name + "." + name
		},
		RequestHandler: func(req *http.Request) (*http.Response, error) {
			// Override the base URL.
			if baseURL != "" {
				origPath := req.URL.Path
				newURL := strings.TrimRight(baseURL, "/") + origPath
				if req.URL.RawQuery != "" {
					newURL += "?" + req.URL.RawQuery
				}
				parsed, err := http.NewRequest(req.Method, newURL, req.Body)
				if err != nil {
					return nil, fmt.Errorf("rebuild request: %w", err)
				}
				parsed.Header = req.Header
				req = parsed
			}
			// Inject auth.
			if token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
			return http.DefaultClient.Do(req)
		},
	}

	openapi2mcp.RegisterOpenAPITools(srv, sd.ops, sd.doc, opts)
}

func (f *MCPFactory) registerJSTools(srv *mcp.Server, app *App, install models.AppInstall) {
	manifests := f.registry.GetTools(app.Name)
	if len(manifests) == 0 {
		return
	}

	// Parse timeout from app.yaml.
	timeout := 5 * time.Second
	if app.Timeout != "" {
		if d, err := time.ParseDuration(app.Timeout); err == nil {
			timeout = d
		}
	}

	// Build secrets and config maps from the user's install settings.
	secrets := make(map[string]string)
	config := make(map[string]string)
	for _, def := range app.Settings {
		val := install.GetSetting(def.Key)
		if def.Type == "secret" {
			secrets[def.Key] = val
		} else {
			config[def.Key] = val
		}
	}

	runtime := NewJSRuntime(app, secrets, config, timeout)

	for _, m := range manifests {
		manifest := m // capture
		namespacedName := app.Name + "." + manifest.Name

		// Build JSON Schema for the tool's params.
		schema := buildToolSchema(manifest)

		tool := &mcp.Tool{
			Name:        namespacedName,
			Description: manifest.Description,
			InputSchema: &schema,
		}

		srv.AddTool(tool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// Extract params from the request.
			params := make(map[string]any)
			if req.Params.Arguments != nil {
				raw, _ := json.Marshal(req.Params.Arguments)
				json.Unmarshal(raw, &params)
			}

			result, err := runtime.Execute(manifest.ScriptPath, params)
			if err != nil {
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: "error: " + err.Error()}},
					IsError: true,
				}, nil
			}

			text, _ := json.MarshalIndent(result, "", "  ")
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: string(text)}},
			}, nil
		})
	}
}

func buildToolSchema(m ToolManifest) jsonschema.Schema {
	schema := jsonschema.Schema{
		Type: "object",
	}
	props := make(map[string]*jsonschema.Schema)
	var required []string

	for name, def := range m.Params {
		propType := "string"
		switch def.Type {
		case "number":
			propType = "number"
		case "boolean":
			propType = "boolean"
		}
		props[name] = &jsonschema.Schema{
			Type:        propType,
			Description: def.Description,
		}
		if def.Required {
			required = append(required, name)
		}
	}

	if len(props) > 0 {
		schema.Properties = props
	}
	if len(required) > 0 {
		schema.Required = required
	}
	return schema
}

func (f *MCPFactory) registerPrompts(srv *mcp.Server, app *App) {
	prompts := f.registry.GetPrompts(app.Name)
	for _, p := range prompts {
		prompt := p // capture
		namespacedName := app.Name + "." + prompt.Name

		// Build MCP prompt arguments.
		var args []*mcp.PromptArgument
		for _, a := range prompt.Arguments {
			args = append(args, &mcp.PromptArgument{
				Name:        a.Name,
				Description: a.Description,
				Required:    a.Required,
			})
		}

		srv.AddPrompt(&mcp.Prompt{
			Name:        namespacedName,
			Description: prompt.Description,
			Arguments:   args,
		}, func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			// Simple template substitution: replace {{key}} with arg value.
			body := prompt.Body
			for k, v := range req.Params.Arguments {
				body = strings.ReplaceAll(body, "{{"+k+"}}", v)
			}
			return &mcp.GetPromptResult{
				Description: prompt.Description,
				Messages: []*mcp.PromptMessage{
					{
						Role: "user",
						Content: &mcp.TextContent{
							Text: body,
						},
					},
				},
			}, nil
		})
	}
}
