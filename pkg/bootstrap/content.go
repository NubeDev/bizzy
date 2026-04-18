package bootstrap

// ---------------------------------------------------------------------------
// Each prompt is defined as a Prompt struct.  The Body field contains the
// rendered markdown (no YAML frontmatter) — exactly what consumers display.
//
// The raw markdown-with-frontmatter needed for the on-disk MCP app is built
// by RenderFrontmatter().
// ---------------------------------------------------------------------------

// RenderFrontmatter returns the full markdown file content (YAML frontmatter +
// body) for a prompt, suitable for writing to disk as an MCP prompt file.
func RenderFrontmatter(p Prompt) string {
	s := "---\nname: " + p.Name + "\n"
	s += "description: \"" + p.Description + "\"\n"
	if len(p.Arguments) == 0 {
		s += "arguments: []\n"
	} else {
		s += "arguments:\n"
		for _, a := range p.Arguments {
			s += "  - name: " + a.Name + "\n"
			s += "    description: \"" + a.Description + "\"\n"
			if a.Required {
				s += "    required: true\n"
			} else {
				s += "    required: false\n"
			}
		}
	}
	s += "---\n\n"
	s += p.Body
	return s
}

// AppYAML is the app.yaml manifest for the bootstrap app.
const AppYAML = `name: bizzy-dev
version: 1.0.0
description: Bizzy framework reference — architecture, APIs, plugins, and app development guides for AI assistants
author: NubeIO

permissions:
  allowedHosts: []
  defaultToolClass: read-only
  secrets: []

settings: []

preamble: |
  You have access to the bizzy-dev prompts which contain the complete developer
  reference for the Bizzy AI agent platform. Use these prompts to understand
  how the system works before making changes:

  - bizzy-dev.overview — Architecture, key concepts, how everything fits together
  - bizzy-dev.api_guide — REST API endpoints, authentication, request/response patterns
  - bizzy-dev.plugin_system — Plugin SDK, NATS protocol, registration, tool calls
  - bizzy-dev.app_development — How to build apps: app.yaml, JS tools, prompts, workflows
  - bizzy-dev.server_testing — How to start/stop the server, test apps, debug, verify via API
  - bizzy-dev.new_app — Step-by-step workflow: create a new app, test it, iterate until working

tags:
  - devops
  - utilities

timeout: 5s
`

// ---------------------------------------------------------------------------
// Prompt definitions
// ---------------------------------------------------------------------------

