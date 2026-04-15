# Scope: Apps & Agent

## Overview

A **multi-user central server** that provides tools, prompts, and API access to AI clients. Multiple users/customers connect to it. Apps are installed per-user. Each user only sees tools from their installed apps.

### Two paths to use the system

**Path 1 — Claude Code (primary, works now, no API key needed on server):**

The server is a tool provider. Claude Code is the LLM. The user adds one line of MCP config, connects, and starts talking. Claude calls the tools, the server executes them. No server-side LLM, no API key, no agent service.

```
Claude Code → connects to /mcp → gets user's scoped tools + prompts
            → user: "check offline devices and restart them"
            → Claude calls device_summary → restart_device
            → done
```

**Path 2 — Server-side agent (future, for users without Claude Code):**

For headless tasks (CLI `nube run`, webhooks, cron), the server runs its own LLM loop. This requires an API key (Anthropic, OpenAI, or local Ollama).

### Three systems on the server

1. **Apps** — installable bundles that add tools, prompts, and API specs to the server
2. **Agent** (Path 2) — headless service that uses LLM APIs to run tasks autonomously
3. **Memory** — persistent knowledge store that lets the agent learn over time

---

## Users & Workspaces

### Data model

```
Workspace (tenant/org)
  └── User (person)
       ├── installed apps (which apps this user has enabled)
       ├── app settings (secrets, base URLs per app)
       ├── memory (personal knowledge + patterns)
       └── agent tasks (history of what the agent did for them)
```

A **workspace** is the top-level tenant — could be a company, a project, a customer. A **user** belongs to a workspace. Apps are installed at the **user level** (personal) for MVP. Workspace-level shared installs (one install, all users) are deferred — the open questions (who owns the secrets? per-user overrides?) add complexity without blocking the core idea.

### Storage — JSON DB

No SQL, no migrations. A JSON file database — one file per collection, read into memory on startup, flushed on write.

```
data/
  workspaces.json        # [{id, name, createdAt}]
  users.json             # [{id, workspaceId, name, email, role, createdAt}]
  app_installs.json      # [{id, appId, workspaceId, userId, enabled, settings, createdAt}]
  agent_tasks.json       # [{id, userId, workspaceId, prompt, status, result, steps, cost, createdAt}]
  memory/
    knowledge/
      {userId}.json      # per-user knowledge facts
    patterns/
      {userId}.json      # per-user patterns
    shared/
      {workspaceId}.json # workspace-level shared knowledge
```

Each file is a JSON array. The server reads it on startup, keeps it in memory, writes back on mutation. No GORM, no SQLite, no schema migrations. If the data grows large enough to need a real DB later, the JSON structure maps 1:1 to tables.

**Hard limits (MVP):**

- Single-node only — no clustering, no distributed writes
- Low-write throughput — file mutex serialises all mutations, fine for <100 concurrent users
- No multi-process writers — one server process owns the `data/` directory
- Best-effort durability — writes are atomic (write-to-temp + rename) but not journaled
- If these limits are hit, migrate to SQLite — the JSON structure maps 1:1 to tables

### Auth model

Every API request is authenticated by a **bearer token** (one token per user, generated at user creation). The authenticated token determines the acting user — there is no `userId` field in request bodies. Tokens can be revoked and rotated via `DELETE /users/{id}/token` (revoke) and `POST /users/{id}/token` (rotate).

```
Authorization: Bearer <user-token>
  → server resolves token → user → workspace
  → all operations scoped to that user
```

**Admin impersonation:** Admins can act on behalf of another user via an explicit header. This is a privileged path with audit logging:

```
Authorization: Bearer <admin-token>
X-Act-As-User: <target-user-id>
  → logged as: "admin X acted as user Y"
```

CLI follows the same model:

```bash
# Normal — uses the token from config/env
agent run "check offline devices"

# Admin impersonation — explicit, logged
agent run --act-as=joe "check offline devices"
```

### Roles

| Role | Can do |
|------|--------|
| **admin** | Manage workspace, users, install/remove apps at workspace level, view all agent tasks |
| **user** | Install apps for themselves, configure their own app settings, run agent tasks, manage own memory |

---

## Apps

### What is an app

An app is a bundle that adds capabilities. It contains any combination of:

