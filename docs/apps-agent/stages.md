# Implementation Stages

## How it works

There are two ways to use the system. Path 1 is the primary experience — it works today.

### Path 1: Claude Code (primary — no API key needed)

The server is a **tool provider**. Claude Code is the LLM. The user talks naturally, Claude calls the tools.

```
User opens Claude Code
  → connects to nube-server /mcp endpoint (one line of config)
  → gets their scoped tools + prompts (only from installed apps)
  → "check offline devices and restart them"
  → Claude calls rubix-developer.device_summary → rubix-developer.restart_device
  → done
```

No API key on the server. No agent service. No token cost on the server side. Claude Code does all the thinking — the server just provides tools, prompts, and API access.

This works with any MCP client: Claude Code, Claude Desktop, VS Code Copilot, Cursor, etc.

### Path 2: Server-side agent (future — for users without Claude Code)

For headless/automated use cases where no human is at a keyboard:

```
CLI:     nube run "generate energy report for building-7"
API:     POST /agent/tasks {prompt: "..."}
Cron:    scheduled tasks, webhooks
```

This is when you need an API key on the server (Anthropic, OpenAI, or local Ollama). The server runs the LLM loop itself. Zero tokens on the client side.

### The user's journey

```
Day 1:  "I use Claude Code"
        → nube login → nube apps install → add MCP config to Claude Code
        → start talking. Everything works. No API key needed on server.

Day 2:  "I want my ops team to get reports without Claude Code"
        → add ANTHROPIC_API_KEY to agent.yaml
        → nube run "daily device report"
        → server runs the task, returns result

Day 3:  "I want this on a cron"
        → POST /agent/tasks with schedule
        → server runs it daily, emails result

Day 4:  "Can I use a cheaper model?"
        → switch to OpenAI or local Ollama in agent.yaml
        → same tools, different LLM
```

---

## Design principles

1. **Claude Code first** — the server provides tools, Claude Code provides the brain. No server-side LLM needed for the primary use case.
2. **Generic server, specific plugins** — the server knows nothing about BMS, devices, or energy. All domain logic lives in apps.
3. **Token efficiency** — fewer tools loaded = fewer tokens burned. Apps are scoped per-user, only installed apps load tools into the session.
4. **Ship fast** — team of 10, not Google. Each stage ships a usable product.
5. **Two interfaces** — MCP for AI clients (Claude Code, Copilot, Claude Desktop), REST + CLI for everything else.

---

## What already exists

### Core MCP engine (`mcp/` — separate Go module)

The `mcp/` package is a **generic OpenAPI-to-MCP engine**. It takes any OpenAPI spec, generates MCP tools, and serves them over stdio or HTTP. Key pieces:

- `mcp/pkg/server` — high-level wrapper: `New(config) → AddTool() → Serve()`
- `mcp/pkg/config` — env-based config (`NUBE_MCP_*`), supports stdio + HTTP transport
- `mcp/pkg/middleware` — before/after/transform hooks on tool calls
- `mcp/register.go` — AI-friendly tool generation from OpenAPI operations
- `mcp/tool.go` — HTTP executor with auth, safety confirmations, error formatting
- `mcp/selftest.go` — OpenAPI validation and linting
- `mcpdemo/` — example plugin: device API spec + custom `device_summary` tool
- `fakeserver/` — mock REST API for testing

### Multi-tenant server (`pkg/`, `cmd/nube-server/` — Stages 1-3)

Built on top of the core engine. One Go binary serves both the REST API and per-user MCP:

- `pkg/jsondb/` — generic file-backed JSON database with in-memory cache, mutex, atomic writes
- `pkg/models/` — Workspace, User, AppInstall data models
- `pkg/auth/` — Bearer token auth middleware, admin impersonation, role guards
- `pkg/apps/` — App registry (scan/parse/reload), prompt loader, JS tool runtime (Goja), host allowlist enforcement, per-user MCP server factory
- `pkg/api/` — Gin REST API: bootstrap, workspace/user CRUD, app catalog, install/uninstall, tools/prompts listing, MCP endpoint
- `cmd/nube-server/` — Entry point: loads DB + apps, serves REST + MCP on `:8090`
- `apps/rubix-developer/` — Sample app: OpenAPI spec (7 device tools) + 2 JS tools + debug prompt
- `apps/nube-marketing/` — Sample app: 2 marketing prompts, zero tools, zero token cost