var promptOverview = Prompt{
	Name:        "overview",
	Description: "Bizzy platform overview — architecture, key concepts, data flow, and project structure",
	Body: `# Bizzy Platform Overview

Bizzy is an AI agent platform. It connects AI providers (Claude, Ollama, OpenAI, etc.) to installable **apps** that give the AI tools, prompts, and workflows. Users interact through a CLI, a Flutter app, or direct API calls.

Core idea: **apps teach the AI what it can do**. An admin installs apps, users install the ones they need, and the AI gets access to those tools via MCP.

## Architecture

` + "```" + `
Clients (CLI, Flutter, API, Slack, Email, Cron, Webhook)
       |
       v
nube-server (single Go binary)
  ├── Auth (Bearer token)
  ├── App Registry (disk + DB apps)
  ├── MCP Factory (per-user tool scoping)
  ├── Memory Store (server + per-user)
  ├── Service Layer
  │   ├── AgentService (prompt enrichment, sessions, providers)
  │   └── ToolService (tool resolution, execution)
  ├── AI Runners (Claude, Ollama, Codex, Copilot)
  ├── Workflow Engine (staged pipelines)
  ├── Command Bus + Event Bus (NATS embedded)
  ├── Plugin Registry (external process plugins over NATS)
  └── Job Store (async AI jobs)
       |
       v
Data: SQLite (bizzy.db), disk apps (data/apps/), memory (data/memory/)
` + "```" + `

## Key Concepts

### Apps
A directory with app.yaml, optional tools (JS), prompts (markdown), and workflows (YAML). Apps are the unit of functionality — they teach the AI how to do things. Served to AI via MCP. Each user only sees tools from their installed apps.

### Plugins
Separate processes (any language) that connect over NATS. Provide tools, prompts, workflows, adapters, or event handlers. Managed externally (systemd, Docker, etc.). Hot-reload by re-registering.

### AI Providers
Multiple backends through one interface. All implement Runner.Run() and emit normalised events.
- **Claude** — CLI, native MCP, session resume
- **Ollama** — API, local models, system prompt
- **OpenAI/Anthropic/Gemini** — Phase 3-4

### MCP (Model Context Protocol)
Per-user endpoint at /mcp. Tools come from: JS tools (Goja sandbox), OpenAPI tools (HTTP proxy), Plugin tools (NATS proxy). All namespaced: appName.toolName or plugin.pluginName.toolName.

### Memory
Persistent context across conversations. Server memory (shared) + User memory (private). Injected into every AI prompt.

### Workflows
Multi-step pipelines chaining tools from multiple apps. Stages: tool calls, AI prompts, or approval gates.

### Command Bus
Unified command syntax for any channel (Slack, email, CLI, cron, webhook). Commands flow down, events flow up via NATS.

## Three Ways to Use AI

| Method | Entry point | Use case |
|---|---|---|
| WebSocket | GET /api/agents/run | Real-time streaming (CLI, Flutter) |
| REST sync | POST /api/agents/run/sync | Simple request/response |
| Async jobs | POST /api/agents/jobs | Fire-and-forget with polling |

## Project Structure

` + "```" + `
bizzy/
  cmd/
    nube-server/         Server entry point
    nube/                CLI entry point
  pkg/
    api/                 HTTP handlers, router, MCP endpoint
    services/            AgentService, ToolService
    airunner/            Provider interface, runners, job store
    apps/                App registry, MCP factory, JS runtime
    plugin/              Plugin registry, proxy, health monitor
    pluginsdk/           SDK for building plugins (no server deps)
    auth/                Bearer token middleware
    bus/                 NATS embedded event bus
    command/             Command parser, router
    adapters/            Slack, Gmail, webhook, cron adapters
    workflow/            Workflow engine
    memory/              Server + per-user memory store
    models/              Core data types
  data/
    apps/                System app directories
    memory/              Memory files
    bizzy.db             SQLite database
  plugins/               Example plugins (starter, github)
  docs/                  Documentation
` + "```" + `

## Data Storage

SQLite (data/bizzy.db) for all structured data. App files and memory on disk.
`,
}