- **JS tools** — custom logic executed via Goja (sandboxed)
- **Prompts** — markdown templates for reusable skills/instructions
- **OpenAPI specs** — auto-generate tools from an API definition

### App structure

```
apps/
  rubix-developer/
    app.yaml              # required — metadata + permissions + settings schema
    tools/                # optional — JS tools
      restart_device.js
      bulk_provision.js
    prompts/              # optional — markdown prompts
      debug_network.md
      review_config.md
    openapi.yaml          # optional — auto-generates tools from spec
```

### app.yaml

```yaml
name: rubix-developer
version: 1.0.0
description: Tools and prompts for Rubix platform development
author: NubeIO

# Permissions — what this app is allowed to do
permissions:
  # Outbound HTTP hosts this app can call (JS tools + OpenAPI)
  allowedHosts:
    - "*.nubedge.com"
    - "localhost:1616"
  # Default tool class for this app's tools (can be overridden per-tool in tool.json)
  defaultToolClass: read-write   # read-only | read-write | destructive
  # Which secrets this app can access (by setting key, not raw env var)
  secrets:
    - rubix_token
    - rubix_host

# Settings schema — what the user fills in when installing
settings:
  - key: rubix_host
    label: Rubix Host URL
    type: url
    required: true
    default: http://localhost:1616
  - key: rubix_token
    label: API Token
    type: secret
    required: true

# Tags applied to all tools from this app
tags:
  - rubix
  - development

# Tool timeout (default 5s)
timeout: 10s
```

Key changes from v1:
- **`permissions`** — declares what the app needs. The server enforces these at runtime.
- **`settings`** — typed install-time config. Users fill these in via API/UI, not env vars.
- **`allowedHosts`** — tools can only call these hosts. Anything else is blocked.
- **`defaultToolClass`** — app-level default, but individual tools can override (see tool.json below).

### Namespaced tool IDs

All tools and prompts are namespaced by app ID to prevent collisions:

```
rubix-developer.restart_device      # JS tool
rubix-developer.debug_network       # prompt
rubix-developer.listDevices         # from OpenAPI spec
data-analysis.export_csv            # different app, no collision
```

Display names can be friendly ("Restart Device"), but the stable ID is always `{app}.{tool}`. The MCP server registers tools with the namespaced ID.

### Session-scoped tool resolution

When two users install the same app with different secrets, tool calls must resolve to the correct user's settings. The rule:

**Every MCP session is authenticated. Tool calls resolve settings from the session's user.**

```
User A installs rubix-developer with token=AAA, host=server-1.nubedge.com
User B installs rubix-developer with token=BBB, host=server-2.nubedge.com

User A calls rubix-developer.restart_device
  → server looks up User A's install → uses token=AAA, host=server-1

User B calls rubix-developer.restart_device
  → server looks up User B's install → uses token=BBB, host=server-2
```

For **stdio transport** (single user, local): the session user is set at startup via config/token.

For **HTTP transport** (multi-user, central server): each request carries the user's bearer token, the server resolves the user, then resolves that user's app install settings for the tool being called.

Tools that the user hasn't installed return a clear error: `"app rubix-developer is not installed — install it at GET /apps/rubix-developer"`.

### Script tools (JS first, Python later)

**MVP: JavaScript only.** JS tools run in-process via Goja with a tight sandbox — no filesystem, no process spawning, HTTP restricted to `allowedHosts`. This is safe for multi-tenant.

