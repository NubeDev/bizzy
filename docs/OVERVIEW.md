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
│   Service Layer (pkg/services/ — no HTTP dependency)         │
│     ├── AgentService  (prompt enrichment, sessions, providers)│
│     └── ToolService   (tool resolution, execution, listing)  │
│                                                              │
│   AI Runner Registry                                         │
│     ├── Claude  (CLI, MCP tools, session resume)             │
│     ├── Ollama  (API, local models, system prompt)           │
│     ├── Codex   (CLI)                                        │
│     └── Copilot (CLI)                                        │
│                                                              │
│   Job Store ─── Session Store ─── Workflow Engine            │
└──────────────────────────────────────────────────────────────┘
        │
        v
┌──────────────────────────────────────────────────────────────┐
│                         Data                                 │
│                                                              │
│   data/bizzy.db        SQLite database (users, sessions,     │
│                        installs, store apps, workflows)      │
│   data/apps/           App directories (YAML, JS, prompts)   │
│   data/memory/         Server + per-user memory (markdown)   │
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

| Provider | Type | Tool calling | System prompt | Session resume |
|---|---|---|---|---|
| Claude | CLI | Yes (native MCP) | Via prompt prefix | Yes |
| Ollama | API (local) | No | Yes (system message) | No |
| Codex | CLI | No | Via prompt prefix | No |
| Copilot | CLI | No | Via prompt prefix | No |

Users can set a default provider or override per-request. Admins configure provider availability and API keys.

API-based providers (Ollama, future OpenAI/Anthropic/Gemini) receive memory and app context as a `system` message, separate from the user's prompt. This improves response quality because models treat system instructions differently from user input.

See: [MULTI-PROVIDER.md](MULTI-PROVIDER.md)

### Memory

Persistent context that carries across conversations. Two scopes:

- **Server memory** — shared by all users, set by admin ("This is the Sydney Office deployment, we use Celsius")
- **User memory** — private to each user ("I prefer detailed responses, my team manages floors 5-8")

Memory is plain markdown, injected into every AI prompt server-side. For API-based providers (Ollama), it goes in the `system` message. For CLI providers (Claude), it's prepended to the user prompt. The user doesn't manage it per-conversation — it's always there.

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

All three go through the same `Runner.Run()` backend via `AgentService`. Prompts are enriched (memory + app context), system prompts are passed separately for API providers, sessions are persisted with detailed tool call logs, and events are normalised regardless of which entry point is used.

---

## Data storage

SQLite database (`data/bizzy.db`) for all structured data. App files and memory remain on disk.

| What | Format | Location |
|---|---|---|
| Users, workspaces | SQLite | `data/bizzy.db` |
| Sessions, app installs | SQLite | `data/bizzy.db` |
| Store apps, reviews, shares | SQLite | `data/bizzy.db` |
| Provider config | SQLite | `data/bizzy.db` |
| Workflow runs | SQLite | `data/bizzy.db` |
| App files | YAML, JS, MD | `data/apps/<name>/` |
| Memory | Markdown | `data/memory/server.md`, `data/memory/users/<id>.md` |

See: [DB.md](DB.md) (schema, indexes, migration)

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
    api/                 Thin HTTP handlers, router, MCP endpoint
    services/            Reusable application logic (AgentService, ToolService)
    airunner/            Provider interface, runners, job store
    apps/                App registry, MCP factory, tool/prompt loading
    auth/                Bearer token middleware
    claude/              Claude CLI spawner, stream-json parser
    cli/                 CLI commands (ask, memory, tools, workflows, etc.)
    database/            SQLite database init, migration
    memory/              Server + per-user memory store
    models/              Core data types (User, Session, StoreApp, etc.)
    workflow/            Workflow engine (definition parsing, stage execution)
  data/
    apps/                App directories (system + user-created)
    memory/              Memory files
    bizzy.db             SQLite database
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
| [BACKEND.md](BACKEND.md) | Server setup, API reference, auth, app loading |
| [DB.md](DB.md) | SQLite database, schema, indexes, JSON migration |
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