var promptAPIGuide = Prompt{
	Name:        "api_guide",
	Description: "Bizzy REST API reference — all endpoints, authentication, request/response patterns",
	Body: `# Bizzy REST API Reference

Base URL: http://localhost:8090
Auth: Bearer token in Authorization header (except /health and /bootstrap)

## Authentication

- All routes require ` + "`" + `Authorization: Bearer <token>` + "`" + ` except /health and /bootstrap
- Dev mode: if no header sent, falls back to first user in DB
- Admin impersonation: ` + "`" + `X-Act-As-User: <userId>` + "`" + ` header (admin-only)
- WebSocket auth: ` + "`" + `?token=<bearer-token>` + "`" + ` query param

## Public Endpoints

| Method | Path | Description |
|---|---|---|
| GET | /health | Health check (status, user count, app count) |
| POST | /bootstrap | Create first workspace + admin (409 if users exist) |

## User Management

| Method | Path | Description |
|---|---|---|
| GET | /users/me | Current user info |
| GET | /users/:id | Get user by ID (admin) |
| DELETE | /users/:id | Delete user (admin) |
| POST | /users/:id/token | Rotate token (self or admin) |
| DELETE | /users/:id/token | Revoke token (self or admin) |

## Workspaces

| Method | Path | Description |
|---|---|---|
| GET | /workspaces | List (admin sees all, user sees own) |
| POST | /workspaces | Create workspace (admin) |
| GET | /workspaces/:id | Get workspace |
| DELETE | /workspaces/:id | Delete workspace (must be empty) |
| POST | /workspaces/:id/users | Create user in workspace (admin) |
| GET | /workspaces/:id/users | List workspace users |

## App Catalog & Installs

| Method | Path | Description |
|---|---|---|
| GET | /apps | List available apps |
| GET | /apps/:name | App detail + prompts |
| POST | /apps/:name/install | Install app for current user |
| GET | /app-installs | List user's installs |
| PATCH | /app-installs/:id | Update install (settings, enable/disable) |
| DELETE | /app-installs/:id | Uninstall |

## App Store

| Method | Path | Description |
|---|---|---|
| GET | /api/store/apps | Browse store (query, category, sort, pagination) |
| GET | /api/store/apps/:id | App detail + installed flag |
| POST | /api/store/apps/:id/install | Install from store |
| GET/POST/DELETE | /api/store/apps/:id/reviews | Reviews CRUD |
| GET/POST/PUT/DELETE | /api/my/apps/... | Author CRUD (create, edit, publish) |

## AI / Agents

| Method | Path | Description |
|---|---|---|
| GET | /api/agents | List agents (from installed apps) |
| POST | /api/agents/tools/:name | Call a tool directly |
| GET | /api/agents/providers | List AI providers |
| POST | /api/agents/run/sync | Synchronous agent run |
| POST | /api/agents/jobs | Submit async AI job |
| GET | /api/agents/jobs/:id | Poll job status (?after=N for incremental) |
| DELETE | /api/agents/jobs/:id | Cancel running job |
| GET | /api/agents/sessions | List session history |
| GET | /api/agents/sessions/:id | Session detail with full result |

## WebSocket

| Path | Description |
|---|---|
| /api/agents/run?token= | Streaming agent chat (any provider) |
| /api/agents/qa?token= | Interactive QA wizard flows |

## MCP

| Path | Description |
|---|---|
| /mcp | Per-user MCP tool serving (StreamableHTTP) |

## Tools & Prompts

| Method | Path | Description |
|---|---|---|
| GET | /my/tools | List tools available to you |
| GET | /my/prompts | List prompts available to you |
| GET | /my/prompts/:name | Render a prompt with arguments |

## Bootstrap Prompts (no install required)

| Method | Path | Description |
|---|---|---|
| GET | /api/bootstrap/prompts | List all bootstrap reference prompts |
| GET | /api/bootstrap/prompts/:name | Get a single prompt by name |

## Plugins (Admin)

| Method | Path | Description |
|---|---|---|
| GET | /api/plugins | List all plugins (?service=tools to filter) |
| GET | /api/plugins/:name | Plugin detail + full manifest |
| DELETE | /api/plugins/:name | Force unload |
| POST | /api/plugins/:name/disable | Disable plugin |
| POST | /api/plugins/:name/enable | Re-enable plugin |

## Command Bus

| Method | Path | Description |
|---|---|---|
| POST | /api/command | Execute a command (text or structured) |
| GET | /api/command/help | Available verbs and targets |
| GET | /api/events/stream | SSE event stream (user-scoped) |
| POST | /hooks/command | Inbound webhook |
| GET/POST/DELETE/PATCH | /api/cron | Scheduled commands CRUD |

## Admin

| Method | Path | Description |
|---|---|---|
| POST | /admin/reload-apps | Reload app registry + rebuild MCP cache |

## Common Patterns

**Install an app and call its tools:**
` + "```" + `bash
# Install
curl -X POST localhost:8090/apps/weather-checker/install \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"settings": {"api_host": "http://localhost:9000"}}'

# Call tool directly
curl -X POST localhost:8090/api/agents/tools/weather-checker.get_weather \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"city": "Sydney"}'

# Or ask AI to use it
curl -X POST localhost:8090/api/agents/run/sync \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"prompt": "what is the weather in Sydney?", "provider": "claude"}'
` + "```" + `
`,
}