### CLI (`pkg/cli/`, `cmd/nube/` — Stage 4)

Commands auto-generated from the server's OpenAPI spec (`api/openapi.yaml`). The spec is embedded in the binary — add an endpoint, rebuild, the command appears.

- `pkg/cli/` — Config management, HTTP client, table/JSON output formatting
- `pkg/cli/openapi/` — OpenAPI-to-Cobra mapper: operations → commands, params → flags
- `pkg/cli/cmd_login.go` — Hand-written login/logout (needs interactive prompts)
- `cmd/nube/` — Entry point: login + auto-generated commands

---

## Stage 1 — Multi-tenant server + auth ✅ DONE

**Goal:** Multiple users connect to one cloud server, each with their own token and scoped access.

**What was built:**

| Task | Detail | File(s) |
|------|--------|---------|
| JSON DB | Generic `Collection[T]` with in-memory cache, file mutex, atomic writes (write-to-temp + rename) | `pkg/jsondb/db.go` |
| Workspace CRUD | REST: create, list, get, delete workspaces | `pkg/api/workspaces.go` |
| User CRUD | REST: create, list, get, delete users. User creation returns a bearer token | `pkg/api/workspaces.go`, `pkg/api/users.go` |
| Auth middleware | Gin middleware resolves `Authorization: Bearer <token>` → user → workspace. Rejects invalid/missing tokens | `pkg/auth/auth.go` |
| Admin impersonation | `X-Act-As-User` header for admins, audit logged to stderr | `pkg/auth/auth.go` |
| Token rotation | `POST /users/{id}/token` (rotate), `DELETE /users/{id}/token` (revoke). Users can rotate own; admins can rotate anyone's | `pkg/api/users.go` |
| Bootstrap | `POST /bootstrap` — creates first workspace + admin. Only works when no users exist | `pkg/api/bootstrap.go` |
| Health check | `GET /health` — no auth required | `pkg/api/router.go` |
| Models | Workspace, User structs with role enum (admin/user), ID/token generation | `pkg/models/models.go` |
| Entry point | `cmd/nube-server/main.go` — loads JSON DB, starts REST API on `:8090` | `cmd/nube-server/main.go` |

**Result:** Deploy one Go binary. Team members each get a token. Connect Claude Code with:
```json
{"url": "https://your-server.com/mcp", "headers": {"Authorization": "Bearer <token>"}}
```

---

## Stage 2 — App loader + install flow ✅ DONE

**Goal:** Apps are directories on disk. Users install them and get scoped tools/prompts in Claude Code. Only installed apps burn tokens in the session.

**What was built:**

| Task | Detail | File(s) |
|------|--------|---------|
| App scanner | On startup: scan `apps/` directory, parse each `app.yaml`, detect OpenAPI specs, prompts, and tools | `pkg/apps/types.go`, `pkg/apps/registry.go` |
| App catalog API | `GET /apps` — list available apps with settings schema, permissions. `GET /apps/{id}` — app detail + prompts | `pkg/api/apps.go` |
| App install API | `POST /apps/{id}/install` — validates required settings, separates secrets from settings, stores in `app_installs.json` | `pkg/api/apps.go` |
| Enable/disable | `PATCH /app-installs/{id}` — toggle without losing settings, update settings | `pkg/api/apps.go` |
| Uninstall | `DELETE /app-installs/{id}` — scoped to owning user | `pkg/api/apps.go` |
| Per-user tool scoping | MCP factory builds a per-user `mcp.Server` with only installed + enabled app tools/prompts. Factory function resolves user from bearer token per session | `pkg/apps/mcpfactory.go`, `pkg/api/mcp.go` |
| OpenAPI loader | Parses `openapi.yaml`, caches specs, generates namespaced tools (`{app}.{operationId}`) with per-user base URL and token injection via custom `RequestHandler` | `pkg/apps/mcpfactory.go` |
| Prompt loader | Parses `prompts/*.md` with YAML frontmatter (name, description, arguments), registers as MCP prompts with `{{key}}` template substitution | `pkg/apps/prompts.go`, `pkg/apps/mcpfactory.go` |
| Settings injection | User's install settings (base URL, API token) injected per-request. Two users, same app, different servers — resolved per session | `pkg/apps/mcpfactory.go` |
| App reload endpoint | `POST /admin/reload-apps` (admin-only) — re-scans `apps/` dir, rebuilds spec cache. No server restart needed | `pkg/api/apps.go`, `pkg/apps/registry.go` |
| Version tracking | Records installed `appVersion`, flags installs as `stale` when app version on disk changes | `pkg/models/appinstall.go`, `pkg/api/apps.go` |
| Duplicate prevention | Rejects install if user already has the same app installed | `pkg/api/apps.go` |
| Sample apps | `apps/rubix-developer/` (OpenAPI spec + 1 prompt = 8 tools), `apps/nube-marketing/` (2 prompts, zero tools) | `apps/` |