**Python is deferred.** Python subprocesses have full filesystem and process access, which is a security risk on a shared central server (a malicious tool could read other users' data). Python support will be added post-MVP with proper isolation (container/nsjail per call).

Each tool has a script file + a `tool.json` manifest:

```
tools/
  restart_device.js         # JS tool (runs in Goja VM, in-process)
  restart_device.json
```

**tool.json** (same format for both languages):

```json
{
  "name": "restart_device",
  "description": "Restart a device by ID",
  "toolClass": "destructive",
  "params": {
    "id": { "type": "string", "required": true, "description": "Device ID" }
  }
}
```

`toolClass` is optional per tool — defaults to the app's `defaultToolClass`. For OpenAPI-generated tools, the class is inferred from the HTTP method: GET/HEAD → `read-only`, POST/PUT/PATCH → `read-write`, DELETE → `destructive`.
```

#### JavaScript tools (`.js`)

Run in-process via Goja (Go's embedded JS runtime). Fast, no external dependencies.

```javascript
// restart_device.js
function handle(params) {
  var resp = http.post(config.rubix_host + "/api/devices/" + params.id + "/restart", {
    headers: { "Authorization": "Bearer " + secrets.rubix_token }
  });
  if (resp.status !== 200) {
    return { error: "restart failed: " + resp.body };
  }
  return { message: "device " + params.id + " restarted" };
}
```

**JS host API:**

| Function | Description |
|----------|-------------|
| `http.get(url, opts)` | HTTP GET — URL must match `allowedHosts` |
| `http.post(url, body, opts)` | HTTP POST |
| `http.put(url, body, opts)` | HTTP PUT |
| `http.patch(url, body, opts)` | HTTP PATCH |
| `http.delete(url, opts)` | HTTP DELETE |
| `secrets.KEY` | Read a secret (from user's install settings) |
| `config.KEY` | Read a non-secret setting |
| `log.info(msg)` | Log to server logs |
| `log.error(msg)` | Log error |
| `files.read(path)` | Read a file within the app directory (relative path only, no escape) |

#### Python tools (`.py`) — DEFERRED

Python tools will run as subprocesses (`python3 script.py`, JSON stdin/stdout). Deferred to post-MVP because subprocesses have full filesystem and process access — unsafe for a multi-tenant central server without container-level isolation. When added, Python tools will require sandboxed execution (nsjail, gVisor, or container-per-call).

#### Sandbox constraints (JS — MVP)

- Timeout per call (default 5s, configurable in app.yaml)
- HTTP restricted to `allowedHosts` (enforced by the Goja host API)
- Secrets accessed via host API (`secrets.*`), not raw env vars
- No filesystem access, no process spawning, no shared state between calls

#### Host allowlist enforcement

The `allowedHosts` check is the main security boundary for outbound HTTP. Rules:

- **Install-time validation:** when a user provides a `baseUrl` or host setting, it is validated against the app's `allowedHosts` before saving. Rejected if it doesn't match.
- **Request-time validation:** every outbound HTTP request (from JS host API or Python proxy) is checked against `allowedHosts` after URL parsing and hostname normalisation.
- **Redirects re-checked:** if a response redirects to a different host, the new host is checked against `allowedHosts`. Blocked if it doesn't match.
- **IP literals blocked by default:** raw IP addresses (127.0.0.1, 10.x, 192.168.x, ::1, etc.) are blocked unless explicitly listed in `allowedHosts`. Prevents SSRF to internal services.
- **Hostname normalisation:** hosts are lowercased, ports are explicit, IDN is converted to ASCII before matching. `*.nubedge.com` matches `foo.nubedge.com` but not `nubedge.com` itself.
- **Empty `allowedHosts`:** app cannot make any outbound HTTP calls (prompt-only or local-data apps).

### Prompts (markdown)

```markdown
---
name: debug_network
description: Debug network issues on a Rubix controller
arguments:
  - name: device_id
    description: The device to debug
    required: true
  - name: symptom
    description: What's happening
    required: false
---

You are debugging a network issue on device {{device_id}}.
...
```

Registered as `rubix-developer.debug_network` in MCP.

### OpenAPI spec in apps

If an app contains `openapi.yaml`, tools are auto-generated. The user's `rubix_host` setting overrides the spec's server URL. All generated tool names are prefixed with the app ID.

### App install flow

```
1. Admin or user browses available apps (from apps/ directory)
2. Clicks "Install" → sees the settings schema from app.yaml
3. Fills in required settings (host URL, API token, etc.)
4. Settings stored in app_installs.json, secrets encrypted at rest
5. App is enabled — tools and prompts appear in MCP
6. User can disable/uninstall at any time
```

No shell access needed. No env vars to set. Settings are per-user — two users can install the same app with different API tokens pointing at different servers.

### App reload (hot-reload)

For a central server, restarting to pick up new apps is not acceptable — it drops all connected sessions. Instead:

- **`POST /admin/reload-apps`** — admin endpoint that re-scans `apps/`, diffs against loaded apps, registers new tools/prompts, unregisters removed ones. No server restart needed.
- **Filesystem watcher (optional)** — watch `apps/` for changes and auto-reload. Nice-to-have, not required for MVP if the admin endpoint exists.

When a new app is added to the server, all users can see it in the catalog immediately after reload. When an app is updated, installed users get the new version on their next MCP session or tool call.

### App versioning (simple)

Apps declare a `version` in `app.yaml`. When a user installs an app, the installed version is recorded. If the app version on disk changes (new tools, changed params), the install is flagged as **stale** — the user is notified and can re-install or update settings. No automatic migration, no side-by-side versions for MVP.

### App examples

| App | Contents | What it adds |
|-----|----------|-------------|
| `rubix-developer` | openapi.yaml + prompts + JS tools | Rubix platform — API tools, debug prompts, deployment |
| `nube-marketing` | prompts only | Marketing plan templates, content review |
| `ui-ux-pro-max` | prompts + JS tool | Design system — 161 palettes, 57 fonts, searchable |
| `data-analysis` | JS tools + prompts | Data export, aggregation, analysis (Python support post-MVP) |
| `anthropic-api` | openapi.yaml | Claude API as MCP tools |
| `openai-api` | openapi.yaml | OpenAI API as MCP tools |

### Example app: ui-ux-pro-max

A real-world example — a design intelligence system ported from a [Claude Code skill](https://github.com/nextlevelbuilder/ui-ux-pro-max-skill). Shows how an existing skill/prompt repo becomes an installable app.

```
apps/
  ui-ux-pro-max/
    app.yaml
    prompts/
      ui-ux-guidelines.md        # the main design system (styles, palettes, fonts, UX rules)
      pre-delivery-checklist.md   # QA checklist for UI delivery
    tools/
      search.js                   # search palettes, fonts, styles by query
      search.json
```

**app.yaml:**

```yaml
name: ui-ux-pro-max
version: 1.0.0
description: Design intelligence — 50+ styles, 161 palettes, 57 font pairings, 99 UX guidelines
author: nextlevelbuilder

permissions:
  allowedHosts: []              # no outbound HTTP — all local knowledge
  toolClass: read-only

settings: []                    # no secrets or config needed

tags:
  - design
  - ui
  - ux
```

**tools/search.json:**

```json
{
  "name": "search",
  "description": "Search the design system for palettes, fonts, styles, or UX guidelines",
  "params": {
    "query": { "type": "string", "required": true, "description": "Search term (e.g. 'dark mode', 'healthcare', 'serif')" },
    "domain": { "type": "string", "required": false, "description": "Filter by domain: palette, font, style, ux (default: all)" }
  }
}
```

**tools/search.js:**

```javascript
// search.js — search the design system for palettes, fonts, styles
function handle(params) {
  var query = (params.query || "").toLowerCase();
  var domain = params.domain || "all";

  // The server provides a host API to read files within the app directory
  var content = files.read("prompts/ui-ux-guidelines.md");

  // Split into sections and search
  var sections = content.split("\n## ");
  var matches = [];
  for (var i = 0; i < sections.length; i++) {
    if (sections[i].toLowerCase().indexOf(query) !== -1) {
      var title = sections[i].split("\n")[0].replace(/^#+\s*/, "");
      var body = sections[i].substring(0, 500);
      matches.push({ title: title, content: body });
    }
  }

  return { query: query, domain: domain, matches: matches, count: matches.length };
}
```

**What the user experiences:**

1. Browses app catalog → sees "UI/UX Pro Max"
2. Clicks install → no settings needed (no API keys)
3. Gets prompt `ui-ux-pro-max.ui-ux-guidelines` and tool `ui-ux-pro-max.search`
4. Asks "design a dashboard for energy monitoring" → the prompt provides the full design system context
5. Asks "find me a color palette for healthcare" → the search tool queries the design data and returns matches

---

## Agent

### What is the agent (Path 2)

**Path 1 (Claude Code) doesn't need this.** In Path 1, Claude Code IS the agent — it connects to `/mcp`, gets the tools, and does the thinking. No server-side LLM.

The agent is **Path 2** — a headless service for users who don't have Claude Code. It takes a task, uses an LLM API to decide which tools to call, executes them, and returns a result. No human in the loop.

```
Path 1 (already works):
  Claude Code → /mcp → user's tools → Claude does the thinking

Path 2 (agent service):
  CLI / API / webhook / cron
    → Agent loads user's installed apps + memory
    → Server-side LLM decides which tools to call
    → Tools execute (with permission checks)
    → LLM summarises result
    → Response returned + memory updated
```

**When you need the agent:**
- `nube run "check offline devices"` from the CLI
- Automated scheduled reports (cron)
- Webhook-triggered tasks (external events)
- Any use case where no one is sitting in Claude Code

**Requires an API key on the server:**
- `ANTHROPIC_API_KEY` for Claude
- `OPENAI_API_KEY` for GPT
- Or local Ollama (free)

### Why it's separate

- The MCP server is a **tool provider** — exposes tools, waits for Claude Code or other MCP clients
- The agent is a **tool consumer** — has its own LLM brain, calls tools itself
- They share apps and tools but serve different use cases
- Most users will use Path 1 (Claude Code) — the agent is the fallback

### Agent config

```yaml
# agent.yaml
llm:
  provider: anthropic       # anthropic | openai | ollama
  model: claude-sonnet-4-20250514
  apiKey: ${ANTHROPIC_API_KEY}

server:
  addr: :8090

# Safety defaults (can be overridden per-task)
defaults:
  maxSteps: 20             # max tool calls per task
  maxCost: 0.50            # max USD per task
  dryRun: false            # log actions without executing

memory:
  enabled: true
  path: ./data/memory
  maxSizeMB: 10
  autoExtract: true
  retrievalLimit: 20
```

### Safety controls

| Control | What it does |
|---------|-------------|
| `maxSteps` | Hard limit on tool calls per task |
| `maxCost` | Token cost budget per task |
| `dryRun` | Log what would happen without executing — tool calls are logged but not sent |
| `toolClass` filter | Per-task: `allowToolClass: read-only` restricts the agent to read-only tools only |
| Confirmation gates | Tools with `toolClass: destructive` are skipped unless the task explicitly opts in with `allowDestructive: true` |
| `allowedApps` | Per-task: restrict the agent to specific apps only |
| `allowedTools` | Per-task: restrict to specific tool IDs (e.g. `["rubix-developer.listDevices"]`) |
| Execution log | Every tool call with input, output, duration, and cost logged to `agent_tasks.json` |
| Idempotency | Tasks have unique IDs — re-submitting the same ID returns the cached result |
| Retry policy | Failed tool calls are retried once. If both fail, the agent reports the error to the LLM rather than retrying indefinitely |

### Agent use cases

| Use case | How it works |
|----------|-------------|
| Scheduled reports | Cron → agent calls data tools → LLM writes summary → posts result |
| Webhook handler | External event → agent diagnoses → calls device tools → returns fix |
| CLI task | `agent run "check offline devices and restart them"` |
| API | `POST /agent/tasks {prompt, allowedApps, maxSteps, allowDestructive}` |

### Cost controls

- `maxSteps` — hard limit on tool calls per task
- `maxCost` — estimated token cost limit per task
- `provider: ollama` — zero-cost local inference option
- All API calls logged with token counts per user

---

## Memory

### Scoping

Memory is **per-user** and **per-workspace**:

| Scope | What it stores | Who can read |
|-------|---------------|-------------|
| **User memory** | Personal knowledge + patterns from that user's tasks | Only that user's agent sessions |
| **Workspace memory** | Shared knowledge across the org (e.g. "our MQTT broker is at 10.0.0.5") | All users in the workspace |

### Three types

| Type | What | Lifetime | Example |
|------|------|----------|---------|
| **Conversation** | Current task context | Single task | "Debugging dev-003, ping works" |
| **Knowledge** | Facts from tool results | Persistent | "dev-003 gateway is 192.168.1.1" |
| **Patterns** | What worked / what didn't | Persistent | "MQTT restart fixes false-offline on RC-5" |

Conversation memory is just the LLM context window — nothing to build.

### Storage

JSON files, scoped by user and workspace:

```
data/
  memory/
    knowledge/
      user-{userId}.json        # personal facts
    patterns/
      user-{userId}.json        # personal patterns
    shared/
      workspace-{workspaceId}.json  # org-wide shared knowledge
```

Each file is a JSON array of facts:

```json
[
  {
    "id": "fact-001",
    "fact": "dev-003 gateway is 192.168.1.1",
    "source": "getDevice tool call",
    "taskId": "task-42",
    "userId": "user-1",
    "time": "2026-04-06T10:00:00Z",
    "confidence": 1.0,
    "scope": "user"
  },
  {
    "id": "fact-002",
    "fact": "MQTT broker restart fixes false-offline on RC-5",
    "source": "user feedback",
    "taskId": "task-45",
    "userId": "user-1",
    "time": "2026-04-06T11:00:00Z",
    "confidence": 0.9,
    "scope": "workspace"
  }
]
```

### How it learns

**1. Automatic** — after a task completes, the agent extracts facts from the full execution log and stores them in user memory. Extraction happens once per task (not per tool call) to avoid burning extra LLM calls on every step.

**2. From feedback** — user confirms or rejects a result:

```
POST /agent/tasks/{taskId}/feedback
{ "outcome": "success", "note": "MQTT restart fixed it" }
```

Stores/updates a pattern. Confidence increases on success, decreases on failure.

**3. Promote to shared** — any user can propose a personal fact for workspace-level shared memory. Admins approve or reject:

```
POST /memory/{factId}/promote          # user proposes
  → fact gets status: "pending_review"

PATCH /memory/{factId}/approve         # admin approves (admin-only)
  → fact moves to workspace shared memory

PATCH /memory/{factId}/reject          # admin rejects (admin-only)
  → fact stays in user's personal memory, status cleared
```

Promoted facts include metadata: who proposed it, who approved it, when.

**4. Retrieval** — before the first LLM call in a task, the agent searches user memory + workspace memory for relevant facts and injects them into context.

**Retrieval strategy (MVP):** keyword/tag-based search against the task prompt. Simple but fast and predictable. The memory schema supports a `tags` field on each fact for structured lookup. Semantic/embedding-based retrieval can be added later behind the same interface without changing storage.

### Constraints

- No fine-tuning — memory is context injection, not model training
- No self-modifying code
- Bounded growth — max size per user (default 10MB), old low-confidence facts pruned
- User memory is private — other users can't read it
- Workspace memory is read by all users, written only by admin approval
- All memory is inspectable and deletable by the owning user (personal) or admin (shared)

---

## Implementation plan

### Phase 1 — Users, workspaces, JSON DB

| Task | Description | Status |
|------|-------------|--------|
| JSON DB | Read/write JSON files with in-memory cache, file locking | ✅ `pkg/jsondb/db.go` |
| Workspace CRUD | Create, list, get, delete workspaces | ✅ `pkg/api/workspaces.go` |
| User CRUD | Create, list, get, delete users within a workspace | ✅ `pkg/api/workspaces.go`, `users.go` |
| Auth | Simple token-based auth (each user gets an API token) | ✅ `pkg/auth/auth.go` |
| Token rotation | `POST /users/{id}/token` (rotate), `DELETE /users/{id}/token` (revoke) | ✅ `pkg/api/users.go` |
| API | REST endpoints for user/workspace management | ✅ `pkg/api/router.go` |

### Phase 2 — App loader + install flow ✅

| Task | Description | Status |
|------|-------------|--------|
| App scanner | Scan `apps/` directory, parse `app.yaml`, validate permissions | ✅ `pkg/apps/registry.go` |
| App catalog API | `GET /apps` — list available apps with settings schema | ✅ `pkg/api/apps.go` |
| App install API | `POST /apps/{id}/install` — user provides settings, stored in DB | ✅ `pkg/api/apps.go` |
| App enable/disable | `PATCH /app-installs/{id}` — toggle without losing settings | ✅ `pkg/api/apps.go` |
| Prompt loader | Read `prompts/*.md`, register as namespaced MCP prompts | ✅ `pkg/apps/prompts.go` |
| OpenAPI loader | Read `openapi.yaml`, register namespaced tools with user's settings | ✅ `pkg/apps/mcpfactory.go` |
| Permission enforcement | Validate `allowedHosts` on every outbound HTTP call | Deferred |
| Secret storage | Encrypt secrets at rest in `app_installs.json` | Deferred (plaintext for now) |
| App reload endpoint | `POST /admin/reload-apps` — re-scan apps dir, register/unregister without restart | ✅ `pkg/api/apps.go` |
| Version tracking | Record installed app version, flag stale installs when app updates | ✅ `pkg/models/appinstall.go` |

### Phase 3 — Script tool runtime (JS only — MVP) ✅

| Task | Description | Status |
|------|-------------|--------|
| Tool loader | Read `tools/*.js` + `tool.json` manifests, paired by filename | ✅ `pkg/apps/toolmanifest.go` |
| Goja VM (JS) | One VM per call, inject host API (`http`, `secrets`, `config`, `log`, `files`) | ✅ `pkg/apps/jsruntime.go` |
| HTTP sandbox | Enforce `allowedHosts` — block calls to non-approved hosts, re-check redirects, block IP literals | ✅ `pkg/apps/hostcheck.go` |
| Secret injection | `secrets.*` in JS — reads from user's install settings | ✅ `pkg/apps/jsruntime.go` |
| Timeout | Configurable per app via `app.yaml`, default 5s, VM interrupted on timeout | ✅ `pkg/apps/jsruntime.go` |
| Tool registration | Namespaced: `{app}.{tool}`, schema built from tool.json params | ✅ `pkg/apps/mcpfactory.go` |

### Phase 4 — Agent service

| Task | Description |
|------|-------------|
| Agent loop | Task → load user's apps + memory → LLM → tool calls → result |
| LLM adapters | Anthropic, OpenAI, Ollama |
| Task API | `POST /agent/tasks` with userId, prompt, constraints |
| Safety | maxSteps, maxCost, dryRun, confirmation gates for destructive tools |
| Execution log | Full log of every tool call per task in `agent_tasks.json` |
| CLI | `agent run --user=joe "do something"` |

### Phase 5 — Memory

| Task | Description |
|------|-------------|
| Knowledge store | Per-user + per-workspace JSON files |
| Auto-extract | Extract facts from full task execution log after task completes (once per task, not per tool call) |
| Pattern store | Record outcomes from user feedback |
| Retrieval | Search memory, inject into LLM context |
| Promote | Move personal fact to workspace shared memory |
| Pruning | Max size, evict old low-confidence facts |
| Feedback API | `POST /agent/tasks/{id}/feedback` |

---

## Directory structure

```
developer-tools/
├── cmd/
│   └── nube-server/                # ✅ multi-tenant server entry point
│       └── main.go                 #    loads DB + apps, serves REST + MCP on :8090
│
├── pkg/                            # ✅ top-level packages (Stage 1 & 2)
│   ├── jsondb/                     # ✅ generic file-backed JSON DB
│   │   └── db.go                   #    Collection[T], in-memory cache, mutex, atomic writes
│   ├── models/                     # ✅ data models
│   │   ├── models.go               #    Workspace, User, Role, token/ID generation
│   │   └── appinstall.go           #    AppInstall with settings, secrets, version tracking
│   ├── auth/                       # ✅ authentication
│   │   └── auth.go                 #    Bearer token middleware, admin impersonation, RequireAdmin
│   ├── apps/                       # ✅ app system
│   │   ├── types.go                #    App, Permissions, SettingDef, Prompt types + app.yaml parser
│   │   ├── registry.go             #    Scan apps/ dir, hold loaded apps, Reload()
│   │   ├── prompts.go              #    Parse markdown prompts with YAML frontmatter
│   │   ├── toolmanifest.go         #    ✅ Parse tool.json manifests for JS tools
│   │   ├── jsruntime.go            #    ✅ Goja JS runtime with sandboxed host API
│   │   ├── hostcheck.go            #    ✅ allowedHosts enforcement for outbound HTTP
│   │   ├── hostcheck_test.go       #    ✅ Tests for host allowlist
│   │   └── mcpfactory.go           #    Per-user MCP server factory (OpenAPI + JS tools + prompts)
│   └── api/                        # ✅ REST API handlers
│       ├── router.go               #    Gin router with all routes
│       ├── bootstrap.go            #    First-run bootstrap (workspace + admin)
│       ├── workspaces.go           #    Workspace CRUD + user creation
│       ├── users.go                #    User self-service, token rotate/revoke
│       ├── apps.go                 #    App catalog, install/uninstall, reload
│       └── mcp.go                  #    MCP HTTP handler (per-session user resolution)
│
├── mcp/                            # core OpenAPI-to-MCP library (separate Go module)
│   ├── *.go                        # openapi2mcp package
│   ├── pkg/config/                 # config
│   ├── pkg/middleware/             # hooks
│   ├── pkg/server/                 # server wrapper
│   ├── cmd/openmcp/                # Cobra CLI
│   └── cmd/openapi-mcp/            # original CLI
│
├── apps/                           # ✅ available apps (bundles)
│   ├── rubix-developer/            # ✅ OpenAPI spec + JS tools + prompts (10 tools, 1 prompt)
│   │   ├── app.yaml
│   │   ├── tools/
│   │   │   ├── device_summary.js   # ✅ JS tool: aggregate device status
│   │   │   ├── device_summary.json
│   │   │   ├── restart_device.js   # ✅ JS tool: restart a device
│   │   │   └── restart_device.json
│   │   ├── prompts/
│   │   │   └── debug-network.md
│   │   └── openapi.yaml
│   ├── nube-marketing/             # ✅ prompts only (0 tools, 2 prompts)
│   │   ├── app.yaml
│   │   └── prompts/
│   │       ├── marketing-plan.md
│   │       └── content-review.md
│   ├── ui-ux-pro-max/              # (planned — not yet created)
│   └── ...
│
├── agent/                          # (planned — Stage 5)
│   ├── main.go
│   ├── loop.go                     # LLM tool-calling loop
│   ├── adapters/                   # LLM provider adapters
│   │   └── anthropic.go
│   ├── memory/                     # memory system
│   │   ├── store.go
│   │   ├── retrieval.go
│   │   ├── extract.go
│   │   └── prune.go
│   └── agent.yaml
│
├── data/                           # ✅ runtime data (JSON DB)
│   ├── workspaces.json             # ✅
│   ├── users.json                  # ✅
│   ├── app_installs.json           # ✅
│   ├── agent_tasks.json            # (planned — Stage 5)
│   └── memory/                     # (planned — Stage 6)
│
├── mcpdemo/                        # demo MCP server
└── fakeserver/                     # fake API for testing
```

---

## API summary

All endpoints require `Authorization: Bearer <token>` unless noted. The token determines the user and workspace. Admin impersonation uses `X-Act-As-User: <user-id>` header (admin-only, audit logged).

### Bootstrap & health (no auth)

```
GET    /health                              # ✅ server status, user/app counts
POST   /bootstrap                           # ✅ create first workspace + admin (only works when no users exist)
```

### User / workspace management

```
POST   /workspaces                          # ✅ create workspace (admin)
GET    /workspaces                          # ✅ list workspaces (admin: all, user: own)
GET    /workspaces/{id}                     # ✅ get workspace
DELETE /workspaces/{id}                     # ✅ delete workspace (admin, must be empty)
POST   /workspaces/{id}/users               # ✅ create user (admin) → returns user token
GET    /workspaces/{id}/users               # ✅ list users in workspace

GET    /users/me                            # ✅ get current user (from token)
GET    /users/{id}                          # ✅ get user (admin)
DELETE /users/{id}                          # ✅ delete user (admin)
POST   /users/{id}/token                    # ✅ rotate token (self or admin)
DELETE /users/{id}/token                    # ✅ revoke token (self or admin)
```

### App management

```
GET    /apps                                # ✅ list available apps (from apps/ dir)
GET    /apps/{id}                           # ✅ app details + settings schema + prompts
POST   /apps/{id}/install                   # ✅ install app (provide settings, validates required fields)
GET    /app-installs                        # ✅ list current user's installed apps (flags stale versions)
PATCH  /app-installs/{id}                   # ✅ enable/disable, update settings
DELETE /app-installs/{id}                   # ✅ uninstall
POST   /admin/reload-apps                   # ✅ re-scan apps dir (admin only, no restart)
```

### MCP endpoint

```
ANY    /mcp                                 # ✅ per-user MCP server (tools + prompts scoped to installed apps)
```

### Agent

```
POST   /agent/tasks                         # submit task {prompt, allowedApps, maxSteps, allowDestructive, dryRun}
GET    /agent/tasks                         # list current user's tasks
GET    /agent/tasks/{id}                    # task detail + full execution log
POST   /agent/tasks/{id}/feedback           # feedback {outcome: "success"|"failure", note: "..."}
```

### Memory

```
GET    /memory                              # list current user's personal facts
GET    /memory/shared                       # list workspace shared facts
DELETE /memory/{id}                         # delete own fact

POST   /memory/{id}/promote                 # propose fact for shared (user) → status: pending_review
PATCH  /memory/{id}/approve                 # approve promotion (admin-only)
PATCH  /memory/{id}/reject                  # reject promotion (admin-only)
```