var promptPluginSystem = Prompt{
	Name:        "plugin_system",
	Description: "Bizzy plugin system — SDK, NATS protocol, registration, tool calls, health monitoring",
	Body: `# Bizzy Plugin System

Plugins are independent processes that extend bizzy by connecting to its embedded NATS server. Written in any language. Managed externally (systemd, Docker, manually).

## Plugin Services

| Service | What it extends |
|---|---|
| tools | AI tool calling (appears in MCP alongside app tools) |
| prompts | Prompt templates served via MCP |
| workflows | Register workflow definitions |
| adapter | New command bus channel (Telegram, Discord, etc.) |
| handler | React to bus events (notifications, logging, sync) |

## NATS Subjects

` + "```" + `
extension.register              — plugin manifest (request/reply)
extension.deregister            — clean shutdown
extension.health.<name>         — heartbeat (default 10s)
tool.call.<plugin>.<tool>       — bizzy → plugin tool call (request/reply)
adapter.command.<name>          — adapter → bizzy inbound command
adapter.reply.<name>            — bizzy → adapter outbound reply
extension.event.<name>.*        — custom plugin events
` + "```" + `

## Plugin Lifecycle

1. **Register** — connect to NATS, send manifest to extension.register (request/reply)
2. **Heartbeat** — publish to extension.health.<name> every 10s
3. **Handle tool calls** — subscribe to tool.call.<name>.* with queue group
4. **Hot-reload** — re-register with same name to update tools/config
5. **Shutdown** — publish to extension.deregister

## Go SDK (pkg/pluginsdk)

` + "```" + `go
package main

import (
    "fmt"
    "github.com/NubeDev/bizzy/pkg/pluginsdk"
)

func main() {
    p := pluginsdk.NewPlugin("my-plugin", "1.0.0", "Description")
    p.SetPreamble("Use plugin.my-plugin.* tools for ...")

    schema := pluginsdk.Params("text", "string", "Input text", true)
    pluginsdk.ParamsAdd(schema, "count", "number", "How many", false)

    p.AddTool(pluginsdk.Tool{
        Name:        "my_tool",
        Description: "Does something useful",
        Parameters:  schema,
        Handler: func(params map[string]any) (any, error) {
            text := params["text"].(string)
            return map[string]any{"result": text}, nil
        },
    })

    p.Run() // blocks until SIGINT/SIGTERM
}
` + "```" + `

## SDK Helpers

` + "```" + `go
// Build JSON Schema for parameters
schema := pluginsdk.Params("city", "string", "City name", true)
pluginsdk.ParamsAdd(schema, "days", "number", "Forecast days", false)

// Plugin configuration
p.SetNATSURL("nats://127.0.0.1:4225")   // default
p.SetHeartbeatInterval(15)                // seconds (5-60)

// Add prompts
p.AddPrompt(pluginsdk.Prompt{
    Name: "report", Description: "Generate report",
    Template: "Analyze data for {{region}}",
    Arguments: []pluginsdk.PromptArg{
        {Name: "region", Description: "Region", Required: true},
    },
})
` + "```" + `

## Tool Call Flow

` + "```" + `
AI calls plugin.weather.get_forecast
  → ToolService → PluginProxy
    → NATS Request to tool.call.weather.get_forecast (timeout 30s)
      → Plugin receives, processes, replies
    ← NATS Reply with result
  ← return to AI
` + "```" + `

## Manifest Example

` + "```" + `json
{
  "api_version": "1.0.0",
  "name": "weather",
  "version": "1.0.0",
  "description": "Weather tools",
  "services": ["tools"],
  "tools": [{
    "name": "get_forecast",
    "description": "Get weather forecast",
    "parameters": {
      "type": "object",
      "properties": {
        "city": {"type": "string", "description": "City name"}
      },
      "required": ["city"]
    }
  }],
  "preamble": "Use plugin.weather.* tools for weather data."
}
` + "```" + `

## REST API for Plugins

| Method | Path | Description |
|---|---|---|
| GET | /api/plugins | List all plugins (?service=tools) |
| GET | /api/plugins/:name | Plugin detail + manifest |
| DELETE | /api/plugins/:name | Force unload |
| POST | /api/plugins/:name/disable | Disable |
| POST | /api/plugins/:name/enable | Re-enable |

## Accessing Plugins from JS Tools

App JS tools can discover and call plugins via the plugins.* host API:

` + "```" + `js
function handle(params) {
  if (plugins.exists("github")) {
    var info = plugins.info("github");
    // info = {name, version, status, services, tools}

    var resp = plugins.call("github", "list_prs", {
      owner: "NubeDev", repo: "bizzy"
    });
    if (!resp.error) {
      return {prs: resp.result};
    }
  }
  return {error: "github plugin not available"};
}
` + "```" + `

| Method | Returns | Description |
|---|---|---|
| plugins.exists(name) | boolean | Is plugin active? |
| plugins.info(name) | object/null | Plugin metadata |
| plugins.list(filter?) | string[] | Active plugin names |
| plugins.call(plugin, tool, params) | {result} or {error} | Call plugin tool |
`,
}

