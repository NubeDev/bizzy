# Bizzy — Overview

Bizzy is an AI agent platform. It connects AI providers (Claude, Ollama, OpenAI, etc.) to installable apps that give the AI tools, prompts, and workflows. Users interact through a CLI, a Flutter app, or direct API calls.

The core idea: **apps teach the AI what it can do**. An admin installs apps (a Rubix BMS controller, a PDF exporter, a marketing template library), users install the ones they need, and the AI gets access to those tools via MCP. The user says "check which devices are offline" and the AI calls the right tools without the user knowing which app provides them.

---

## How it fits together

```
┌──────────────────────────────────────────────────────────────┐
│                        Clients                               │
│                                                              │
│   nube CLI          Flutter App          API / Scripts        │
│   (nube ask)        (desktop/mobile)     (REST / WS)         │
│       │                  │                    │               │
└───────┼──────────────────┼────────────────────┼──────────────┘
        │                  │                    │
        v                  v                    v
┌──────────────────────────────────────────────────────────────┐
│                     nube-server                              │
│                                                              │
│   Auth ─── App Registry ─── MCP Factory ─── Memory Store     │
│     │           │                │               │           │
│     │     ┌─────┴─────┐    ┌────┴────┐     ┌────┴────┐      │
│     │     │   Apps     │    │  Tools  │     │ Server  │      │
│     │     │ app.yaml   │    │  JS/API │     │  User   │      │
│     │     │ tools/     │    │ Prompts │     │ memory  │      │
│     │     │ prompts/   │    │         │     │         │      │
│     │     │ workflows/ │    │         │     │         │      │
│     │     └───────────┘    └─────────┘     └─────────┘      │
│     │                                                        │
│   AI Runner Registry                                         │
│     ├── Claude  (CLI, MCP tools, session resume)             │
│     ├── Ollama  (API, local models)                          │
│     ├── Codex   (CLI)                                        │
│     └── Copilot (CLI)                                        │
│                                                              │
│   Job Store ─── Session Store ─── Workflow Engine            │
└──────────────────────────────────────────────────────────────┘
        │
        v
┌──────────────────────────────────────────────────────────────┐
│                    Data (all file-based)                      │
│                                                              │
│   data/apps/           App directories (YAML, JS, prompts)   │
│   data/*.json          Users, sessions, installs, store      │
│   data/memory/         Server + per-user memory (markdown)   │
│   data/workflow_runs/  Workflow execution history            │
└──────────────────────────────────────────────────────────────┘
```

---

## Key concepts

### Apps

An app is a directory with an `app.yaml`, optional tools (JavaScript), prompts (markdown), and workflows (YAML). Apps are the unit of functionality — they teach the AI how to do things.

```
rubix/
  app.yaml              # name, description, permissions, settings
  preamble              # context injected into AI conversations
  tools/
    query_nodes.js      # executable tool logic
    query_nodes.json    # tool schema (name, params, description)
  prompts/
    getting_started.md  # reusable prompt template
  workflows/
    weekly_report.yaml  # multi-step staged workflow
```

Apps are served to AI clients via MCP. Each user only sees tools from their installed apps.

See: [BACKEND.md](BACKEND.md) (app loading, registry), [MCP.md](MCP.md) (tool serving)

### AI providers

Multiple AI backends through one interface. All providers implement `Runner.Run()` and emit the same normalised events (`connected`, `text`, `tool_call`, `done`, `error`).

| Provider | Type | Tool calling | Session resume |
|---|---|---|---|
| Claude | CLI | Yes (native MCP) | Yes |
| Ollama | API (local) | No | No |
| Codex | CLI | No | No |
| Copilot | CLI | No | No |

Users can set a default provider or override per-request. Admins configure provider availability and API keys.

See: [MULTI-PROVIDER.md](MULTI-PROVIDER.md)

### Memory

Persistent context that carries across conversations. Two scopes:

- **Server memory** — shared by all users, set by admin ("This is the Sydney Office deployment, we use Celsius")
- **User memory** — private to each user ("I prefer detailed responses, my team manages floors 5-8")

Memory is plain markdown, prepended to every AI prompt server-side. The user doesn't manage it per-conversation — it's always there.

See: [MEMORY.md](MEMORY.md)

### Workflows

Multi-step pipelines that chain tools from multiple apps. Each stage runs in order, passes its result to the next, and stops on failure. Stages can be tool calls, AI prompts, or approval gates (pause and wait for user review).

```
[research] ──ok──> [draft] ──ok──> [review] ──approved──> [render] ──ok──> [done]
     │                                │                       │
     └── fail ──> STOP                └── rejected ──> STOP   └── fail ──> STOP
```

See: [MULTI-APP-WORKFLOW.md](MULTI-APP-WORKFLOW.md)

### MCP (Model Context Protocol)

