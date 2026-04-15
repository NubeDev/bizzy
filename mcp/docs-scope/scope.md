# MCP Library — Scope

## Goal

A reusable Go library that reads RAS YAML and auto-builds an MCP server. No hand-written tool definitions. Any Nube service with `configs/ras/` gets a fully working MCP server from one line of Go.

## Problem

We already define every API in RAS YAML. Writing MCP tool registrations by hand duplicates that work and drifts out of sync. The library should read RAS and do everything else automatically.

## How it works

```
configs/ras/*.yaml → library reads + parses → auto-builds MCP tools → serves
```

That's it. No intermediate step. No code generation. No tool definitions to maintain.

### Consumer code

```go
// cmd/mcp/main.go — this is ALL you write per service
package main

import "github.com/NubeIO/developer-tools/pkg/mcplib"

func main() {
    mcplib.QuickStart("my-service", "0.1.0", "./configs/ras")
}
```

### What the library does automatically

1. Loads all `*.yaml` from the RAS directory, merges them
2. For each `resource.action` found:
   - Generates tool name: `{resource}_{action}` (e.g. `nodes_create`)
   - Pulls description from `action.description`
   - Builds input schema by combining `action.args` + resolved `action.body.schemaRef`
   - Creates an HTTP handler from `action.rest.method` + `prefix.rest` + `action.rest.path`
3. Registers all tools on the MCP server
4. Loads prompts from markdown files (if a prompts dir exists)
5. Starts serving (stdio or HTTP)

### End-to-end example: creating a node

User says to Claude: "Add a 5-second timer to my office controller"

```
Claude calls tool: nodes_create
  params: { orgId: "default", deviceId: "office-ctrl", type: "trigger.timer",
            position: {x:0, y:0}, data: {settings: {interval: 5000}} }
       ↓
Library sees nodes_create was built from RAS action:
  rest.method: POST
  rest.path:   /orgs/{orgId}/devices/{deviceId}/nodes
  args:        orgId (path), deviceId (path), allowUnknown (query), parentId (query)
  body:        schemaRef → NodeCreate
       ↓
Library auto-separates params by source:
  path:  orgId, deviceId         → substituted into URL
  query: allowUnknown, parentId  → appended as ?key=value
  body:  everything else         → JSON request body
       ↓
Sends: POST http://localhost:9000/api/v1/orgs/default/devices/office-ctrl/nodes
       Authorization: Bearer eyJ...
       Body: {"type":"trigger.timer","position":{"x":0,"y":0},"data":{...}}
       ↓
Returns Rubix JSON response as MCP tool result
       ↓
Claude: "Done — created a timer node on your office controller."
```

No code was written for this tool. The library built it from the RAS YAML that already existed.

## Package structure

```
pkg/mcplib/
│
├── mcplib.go                       # QuickStart() and NewServer() — top-level entry
│
├── ras/                            # RAS YAML loading and parsing
│   ├── loader.go                   # LoadRAS() — reads dir, merges all YAML files
│   ├── types.go                    # RAS, Resource, Action, Arg, Body, Response, Schema
│   ├── validate.go                 # Checks for missing fields, bad refs, etc.
│   └── loader_test.go
│
├── schema/                         # Schema resolution
│   ├── resolve.go                  # Resolves schemaRef and $ref to inline schemas
│   ├── merge.go                    # Merges args + body schema into one input schema
│   └── resolve_test.go
│
├── tools/                          # Auto-builds MCP tools from parsed RAS
│   ├── builder.go                  # Iterates resources/actions, builds tool defs
│   ├── registry.go                 # Holds built tools + optional custom tools
│   ├── naming.go                   # resource_action naming, collision handling
│   └── builder_test.go
│
├── handlers/                       # Executes tool calls (shared by server + CLI)
│   ├── handler.go                  # Handler interface
│   ├── http.go                     # HTTPHandler — builds + sends request from RAS metadata
│   ├── exec.go                     # ExecHandler — runs shell commands (custom tools)
│   ├── file.go                     # FileHandler — reads files/logs (custom tools)
│   └── http_test.go
│
├── client/                         # HTTP client for target service
│   ├── client.go                   # Client struct (base URL, headers, timeout)
│   ├── auth.go                     # Bearer token, API key strategies
│   ├── request.go                  # Builds URL from path template + params
│   └── client_test.go
│
├── config/                         # Configuration (env vars + viper)
│   ├── config.go                   # Config struct + Load()
│   ├── env.go                      # MCP_BASE_URL, MCP_TOKEN, etc.
│   └── viper.go                    # Reads from config.yaml if present
│
├── server/                         # MCP protocol server
│   ├── server.go                   # Registers tools from registry onto mcp-go server
│   ├── options.go                  # WithTransport(), WithAddr(), etc.
│   ├── stdio.go                    # stdio transport
│   ├── http.go                     # HTTP/SSE transport
│   └── logging.go                  # Log file setup
│
├── prompts/                        # Optional: MCP prompts from markdown
│   ├── loader.go                   # Reads .md files with frontmatter
│   ├── types.go                    # Prompt, PromptArg
│   └── loader_test.go
│
└── cli/                            # Cobra integration
    ├── command.go                  # NewMCPCommand() → *cobra.Command
    ├── flags.go                    # --transport, --addr, --log, --ras
    └── serve.go                    # Wires config + server + run
```