var promptAppDevelopment = Prompt{
	Name:        "app_development",
	Description: "How to build Bizzy apps — app.yaml, JS tools, prompts, workflows, and patterns",
	Body: `# Building Bizzy Apps

An app is a directory under data/apps/<app-name>/ with this layout:

` + "```" + `
data/apps/<app-name>/
  app.yaml              # Required — metadata, permissions, settings
  prompts/              # Optional — reusable prompt templates (markdown)
    getting_started.md
  tools/                # Optional — executable tools
    _helpers.js         # Optional — shared JS loaded before every tool
    my_tool.js          # Tool logic
    my_tool.json        # Tool schema
` + "```" + `

## app.yaml

` + "```" + `yaml
name: my-app                    # Unique ID (lowercase, hyphens)
version: 1.0.0
description: What this app does
author: YourName

permissions:
  allowedHosts:                 # Domains JS tools can call
    - "api.example.com"
    - "localhost:*"
  defaultToolClass: read-only   # "read-only" or "read-write"
  secrets:
    - my_api_token

settings:                       # User-configurable values
  - key: api_host
    label: API Host URL
    type: url                   # url | string | number | secret | boolean
    required: true
    default: http://localhost:8080
  - key: my_api_token
    label: API Token
    type: secret
    required: false

preamble: |                     # Injected into AI conversations
  You are a helpful assistant for My App.
  Use the my-app.* tools to help the user.

plugins:                        # Optional — declare plugin dependencies
  required:
    - name: github
      reason: "Needed for PR tracking"
  optional:
    - name: slack-adapter

tags:
  - utilities
timeout: 15s
` + "```" + `

## JS Tools

Two files per tool: tool_name.json (schema) + tool_name.js (logic).

**Schema (tool_name.json):**
` + "```" + `json
{
  "name": "tool_name",
  "description": "What this tool does — AI reads this to decide when to call it",
  "toolClass": "read-only",
  "params": {
    "city": { "type": "string", "required": true, "description": "City name" },
    "limit": { "type": "number", "required": false, "description": "Max results" }
  }
}
` + "```" + `

**Logic (tool_name.js):**
` + "```" + `js
function handle(params) {
  if (!params.city) return { error: "city is required" };

  var resp = http.get("https://api.example.com/data?city=" + encodeURIComponent(params.city));
  if (resp.status !== 200) {
    return { error: "API failed (" + resp.status + ")" };
  }

  return { city: resp.json.name, value: resp.json.value };
}
` + "```" + `

**Rules:** ES5 only (no let/const, no arrow functions, no async). Must export handle(params). Return plain objects.

## Available Globals in JS Tools

| Global | Description |
|---|---|
| http.get/post/put/patch/delete | HTTP requests → {status, body, json, headers} |
| config.<key> | App settings from app.yaml |
| secrets.<key> | App secrets |
| log.info/error | Logging |
| files.read(path) | Read file from app directory |
| tools.call(name, params) | Call another tool in the same app → {result} or {error} |
| plugins.exists(name) | Check if a plugin is active |
| plugins.info(name) | Plugin metadata or null |
| plugins.list(filter?) | List active plugin names |
| plugins.call(plugin, tool, params) | Call a plugin tool → {result} or {error} |
| base64.encode(str) / base64.decode(str) | Base64 encoding/decoding |
| url.buildQuery({k: v}) | Build query string from object |
| url.parse(urlStr) | Parse URL → {protocol, host, path, query, hash} |
| crypto.sha256/sha1/md5(data) | Hash → hex string |
| crypto.hmac(algo, key, data) | HMAC signature → hex string |
| env.get(key) | Read env var (allowlisted prefixes: BIZZY_, NUBE_, OLLAMA_, GITHUB_) |

## Shared Helpers (_helpers.js)

Loaded before every tool in the app:
` + "```" + `js
function apiGet(path) {
  return http.get(config.api_host + path, {
    headers: { "Authorization": "Bearer " + secrets.api_token }
  });
}
` + "```" + `

## Prompts

Markdown files in prompts/ with frontmatter:
` + "```" + `markdown
---
name: weather_report
description: Generate a weather report
arguments:
  - name: city
    description: City to report on
    required: true
---

Use the get_weather tool to fetch weather for {{city}}, then write a report.
` + "```" + `

## Prompt-Mode Tools

A tool where the AI follows instructions instead of running JS. Only needs .json:
` + "```" + `json
{
  "name": "navigation",
  "description": "View or edit the site hierarchy",
  "mode": "prompt",
  "prompt": "Help the user manage the hierarchy.\nFirst call my-app.list_items...",
  "params": {
    "action": { "type": "string", "required": false, "options": ["show", "create", "delete"] }
  }
}
` + "```" + `

## QA-Mode Tools (Interactive Forms)

Schema has "mode": "qa". JS implements chatMode, formDefinition, formSubmit paths.

## Common Patterns

| Pattern | How |
|---|---|
| API data fetch | JS tool + http.get() + _helpers.js for auth |
| CRUD operations | Multiple JS tools + shared _helpers.js |
| Multi-step workflow | Prompt-mode tool that orchestrates other tools |
| Interactive form | QA-mode tool with chatMode/formDefinition |
| Prompt-only app | No tools, just prompts/ directory |
| Plugin integration | plugins.exists() check + plugins.call() |

## Checklist

1. Create data/apps/<name>/app.yaml
2. Create tools/ directory, write .json + .js pairs
3. Add _helpers.js if tools share auth
4. Create prompts/ directory with .md files if needed
5. Reload: POST /admin/reload-apps or restart server
6. Install: POST /apps/<name>/install
`,
}