**Tested results — per-user scoping in Claude Code:**

| User | Apps installed | Tools in Claude Code | Prompts in Claude Code |
|------|---------------|---------------------|----------------------|
| Admin | rubix-developer + nube-marketing | 10 | 3 |
| Joe (no installs) | — | 0 | 0 |
| Joe (marketing only) | nube-marketing | 0 | 2 |

**Token impact:** A user who only installs `rubix-developer` (10 tools) burns ~600 tokens/turn on tool schemas. Without scoping, 50 tools across all apps = ~3000 tokens/turn. This saves 80%+ on every Claude Code interaction.

**Still TODO (deferred):**
- Secret encryption at rest (currently plaintext in `app_installs.json`)
- `allowedHosts` validation at install time

---

## Stage 3 — Script tools (JS only — MVP) ✅ DONE

**Goal:** Non-Go developers write tools in JS without recompiling the server. Drop a `.js` file in an app directory, reload, and it appears in Claude Code.

**What was built:**

| Task | Detail | File(s) |
|------|--------|---------|
| Tool manifest | `tools/*.json` — name, description, params, toolClass. Paired with matching `.js` file by name | `pkg/apps/toolmanifest.go` |
| JS runtime (Goja) | In-process VM per call. One fresh VM per execution, no shared state. Host API: `http.*`, `secrets.*`, `config.*`, `log.*`, `files.read()` | `pkg/apps/jsruntime.go` |
| HTTP sandbox | All outbound HTTP checked against app's `allowedHosts`. Redirects re-checked. IP literals blocked by default. Empty `allowedHosts` = no outbound HTTP | `pkg/apps/hostcheck.go`, `pkg/apps/hostcheck_test.go` |
| Timeout | Per-app configurable via `timeout` in `app.yaml`, default 5s. VM interrupted on timeout | `pkg/apps/jsruntime.go` |
| Namespaced registration | Tools registered as `{app}.{tool}` alongside OpenAPI-generated tools. Schema built from tool.json params | `pkg/apps/mcpfactory.go` |
| Registry integration | Tool manifests loaded on startup and reload, alongside OpenAPI specs and prompts | `pkg/apps/registry.go` |

**Sample tools (used from Claude Code):**

| Tool | What it does |
|------|-------------|
| `rubix-developer.device_summary` | Calls device API, returns total/online/offline count |
| `rubix-developer.restart_device` | Takes device offline then back online via PATCH |

**Python is deferred.** Python subprocesses have full filesystem access — unsafe for a multi-tenant central server without container-level isolation. JS covers 90% of use cases.

---

## Stage 4 — CLI ✅ DONE

**Goal:** Manage the server, browse apps, install/configure — without a web UI. Also browse available tools and prompts before using them in Claude Code.

**Key design: commands auto-generated from OpenAPI spec.** The server's REST API has an OpenAPI spec (`api/openapi.yaml`). Each operation has an `x-cli` annotation that maps it to a Cobra command. Add an endpoint to the spec, rebuild, the command appears.

**What was built:**

