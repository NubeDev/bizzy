# Current State: openapi-mcp

Last updated: 2026-04-06

## Module

- **Path:** `github.com/NubeIO/openapi-mcp`
- **Go:** 1.24
- **MCP SDK:** `github.com/modelcontextprotocol/go-sdk v0.6.0` (official SDK, maintained with Google)
- **OpenAPI parser:** `github.com/getkin/kin-openapi v0.133.0`

## What's done

### Core library (root package `openapi2mcp`)

The entire OpenAPI-to-MCP conversion engine is working:

| File | What it does |
|------|-------------|
| `spec.go` | `LoadOpenAPISpec()`, `ExtractOpenAPIOperations()`, `ExtractFilteredOpenAPIOperations()` |
| `register.go` | `RegisterOpenAPITools()` — converts operations to MCP tools with AI-friendly descriptions, example generation, auth info |
| `tool.go` | HTTP request building — path/query/header/cookie params, request body, auth injection, AI-friendly error responses |
| `schema.go` | `BuildInputSchema()` — OpenAPI params + requestBody → JSON Schema. Handles allOf/oneOf/anyOf, discriminators, bracket notation |
| `server.go` | `NewServer()`, `NewServerWithOps()` — create MCP server from spec |
| `error.go` | AI-optimized error responses for 400/401/403/404/5xx with troubleshooting guidance |
| `selftest.go` | `SelfTestOpenAPIMCP()` — validate generated tools match the spec |
| `http_lint.go` | HTTP linting endpoint with CORS and caching |
| `log.go` | HTTP request/response logging with auth header redaction |
| `openapi2mcp.go` | `OpenAPIOperation`, `ToolGenOptions` types |
| `types.go` | `LintIssue`, `LintResult`, `HTTPLintRequest` types |
| `summary.go` | `PrintToolSummary()` |

Auth support: API Key (header/query/cookie), Bearer Token, Basic Auth, OAuth2.

### pkg/config

`Config` struct with all fields needed to run a server. Environment variable overrides via `NUBE_MCP_` prefix (e.g. `NUBE_MCP_SPEC`, `NUBE_MCP_BASE_URL`). Legacy env compat (`BEARER_TOKEN`, `API_KEY`, `OPENAPI_BASE_URL`). Validation, defaults, auth propagation.

### pkg/middleware

`Before`, `After`, `Transform` middleware hooks with a `Chain` that runs them in order. Integrated into `pkg/server` — middleware wraps the HTTP client used for upstream API calls.

### pkg/server

High-level wrapper that ties everything together:

- `New(cfg)` — load spec, extract ops, apply tag/read-only filters
- `Build()` — create MCP server, register OpenAPI tools + custom tools
- `Serve()` — start stdio or HTTP transport
- `QuickStart(cfg)` — one-call entry point
- `AddTool(t)` — register hand-written tools alongside OpenAPI ones
- `Use(mw)` — add middleware
- `HTTPHandler()` — returns `http.Handler` for mounting on Gin/chi/etc.

### cmd/openmcp (Cobra CLI)

| Command | What it does |
|---------|-------------|
| `serve` | Start MCP server from an OpenAPI spec (stdio or HTTP) |
| `lint` | Comprehensive linting with best-practice suggestions |
| `validate` | Validate spec + run MCP self-test |
| `tools` | List tools that would be generated (with tag/regex filtering) |

### cmd/openapi-mcp (original CLI)

The original CLI from the fork. Supports `filter`, `validate`, `lint`, `--dry-run`, `--doc`, `--mount`. Still builds but `cmd/openmcp` is the preferred CLI going forward.

## Demo app (`mcpdemo/`)

Separate binary that proves the core works. Points at an external API via `--base-url`.

- Uses `pkg/server.New()` + `Serve()`
- Adds a custom `device_summary` tool
- Adds a logging middleware
- OpenAPI spec at `mcpdemo/openapi.yaml` (7 endpoints: health, CRUD devices, CRUD points)
- Works with Claude Code, Copilot, and Codex via `.vscode/mcp.json`

### Fake API server (`fakeserver/`)

Standalone Gin server with seed data (3 devices, 6 points). Run it separately, point the MCP server at it. Simulates a real Nube API for testing.

## What's NOT done

| Item | Status | Notes |
|------|--------|-------|
| Goja JS plugins | Not started | Needs design doc first (timeout/sandbox/concurrency model) |
| Viper config file loading | Not started | `pkg/config` does env + flags but not `config.yaml` |
| Default param injection | Not started | `Config.DefaultParams` field exists but isn't wired |
| Spec from URL | Not started | Currently file-only |
| Caching (GET responses) | Not started | |
| Rate limiting | Not started | |
| Audit logging | Not started | |
| CI/CD (GitHub Actions) | Not started | |

## Architecture

```
github.com/NubeIO/openapi-mcp
│
├── *.go                        # Core library (openapi2mcp package)
│   ├── spec.go                 # Load/parse/extract OpenAPI specs
│   ├── register.go             # Register operations as MCP tools
│   ├── tool.go                 # HTTP request building + execution
│   ├── schema.go               # JSON Schema conversion
│   ├── server.go               # NewServer(), NewServerWithOps()
│   ├── error.go                # AI-friendly error responses
│   ├── selftest.go             # Tool validation
│   ├── http_lint.go            # HTTP linting endpoint
│   └── ...
│
├── pkg/config/config.go        # Config struct + env overrides
├── pkg/middleware/middleware.go # Before/After/Transform hooks
├── pkg/server/server.go        # High-level server wrapper
│
├── cmd/openmcp/main.go         # Cobra CLI (serve, lint, validate, tools)
└── cmd/openapi-mcp/            # Original CLI (filter, validate, lint, dry-run, doc)
```

## Consumer code

### Simplest

```go
server.QuickStart(config.Config{
    Name:    "my-api",
    Spec:    "./openapi.yaml",
    BaseURL: "http://localhost:9000",
})
```

### With custom tools and middleware

```go
srv, _ := server.New(config.Config{
    Name:    "my-api",
    Spec:    "./openapi.yaml",
    BaseURL: "http://localhost:9000",
})

srv.Use(middleware.Before(func(_ context.Context, tool string, _ map[string]any) error {
    log.Printf("[%s] called", tool)
    return nil
}))

srv.AddTool(server.Tool{
    Name:        "my_custom_tool",
    Description: "Does something custom",
    Handler: func(ctx context.Context, params map[string]any) (string, error) {
        return "result", nil
    },
})

srv.Serve()
```

### CLI

```bash
openmcp serve --spec=./api.yaml --base-url=http://localhost:9000
openmcp serve --spec=./api.yaml --transport=http --mcp-addr=:8080
openmcp lint ./api.yaml
openmcp validate ./api.yaml
openmcp tools --tag=devices ./api.yaml
```

### VS Code / Claude Code config

```json
{
  "servers": {
    "my-api": {
      "type": "stdio",
      "command": "./bin/mcpdemo",
      "args": ["--base-url", "http://localhost:9000"]
    }
  }
}
```
