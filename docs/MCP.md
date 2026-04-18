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
MCP Client (Claude CLI, Cursor, etc.)
       |
       v
  POST /mcp  (Bearer token)
       |
       v
 buildMCPHandler()          <- pkg/api/mcp.go
   +-- Extracts token, resolves User
   +-- Queries AppInstall records (enabled only)
   +-- Calls MCPFactory.BuildServer(installs)
              |
              v
        MCPFactory           <- pkg/apps/mcpfactory.go
          +-- OpenAPI tools   (from openapi.yaml per app)
          +-- JS tools        (from tools/*.js per app)
          +-- Prompts         (from prompts/*.md + auto-generated QA prompts)
              |
              v
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

Called per-request. Creates a fresh `mcp.Server` scoped to the user's enabled installs, plus active plugin tools:

```
for each enabled AppInstall:
  1. Look up App in registry
  2. If app has OpenAPI  -> registerOpenAPITools()
  3. If app has JS tools -> registerJSTools()
  4. Always             -> registerPrompts()

for each active Plugin:
  5. Register plugin tools -> registerPluginTools()   (NATS req/reply proxy)
  6. Register plugin prompts -> registerPluginPrompts()
```

See [PLUGIN-SYSTEM.md](PLUGIN-SYSTEM.md) for how plugins register tools/prompts over NATS.

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
| `plugins.*` | Discover and call plugin tools (`exists`, `info`, `list`, `call`) |
| `tools.call` | Call another tool in the same app |
| `base64.*` | Base64 encode/decode |
| `url.*` | URL query building and parsing |
| `crypto.*` | SHA-256, SHA-1, MD5 hashing and HMAC signatures |
| `env.get` | Read allowlisted environment variables |

Each tool has a JSON manifest defining its name, description, parameters, and execution mode. Timeout defaults to 5s (configurable via `timeout` in `app.yaml`).

**Prompt-mode tools**: If a JS tool's manifest sets `mode: "prompt"`, the factory registers it as an MCP prompt instead of a tool. The prompt template in the manifest is sent to the AI with argument substitution. No JS execution happens — the AI handles the task using the prompt and other available tools.

**QA-mode tools**: If a JS tool's manifest sets `mode: "qa"`, the factory auto-registers a companion MCP prompt that drives the tool as a wizard — asking the user each question one at a time via the `/api/agents/qa` WebSocket, then calling the tool with all collected answers.

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

## How MCP connects to AI providers

### Claude (native MCP)

Claude Code CLI has built-in MCP support. When a user runs a prompt through the server:

1. Server writes a temporary MCP config file pointing to `http://localhost:8090/mcp` with the user's token.
2. Claude CLI is spawned with `--mcp-config <file>`.
3. Claude discovers tools, calls them natively, and streams results back.

This is the **Path 1** integration — zero server-side orchestration needed. Claude handles the full tool-calling loop.

### Other providers (agent loop — Phase 4)

Non-Claude providers (Ollama, OpenAI, Anthropic, Gemini) don't have native MCP support. For these, the server becomes the MCP client:

```
User prompt
  -> Server loads user's installed tools (from MCPFactory)
  -> Convert MCP tool schemas to provider's function-calling format
  -> Send prompt + tool definitions to LLM API
  -> LLM responds with tool call request
  -> Execute tool via existing JS/OpenAPI runtime
  -> Feed result back to LLM
  -> Loop until final text response or maxSteps hit
```

This is **not yet built** — Phase 2 (Ollama) runs text-only chat without tools. Phase 4 adds the agent loop.

### Provider capability matrix

| Provider | MCP integration | Tool calling | Status |
|---|---|---|---|
| Claude (CLI) | Native — CLI handles everything | Full MCP tools | Working |
| Ollama | Server-side agent loop needed | Text-only for now | Phase 2 done, tools in Phase 4 |
| OpenAI | Server-side agent loop needed | Function calling API | Phase 3-4 |
| Anthropic | Server-side agent loop needed | Tool use API | Phase 3-4 |
| Gemini | Server-side agent loop needed | Function calling API | Phase 3-4 |

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
