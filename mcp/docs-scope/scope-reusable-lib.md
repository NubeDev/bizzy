# OpenMCP — Reusable Go MCP Server Library

## One-liner

An open-source Go library that reads an OpenAPI spec and auto-builds a fully working MCP server — stdio, HTTP, and CLI out of the box.

## Why OpenAPI (not RAS)

RAS is a custom Nube format. Nobody outside Nube has one. OpenAPI is the industry standard — every API has one or can generate one. Building on OpenAPI means:

- Any team with an OpenAPI spec gets an MCP server for free
- Nube services already generate OpenAPI from RAS (via `cmd/codgen/openapi`)
- Massive existing ecosystem: validators, editors, generators
- Zero learning curve for contributors

## Problem

Every team building MCP servers writes the same boilerplate:

1. Define tool names, descriptions, and input schemas by hand
2. Write HTTP handlers that map tool params to API calls
3. Set up stdio and HTTP transports
4. Wire auth, logging, config

This is tedious and drifts out of sync with the actual API. If you already have an OpenAPI spec, the MCP server should build itself.

## How it works

```
openapi.json → library reads + parses → auto-builds MCP tools → serves
```

Each OpenAPI operation becomes an MCP tool. Parameters, request bodies, and auth are wired automatically. No code generation — it reads the spec at startup.

### Minimal example

```go
package main

import "github.com/NubeIO/openmcp"

func main() {
    openmcp.QuickStart(openmcp.Config{
        Name:    "my-api",
        Version: "1.0.0",
        Spec:    "./openapi.json",
        BaseURL: "http://localhost:9000",
    })
}
```

That's it. Every operation in the spec is now an MCP tool.

### What the library does automatically

1. Loads and validates the OpenAPI 3.x spec (JSON or YAML)
2. For each operation found:
   - Generates tool name from `operationId` (e.g. `nodes_create`)
   - Pulls description from `summary` / `description`
   - Builds input schema from `parameters` + `requestBody`
   - Creates an HTTP handler from `method` + `path` + `servers[0]`
3. Separates params by location (`in: path`, `in: query`, `in: header`)
4. Resolves `$ref` to inline schemas using the spec's `components/schemas`
5. Registers all tools on the MCP server
6. Starts serving (stdio or HTTP)

### End-to-end example

User says to Claude: "Add a 5-second timer to my office controller"

```
Claude calls tool: nodes_create
  params: { orgId: "default", deviceId: "office-ctrl", type: "trigger.timer",
            position: {x:0, y:0}, data: {settings: {interval: 5000}} }
       ↓
Library sees nodes_create was built from OpenAPI operation:
  method: POST
  path:   /api/v1/orgs/{orgId}/devices/{deviceId}/nodes
  parameters: orgId (path), deviceId (path), allowUnknown (query)
  requestBody: $ref → #/components/schemas/NodeCreate
       ↓
Library auto-separates params by source:
  path:  orgId, deviceId         → substituted into URL
  query: allowUnknown            → appended as ?key=value
  body:  everything else         → JSON request body
       ↓
Sends: POST http://localhost:9000/api/v1/orgs/default/devices/office-ctrl/nodes
       Authorization: Bearer eyJ...
       Body: {"type":"trigger.timer","position":{"x":0,"y":0},"data":{...}}
       ↓
Returns JSON response as MCP tool result
```

No tool code was written. The library built it from the OpenAPI spec.

## Package structure