## Layer diagram

```
┌────────────────────────────────────────────────┐
│  Your code: mcplib.QuickStart("svc", "1.0.0", │
│             "./configs/ras")                    │
└────────────────────┬───────────────────────────┘
                     │
          ┌──────────▼──────────┐
          │  mcplib.go          │
          │  wires everything   │
          └──────────┬──────────┘
                     │
     ┌───────────────┼───────────────┐
     │               │               │
     ▼               ▼               ▼
  ┌──────┐     ┌──────────┐    ┌─────────┐
  │ ras/ │     │ config/  │    │prompts/ │
  │ load │     │ env+yaml │    │ .md     │
  │ parse│     └────┬─────┘    └────┬────┘
  └──┬───┘          │               │
     │              │               │
     ▼              │               │
  ┌────────┐        │               │
  │schema/ │        │               │
  │resolve │        │               │
  │merge   │        │               │
  └──┬─────┘        │               │
     │              │               │
     ▼              ▼               │
  ┌──────────────────────┐          │
  │ tools/               │          │
  │ auto-builds from RAS │          │
  │ registry holds them  │          │
  └──────────┬───────────┘          │
             │                      │
     ┌───────┴───────┐              │
     │               │              │
     ▼               ▼              │
  ┌──────────┐  ┌────────┐         │
  │handlers/ │  │client/ │         │
  │ http     │──│ request│         │
  │ exec     │  │ auth   │         │
  │ file     │  └────────┘         │
  └──────────┘                     │
             │                     │
     ┌───────┴─────────────────────┘
     │
     ▼
  ┌────────────────────────┐
  │ server/                │
  │ registers tools+prompts│
  │ serves stdio or http   │
  └────────────────────────┘
     │
     │  OR
     ▼
  ┌────────────────────────┐
  │ cli/                   │
  │ cobra subcommand       │
  │ same tools, same logic │
  └────────────────────────┘
```

## Key interfaces

### Handler (handlers/handler.go)

```go
type Handler interface {
    Handle(ctx context.Context, params map[string]any) (*Result, error)
}

type Result struct {
    Data    any
    IsError bool
}
```

HTTPHandler, ExecHandler, FileHandler all implement this. Server and CLI both call the same handlers — the transport doesn't matter.

### HTTPHandler (handlers/http.go)

This is the main one. Auto-created for every RAS action. It knows:
- Which params are path, query, or body (from `arg.In`)
- The HTTP method and URL template (from `action.rest`)
- Auth config (from `config/`)

It doesn't contain business logic. It just builds and sends the HTTP request that the RAS action describes.

## Config

Priority: env vars → viper config file → defaults

| Env var | Viper key | Description |
|---------|-----------|-------------|
| `MCP_BASE_URL` | `mcp.base_url` | Target service URL (e.g. `http://localhost:9000`) |
| `MCP_TOKEN` | `mcp.token` | Bearer token for API calls |
| `MCP_TRANSPORT` | `mcp.transport` | `stdio` (default) or `http` |
| `MCP_ADDR` | `mcp.addr` | HTTP listen address (default `:8080`) |
| `MCP_LOG` | `mcp.log` | Log file path |
| `MCP_ORG_ID` | `mcp.org_id` | Default org ID (optional) |
| `MCP_DEVICE_ID` | `mcp.device_id` | Default device ID (optional) |