var promptServerTesting = Prompt{
	Name:        "server_testing",
	Description: "How to start/stop the server, test apps, debug JS tools, and verify everything works via the API",
	Body: `# Server Testing & Debugging Guide

## Starting and Stopping

**Start the server (foreground):**
` + "```" + `bash
make start        # builds + starts on :8090
# or
make server       # same thing
` + "```" + `

**Stop:**
` + "```" + `bash
make stop          # kills background nube-server
# or Ctrl+C if running in foreground
` + "```" + `

**Reset (wipe data):**
` + "```" + `bash
make reset         # stops server + wipes data files
` + "```" + `

**Rebuild after code changes:**
` + "```" + `bash
make build         # builds bin/nube-server and bin/nube
` + "```" + `

The server auto-generates the bizzy-dev app on every startup. No manual setup needed.

## First-Time Setup

After starting a fresh server:

` + "```" + `bash
# 1. Bootstrap (creates first admin user)
curl -s -X POST http://localhost:8090/bootstrap \
  -H 'Content-Type: application/json' \
  -d '{"workspaceName":"Dev","adminName":"Admin","adminEmail":"admin@dev.com"}'
# Returns: {workspace, admin: {token: "..."}}

# 2. Save the token
export TOKEN="<token-from-above>"

# 3. Install your app
curl -s -X POST http://localhost:8090/apps/my-app/install \
  -H "Authorization: Bearer $TOKEN" \
  -d '{}'
` + "```" + `

## Testing an App End-to-End

### Step 1: Verify the app loaded

` + "```" + `bash
# Check health — apps count should include your app
curl -s http://localhost:8090/health | jq .apps

# List all apps
curl -s http://localhost:8090/apps -H "Authorization: Bearer $TOKEN" | jq '.[].name'

# Get app detail
curl -s http://localhost:8090/apps/my-app -H "Authorization: Bearer $TOKEN" | jq
` + "```" + `

### Step 2: Install and check tools appear

` + "```" + `bash
# Install
curl -s -X POST http://localhost:8090/apps/my-app/install \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"settings": {"api_host": "http://localhost:9000"}}' | jq

# List your tools — should include my-app.*
curl -s http://localhost:8090/my/tools -H "Authorization: Bearer $TOKEN" | jq '.[].name'

# List your prompts — should include my-app.*
curl -s http://localhost:8090/my/prompts -H "Authorization: Bearer $TOKEN" | jq '.[].name'
` + "```" + `

### Step 3: Call a tool directly

` + "```" + `bash
# Call via REST
curl -s -X POST http://localhost:8090/api/agents/tools/my-app.my_tool \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"city": "Sydney"}' | jq
` + "```" + `

### Step 4: Test via MCP

` + "```" + `bash
# MCP initialize
curl -s -X POST http://localhost:8090/mcp \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' \
  -D /dev/stderr 2>&1 | head -5
# Look for Mcp-Session-Id in response headers
` + "```" + `

## Debugging JS Tools

### Common issues

| Problem | Cause | Fix |
|---|---|---|
| Tool not in list | app.yaml name mismatch or .json missing | Check name field matches directory name |
| "allowedHosts" error | URL not in permissions.allowedHosts | Add the domain to app.yaml allowedHosts |
| "handle() not found" | JS file missing handle function | Add: function handle(params) { ... } |
| Timeout error | Tool takes > timeout | Increase timeout in app.yaml |
| "script error" | JS syntax error | Check for ES6 syntax (use var, not let/const) |
| Settings empty | Not passed at install time | Reinstall with settings, or PATCH the install |

### Check server logs

The server logs JS tool execution:
` + "```" + `
[jsruntime] executing my_tool.js
[jsruntime] my_tool.js completed OK
# or
[jsruntime] my_tool.js returned error: ...
` + "```" + `

### Test tool in isolation

` + "```" + `bash
# Use the test-tool endpoint (no install needed)
curl -s -X POST http://localhost:8090/api/store/apps/test-tool \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "script": "function handle(params) { return {hello: params.name}; }",
    "params": {"name": "world"},
    "allowedHosts": [],
    "settings": {},
    "secrets": {}
  }' | jq
` + "```" + `

### Hot-reload after changes

` + "```" + `bash
# Reload all apps without restart
curl -s -X POST http://localhost:8090/admin/reload-apps \
  -H "Authorization: Bearer $TOKEN" | jq
` + "```" + `

## Running Tests

` + "```" + `bash
make test          # Go unit + integration tests
make test-api      # Full API test script against running server
go test ./pkg/apps/ -v -run TestPluginsAPI   # specific test
` + "```" + `

## Verifying Plugins

` + "```" + `bash
# List registered plugins
curl -s http://localhost:8090/api/plugins -H "Authorization: Bearer $TOKEN" | jq

# Check a specific plugin
curl -s http://localhost:8090/api/plugins/starter -H "Authorization: Bearer $TOKEN" | jq

# Call a plugin tool
curl -s -X POST http://localhost:8090/api/agents/tools/plugin.starter.echo \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"text": "hello"}' | jq
` + "```" + `
`,
}