```
pkg/openmcp/
│
├── openmcp.go                      # QuickStart() and NewServer() — top-level entry
│
├── spec/                           # OpenAPI spec loading and parsing
│   ├── loader.go                   # Load() — reads JSON/YAML, validates
│   ├── types.go                    # Thin wrappers over OpenAPI types
│   └── loader_test.go
│
├── tools/                          # Auto-builds MCP tools from parsed spec
│   ├── builder.go                  # Iterates operations, builds tool defs
│   ├── registry.go                 # Holds built tools + optional custom tools
│   ├── naming.go                   # operationId → tool name, collision handling
│   └── builder_test.go
│
├── handlers/                       # Executes tool calls
│   ├── handler.go                  # Handler interface
│   ├── http.go                     # HTTPHandler — builds + sends request from spec metadata
│   └── http_test.go
│
├── client/                         # HTTP client for target API
│   ├── client.go                   # Client struct (base URL, headers, timeout)
│   ├── auth.go                     # Bearer token, API key, basic auth
│   ├── request.go                  # Builds URL from path template + params
│   └── client_test.go
│
├── config/                         # Configuration
│   └── config.go                   # Config struct, env var overrides
│
├── server/                         # MCP protocol server
│   ├── server.go                   # Registers tools onto mcp-go server
│   ├── stdio.go                    # stdio transport
│   ├── http.go                     # HTTP/SSE transport
│   └── logging.go                  # Log setup
│
├── plugins/                        # JavaScript plugin system (Goja)
│   ├── loader.go                   # Scan plugins dir, load plug.js + plugin.yaml
│   ├── runtime.go                  # Goja VM setup, inject host API (http, log, env)
│   ├── watcher.go                  # fsnotify hot-reload, debounce, atomic swap
│   └── loader_test.go
│
└── cli/                            # Optional Cobra integration
    ├── command.go                  # NewMCPCommand() → *cobra.Command
    └── flags.go                    # --transport, --addr, --log, --spec, --plugins
```

## Layer diagram

```
┌──────────────────────────────────────────────┐
│  Your code: openmcp.QuickStart(config)       │
└─────────────────────┬────────────────────────┘
                      │
           ┌──────────▼──────────┐
           │  openmcp.go         │
           │  wires everything   │
           └──────────┬──────────┘
                      │
      ┌───────────────┼───────────────┐
      │               │               │
      ▼               ▼               ▼
   ┌──────┐     ┌──────────┐    ┌─────────┐
   │spec/ │     │ config/  │    │ cli/    │
   │load  │     │ env+struct│    │ cobra  │
   │parse │     └────┬─────┘    └────┬────┘
   └──┬───┘          │               │
      │              │               │
      ▼              │               │
   ┌──────────┐      │               │
   │ tools/   │      │               │
   │ build    │      │               │
   │ registry │      │               │
   └──────┬───┘      │               │
          │          │               │
   ┌──────┴───┐      │               │
   │          │      │               │
   ▼          ▼      ▼               │
┌──────────┐ ┌────────┐             │
│handlers/ │ │client/ │             │
│ http     │─│ request│             │
│          │ │ auth   │             │
└──────────┘ └────────┘             │
          │                         │
   ┌──────┴─────────────────────────┘
   │
   ▼
┌────────────────────────┐
│ server/                │
│ registers tools        │
│ serves stdio or http   │
└────────────────────────┘
```

## Key interface

### Handler

```go
type Handler interface {
    Handle(ctx context.Context, toolName string, params map[string]any) (*Result, error)
}

type Result struct {
    Data    any
    IsError bool
}
```

HTTPHandler is the main implementation. It knows which params are path, query, header, or body — all derived from the OpenAPI spec. It builds and sends the HTTP request. No business logic.

## Config

```go
type Config struct {
    Name      string   // MCP server name
    Version   string   // MCP server version
    Spec      string   // Path or URL to OpenAPI spec (JSON or YAML)
    BaseURL   string   // Target API base URL (overrides spec servers[])
    Token     string   // Bearer token
    APIKey    string   // API key (header)
    Transport string   // "stdio" (default) or "http"
    Addr      string   // HTTP listen address (default ":8080")
    LogFile   string   // Log file path

    // Optional defaults injected into every tool call
    Defaults  map[string]string  // e.g. {"orgId": "default"}
}
```

All fields can be overridden by env vars:

| Env var | Field | Description |
|---------|-------|-------------|
| `OPENMCP_SPEC` | Spec | Path or URL to OpenAPI spec |
| `OPENMCP_BASE_URL` | BaseURL | Target API base URL |
| `OPENMCP_TOKEN` | Token | Bearer token |
| `OPENMCP_API_KEY` | APIKey | API key |
| `OPENMCP_TRANSPORT` | Transport | `stdio` or `http` |
| `OPENMCP_ADDR` | Addr | HTTP listen address |
| `OPENMCP_LOG` | LogFile | Log file path |

## Usage patterns

### Pattern 1: One-liner from OpenAPI spec

```go
func main() {
    openmcp.QuickStart(openmcp.Config{
        Name:    "my-api",
        Version: "1.0.0",
        Spec:    "./openapi.json",
        BaseURL: "http://localhost:9000",
        Token:   os.Getenv("API_TOKEN"),
    })
}
```

