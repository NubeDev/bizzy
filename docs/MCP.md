# MCP Tool Serving

The server exposes a per-user [Model Context Protocol](https://modelcontextprotocol.io/) (MCP) endpoint so that AI clients (Claude, Cursor, etc.) can discover and call tools from installed apps.

---

## Endpoint

| Path | Transport | Auth |
|---|---|---|
| `/mcp`, `/mcp/*path` | StreamableHTTP (SSE) | `Authorization: Bearer <token>` |

A single `StreamableHTTPHandler` resolves the calling user from the bearer token, looks up their enabled app installs, and builds a scoped MCP server on the fly.

---

## Architecture

```
MCP Client (Claude, Cursor, etc.)
       │
       ▼
  POST /mcp  (Bearer token)
       │
       ▼
 buildMCPHandler()          ← pkg/api/mcp.go
   ├─ Extracts token, resolves User
   ├─ Queries AppInstall records (enabled only)
   └─ Calls MCPFactory.BuildServer(installs)
              │
              ▼
        MCPFactory           ← pkg/apps/mcpfactory.go
          ├─ OpenAPI tools   (from openapi.yaml per app)
          ├─ JS tools        (from tools/*.js per app)
          └─ Prompts         (from prompts/*.md + auto-generated QA prompts)
              │
              ▼
        Per-user MCP Server returned to client
```

### Key files

| File | Responsibility |
|---|---|
| `pkg/api/mcp.go` | HTTP handler — authenticates the user and builds their scoped server |
| `pkg/api/router.go` | Mounts `/mcp` routes |
| `pkg/apps/mcpfactory.go` | Per-user server factory; registers OpenAPI, JS, and prompt tools |
| `mcp/` (external module) | OpenAPI-to-MCP conversion library (`github.com/NubeIO/openapi-mcp`) |

---

## MCPFactory

`MCPFactory` is created once at startup and reused for every request.

### Startup

1. Iterates all apps in the `Registry`.
2. For each app with an `openapi.yaml` (or `.json`), parses the spec and caches it in `specCache`.
3. Cache can be rebuilt at runtime via `MCPFactory.Rebuild()` (called by `POST /admin/reload-apps`).

### BuildServer(installs)

Called per-request. Creates a fresh `mcp.Server` scoped to the user's enabled installs:

```
for each enabled AppInstall:
  1. Look up App in registry
  2. If app has OpenAPI  → registerOpenAPITools()
  3. If app has JS tools → registerJSTools()
  4. Always             → registerPrompts()
```

---

## Tool types

### 1. OpenAPI tools

Source: `apps/<name>/openapi.yaml`

Each OpenAPI operation becomes an MCP tool. The factory:

- Reads the base URL from the user's install settings (falls back to the spec's `servers[0].url`, then `http://localhost:8080`).
- Injects the user's auth token (from install secrets) as a `Bearer` header.
- Generates AI-friendly descriptions with parameter guides and examples.
- Namespaces the tool: `appName.operationId` (e.g. `rubix-developer.getDevice`).

When called, the tool handler builds an HTTP request from the MCP arguments (path, query, header, body params), executes it against the target API, and returns the response.

### 2. JS tools

Source: `apps/<name>/tools/*.js` + `tools/*.json`

JavaScript tools run in a sandboxed [Goja](https://github.com/dop251/goja) runtime with host APIs:

| API | Purpose |
|---|---|
| `http.*` | Make HTTP requests |
| `secrets.*` | Read install secrets |
| `config.*` | Read install settings |
| `log.*` | Structured logging |
| `files.read` | Read files from the app directory |

Each tool has a JSON manifest defining its name, description, parameters, and execution mode. Timeout defaults to 5s (configurable via `timeout` in `app.yaml`).

**QA-mode tools**: If a JS tool's manifest sets `mode: "qa"`, the factory auto-registers a companion MCP prompt that drives the tool as a wizard — asking the user each question one at a time, then calling the tool with all collected answers.

### 3. Prompts

Source: `apps/<name>/prompts/*.md`

Markdown templates with `{{key}}` placeholders. Registered as MCP prompts so clients can request them with argument substitution. Namespaced as `appName.promptName`.

---

## Per-user scoping

Every piece of MCP state is scoped to the authenticated user:

- **Tool visibility**: only tools from enabled installs are registered.
- **Base URLs**: each user configures their own target URL per app (via install settings).
- **Secrets**: each user provides their own API tokens per app (via install secrets).
- **No cross-user leakage**: a fresh `mcp.Server` is built per request.

---

## Example: connecting Claude Desktop

Add to your Claude Desktop MCP config (`claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "nube": {
      "url": "http://localhost:8090/mcp",
      "headers": {
        "Authorization": "Bearer <your-token>"
      }
    }
  }
}
```

Claude will discover all tools from your installed apps and can call them directly.

---

## Admin operations

| Action | How |
|---|---|
| Reload apps + rebuild MCP cache | `POST /admin/reload-apps` |
| Change a user's app settings | `PATCH /app-installs/:id` with `settings` / `secrets` |
| Disable an app for a user | `PATCH /app-installs/:id` with `enabled: false` |