The server exposes a per-user MCP endpoint at `/mcp`. When Claude (or any MCP client) connects, it gets a scoped set of tools based on that user's installed apps. Tools come from two sources:

- **OpenAPI tools** — auto-generated from an app's `openapi.yaml` spec
- **JS tools** — custom logic in `tools/*.js` files, executed in a Goja runtime

See: [MCP.md](MCP.md)

### App store

Built into the server. All apps — system-shipped and user-created — live in one unified system. Users can create apps, test privately, publish to the store, and browse/install apps from others.

See: [STORE.md](STORE.md)

---

## Three ways to use AI

| Method | Entry point | Use case |
|---|---|---|
| **WebSocket** | `GET /api/agents/run` | Real-time streaming (CLI `nube ask`, Flutter app, frontend) |
| **REST sync** | `POST /api/agents/run/sync` | Simple request/response (scripts, testing) |
| **Async jobs** | `POST /api/agents/jobs` | Fire-and-forget with polling (cron, CI, webhooks) |

All three go through the same `Runner.Run()` backend. Memory is injected, sessions are persisted, and events are normalised regardless of which entry point is used.

---

## Data storage

Everything is file-based. No database required.

| What | Format | Location |
|---|---|---|
| Users, workspaces | JSON | `data/users.json`, `data/workspaces.json` |
| App installs | JSON | `data/app_installs.json` |
| Sessions | JSON | `data/sessions.json` |
| Store apps | JSON | `data/store_apps.json` |
| Provider config | JSON | `data/provider_config.json` |
| App files | YAML, JS, MD | `data/apps/<name>/` |
| Memory | Markdown | `data/memory/server.md`, `data/memory/users/<id>.md` |

JSON collections use `pkg/jsondb` — file-backed with in-memory caching, thread-safe reads/writes, atomic file operations.

See: [BACKEND.md](BACKEND.md) (data model details)

---

## Clients

### CLI (`nube`)

The primary developer interface. Hand-written commands for AI, tools, prompts, memory, and workflows. Additional admin commands auto-generated from an embedded OpenAPI spec.

```bash
nube login http://localhost:8090 <token>
nube ask "check which devices are offline"
nube ask --provider ollama --model gemma3 "summarise alarms"
nube memory me add "My team manages floors 5-8"
nube workflow run sales-brochure create-brochure --product "Rubix Compute"
nube tools
nube prompts
```

See: [CLI.md](CLI.md)

### Flutter app

Desktop, mobile, and web client. Connects to nube-server for AI chat, app browsing, and tool management.

See: [APP.md](APP.md)

### Direct API

Any HTTP client can use the REST API. WebSocket for streaming, REST for sync, jobs for async.

See: [BACKEND.md](BACKEND.md) (full API reference)

---

## Project structure

```
bizzy/
  cmd/
    nube-server/         Server entry point
    nube/                CLI entry point (embeds OpenAPI spec)
  pkg/
    api/                 REST/WS handlers, router, MCP endpoint
    airunner/            Provider interface, runners, job store
    apps/                App registry, MCP factory, tool/prompt loading
    auth/                Bearer token middleware
    claude/              Claude CLI spawner, stream-json parser
    cli/                 CLI commands (ask, memory, tools, workflows, etc.)
    jsondb/              File-backed JSON collections
    memory/              Server + per-user memory store
    models/              Core data types (User, Session, StoreApp, etc.)
    workflow/            Workflow engine (definition parsing, stage execution)
  data/
    apps/                App directories (system + user-created)
    memory/              Memory files
    *.json               Runtime data (users, sessions, installs, etc.)
  app/
    nube_agent/          Flutter app
  mcp/                   OpenAPI-to-MCP conversion library
  docs/                  Documentation
  frontend/              Web frontend
```

---

## Documentation index

| Doc | What it covers |
|---|---|
| [BACKEND.md](BACKEND.md) | Server setup, data storage, API reference, auth, app loading |
| [CLI.md](CLI.md) | CLI commands, configuration, authentication |
| [FRONTEND.md](FRONTEND.md) | Web frontend |
| [APP.md](APP.md) | Flutter app architecture and setup |
| [MCP.md](MCP.md) | MCP tool serving, MCPFactory, per-user scoping |
| [MEMORY.md](MEMORY.md) | Persistent AI memory (server + user), API, CLI, injection |
| [MULTI-PROVIDER.md](MULTI-PROVIDER.md) | AI providers, Runner interface, jobs, sessions, config |
| [MULTI-APP-WORKFLOW.md](MULTI-APP-WORKFLOW.md) | Staged workflows, approval gates, failure handling |
| [STORE.md](STORE.md) | App store, publishing, sharing, reviews |
| [RUBIX.md](RUBIX.md) | Rubix BMS integration example |
| [NEW-IDEAS.md](NEW-IDEAS.md) | Planned improvements and ideas |