var promptNewApp = Prompt{
	Name:        "new_app",
	Description: "AI-driven workflow: create a new Bizzy app from a description, test it, and iterate until working",
	Arguments: []PromptArgument{
		{Name: "description", Description: "What the app should do (e.g. 'weather checker that fetches forecasts')", Required: true},
	},
	Body: `You are building a new Bizzy app. Follow this workflow step by step. After each step, verify it worked before moving to the next. If something fails, diagnose and fix it before continuing.

## What the user wants

{{description}}

## Step 1: Design the app

Based on the description, plan:
1. App name (lowercase, hyphens)
2. What tools it needs (list each with name, description, params)
3. What external APIs it calls (for allowedHosts)
4. What settings/secrets the user needs to configure
5. Whether it needs prompts or is tools-only

## Step 2: Create the app directory

Create the files under data/apps/<app-name>/:

1. **app.yaml** — name, version, description, permissions (allowedHosts!), settings, preamble
2. **tools/<tool_name>.json** — schema for each tool (name, description, toolClass, params)
3. **tools/<tool_name>.js** — implementation for each tool (ES5 only!)
4. **tools/_helpers.js** — if multiple tools share auth/base URL
5. **prompts/*.md** — if the app needs prompt templates

**Critical rules:**
- JS must be ES5: use var (not let/const), no arrow functions, no template literals, no async
- Every .js tool needs a matching .json schema file
- Tool names use underscores (get_weather.js), not hyphens
- The handle(params) function must exist in every .js file
- Add all external domains to allowedHosts in app.yaml
- Return plain objects from handle(), not strings

## Step 3: Reload and verify

` + "```" + `bash
# Reload apps
curl -s -X POST http://localhost:8090/admin/reload-apps \
  -H "Authorization: Bearer $TOKEN" | jq

# Check the app loaded
curl -s http://localhost:8090/apps/<app-name> \
  -H "Authorization: Bearer $TOKEN" | jq '.app.name, .app.hasTools, .tools'
` + "```" + `

If the app doesn't appear, check:
- app.yaml has a valid name field
- The directory is under data/apps/
- No YAML syntax errors

## Step 4: Install the app

` + "```" + `bash
# Install (with settings if needed)
curl -s -X POST http://localhost:8090/apps/<app-name>/install \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"settings": {}}' | jq

# Verify tools are visible
curl -s http://localhost:8090/my/tools \
  -H "Authorization: Bearer $TOKEN" | jq '.[] | select(.name | startswith("<app-name>"))'
` + "```" + `

## Step 5: Test each tool

For each tool, call it directly and check the response:

` + "```" + `bash
curl -s -X POST http://localhost:8090/api/agents/tools/<app-name>.<tool_name> \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"param1": "value1"}' | jq
` + "```" + `

**If it fails:**
1. Read the error message carefully
2. Check server logs (terminal where make start is running)
3. Common fixes:
   - "allowedHosts" → add the domain to app.yaml permissions.allowedHosts
   - "handle() not found" → check function name is exactly handle
   - "script error" → check for ES6 syntax, fix to ES5
   - "timeout" → increase timeout in app.yaml or check if the API is reachable
4. Edit the file, reload (POST /admin/reload-apps), and test again

## Step 6: Iterate and improve

Repeat steps 3-5 until all tools work. For each iteration:
1. Fix any errors from the previous test
2. Reload apps
3. Re-test the failing tool
4. Once it passes, move to the next tool

When all tools pass:
1. Test edge cases (missing params, invalid input, API errors)
2. Check the preamble makes sense for AI usage
3. Verify prompts render correctly if the app has any

## Step 7: Summary

Report what was created:
- App name and location
- Tools created and their status (pass/fail)
- Any settings the user needs to configure
- How to use it: which tools to call and what they do

## Environment

- Server: http://localhost:8090
- Apps directory: data/apps/
- Reload endpoint: POST /admin/reload-apps
- Token: use the $TOKEN environment variable or the token from /users/me
- Dev mode: requests without Authorization header fall back to the first user
`,
}
