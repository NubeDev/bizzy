# Backend: nube-server

The main API server for NubeIO. Manages workspaces, users, the app store, app installs, MCP tool serving, AI agent sessions, and multi-provider AI execution — all in a single process.

---

## Quick start

```bash
make server          # starts on :8090
```

Bootstrap the first admin:

```bash
curl -X POST http://localhost:8090/bootstrap \
  -H 'Content-Type: application/json' \
  -d '{"workspaceName":"My Org","adminName":"Admin","adminEmail":"admin@example.com"}'
```

Returns `{workspace, user, token}`. Use the token as `Authorization: Bearer <token>`.

---

## Environment variables

| Variable | Default | Description |
|---|---|---|
| `NUBE_ADDR` | `:8090` | Listen address |
| `NUBE_DATA_DIR` | `./data` | Data directory (resolved to absolute path on startup) |
| `OLLAMA_HOST` | `http://localhost:11434` | Ollama server address |
| `OPENAI_API_KEY` | — | OpenAI API key (Phase 3) |
| `ANTHROPIC_API_KEY` | — | Anthropic API key (Phase 3) |
| `GEMINI_API_KEY` | — | Gemini API key (Phase 3) |

Apps live at `$NUBE_DATA_DIR/apps/`. There is no separate apps directory env var.

---

## Data storage

Everything lives under `NUBE_DATA_DIR/`:

```
data/
  apps/                    # All app disk files (system + user-created)
    nube-admin/
      app.yaml
      tools/*.js + *.json
    nube-marketing/
      app.yaml
      tools/*.js + *.json
      prompts/*.md
    weather-checker/       # Created via store API
      app.yaml
      tools/*.js + *.json
      prompts/*.md
  store_apps.json          # Metadata for ALL apps (ratings, visibility, author, etc.)
  app_installs.json        # Per-user app installations
  app_shares.json          # Share invites (by user or link)
  app_reviews.json         # Ratings and comments
  workspaces.json          # Multi-tenant workspaces
  users.json               # Users with bearer tokens
  sessions.json            # Agent session history (provider, model, cost, tokens)
```

All JSON collections use `pkg/jsondb.Collection[T]` -- thread-safe, atomic writes (tmp + rename).

### Startup migrations

On startup the server runs:

1. **Session provider backfill**: existing sessions without a `provider` field get `provider="claude"`.
2. **Store-to-disk migration**: any `StoreApp` record with inline tools/prompts but no disk directory gets its files written to `data/apps/`.
3. **Disk-to-store sync**: any app on disk without a `store_apps.json` record gets one auto-created (visibility=public, author from app.yaml).

---

## Authentication

Bearer token middleware on all routes except `/health` and `/bootstrap`.

- Token resolved to `User` from the `users.json` collection.
- **Dev mode**: if no `Authorization` header is sent, falls back to the first user in DB.
- **Admin impersonation**: set `X-Act-As-User: <userId>` header (admin-only).
- **WebSocket auth**: `?token=<bearer-token>` query param. Dev mode (`?token=dev` or no token) falls back to first user.

---

## API routes

### Public (no auth)

| Method | Path | Description |
|---|---|---|
| `GET` | `/health` | Health check (`{status, users, apps}`) |
| `POST` | `/bootstrap` | Create first workspace + admin (409 if users exist) |

### User management

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/users/me` | user | Current user info |
| `GET` | `/users/:id` | admin | Get user by ID |
| `DELETE` | `/users/:id` | admin | Delete user |
| `POST` | `/users/:id/token` | self/admin | Rotate token |
| `DELETE` | `/users/:id/token` | self/admin | Revoke token |

### Workspaces

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/workspaces` | user | List (scoped by role) |
| `GET` | `/workspaces/:id` | user | Get workspace |
| `POST` | `/workspaces` | admin | Create workspace |
| `DELETE` | `/workspaces/:id` | admin | Delete workspace |
| `POST` | `/workspaces/:id/users` | admin | Create user in workspace |
| `GET` | `/workspaces/:id/users` | user | List workspace users |

### App store — see [STORE.md](STORE.md) for full details

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/store/apps` | Browse all apps (query, category, sort, pagination) |
| `GET` | `/api/store/apps/:id` | App detail + installed flag |
| `POST` | `/api/store/apps/:id/install` | Install app for current user |
| `GET/POST/DELETE` | `/api/store/apps/:id/reviews` | Reviews CRUD |
| `GET/POST/PUT/DELETE` | `/api/my/apps/...` | Author CRUD (create, edit, publish, tools, prompts, sharing) |

### App installs

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/apps/:id/install` | user | Install an app |
| `GET` | `/app-installs` | user | List user's installs |
| `PATCH` | `/app-installs/:id` | user | Update install (settings, enable/disable) |
| `DELETE` | `/app-installs/:id` | user | Uninstall |