### Pattern 2: Hand-written tools (no OpenAPI)

```go
func main() {
    srv := openmcp.NewServer("dev-tools", "1.0.0")

    srv.AddTool(openmcp.Tool{
        Name:        "ping",
        Description: "Health check",
        Handler: func(ctx context.Context, _ string, _ map[string]any) (*openmcp.Result, error) {
            return openmcp.OK(map[string]any{"status": "pong"})
        },
    })

    srv.Serve() // stdio by default
}
```

### Pattern 3: OpenAPI + custom tools together

```go
func main() {
    srv := openmcp.NewServer("my-api", "1.0.0")
    srv.LoadSpec("./openapi.json", openmcp.WithBaseURL("http://localhost:9000"))

    // Custom tool alongside the auto-built ones
    srv.AddTool(openmcp.Tool{
        Name:        "run_tests",
        Description: "Run the test suite",
        Handler:     openmcp.ExecHandler("make", "test"),
    })

    srv.Serve()
}
```

### Pattern 4: Cobra subcommand in existing CLI

```go
rootCmd.AddCommand(openmcp.NewCommand("my-api", "1.0.0"))
```

```bash
my-api mcp serve --spec=./openapi.json --transport=http --addr=:8080
```

### Pattern 5: Filter tools (expose subset of API)

```go
srv := openmcp.NewServer("my-api", "1.0.0")
srv.LoadSpec("./openapi.json", openmcp.WithBaseURL("http://localhost:9000"))

// Only expose GET endpoints
srv.Filter(func(t openmcp.ToolDef) bool {
    return t.Method == "GET"
})

srv.Serve()
```

### Pattern 6: Mount on existing Gin router

```go
r := gin.Default()
r.GET("/health", healthHandler)
r.Any("/mcp", gin.WrapH(srv.HTTPHandler()))
r.Run(":8080")
```

## Tool naming

Tools are named from `operationId` in the OpenAPI spec:

| OpenAPI operationId | MCP tool name |
|---------------------|---------------|
| `nodes_create` | `nodes_create` |
| `getUsers` | `getUsers` |
| `admin_disable-org` | `admin_disable_org` (dashes → underscores) |

If no `operationId`, falls back to `{method}_{path}` (e.g. `post_api_v1_nodes`).

Collisions are resolved by appending `_2`, `_3`, etc.

## Extensibility

### Override a single auto-built handler

```go
srv.LoadSpec("./openapi.json", openmcp.WithBaseURL("http://localhost:9000"))

srv.Override("nodes_create", func(ctx context.Context, name string, params map[string]any) (*openmcp.Result, error) {
    // Custom validation before calling the API
    if params["type"] == "" {
        return openmcp.Error("type is required")
    }
    return srv.CallAPI(ctx, "nodes_create", params)
})
```

### Middleware (before/after hooks)

```go
srv.Use(openmcp.Before(func(ctx context.Context, toolName string, params map[string]any) error {
    log.Printf("[%s] called with %v", toolName, params)
    return nil
}))

srv.Use(openmcp.After(func(ctx context.Context, toolName string, result *openmcp.Result, dur time.Duration) {
    log.Printf("[%s] completed in %s", toolName, dur)
}))
```

### Response transform

```go
// Unwrap {data, meta} envelope
srv.Use(openmcp.Transform(func(ctx context.Context, name string, r *openmcp.Result) *openmcp.Result {
    if m, ok := r.Data.(map[string]any); ok {
        if data, exists := m["data"]; exists {
            r.Data = data
        }
    }
    return r
}))
```

### Access raw mcp-go server

```go
raw := srv.Raw() // *server.MCPServer from mark3labs/mcp-go
raw.AddResource(...)
```

## Plugins (JavaScript via Goja)

Plugins let anyone add MCP tools by dropping files in a folder. No Go, no compiling, no toolchain. Edit a `.js` file, the server picks it up live.

