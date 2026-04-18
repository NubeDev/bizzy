package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/NubeDev/bizzy/pkg/models"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

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
	if f.pluginQuery != nil {
		runtime.SetPluginQuery(f.pluginQuery)
	}
	// Wire same-app tool calling: tools.call("other_tool", params)
	runtime.SetToolCaller(&sameAppToolCaller{runtime: runtime, manifests: manifests})

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

			var result map[string]any
			var err error
			if manifest.Script != "" {
				// DB-loaded tool: execute inline script directly.
				result, err = runtime.ExecuteScript(manifest.Script, "", params)
			} else {
				// Disk-loaded tool: execute from file.
				result, err = runtime.Execute(manifest.ScriptPath, params)
			}
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

// sameAppToolCaller implements ToolCaller by executing tools within the same
// app's JSRuntime. This lets tools.call("other_tool", params) work without
// crossing app boundaries.
type sameAppToolCaller struct {
	runtime   *JSRuntime
	manifests []ToolManifest
}

func (c *sameAppToolCaller) CallTool(toolName string, params map[string]any) (map[string]any, error) {
	for _, m := range c.manifests {
		if m.Name != toolName {
			continue
		}
		if m.Script != "" {
			return c.runtime.ExecuteScript(m.Script, "", params)
		}
		if m.ScriptPath != "" {
			return c.runtime.Execute(m.ScriptPath, params)
		}
		return nil, fmt.Errorf("tool %s has no script", toolName)
	}
	return nil, fmt.Errorf("tool %s not found in this app", toolName)
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