### Agents — AI execution

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/agents` | List agents (derived from installed apps) |
| `POST` | `/api/agents/tools/:name` | Call a tool directly (e.g. `weather-checker.get_weather`) |
| `GET` | `/api/agents/providers` | List AI providers (name, available, type, models[]) |
| `POST` | `/api/agents/run/sync` | Synchronous agent run (blocks until done) |
| `POST` | `/api/agents/jobs` | Submit async AI job (returns job_id immediately) |
| `GET` | `/api/agents/jobs/:id` | Poll job status and events (`?after=N` for incremental) |
| `DELETE` | `/api/agents/jobs/:id` | Cancel a running job |
| `GET` | `/api/agents/sessions` | List session history (provider, model, cost, tokens) |
| `GET` | `/api/agents/sessions/:id` | Get session detail with full result text |

### WebSocket

| Path | Auth | Description |
|---|---|---|
| `/api/agents/run` | `?token=` | Streaming agent chat (any provider) |
| `/api/agents/qa` | `?token=` | Interactive QA wizard flows |

### MCP

| Path | Description |
|---|---|
| `/mcp`, `/mcp/*path` | Per-user MCP tool serving (StreamableHTTP) |

### Admin

| Method | Path | Description |
|---|---|---|
| `POST` | `/admin/reload-apps` | Reload apps from disk + rebuild MCP cache |

---

## Service layer

Reusable application logic lives in `pkg/services/`, decoupled from HTTP handlers. The services can be consumed by REST handlers, CLI commands, workflow engines, or any other Go code without importing `pkg/api` or depending on gin.

### AgentService (`pkg/services/agent.go`)

| Method | Description |
|---|---|
| `EnrichPrompt(userID, prompt)` | Prepends memory + app context to a prompt (for CLI providers that don't support system messages) |
| `BuildSystemPrompt(userID)` | Returns memory + app context as a separate string for providers that support system messages (Ollama, OpenAI, Anthropic) |
| `ResolveProvider(reqProvider, reqModel, user)` | Returns provider and model with user defaults applied |
| `GetRunner(provider)` | Returns a runner, checking availability |
| `SaveSession(params)` | Persists a completed session with tool call log |
| `ListSessions(userID)` | Returns sessions for a user |
| `GetSession(id, userID)` | Returns a single session with ownership check |
| `ListAgents(userID)` | Returns agents (installed+enabled apps) for a user |
| `MCPURL()` | Returns the MCP endpoint URL |

### ToolService (`pkg/services/tools.go`)

| Method | Description |
|---|---|
| `ResolveTool(userID, toolName)` | Finds a JS tool by namespaced name, returns configured runtime |
| `CallTool(userID, toolName, params)` | Resolve + execute in one step |
| `ListTools(userID)` | Returns all tools from user's installed apps |
| `ListPrompts(userID)` | Returns all prompts from user's installed apps |
| `GetPrompt(userID, name, args)` | Renders a prompt with argument substitution |

### Prompt enrichment

Every AI run goes through prompt enrichment before being sent to the provider:

1. **Server memory** — shared context set by admin (e.g. deployment info, units)
2. **User memory** — per-user context (e.g. preferences, team assignments)
3. **App context** — auto-composed from installed apps: one line per app with description, plus per-tool descriptions from JS manifests and OpenAPI specs

For API-based providers (Ollama, future OpenAI/Anthropic/Gemini), the memory and app context are passed as a `system` message, keeping the user's prompt clean in the `user` message. For CLI-based providers (Claude), everything is prepended to the prompt since the CLI doesn't expose a system message parameter.

---

## AI provider system

The server supports multiple AI providers through a unified `Runner` interface. See [MULTI-PROVIDER.md](MULTI-PROVIDER.md) for full details.

### Available providers

| Provider | Type | How it works | Status |
|---|---|---|---|
| **Claude** | CLI | Shells out to `claude` binary, native MCP + session resume | Production |
| **Ollama** | API | HTTP to local `/api/chat`, streaming, model discovery | Production |
| **OpenAI** | API | OpenAI chat completions API | Phase 3 |
| **Anthropic** | API | Anthropic Messages API | Phase 3 |
| **Gemini** | API | OpenAI-compatible endpoint | Phase 3 |
| **Codex** | CLI | OpenAI Codex CLI binary | Legacy |
| **Copilot** | CLI | GitHub Copilot via `gh copilot` | Legacy |

### Provider selection

Every AI endpoint accepts `provider` and `model` parameters:

```bash
# WS (frontend/CLI)
{"prompt": "...", "provider": "ollama", "model": "gemma3"}

# REST sync
POST /api/agents/run/sync  {"prompt": "...", "provider": "ollama", "model": "gemma3"}

# Job submit
POST /api/agents/jobs      {"prompt": "...", "provider": "ollama", "model": "gemma3"}

# CLI
nube ask --provider ollama --model gemma3 "hello"
```

Default provider is `claude` if not specified.

### Job system

For async/automation use. Submit a job, get a UUID, poll for events:

```bash
# Submit
curl -X POST localhost:8090/api/agents/jobs \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"prompt":"generate report","provider":"ollama","model":"gemma3"}'
# Returns: {"job_id": "job-xxx", "status": "running"}

# Poll (incremental)
curl localhost:8090/api/agents/jobs/job-xxx?after=2 \
  -H "Authorization: Bearer $TOKEN"

# Cancel
curl -X DELETE localhost:8090/api/agents/jobs/job-xxx \
  -H "Authorization: Bearer $TOKEN"
```

Jobs store events in memory with a 10-minute cleanup after completion. Sessions are persisted to `sessions.json` on job completion.

---

## MCP tool serving

The `MCPFactory` builds a per-user MCP server scoped to their installed apps:

1. Iterates the user's `AppInstall` records.
2. For each enabled install, looks up the app in the registry.
3. Registers three types per app:
   - **OpenAPI tools** — from `openapi.yaml`, base URL + auth token from user settings.
   - **JS tools** — sandboxed Goja runtime with host APIs (`http.*`, `secrets.*`, `config.*`, `log.*`, `files.read`). Timeout per app (default 5s).
   - **Prompts** — markdown templates with `{{key}}` substitution.
4. All names are namespaced: `appName.toolName`, `appName.promptName`.

See [MCP.md](MCP.md) for full details.

---

## Models

### Workspace

| Field | Type | Notes |
|---|---|---|
| `id` | string | `ws-` prefix |
| `name` | string | |
| `createdAt` | time | |

### User

| Field | Type | Notes |
|---|---|---|
| `id` | string | `usr-` prefix |
| `workspaceId` | string | |
| `name` | string | |
| `email` | string | |
| `role` | string | `admin` or `user` |
| `token` | string | 32-byte hex bearer token |
| `createdAt` | time | |

### Session

| Field | Type | Notes |
|---|---|---|
| `id` | string | `ses-` prefix |
| `provider` | string | `claude`, `ollama`, `openai`, etc. |
| `model` | string | Model name used |
| `claude_session_id` | string | Claude CLI session ID for `--resume` |
| `agent` | string | App/agent name |
| `prompt` | string | User's prompt |
| `result` | string | Full response text |
| `status` | string | `done`, `error`, `cancelled` |
| `duration_ms` | int | Execution time |
| `cost_usd` | float | API cost ($0 for local providers) |
| `input_tokens` | int | Prompt tokens |
| `output_tokens` | int | Completion tokens |
| `tool_calls` | int | Number of tool invocations |
| `tool_call_log` | []ToolCallEntry | Detailed per-tool records (name, duration, status, error, payload sizes) |
| `user_id` | string | Owner |
| `created_at` | time | |

### ToolCallEntry

| Field | Type | Notes |
|---|---|---|
| `name` | string | Namespaced tool name (e.g. `rubix.query_nodes`) |
| `duration_ms` | int | Time taken for this tool call |
| `status` | string | `ok` or `error` |
| `error` | string | Error message if status is `error` |
| `input_bytes` | int | Request payload size |
| `output_bytes` | int | Response payload size |

### AppInstall

| Field | Type | Notes |
|---|---|---|
| `id` | string | `inst-` prefix |
| `appName` | string | Links to app by name |
| `appVersion` | string | |
| `workspaceId` | string | |
| `userId` | string | |
| `enabled` | bool | |
| `settings` | map[string]string | Non-secret config |
| `secrets` | map[string]string | Secret config (checked first by `GetSetting`) |
| `stale` | bool | True if app version changed since install |
| `createdAt` | time | |
| `updatedAt` | time | |

---

## Make targets

```
make server        # nube-server on :8090
make start         # same as server (foreground)
make stop          # stop servers
make reset         # wipe data + stop
make build         # build all binaries to bin/
make test          # Go unit + integration tests
make test-api      # run API test script against running servers
```
