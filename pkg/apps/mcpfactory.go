package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
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
	// Cached parsed OpenAPI docs per app name (static files).
	specCache map[string]*specData
	// Cached remote OpenAPI specs with TTL.
	remoteCache   map[string]*remoteSpecEntry
	remoteCacheMu sync.RWMutex
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

	if b.Len() == 0 {
		return ""
	}

	return "[Installed Apps]\n" + b.String() + "\n"
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
	var sd *specData

	if app.OpenAPIRemote != nil {
		// Resolve the remote URL by substituting {{key}} placeholders from settings.
		resolved := f.resolveRemoteSpec(app, install)
		if resolved == nil {
			return
		}
		sd = resolved
	} else {
		cached, ok := f.specCache[app.Name]
		if !ok {
			return
		}
		sd = cached
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
		TagFilter: app.openAPIIncludeTags(),
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

// resolveRemoteSpec fetches a remote OpenAPI spec, using a cache with TTL.
func (f *MCPFactory) resolveRemoteSpec(app *App, install models.AppInstall) *specData {
	// Resolve {{key}} placeholders in the URL from user settings.
	url := app.OpenAPIRemote.URL
	for _, def := range app.Settings {
		val := install.GetSetting(def.Key)
		url = strings.ReplaceAll(url, "{{"+def.Key+"}}", val)
	}
	if url == "" {
		return nil
	}

	cacheKey := app.Name + "|" + url

	// Check cache.
	f.remoteCacheMu.RLock()
	entry, ok := f.remoteCache[cacheKey]
	f.remoteCacheMu.RUnlock()
	if ok && time.Since(entry.fetchedAt) < remoteSpecTTL {
		return entry.data
	}

	// Fetch the spec.
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("[mcpfactory] %s: failed to fetch remote openapi from %s: %v", app.Name, url, err)
		if entry != nil {
			return entry.data // serve stale on error
		}
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[mcpfactory] %s: remote openapi returned %d from %s", app.Name, resp.StatusCode, url)
		if entry != nil {
			return entry.data
		}
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[mcpfactory] %s: failed to read remote openapi body: %v", app.Name, err)
		if entry != nil {
			return entry.data
		}
		return nil
	}

	doc, err := openapi2mcp.LoadOpenAPISpecFromBytesLenient(body)
	if err != nil {
		log.Printf("[mcpfactory] %s: failed to parse remote openapi: %v", app.Name, err)
		if entry != nil {
			return entry.data
		}
		return nil
	}

	ops := openapi2mcp.ExtractOpenAPIOperations(doc)

	sd := &specData{doc: doc, ops: ops}
	f.remoteCacheMu.Lock()
	f.remoteCache[cacheKey] = &remoteSpecEntry{data: sd, fetchedAt: time.Now()}
	f.remoteCacheMu.Unlock()

	log.Printf("[mcpfactory] %s: fetched remote openapi from %s (%d operations)", app.Name, url, len(ops))
	return sd
}

