# Fork Plan: openapi-mcp

## Source

- **Repo:** https://github.com/evcc-io/openapi-mcp
- **Fork to:** `github.com/NubeIO/openapi-mcp`
- **License:** Open source (MIT)
- **State:** 112 commits, appears stale — we take ownership

## Why fork (not depend, not start from zero)

| Option | Problem |
|--------|---------|
| Start from zero | 2-3 months to rebuild what already exists |
| Depend on it | Stale upstream, can't rely on PRs getting merged |
| **Fork** | **Get the working core, full control to evolve it** |

## What the fork gives us (day one)

Already working out of the box:

- `LoadOpenAPISpec()` — load JSON/YAML specs
- `ExtractOpenAPIOperations()` — parse all operations from spec
- `ExtractFilteredOpenAPIOperations()` — regex include/exclude filtering
- `BuildInputSchema()` — converts OpenAPI params + requestBody → JSON Schema
- `RegisterOpenAPITools()` — register operations as MCP tools on server
- `ServeStdio()` — stdio transport
- `ServeStreamableHTTP()` — HTTP transport
- `HandlerForStreamableHTTP()` — HTTP handler for custom routers (Gin)
- `LintOpenAPISpec()` — validate spec for issues
- `SelfTestOpenAPIMCP()` — verify MCP server matches spec
- Auth: API key, Bearer token, Basic, OAuth2
- Param handling: path, query, header, cookie, body — all automatic

## What we add on top

These are the things that make it ours — the value-add over the original:

### Phase 1 — make it ours

| Task | Description |
|------|-------------|
| Update module path | `github.com/NubeIO/openapi-mcp` |
| Update to latest Go | Go 1.22+ (currently requires 1.21) |
| Evaluate mcp-go vs official SDK | They use mark3labs/mcp-go — consider migrating to official `modelcontextprotocol/go-sdk` (v1.4.1, maintained with Google) |
| Review + clean | Remove anything we don't need, understand the internals |
| CI/CD | GitHub Actions for test, lint, build |
| QuickStart API | One-function entry point for simple use cases |

### Phase 2 — config and CLI

| Task | Description |
|------|-------------|
| Config struct | Name, version, spec path, base URL, token, transport, addr, log |
| Env var overrides | `OPENMCP_SPEC`, `OPENMCP_BASE_URL`, `OPENMCP_TOKEN`, etc. |
| Viper integration | Optional config.yaml loading |
| Default param injection | Auto-fill orgId, deviceId from config into every tool call |
| Cobra CLI | `serve --spec=... --transport=... --addr=... --plugins=...` |

### Phase 3 — JS plugins (Goja)

| Task | Description |
|------|-------------|
| Plugin loader | Scan `plugins/<name>/plug.js` + optional `plugin.yaml` |
| Goja runtime | JS VM with host API: `http`, `log`, `env`, `config` |
| Hot reload | fsnotify watcher, debounce (200ms), atomic tool swap |
| Plugin → MCP tool | Auto-register each plugin as an MCP tool |
| Precedence | Plugins override OpenAPI tools with same name |

### Phase 4 — middleware and extensibility

| Task | Description |
|------|-------------|
| Before/after hooks | Run logic before/after every tool call |
| Response transform | Unwrap `{data, meta}` envelopes, etc. |
| Tool filtering | Expose subset of tools (read-only server, etc.) |
| Override handlers | Replace auto-built handler with custom Go function |
| Gin mount | `gin.WrapH(srv.HTTPHandler())` for existing routers |
| Custom tools | `srv.AddTool()` for hand-written Go tools alongside OpenAPI ones |

### Phase 5 — nice to have

| Task | Description |
|------|-------------|
| Spec from URL | Fetch remote OpenAPI spec at startup |
| Dry-run mode | Log requests without sending |
| Caching | Cache GET responses for N seconds |
| Rate limiting | Throttle tool calls per minute |
| Audit logging | Log every tool call + result |
| Error mapping | HTTP 404 → "not found", 401 → "check token" |

