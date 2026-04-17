package apps

import (
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/NubeDev/bizzy/pkg/models"
	openapi2mcp "github.com/NubeIO/openapi-mcp"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gorm.io/gorm"
) // PluginToolSource provides plugin tools and prompts to MCPFactory.
// Implemented by plugin.Registry — defined here as an interface to avoid
// circular imports and to keep the coupling minimal.
type PluginToolSource interface {
	// ActivePluginTools returns all tools from active plugins.
	// Each entry has FullName (e.g. "plugin.weather.get_forecast"),
	// PluginName, and the tool spec including Parameters.
	ActivePluginTools() []PluginToolEntry
	// ActivePluginPrompts returns all prompts from active plugins.
	ActivePluginPrompts() []PluginPromptEntry
	// CallTool dispatches a tool call to a plugin over NATS.
	CallTool(pluginName, toolName string, params map[string]any) (any, error)
}

// PluginToolEntry is a single plugin tool with its namespaced name.
type PluginToolEntry struct {
	FullName    string
	PluginName  string
	Name        string
	Description string
	Parameters  map[string]any // JSON Schema
}

// PluginPromptEntry is a single plugin prompt with its namespaced name.
type PluginPromptEntry struct {
	FullName    string
	PluginName  string
	Name        string
	Description string
	Template    string
	Arguments   []PluginPromptArg
}

// PluginPromptArg mirrors plugin.PromptArg without importing the package.
type PluginPromptArg struct {
	Name        string
	Description string
	Required    bool
}

// MCPFactory builds per-user MCP servers with only the tools from their installed apps.
type MCPFactory struct {
	registry *Registry
	// Cached parsed OpenAPI docs per app name (static files).
	specCache map[string]*specData
	// Cached remote OpenAPI specs with TTL.
	remoteCache   map[string]*remoteSpecEntry
	remoteCacheMu sync.RWMutex
	// Plugin tool source (optional — nil if plugin system is not enabled).
	pluginSource PluginToolSource
}

type specData struct {
	doc *openapi3.T
	ops []openapi2mcp.OpenAPIOperation
}

type remoteSpecEntry struct {
	data      *specData
	fetchedAt time.Time
}

const remoteSpecTTL = 5 * time.Minute

// NewMCPFactory creates a factory that builds per-user MCP servers.
func NewMCPFactory(registry *Registry) *MCPFactory {
	f := &MCPFactory{
		registry:    registry,
		specCache:   make(map[string]*specData),
		remoteCache: make(map[string]*remoteSpecEntry),
	}
	f.preloadSpecs()
	return f
}

func (f *MCPFactory) preloadSpecs() {
	for _, app := range f.registry.List() {
		if !app.HasOpenAPI || app.OpenAPIRemote != nil {
			continue // remote specs are fetched lazily per-user
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
	f.remoteCacheMu.Lock()
	f.remoteCache = make(map[string]*remoteSpecEntry)
	f.remoteCacheMu.Unlock()
	f.preloadSpecs()
}

// SetPluginSource wires the plugin registry into the MCP factory.
// Called once during server startup after both MCPFactory and plugin.Registry exist.
func (f *MCPFactory) SetPluginSource(src PluginToolSource) {
	f.pluginSource = src
}

// BuildAppContext generates a text summary of installed apps and their tools
// for injection into AI prompts. This gives the AI awareness of what tools are
// available and what they do, without the user having to pick an agent.
func (f *MCPFactory) BuildAppContext(installs []models.AppInstall) string {
	var b strings.Builder
	toolCount := 0
	const maxTools = 30 // cap total tool lines to control context budget

	for _, install := range installs {
		if !install.Enabled {
			continue
		}
		app, ok := f.registry.Get(install.AppName)
		if !ok {
			continue
		}

		desc := app.Description
		if desc == "" {
			desc = "No description"
		}
		b.WriteString("- " + app.Name + ": " + desc + "\n")

		// Collect tool descriptions from JS manifests.
		if app.HasTools {
			for _, m := range f.registry.GetTools(app.Name) {
				if toolCount >= maxTools {
					break
				}
				toolDesc := m.Description
				if toolDesc == "" {
					toolDesc = m.Name
				}
				b.WriteString("    " + app.Name + "." + m.Name + " — " + toolDesc + "\n")
				toolCount++
			}
		}

		// Collect tool descriptions from OpenAPI operations.
		if app.HasOpenAPI && app.OpenAPIRemote == nil {
			if sd, ok := f.specCache[app.Name]; ok {
				for _, op := range sd.ops {
					if toolCount >= maxTools {
						break
					}
					opDesc := op.Summary
					if opDesc == "" {
						opDesc = op.Description
					}
					if opDesc == "" {
						opDesc = op.OperationID
					}
					b.WriteString("    " + app.Name + "." + op.OperationID + " — " + opDesc + "\n")
					toolCount++
				}
			}
		}
	}

	// Platform tools are always available — append them to the context.
	b.WriteString("- platform: Query your own platform data — sessions, installs, app store, and usage stats\n")
	b.WriteString("    platform.list_sessions — List your recent AI sessions with provider, model, cost, and duration\n")
	b.WriteString("    platform.get_session — Get full details of a specific session including the AI response\n")
	b.WriteString("    platform.list_installs — List your installed apps and their status\n")
	b.WriteString("    platform.search_apps — Search the app store for apps by name, description, or category\n")
	b.WriteString("    platform.usage_stats — Get your usage summary: sessions, tokens, and cost over a time range\n")

	return "[Installed Apps]\n" + b.String() + "\n"
}

// BuildServer creates an MCP server with tools scoped to the user's installed apps.
// Platform tools (platform.*) are always registered when db and userID are provided,
// giving the AI read-only access to sessions, installs, app store, and usage stats.
func (f *MCPFactory) BuildServer(installs []models.AppInstall, db *gorm.DB, userID string) *mcp.Server {
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

	// Register plugin tools (system-wide, not per-user).
	if f.pluginSource != nil {
		f.registerPluginTools(srv)
		f.registerPluginPrompts(srv)
	}

	// Register Go-native platform tools — always available for every user.
	if db != nil && userID != "" {
		registerPlatformTools(srv, userID, db)
	}

	return srv
}

// buildSchemaFromMap converts a raw JSON-schema map (as provided by plugins)
// into a typed jsonschema.Schema for use with the MCP SDK.
func buildSchemaFromMap(params map[string]any) jsonschema.Schema {
	schema := jsonschema.Schema{Type: "object"}
	if params != nil {
		raw, _ := json.Marshal(params)
		json.Unmarshal(raw, &schema)
	}
	return schema
}