Defaults (like `org_id`) are auto-injected into tool calls when the user doesn't provide them. This means Claude doesn't have to ask "which org?" every time.

## Custom tools

Most tools are auto-built from RAS. But some tools don't map to an API endpoint (dev tools, shell commands, file reads). These are added manually:

```go
srv := mcplib.NewServer("my-service", "0.1.0")
srv.LoadRAS("./configs/ras")

// Custom tools — not in RAS
srv.AddExec("run_tests", "Run the test suite", "make", "test")
srv.AddExec("git_log", "Recent commits", "git", "log", "--oneline", "-10")
srv.AddFile("read_log", "Tail service logs", "./logs/app.log", 100)

srv.Serve()
```

## Prompt markdown format

Optional. Place `.md` files in a prompts directory:

```markdown
---
name: create-node
description: Help the user create a new node
args:
  - name: device
    description: Target device name
    required: false
---

Help the user add a new node.

1. Ask what kind of node they want
2. Use `nodes_create` to build it on device "{{.device}}"
```

Loaded automatically if a prompts dir is passed to the server.

## Usage patterns

### Pattern 1: Standalone MCP server (most common)

```go
func main() {
    mcplib.QuickStart("rubix", "1.0.0", "./configs/ras")
}
```

### Pattern 2: Cobra subcommand in existing service

```go
// In your existing service
rootCmd.AddCommand(cli.NewMCPCommand("rubix", "1.0.0", "./configs/ras"))
```

```bash
rubix mcp serve --transport=http --addr=:8080
```

### Pattern 3: Custom tools alongside RAS tools

```go
srv := mcplib.NewServer("rubix", "1.0.0")
srv.LoadRAS("./configs/ras")
srv.AddExec("run_tests", "Run tests", "make", "test")
srv.Serve()
```

### Pattern 4: Multiple RAS sources

```go
srv := mcplib.NewServer("platform", "1.0.0")
srv.LoadRAS("./services/rubix/configs/ras")
srv.LoadRAS("./services/alerts/configs/ras")
srv.Serve()
```

## Adding a new service

1. You already have `configs/ras/*.yaml` (you write these for every service anyway)
2. Create `cmd/mcp/main.go`:
   ```go
   package main

   import "github.com/NubeIO/developer-tools/pkg/mcplib"

   func main() {
       mcplib.QuickStart("my-service", "0.1.0", "./configs/ras")
   }
   ```
3. Build and run:
   ```bash
   go build -o bin/mcp-server ./cmd/mcp
   ./bin/mcp-server
   ```

No tool definitions. No schemas to write. No handler code. The library reads your RAS and does everything.

## Deployment modes

| Mode | Command | Use case |
|------|---------|----------|
| stdio | `./mcp-server` | Local dev — Claude Code, Codex |
| HTTP | `./mcp-server --transport=http` | Cloud / shared — remote clients |
| Embedded | `myservice mcp serve` | Built into existing service CLI |

## Extensibility — custom logic and special use cases

The library auto-builds everything from RAS, but you can break out of that at any level. The server is always exposed — nothing is locked away.

### Override a single tool handler

RAS auto-builds `nodes_create` but you need custom logic (validation, side effects, etc.):

```go
srv := mcplib.NewServer("rubix", "1.0.0")
srv.LoadRAS("./configs/ras")

// Override the auto-built handler with your own logic
srv.Override("nodes_create", func(ctx context.Context, params map[string]any) (*mcplib.Result, error) {
    // Custom validation
    nodeType, _ := params["type"].(string)
    if !strings.HasPrefix(nodeType, "trigger.") && !strings.HasPrefix(nodeType, "sensor.") {
        return mcplib.ErrorResult("invalid node type: must start with trigger. or sensor.")
    }

    // Still call the original API — use the built-in HTTP client
    return srv.CallAPI(ctx, "nodes_create", params)
})

srv.Serve()
```

### Add custom tools with full handler control