## Architecture after fork

```
github.com/NubeIO/openapi-mcp
│
├── pkg/openapi2mcp/            # (from fork) core OpenAPI → MCP conversion
│   ├── loader.go               # LoadOpenAPISpec()
│   ├── operations.go           # ExtractOpenAPIOperations()
│   ├── tools.go                # RegisterOpenAPITools(), BuildInputSchema()
│   ├── server.go               # ServeStdio(), ServeStreamableHTTP()
│   ├── handlers.go             # HandlerForStreamableHTTP()
│   ├── lint.go                 # LintOpenAPISpec()
│   └── selftest.go             # SelfTestOpenAPIMCP()
│
├── pkg/plugins/                # (NEW) JS plugin system
│   ├── loader.go               # Scan dir, parse plugin.yaml, load plug.js
│   ├── runtime.go              # Goja VM, inject host API
│   ├── watcher.go              # fsnotify hot-reload
│   └── loader_test.go
│
├── pkg/config/                 # (NEW) Configuration
│   ├── config.go               # Config struct, Load()
│   └── env.go                  # Env var overrides
│
├── pkg/middleware/              # (NEW) Before/after/transform hooks
│   └── middleware.go
│
├── pkg/server/                 # (NEW) Wraps openapi2mcp with our additions
│   └── server.go               # NewServer(), QuickStart(), Serve()
│
├── cmd/openmcp/                # (REPLACE) Our CLI
│   └── main.go                 # Cobra: serve, lint, validate, list-tools
│
└── examples/
    ├── quickstart/             # Minimal one-file example
    ├── with-plugins/           # OpenAPI + JS plugins
    └── gin-mount/              # Mount on existing Gin router
```

## Consumer code after all phases

### Simplest

```go
package main

import "github.com/NubeIO/openapi-mcp/pkg/server"

func main() {
    server.QuickStart(server.Config{
        Name:    "my-api",
        Version: "1.0.0",
        Spec:    "./openapi.json",
        BaseURL: "http://localhost:9000",
        Plugins: "./plugins",
    })
}
```

### With custom tools and middleware

```go
srv := server.New(server.Config{
    Name:    "my-api",
    Version: "1.0.0",
    Spec:    "./openapi.json",
    BaseURL: "http://localhost:9000",
})

srv.LoadPlugins("./plugins")

srv.Use(server.Before(func(ctx context.Context, tool string, params map[string]any) error {
    log.Printf("[%s] %v", tool, params)
    return nil
}))

srv.Use(server.Transform(func(ctx context.Context, tool string, r *server.Result) *server.Result {
    // Unwrap {data, meta} envelope
    if m, ok := r.Data.(map[string]any); ok {
        if data, exists := m["data"]; exists {
            r.Data = data
        }
    }
    return r
}))

srv.AddTool(server.Tool{
    Name:        "run_tests",
    Description: "Run the test suite",
    Handler:     server.ExecHandler("make", "test"),
})

srv.Serve()
```

### CLI

```bash
# Serve from OpenAPI spec with plugins
openmcp serve --spec=./openapi.json --base-url=http://localhost:9000 --plugins=./plugins

# Lint a spec
openmcp lint ./openapi.json

# List tools that would be generated
openmcp tools ./openapi.json

# Validate MCP server matches spec
openmcp validate ./openapi.json
```

## Fork checklist

- [ ] Fork `evcc-io/openapi-mcp` → `NubeIO/openapi-mcp`
- [ ] Update `go.mod` module path
- [ ] Update Go version to 1.22+
- [ ] Run existing tests, fix if needed
- [ ] Review dependencies — update or replace stale ones
- [ ] Read through internals — understand what we own now
- [ ] Set up CI (GitHub Actions: test, lint, build)
- [ ] Strip or update branding/docs to reflect NubeIO ownership
- [ ] Tag v0.1.0 as our baseline