// openAPIIncludeTags returns the tag filter from OpenAPIRemote config, or nil.
func (app *App) openAPIIncludeTags() []string {
	if app.OpenAPIRemote == nil {
		return nil
	}
	return app.OpenAPIRemote.IncludeTags
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

		// Prompt-mode tools are MCP prompts only — no callable tool.
		if manifest.Mode == "prompt" && manifest.Prompt != "" {
			f.registerPromptTool(srv, app, manifest)
			continue
		}

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

		// Auto-register an MCP prompt for QA-mode tools so Claude can
		// drive the wizard conversationally.
		if manifest.Mode == "qa" {
			f.registerQAPrompt(srv, app, manifest)
		}
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

// registerQAPrompt auto-generates an MCP prompt for a QA-mode tool so that
// Claude (or any MCP client) can drive the wizard conversationally — asking
// the user each question one at a time, then calling the tool with all answers.
func (f *MCPFactory) registerQAPrompt(srv *mcp.Server, app *App, manifest ToolManifest) {
	namespacedTool := app.Name + "." + manifest.Name
	promptName := namespacedTool // same name, registered as prompt

	// Collect user-facing params (skip internal fields like _submit, _answers),
	// sorted by the "order" field so the prompt asks questions in the right sequence.
	type paramEntry struct {
		name  string
		desc  string
		order int
	}
	var params []paramEntry
	for name, def := range manifest.Params {
		if strings.HasPrefix(name, "_") {
			continue
		}
		params = append(params, paramEntry{name: name, desc: def.Description, order: def.Order})
	}
	sort.Slice(params, func(i, j int) bool {
		oi, oj := params[i].order, params[j].order
		if oi == 0 {
			oi = 1<<31 - 1 // unset order sorts last
		}
		if oj == 0 {
			oj = 1<<31 - 1
		}
		return oi < oj
	})

	var promptBody string

	if manifest.QAPrompt != "" {
		// Use the custom prompt template, replacing {{tool}} with the namespaced tool name.
		promptBody = strings.ReplaceAll(manifest.QAPrompt, "{{tool}}", namespacedTool)
	} else {
		// Auto-generate a conversational QA prompt.
		var b strings.Builder
		b.WriteString("You are running the \"" + manifest.Description + "\" wizard.\n\n")
		b.WriteString("You are a conversational assistant inside a chat interface (such as Claude Code in VS Code).\n")
		b.WriteString("There is no popup or form UI — you must drive the entire Q&A through chat messages.\n\n")
		b.WriteString("Ask the user the following questions ONE AT A TIME. Wait for each answer before asking the next.\n")
		b.WriteString("Keep your questions short and friendly. Present options clearly when available.\n\n")
		for i, p := range params {
			b.WriteString(fmt.Sprintf("%d. **%s** — %s\n", i+1, p.name, p.desc))
		}
		b.WriteString("\nOnce you have all answers, call the tool `" + namespacedTool + "` with `_submit` set to `true` and all collected answers as parameters.\n")
		b.WriteString("Present the result to the user in a clear, friendly format using markdown.\n")
		b.WriteString("If the result contains a follow-up question or quiz (e.g. multiple choice options), present it to the user, wait for their answer, then call the tool again with the additional answer included.\n")
		promptBody = b.String()
	}

	srv.AddPrompt(&mcp.Prompt{
		Name:        promptName,
		Description: manifest.Description,
	}, func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return &mcp.GetPromptResult{
			Description: manifest.Description,
			Messages: []*mcp.PromptMessage{
				{
					Role: "user",
					Content: &mcp.TextContent{
						Text: promptBody,
					},
				},
			},
		}, nil
	})

	log.Printf("[mcpfactory] %s: auto-registered QA prompt for tool %s", app.Name, manifest.Name)
}

// registerPromptTool registers a prompt-mode tool as an MCP prompt so it
// appears as a /nube:app.name slash command in Claude Code / VS Code.
func (f *MCPFactory) registerPromptTool(srv *mcp.Server, app *App, manifest ToolManifest) {
	namespacedName := app.Name + "." + manifest.Name

	var args []*mcp.PromptArgument
	for name, def := range manifest.Params {
		if strings.HasPrefix(name, "_") {
			continue
		}
		args = append(args, &mcp.PromptArgument{
			Name:        name,
			Description: def.Description,
			Required:    def.Required,
		})
	}

	srv.AddPrompt(&mcp.Prompt{
		Name:        namespacedName,
		Description: manifest.Description,
		Arguments:   args,
	}, func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		body := manifest.Prompt
		for k, v := range req.Params.Arguments {
			body = strings.ReplaceAll(body, "{{"+k+"}}", v)
		}
		if app.Preamble != "" {
			body = app.Preamble + "\n\n---\n\n" + body
		}
		return &mcp.GetPromptResult{
			Description: manifest.Description,
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

	log.Printf("[mcpfactory] %s: registered prompt-mode tool %s as MCP prompt", app.Name, manifest.Name)
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
			// Prepend app-level preamble (auth, context, common instructions).
			if app.Preamble != "" {
				body = app.Preamble + "\n\n---\n\n" + body
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
