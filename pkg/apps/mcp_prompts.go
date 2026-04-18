package apps

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (f *MCPFactory) registerPluginTools(srv *mcp.Server) {
	tools := f.pluginSource.ActivePluginTools()
	src := f.pluginSource // capture for closure

	for _, t := range tools {
		tool := t // capture

		schema := buildSchemaFromMap(tool.Parameters)

		srv.AddTool(&mcp.Tool{
			Name:        tool.FullName,
			Description: tool.Description,
			InputSchema: &schema,
		}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			params := make(map[string]any)
			if req.Params.Arguments != nil {
				raw, _ := json.Marshal(req.Params.Arguments)
				json.Unmarshal(raw, &params)
			}

			result, err := src.CallTool(tool.PluginName, tool.Name, params)
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

func (f *MCPFactory) registerPluginPrompts(srv *mcp.Server) {
	prompts := f.pluginSource.ActivePluginPrompts()

	for _, p := range prompts {
		prompt := p // capture

		var args []*mcp.PromptArgument
		for _, a := range prompt.Arguments {
			args = append(args, &mcp.PromptArgument{
				Name:        a.Name,
				Description: a.Description,
				Required:    a.Required,
			})
		}

		srv.AddPrompt(&mcp.Prompt{
			Name:        prompt.FullName,
			Description: prompt.Description,
			Arguments:   args,
		}, func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			body := prompt.Template
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