Uses [Goja](https://github.com/dop251/goja) — a pure-Go JavaScript engine (ES5.1+). Compiles into the binary. Zero install for users on any platform.

### Plugin directory structure

```
plugins/
├── restart-device/
│   ├── plug.js              # required — handler logic
│   └── plugin.yaml          # optional — metadata, args, config
├── slack-notify/
│   ├── plug.js
│   └── plugin.yaml
└── health-check/
    └── plug.js              # minimal plugin — no yaml needed
```

Each subfolder in `plugins/` is one plugin. Only `plug.js` is required.

### plugin.yaml (optional)

Declares metadata and args so `plug.js` stays pure logic. If omitted, the plugin must export `name` and `description` from JS.

```yaml
name: restart-device
description: Restart an edge device by ID
version: 0.1.0

args:
  - name: deviceId
    type: string
    required: true
    description: The device to restart
  - name: force
    type: boolean
    required: false
    description: Force restart even if device is mid-operation

# Optional static config passed to the handler
config:
  timeout: 30
  retries: 2
```

### plug.js

```js
// plugins/restart-device/plug.js

function handle(params, config) {
    if (params.force) {
        log.warn("Force restarting device: " + params.deviceId)
    }

    var resp = http.post("/api/v1/devices/" + params.deviceId + "/restart", {
        timeout: config.timeout
    })

    if (resp.status === 404) {
        return error("Device not found: " + params.deviceId)
    }

    return resp.data
}
```

If there's no `plugin.yaml`, export metadata from JS:

```js
// plugins/health-check/plug.js — minimal, no yaml

var name = "health_check"
var description = "Check if the target API is reachable"

function handle(params) {
    var resp = http.get("/health")
    return { status: resp.status, ok: resp.status === 200 }
}
```

### Host API exposed to plugins

The server injects these globals into the JS runtime:

| Global | Description |
|--------|-------------|
| `http.get(path, opts?)` | GET request to the target API (uses server's base URL + auth) |
| `http.post(path, body?, opts?)` | POST request |
| `http.put(path, body?, opts?)` | PUT request |
| `http.patch(path, body?, opts?)` | PATCH request |
| `http.delete(path, opts?)` | DELETE request |
| `log.info(msg)` | Log at info level |
| `log.warn(msg)` | Log at warn level |
| `log.error(msg)` | Log at error level |
| `error(msg)` | Return an MCP error result |
| `env(key)` | Read an environment variable |
| `config` | The `config` object from `plugin.yaml` (or `{}`) |

Plugins reuse the server's HTTP client — they get base URL, auth, and timeout for free. No raw URL construction needed.

**Not exposed:** filesystem access, network access to arbitrary hosts, exec/shell. Plugins can only talk to the target API through the provided `http` object.

### Hot reloading

Plugins reload automatically when files change. No server restart needed.

**How it works:**

1. Server watches the `plugins/` directory using [fsnotify](https://github.com/fsnotify/fsnotify) (pure Go, cross-platform)
2. On file change (create, modify, delete):
   - **Changed `plug.js` or `plugin.yaml`** → reload that plugin (re-parse yaml, re-evaluate JS, re-register tool)
   - **New subfolder with `plug.js`** → load new plugin, register new tool
   - **Deleted subfolder or `plug.js`** → unregister tool
3. Tool registry is swapped atomically — in-flight tool calls finish on the old version, new calls use the new version
4. Reload events are logged: `[plugins] reloaded restart-device (plug.js changed)`

**What hot reload enables:**

- Edit a plugin while Claude is connected — next tool call uses the new code
- Develop plugins interactively: change logic, test with Claude, iterate
- Deploy plugin updates to a running server without downtime
- Add new tools to a live server by dropping a folder in `plugins/`

**Debounce:** File changes are debounced (200ms) to avoid reloading mid-save when editors write to temp files first.

### Loading order and precedence

```
1. OpenAPI spec tools load first      (auto-built from spec)
2. Go-registered custom tools         (srv.AddTool(...))
3. Plugin tools load last             (from plugins/ directory)
```

If a plugin has the same name as an OpenAPI or Go tool, the plugin wins (acts as an override). This lets you patch auto-built tool behavior with a script without touching Go code.

### Plugin lifecycle

```
Server starts
  │
  ├─ Scan plugins/ directory
  │   └─ For each subfolder with plug.js:
  │       ├─ Read plugin.yaml (if exists) → name, description, args, config
  │       ├─ Read plug.js → compile with Goja
  │       ├─ Extract name/description from JS (if no yaml)
  │       ├─ Inject host API (http, log, env, config)
  │       └─ Register as MCP tool
  │
  ├─ Start fsnotify watcher on plugins/
  │
  └─ Serve (stdio or HTTP)
        │
        ├─ File change detected → reload affected plugin
        ├─ New folder detected → load new plugin
        └─ Folder deleted → unregister plugin
```

### Go API for plugins

```go
srv := openmcp.NewServer("my-api", "1.0.0")
srv.LoadSpec("./openapi.json", openmcp.WithBaseURL("http://localhost:9000"))

// Load plugins from directory (enables hot reload)
srv.LoadPlugins("./plugins")

srv.Serve()
```

Or via CLI:

```bash
my-api mcp serve --spec=./openapi.json --plugins=./plugins
```

### Example: multi-step plugin

Plugins can chain API calls — something config alone can't do:

```js
// plugins/provision-device/plug.js

function handle(params) {
    // Step 1: Create the device
    var device = http.post("/api/v1/orgs/" + params.orgId + "/devices", {
        serial: params.serial,
        name: params.name
    })

    if (device.status !== 200) {
        return error("Failed to create device: " + device.data.message)
    }

    // Step 2: Assign default config
    http.put("/api/v1/devices/" + device.data.data.id + "/config", {
        pollInterval: 5000,
        timezone: params.timezone || "UTC"
    })

    // Step 3: Start runtime
    http.post("/api/v1/devices/" + device.data.data.id + "/runtime/start")

    return {
        message: "Device provisioned and started",
        deviceId: device.data.data.id
    }
}
```

```yaml
# plugins/provision-device/plugin.yaml
name: provision_device
description: Create a device, apply default config, and start its runtime
args:
  - name: orgId
    type: string
    required: true
  - name: serial
    type: string
    required: true
  - name: name
    type: string
    required: true
  - name: timezone
    type: string
    required: false
    description: Device timezone (default UTC)
```

## What the library does NOT do

- No AI/LLM calls — tool provider only, the AI client does the thinking
- No code generation — reads OpenAPI at startup, runtime only
- No opinions on your API design — wraps whatever the spec defines
- No custom spec formats — OpenAPI 3.x only (use converters for Swagger 2.0)

## Implementation order

### Phase 1 — core (get it working)

1. `spec/` — OpenAPI 3.x loader (JSON + YAML), validate, extract operations
2. `tools/` — auto-build MCP tools from operations, naming, registry
3. `handlers/` — Handler interface, HTTPHandler (path/query/body separation)
4. `client/` — HTTP client with auth + URL templating
5. `config/` — Config struct, env var overrides
6. `server/` — register tools onto mcp-go, serve stdio + HTTP
7. `openmcp.go` — QuickStart + NewServer entry points

### Phase 2 — usability

8. Default param injection (fill orgId, etc. from config)
9. Error mapping (HTTP 404 → "not found", 401 → "check token")
10. Rich auto-descriptions (append param info to tool descriptions)
11. `cli/` — Cobra command integration
12. Response transform middleware

### Phase 3 — plugins

13. `plugins/loader.go` — scan dir, parse plugin.yaml, load plug.js
14. `plugins/runtime.go` — Goja VM, inject host API (http, log, env, config)
15. `plugins/watcher.go` — fsnotify hot-reload with debounce + atomic swap
16. `--plugins` CLI flag

### Phase 4 — extensibility

17. Before/after middleware hooks
18. Tool filtering
19. Override individual handlers
20. Gin HTTP handler mount
21. Custom tool helpers (ExecHandler, FileHandler)

### Phase 5 — nice to have

22. Spec from URL (fetch remote OpenAPI spec at startup)
23. Dry-run mode (log requests without sending)
24. Caching for GET tools
25. Rate limiting
26. Audit logging

## Out of scope

| Feature | Why not |
|---------|---------|
| AI/LLM calls from tools | Too expensive. Tools return data, Claude does the thinking. |
| Code generation | Runtime loading is simpler, no build step. |
| Swagger 2.0 native support | Use existing converters (swagger2openapi). |
| GUI / dashboard | CLI and config are fine. |
| Custom spec formats (RAS, etc.) | OpenAPI only keeps the library universal. Internal formats should convert to OpenAPI first. |

## How Nube services use this

Nube services define APIs in RAS YAML (internal format). The existing `cmd/codgen/openapi` tool converts RAS → OpenAPI. So the flow is:

```
configs/ras/*.yaml → codgen/openapi → openapi.json → openmcp → MCP server
```

This keeps RAS as the internal source of truth while the open-source library only depends on the standard.