```go
srv.AddTool(mcplib.Tool{
    Name:        "provision_device",
    Description: "Provision a new edge device with certificates and default config",
    Inputs: mcplib.Inputs{
        {Name: "serial", Type: "string", Required: true, Description: "Device serial number"},
        {Name: "site",   Type: "string", Required: true, Description: "Site to assign to"},
    },
    Handler: func(ctx context.Context, params map[string]any) (*mcplib.Result, error) {
        serial := params["serial"].(string)
        site := params["site"].(string)

        // Custom multi-step logic that doesn't map to a single API call
        cert, err := generateCert(serial)
        if err != nil {
            return nil, err
        }
        device, err := srv.CallAPI(ctx, "devices_create", map[string]any{
            "orgId": srv.Config().Default("org_id"),
            "serial": serial,
            "siteRef": site,
        })
        if err != nil {
            return nil, err
        }
        return mcplib.OK(map[string]any{
            "device": device.Data,
            "cert":   cert.Fingerprint,
        })
    },
})
```

### Access the underlying mcp-go server

For advanced MCP features the library doesn't wrap:

```go
srv := mcplib.NewServer("rubix", "1.0.0")
srv.LoadRAS("./configs/ras")

// Get the raw mcp-go server to do whatever you want
raw := srv.Raw() // returns *server.MCPServer from mark3labs/mcp-go

// Add MCP resources, custom capabilities, etc.
raw.AddResource(...)
```

### Middleware / hooks

Run logic before or after every tool call. Works for both auto-built and custom tools, both stdio and HTTP transport.

```go
srv := mcplib.NewServer("rubix", "1.0.0")
srv.LoadRAS("./configs/ras")

// Before every tool call
srv.Use(mcplib.Before(func(ctx context.Context, toolName string, params map[string]any) error {
    log.Printf("[%s] called with %v", toolName, params)

    // Block write tools for read-only users
    if isReadOnlyUser(ctx) && isWriteTool(toolName) {
        return fmt.Errorf("read-only access: cannot call %s", toolName)
    }
    return nil
}))

// After every tool call (for logging, metrics, etc.)
srv.Use(mcplib.After(func(ctx context.Context, toolName string, result *mcplib.Result, dur time.Duration) {
    log.Printf("[%s] completed in %s", toolName, dur)
}))

// Transform responses (e.g. unwrap the {data, meta} envelope)
srv.Use(mcplib.Transform(func(ctx context.Context, toolName string, result *mcplib.Result) *mcplib.Result {
    // Rubix wraps responses in {data: ..., meta: ...} — unwrap for cleaner MCP output
    if m, ok := result.Data.(map[string]any); ok {
        if data, exists := m["data"]; exists {
            result.Data = data
        }
    }
    return result
}))
```

### Filter tools (access control)

Expose different tools to different users. Auto-built tools are tagged by resource name and HTTP method.

```go
// End-user server — read-only, no admin tools
srv := mcplib.NewServer("rubix-user", "1.0.0")
srv.LoadRAS("./configs/ras")
srv.FilterTools(func(t mcplib.ToolDef) bool {
    // Only expose GET endpoints + specific write tools
    return t.Method == "GET" || t.Name == "nodes_create"
})
srv.Serve()
```

```go
// Dev server — everything
srv := mcplib.NewServer("rubix-dev", "1.0.0")
srv.LoadRAS("./configs/ras")
srv.Serve() // all tools exposed
```

### Extend the HTTP transport

If you need custom HTTP routes alongside the MCP endpoint (health checks, webhooks, etc.):

```go
srv := mcplib.NewServer("rubix", "1.0.0")
srv.LoadRAS("./configs/ras")

// Get the HTTP handler to mount on your own router
mcpHandler := srv.HTTPHandler()

// Use with Gin (your standard stack)
r := gin.Default()
r.Any("/mcp", gin.WrapH(mcpHandler))
r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
r.GET("/metrics", metricsHandler)
r.Run(":8080")
```

## Gaps, nice-to-haves, and future ideas

### Should have (build soon after core)