| Task | Detail | File(s) |
|------|--------|---------|
| OpenAPI spec | Full spec for the nube-server REST API with `x-cli` annotations | `api/openapi.yaml` |
| Config management | `nube login` / `nube logout` — stores server URL + token in `~/.nube/config.json` | `pkg/cli/config.go`, `pkg/cli/cmd_login.go` |
| HTTP client | Auth injection from config, flag overrides (`--server`, `--token`) | `pkg/cli/client.go` |
| Output formatting | Table (default) or JSON (`-o json`). Auto-detects arrays vs objects | `pkg/cli/output.go` |
| OpenAPI → Cobra mapper | Parses spec, maps operations → command tree, params → flags, body → flags, executes HTTP request | `pkg/cli/openapi/loader.go`, `pkg/cli/openapi/mapper.go` |
| Embedded spec | `go:embed` embeds the OpenAPI spec in the binary — no external files | `cmd/nube/main.go` |
| Tools/prompts REST endpoints | `GET /my/tools`, `GET /my/prompts`, `GET /my/prompts/{name}` — list and render user's available tools/prompts | `pkg/api/tools.go` |

**All CLI commands (auto-generated from spec + hand-written login):**

```
nube login <url> <token>                       # save credentials
nube logout                                    # clear credentials
nube status                                    # GET /health
nube bootstrap                                 # POST /bootstrap

nube apps list                                 # browse app catalog
nube apps get <id>                             # app detail + prompts + settings schema
nube apps install <id> --settings k=v,k=v      # install with settings

nube installs list                             # my installed apps
nube installs update <id> --enabled            # enable/disable
nube installs delete <id>                      # uninstall

nube tools list                                # my available tools (from installed apps)
nube prompts list                              # my available prompts
nube prompts get <name> --product X --audience Y   # render a prompt with arguments

nube users me                                  # who am I
nube users create <ws-id> --name X --email Y   # create user (admin)
nube users get <id>                            # get user (admin)
nube users delete <id>                         # delete user (admin)

nube tokens rotate <id>                        # rotate token
nube tokens revoke <id>                        # revoke token

nube workspaces list                           # list workspaces
nube workspaces create --name X                # create workspace (admin)
nube workspaces get <id>                       # get workspace
nube workspaces delete <id>                    # delete workspace (admin)
nube workspaces users <id>                     # list users in workspace

nube admin reload                              # re-scan apps directory (admin)
```

**Typical workflow:**

```bash
# Setup
nube login http://localhost:8090 <admin-token>

# Browse and install apps
nube apps list
nube apps install rubix-developer --settings rubix_host=http://myserver:1616,rubix_token=xxx

# Check what you have
nube tools list          # see available tools
nube prompts list        # see available prompts
nube prompts get rubix-developer.debug_network --device_id dev-003

# Then use them in Claude Code — add MCP config and start talking
```

---

## Stage 5 — Server-side agent (Path 2)

**Goal:** For users without Claude Code. Headless task runner — takes a prompt, picks tools, executes, returns result. No human in the loop.

**This is Path 2.** Path 1 (Claude Code) already works. This stage adds server-side LLM for CLI `nube run`, webhooks, and cron.

**Requires an API key on the server** (Anthropic, OpenAI, or local Ollama).

**What to build:**

| Task | Detail |
|------|--------|
| Agent loop | Task → load user's installed apps → LLM decides tools → execute → summarise |
| LLM adapter: Anthropic | Claude API with tool use. Start here |
| Task API | `POST /agent/tasks {prompt, allowedApps, maxSteps, allowDestructive, dryRun}` |
| Task status | `GET /agent/tasks/{id}` — status, steps, result, cost |
| Safety controls | `maxSteps` (default 20), `maxCost` ($0.50), `dryRun`, `allowDestructive` gate |
| Execution log | Every tool call logged: input, output, duration, token count. Stored in `agent_tasks.json` |
| CLI: `nube run` | `nube run "check offline devices"` → `POST /agent/tasks` → poll result |

**Token cost:** The agent runs server-side with only the tools it needs. A task scoped to `rubix-developer` loads 10 tools. The user's client (CLI, webhook) spends 0 tokens.

**Result:** `nube run "generate energy report for building-7"` → server calls device APIs, aggregates data, writes report, returns it.

---

## Stage 6 — Memory

**Goal:** The agent learns from past tasks. Doesn't re-discover the same facts every time.

**What to build:**