| Feature | Why |
|---------|-----|
| **Response unwrapping** | Rubix wraps all responses in `{data, meta}`. Library should auto-unwrap so Claude sees clean data. Configurable per service since not all APIs wrap the same way. |
| **Default param injection** | If `MCP_ORG_ID` is set, auto-fill `orgId` in every tool call. Claude shouldn't have to ask "which org?" every time. Already in config — just needs wiring into the handler. |
| **NATS handler** | RAS defines NATS subjects alongside REST. Add a `NATSHandler` that publishes/subscribes instead of HTTP. Same tool, different transport to the target service. |
| **Retry + timeout** | HTTP calls to target services should have configurable timeout and retry. One flaky call shouldn't kill the MCP session. |
| **Error mapping** | Map HTTP 404 → "not found", 401 → "unauthorized — check MCP_TOKEN", 500 → "server error". Give Claude actionable error messages, not raw status codes. |
| **Tool descriptions from RAS** | Auto-enhance descriptions with arg info: "Create a new node. Requires: orgId, deviceId. Body: type (string), position ({x,y})." Claude picks better tools when descriptions are rich. |

### Nice to have (build when needed)

| Feature | Why |
|---------|-----|
| **Dry-run mode** | `srv.SetDryRun(true)` — tools return "would send POST to /api/v1/..." without actually calling the API. Useful for testing and demos. |
| **Batch tool calls** | Some workflows need multiple API calls as one tool. E.g. "create node + connect edge + start runtime" as a single `create_and_wire` tool. The `composite` handler chains multiple RAS actions. |
| **OpenAPI import** | Some services might not have RAS but do have an OpenAPI spec. Add a loader that reads `openapi.json` and builds tools from that. Widens the library beyond Nube services. |
| **Tool versioning** | When RAS changes, tool schemas change. Add a version or hash to tool names so clients can detect changes. |
| **WebSocket streaming** | Some Rubix endpoints stream data (live node values). Support MCP streaming results for real-time tools. |
| **Auto-discovery** | Point at a running service URL, fetch its OpenAPI spec or RAS, auto-build tools. No local config files needed. |
| **Rate limiting** | Throttle tool calls per minute. Prevent Claude from hammering your API in a loop. |
| **Caching** | Cache GET responses for N seconds. If Claude calls `nodes_getAll` three times in one conversation, only hit the API once. |
| **Audit log** | Write every tool call + result to a log file. Who called what, when, with what params. Important for the end-user server. |
| **Multi-auth** | Different tools might need different auth. Public endpoints (login) vs authed endpoints vs admin endpoints. RAS already has `requiresAuth` and `requiresRole` — wire those into the handler. |

### Not doing (out of scope)

| Feature | Why not |
|---------|---------|
| AI/LLM calls from tools | Too expensive. Tools return data, Claude does the thinking. |
| Code generation | Runtime loading is simpler and doesn't need a build step. |
| Custom MCP protocol extensions | Stick to the standard MCP spec. |
| GUI / dashboard | CLI and config files are fine for our use case. |

## What the library does NOT do

- No AI/LLM calls — tool provider only
- No code generation — reads RAS at startup
- No service-specific logic — generic for any service
- No opinions on your API design — wraps whatever RAS defines
- No hand-written tool definitions required — auto-built from RAS

## Implementation order

### Phase 1 — core (get it working)

1. `ras/` — extract RAS loader from Rubix, make standalone
2. `schema/` — resolve refs, merge args + body into input schema
3. `handlers/` — Handler interface, HTTPHandler
4. `client/` — HTTP client with auth + URL templating
5. `tools/` — auto-build tools from parsed RAS, registry
6. `config/` — env + viper loading
7. `server/` — register tools onto mcp-go, serve stdio/http
8. `mcplib.go` — QuickStart convenience

### Phase 2 — usability

9. Default param injection (org_id, device_id)
10. Response unwrapping ({data, meta} → data)
11. Error mapping (HTTP status → readable messages)
12. Rich auto-descriptions
13. `cli/` — Cobra command integration

### Phase 3 — extensibility

14. Middleware (before/after/transform hooks)
15. Tool filtering (access control)
16. Override individual tool handlers
17. Raw server access
18. Gin HTTP handler mount
19. `prompts/` — markdown prompt loader

### Phase 4 — extras

20. `handlers/exec.go` + `handlers/file.go` — custom tool types
21. NATS handler
22. Dry-run mode
23. Caching for GET tools
24. Audit logging