| Task | Detail |
|------|--------|
| Knowledge store | Per-user JSON files. Facts extracted from tool results |
| Pattern store | What worked / what didn't. Updated from user feedback |
| Workspace shared memory | Org-wide facts (e.g. "MQTT broker is at 10.0.0.5"). Admin-approved |
| Auto-extract | After task completes, LLM extracts facts from full execution log (once per task, not per tool call — avoids burning extra LLM calls) |
| Retrieval | Before the first LLM call, keyword/tag-based search of memory for relevant facts, inject into context. Semantic/embedding search can replace this later behind the same interface |
| Feedback API | `POST /agent/tasks/{id}/feedback {outcome, note}` — updates patterns |
| Promote/approve | User proposes fact for shared → admin approves/rejects |
| Pruning | Max 10MB per user. Old low-confidence facts evicted |

**Token impact:** Memory reduces repeat tool calls. Fewer tool calls = fewer LLM turns = fewer tokens. The agent gets faster and cheaper over time.

---

## Stage 7 — Web dashboard

**Goal:** Non-technical users (customers, marketing) can browse apps, install them, and use prompts from a browser.

**What to build:**

| Task | Detail |
|------|--------|
| Serve static files from Go binary | Embed a `web/` directory. No separate frontend deploy |
| Login page | Token-based auth (same as API) |
| App catalog | Browse, install, configure apps |
| Task launcher | Submit a task, see progress, view result (requires Stage 5) |
| Task history | List past tasks with status, cost, result |
| Report viewer | Render agent-generated reports (markdown → HTML) |
| Memory inspector | View/delete personal knowledge. Admins: manage shared memory |

**Keep it simple.** Server-rendered HTML + htmx, or a lightweight SPA (Preact/Svelte). Not React. Not Next.js. The Go binary serves everything.

---

## What to skip (until proven needed)

| Skip | Why |
|------|-----|
| Python tool runtime | Subprocess sandbox is unsafe for multi-tenant. Defer until container-level isolation (nsjail/gVisor) is built. JS covers 90% of use cases |
| Workspace-level app installs | Open questions (who owns secrets? per-user overrides?) add complexity. User-level installs cover the MVP |
| Flutter / native mobile app | Web dashboard covers 90% of the use case. Revisit when customers demand native mobile |
| OpenAI / Ollama LLM adapters | Start with Anthropic only. Add others when someone asks |
| Clustering / distributed DB | JSON DB handles <100 concurrent users. Migrate to SQLite when you hit the limit |
| OAuth2 / SSO | Bearer tokens work for a team of 10 + early customers. Add OAuth when onboarding enterprises |
| App marketplace / app store | Apps live in the `apps/` directory on the server. No need for a registry, versioning, or publishing flow yet |
| Embedding-based memory retrieval | Keyword/tag search is good enough for MVP. Add vector embeddings when memory grows large enough to need semantic search |

---

## Delivery timeline

| Stage | Status | What it unlocks |
|-------|--------|-----------------|
| 1 — Multi-tenant + auth | ✅ Done | Users connect to one server with their own tokens |
| 2 — App loader + install | ✅ Done | Per-user tool scoping in Claude Code — only installed apps load |
| 3 — Script tools (JS) | ✅ Done | Write tools in JS, drop in app dir, available in Claude Code |
| 4 — CLI | ✅ Done | Manage server, browse/install apps, preview prompts — all from terminal |
| 5 — Agent service | Next | `nube run` from CLI, webhooks, cron — for users without Claude Code |
| 6 — Memory | — | Agent learns from past tasks, fewer repeat tool calls |
| 7 — Web dashboard | — | Non-technical users can browse/use apps in a browser |

---

## Token cost summary

| How you use it | Who pays tokens | Cost |
|----------------|----------------|------|
| **Claude Code + MCP** (Path 1) | User's Claude subscription | Only installed app tools loaded — 80%+ savings vs loading all |
| **CLI direct commands** (`nube apps list`) | Nobody | Zero — REST calls, no LLM |
| **CLI `nube run`** (Path 2, Stage 5) | Server (your API key) | Medium — server-side LLM, scoped tools |
| **Agent tasks** (Path 2, Stage 5) | Server (your API key) | Medium — scoped tools, multi-step |
| **Agent + memory** (Stage 6) | Server (your API key) | Lower over time — fewer repeat tool calls |

The biggest win is **per-user app scoping** (Stage 2) — it cuts token cost for every Claude Code interaction. The second biggest is **memory** (Stage 6) — the agent stops re-discovering facts it already knows.
