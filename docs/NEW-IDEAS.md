# Ideas for Improving the Framework

A collection of concrete improvements to bizzy, prioritised by impact.

---

## 1. Preamble Composition — DONE

**Problem:** When a user runs `nube ask` without `--agent`, the AI has tools available via MCP but no context about what they do or how to combine them. The AI sees `nube-hardware.get_product` but doesn't know when to use it.

**Idea:** Auto-compose preambles from all installed apps and inject them alongside memory.

When building the prompt, the server would prepend:

```
[Server Memory]
...

[User Memory]
...

[Installed Apps]
- nube-hardware: Query device nodes, read sensor values, check device status.
  Tools: get_product — look up product info by ID
         query_nodes — list or filter BMS network nodes
         get_alarms  — fetch active alarm conditions
- pdf-export: Render documents and reports to PDF format.
  Tools: render         — convert data to PDF using a template
         list_templates — list available PDF templates
- rubix: Interact with the Rubix BMS runtime.
  Tools: query_nodes    — read point values from the BMS network
         write_value    — write a value to a BMS point
         discover_device — scan for new devices on a network

[System Prompt]
You are a helpful AI assistant...
```

The AI now knows what it can do without the user picking an agent. Each app already has a `description` field and a `preamble` file — this just composes them.

**What changes:**
- Add a `BuildAppContext(installs []models.AppInstall) string` method to `MCPFactory` or `Registry` (note: user lookup happens in the caller — the factory works with installs, not user IDs)
- For each install, pull the app description + per-tool descriptions from the JSON manifests and OpenAPI specs (these already exist)
- Inject after memory prefix in `agents_ws.go`, `agents_rest.go`, `agents_jobs.go`
- Include a one-line description per tool, not just tool names — the AI needs to know *what* each tool does, not just that it exists
- Cap output: if a user has many apps, limit to the top ~20 tools or use tiered detail (full descriptions for recently-used apps, name-only for the rest)

---

## 2. Tool Result Caching per Session

**Problem:** Follow-up questions re-call the same tools with the same params.

```
User: "which devices are offline?"
AI: calls rubix.query_nodes → 847 nodes → answers

User: "what about floor 3?"
AI: calls rubix.query_nodes AGAIN → same 847 nodes → filters to floor 3
```

**Idea:** Cache tool results within a session. If the same tool+params were called in the last N minutes, serve the cached result.

**What changes:**
- Add `ToolCache map[string]CachedResult` to the session or job context
- Cache key = `toolName + sha256(params)`
- TTL configurable per app (default 5 minutes, real-time data apps might set 30 seconds)
- `app.yaml` addition:
  ```yaml
  cache:
    default_ttl: 300   # seconds
    tools:
      query_nodes: 60  # override per tool
  ```
- Cache lives in memory only (not persisted) — it's session-scoped
- **Cache bypass:** if the user prompt contains "refresh", "re-check", "latest", or "update" — skip cache for that turn. Without this, users get frustrated when they know data changed but the AI keeps serving stale results
- **Track payload sizes** (`output_bytes` per cached result) — large results (847 nodes) benefit most from caching; small results might be cheaper to just re-fetch

---

## 3. App Settings / Secrets Separation (partially done)

**Problem:** Secrets (API keys, tokens) should not be returned in API responses, logged, or serialized to JSON in the clear.

**Current state:** The data model split is **already implemented**. `AppInstall` has both `Settings map[string]string` and `Secrets map[string]string` fields, and `GetSetting(key)` checks secrets first, then settings. JS tools already access secrets via the `secrets.*` host API.

**What's still missing:**

```yaml
# app.yaml — declare which fields are secrets vs config
settings:
  - key: rubix_host
    label: Rubix Host URL
    type: url
    required: true

secrets:
  - key: api_key
    label: API Key
    type: secret
```

**Remaining work:**
- API response masking: `GET /app-installs` should never return raw secret values (`"api_key": "sk-...***"`)
- `app.yaml` declaration of which fields are settings vs secrets (currently only the API caller decides)
- Encryption at rest (AES-GCM with a server key) — there's a `TODO: encrypt at rest` comment in the model
- Exclude secrets from structured log output

---

## 4. Event Hooks for Apps

**Problem:** Apps can only provide tools (things the AI calls). They can't react to lifecycle events — no setup on install, no context injection at session start, no cleanup on uninstall.

**Idea:** Let apps declare hooks that run on specific events.

```yaml
# app.yaml
hooks:
  on_install: hooks/setup.js          # runs once when user installs the app
  on_uninstall: hooks/cleanup.js      # runs on uninstall
  on_session_start: hooks/inject.js   # inject fresh context at conversation start
  on_tool_error: hooks/fallback.js    # handle tool failures gracefully
```

**Use cases:**
- `on_install`: Seed initial user memory ("This user has rubix installed, default site is X")
- `on_session_start`: Fetch fresh device count, inject as context so the AI knows current state without a tool call
- `on_tool_error`: Retry with different params, or return a helpful error message instead of a stack trace
- `on_uninstall`: Clean up any data the app created

**What changes:**
- New `hooks/` directory convention in app structure
- `HookRunner` that executes JS hooks with the same Goja runtime as tools
- Hook invocation points in install/uninstall handlers and agent run flow
- Hooks are fire-and-forget (don't block the main flow) except `on_session_start` which injects context

**Recommendation:** Start with only `on_install` and `on_uninstall`. Defer `on_session_start` and `on_tool_error`:
- `on_session_start` adds latency and context cost to every conversation even when irrelevant — preamble composition (idea #1) solves the "AI needs context" problem more cleanly
- `on_tool_error` is premature without observability (idea #6) — build tracking first, then you'll know which errors are worth handling
- Implicit execution (code runs without user/AI requesting it) is powerful but hard to debug when hooks misbehave

---

## 5. Prompt Chaining / Slash Commands

**Problem:** Prompts are single-shot. A user can't compose multiple prompts into a sequence, and there's no shorthand for running a prompt from the CLI.

**Idea:** Let prompts reference other prompts and support slash-command invocation.

```markdown
---
name: weekly-report
description: Generate weekly operations report
chain:
  - rubix.site-summary
  - rubix.alarm-report
  - nube-hardware.offline-devices
---
Combine the results above into a weekly operations report for management.
Format: executive summary (3 bullets), then detail by category.
```

The system would:
1. Execute each chained prompt in order, collecting results
2. Inject all results into the final prompt body
3. Send the composed prompt to the AI

CLI shorthand:

```bash
nube ask "/weekly-report"                    # run the chain
nube ask "/site-summary floor3"              # single prompt with args
nube ask "/marketing-plan Rubix Compute"     # prompt with positional arg
```

The `/` prefix is already parsed in the ask flow — this extends it to support chaining.

**What changes:**
- `chain` field in prompt frontmatter (list of prompt names)
- Prompt resolver that executes chains before the final prompt
- CLI: `/name` syntax already partially supported, just needs chain execution
- Results from chained prompts injected as `[Result: prompt-name]\n...\n\n`

**Design consideration:** Clarify the execution model — if each chained prompt goes through the full AI loop (prompt -> LLM -> tool calls -> response), a 3-step chain means 3 AI calls plus the final synthesis = 4 total. That's expensive and slow. Where possible, execute chained steps as **tool-only** calls (run the tool directly, collect raw data) and let the final prompt do all the AI reasoning in one shot. For example, if `rubix.site-summary` is a prompt that just calls `rubix.query_nodes` and formats the result, run the tool directly and feed the raw result into the final prompt.

---

## 6. Observability: Tool Call Tracking — DONE

**Problem:** Sessions track `ToolCalls` as a count but not which tools were called, how long each took, or which ones failed. Can't answer "which tools are slow?" or "which tools fail most?"

**Idea:** Store a `ToolCallLog` array on each session.

```go
type ToolCallEntry struct {
    Name        string `json:"name"`         // "rubix.query_nodes"
    DurationMS  int    `json:"duration_ms"`
    Status      string `json:"status"`       // "ok", "error"
    Error       string `json:"error,omitempty"`
    InputBytes  int    `json:"input_bytes"`  // request payload size
    OutputBytes int    `json:"output_bytes"` // response payload size
}
```

**Current state:** `Session.ToolCalls` is an integer counter only (incremented when `ev.Type == "tool_call"` in the Claude runner). No tool names, durations, or error details are tracked.

**What it unlocks:**
- Dashboard: "pdf-export.render fails 30% of the time"
- Dashboard: "rubix.query_nodes averages 4.2s — the slowest tool"
- Per-session view: exactly which tools were called and in what order
- Debugging: when a session gives a bad answer, see what tool data the AI received
- Informed caching decisions: know which tools return large payloads that blow context budget

**What changes:**
- Add `ToolCallLog []ToolCallEntry` to `models.Session`
- Add `InputBytes` and `OutputBytes` fields to `ToolCallEntry` — answers "which tools return huge payloads?"
- Parse tool call events from the AI stream (already emitted as `tool_call` events in the Claude runner)
- Aggregate in a new `GET /api/analytics/tools` endpoint
- CLI: `nube tools stats` shows top tools by usage, latency, error rate

---

## 7. App Templates / Scaffolding

**Problem:** Creating a new app requires knowing the directory structure, file formats, JSON manifests, etc. Easy to get wrong.

**Idea:** `nube app create` scaffolds a new app from a template.

```bash
nube app create my-reports --template basic
# Creates:
#   data/apps/my-reports/
#     app.yaml           (pre-filled with name, version)
#     preamble            (empty template)
#     tools/
#       example.js        (hello world tool)
#       example.json      (tool manifest)
#     prompts/
#       example.md        (prompt template)

nube app create my-api --template openapi
# Creates app configured for OpenAPI remote spec
```

Low effort, saves friction for app authors. Currently apps are created via the REST API (`POST /api/my/apps`) — there is no CLI scaffolding command.

**Simplification:** Skip the `--template` flag initially. A single `nube app create <name>` that generates the minimal structure (app.yaml + one example tool) is enough. Templates add a decision point that slows people down when they just want to start.

---

## 8. Server-side Agent Loop for Non-Claude Providers (high priority, high effort)

**Problem:** Only Claude has tool-calling support via native MCP. Ollama, OpenAI, Anthropic, and Gemini users get text-only chat — they can see tools exist but the AI can't call them. This is the largest capability gap in the platform.

**Current state:** The `Runner` interface (`pkg/airunner/runner.go`) delegates tool handling entirely to the provider. Claude's CLI handles MCP natively, but the Ollama runner is text-only (no tool calling). OpenAI, Anthropic, and Gemini runners are not yet implemented (stubs only in the registry).

**Idea:** Build a server-side agent loop that converts MCP tool schemas to each provider's function-calling format and handles the call/response cycle.

```
User prompt
  → Server loads user's installed tools (from MCPFactory.BuildServer)
  → Convert MCP tool schemas to provider's function-calling format
  → Send prompt + tool definitions to LLM API
  → LLM responds with tool_call request
  → Execute tool via existing JS/OpenAPI runtime
  → Feed result back to LLM
  → Loop until final text response or maxSteps hit
```

**What changes:**
- New `AgentLoop` struct that wraps any `Runner` and adds tool-calling orchestration
- Tool schema converter: MCP `inputSchema` → OpenAI `functions[]` / Anthropic `tools[]` / Ollama `tools[]`
- Max steps guard (default 10) to prevent runaway loops
- Works with the existing `MCPFactory` — no new tool infrastructure needed
- Ollama already supports tool calling in its API (`tools` field in `/api/chat`) — just not wired up yet

**Provider-specific notes:**
- **Ollama**: supports `tools` in `/api/chat` requests natively — lowest effort to add
- **OpenAI**: standard `functions` / `tool_choice` in chat completions API
- **Anthropic**: `tools` array in Messages API
- **Gemini**: function calling via OpenAI-compatible endpoint

---

## 9. Replace JSON DB with GORM + SQLite/PostgreSQL — DONE

**Problem:** All data lives in JSON files (`pkg/jsondb.Collection[T]`). Every query is a linear scan — `FindFunc()` loads the entire collection into memory and iterates. There are no indexes, no DB-level filtering, no proper pagination, and no referential integrity.

**Current pain points:**
- **Sessions** (~465+ records, growing fast): filtered by `user_id` via full scan on every request. At 100 users × 10 runs/day = 365k sessions/year — queries get slow.
- **Store apps browse** (~232 records): `All()` → in-memory filter by category → in-memory text search → in-memory sort → in-memory pagination. Every browse request does a full table scan.
- **App installs check**: every install request and every app detail page scans ALL installs to check if installed.
- **Rating recalculation**: on every review submit, scans ALL reviews for that app to recompute average.
- **Every write flushes the entire collection** to disk as JSON (tmp + rename). Cost scales with collection size.

**Idea:** Replace `jsondb.Collection[T]` with GORM, using SQLite as the default (embedded, zero-config, single-file) and PostgreSQL as the production option.

**Why GORM + SQLite, not a graph DB:**
- The data model is relational with shallow relationships (User → Sessions, User → AppInstalls, App → Reviews). All 1-to-many, max 2 hops. Graph DBs (Neo4j, Dgraph) shine at deep traversals ("friends of friends who bought X") which this system doesn't have.
- The AI doesn't query the database directly — it calls tools via MCP. What helps the AI is tools that respond fast with precise results, which means proper indexes on a relational DB, not a different query paradigm.
- Graph DBs add operational complexity (separate server, Cypher query language) with no payoff for this data model.
- SQLite is embedded (no separate server), file-based (like current JSON approach), and supports full-text search (FTS5) natively.

**What changes:**

```go
// Before (jsondb):
sessions := a.Sessions.FindFunc(func(s models.Session) bool {
    return s.UserID == user.ID
})

// After (GORM):
var sessions []models.Session
a.DB.Where("user_id = ?", user.ID).Order("created_at DESC").Limit(50).Find(&sessions)
```

- Replace `jsondb.Collection[T]` with `*gorm.DB` in the `App` struct
- Add GORM tags to all models (`gorm:"primaryKey"`, `gorm:"index"`, etc.)
- Auto-migrate on startup (GORM handles schema creation)
- Driver selection via env var: `NUBE_DB_DRIVER=sqlite` (default) or `NUBE_DB_DRIVER=postgres` with `NUBE_DB_DSN=...`
- Startup migration: read existing JSON files, insert into DB, rename JSON files to `.json.bak`
- Add proper indexes:

```sql
-- Hot path queries
CREATE INDEX idx_sessions_user_created ON sessions(user_id, created_at DESC);
CREATE INDEX idx_app_installs_user_app ON app_installs(user_id, app_name);
CREATE INDEX idx_store_apps_category ON store_apps(category);
CREATE INDEX idx_store_apps_author ON store_apps(author_id);
CREATE INDEX idx_app_reviews_app ON app_reviews(app_id);
CREATE INDEX idx_workflow_runs_user ON workflow_runs(user_id, status);
```

**Full-text search for app store:**

```sql
-- SQLite FTS5
CREATE VIRTUAL TABLE store_apps_fts USING fts5(name, display_name, description, tags);

-- PostgreSQL equivalent
CREATE INDEX idx_store_apps_search ON store_apps USING gin(to_tsvector('english', name || ' ' || description));
```

**Migration approach:**
1. Add GORM alongside jsondb (both work during transition)
2. Startup: if JSON files exist and DB is empty, import from JSON → DB
3. Once stable, remove jsondb dependency
4. Keep JSON export as a backup/debug feature (`nube admin export`)

**AI-specific benefit — vector search (future):**
Once on a real DB, adding vector search becomes straightforward:
- **sqlite-vec** or **pgvector** extension for embedding storage
- Semantic app discovery: "find apps similar to rubix" → vector similarity on app descriptions
- Session search: "find conversations about alarms" → vector similarity on session prompts/results
- Memory search: semantic recall across user/server memory

This is not part of the initial migration but becomes trivial once off JSON files.

---

## 10. Platform Tools — AI Access to Internal Data (medium priority, low effort)

**Problem:** The AI has tools to query external systems (Rubix, weather APIs, etc.) but can't query the platform itself. It can't answer "how many sessions did I run this week?", "which apps do I have installed?", or "show me my recent errors." The user has to leave the AI and check the dashboard.

**Why not a regular app with JS tools?**
- A JS tool calling `http.get("http://localhost:8090/api/agents/sessions")` is a round-trip to yourself — the server runs JS which calls HTTP back to the same server. Wasteful.
- Auth is messy — the JS runtime doesn't have the calling user's bearer token, so you'd need to inject it or rely on dev-mode fallback.
- A standalone app on disk (`data/apps/nube-platform/`) with JS files that just wrap Go calls is unnecessary indirection.

**Idea:** Register Go-native tools directly in `MCPFactory.BuildServer()`, namespaced as `platform.*`. No app directory, no JS, no HTTP. Direct DB access, user-scoped automatically.

```go
// In MCPFactory.BuildServer(), after registering app tools:
f.registerPlatformTools(srv, userID, db)
```

The AI sees these as normal MCP tools — they work identically in MCP clients (Claude Desktop, Cursor) and the CLI (`nube ask`). No special handling needed.

**Tools to register:**

```
platform.list_sessions     — "List my recent AI sessions (provider, model, cost, duration)"
  params: limit (default 20), provider (optional), status (optional)
  returns: [{id, provider, model, prompt_preview, status, duration_ms, cost_usd, created_at}]

platform.get_session       — "Get full details of a specific session including the AI response"
  params: session_id
  returns: {id, provider, model, prompt, result, status, duration_ms, cost_usd, tool_calls, ...}

platform.list_installs     — "List my installed apps and their status"
  params: none
  returns: [{app_name, enabled, version, stale, installed_at}]

platform.search_apps       — "Search the app store for apps by name or category"
  params: query (optional), category (optional), limit (default 20)
  returns: [{name, description, category, avg_rating, install_count}]

platform.usage_stats       — "Get my usage summary (sessions, tokens, cost) for a time range"
  params: days (default 7)
  returns: {total_sessions, total_tokens, total_cost, by_provider: [{provider, sessions, cost}]}
```

**What changes:**

```go
// New method on MCPFactory — registers platform tools with direct DB access
func (f *MCPFactory) registerPlatformTools(srv *mcp.Server, userID string, db *gorm.DB) {
    // platform.list_sessions
    srv.AddTool(&mcp.Tool{
        Name:        "platform.list_sessions",
        Description: "List your recent AI sessions with provider, model, cost, and duration",
        InputSchema: &sessionsSchema,
    }, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        var sessions []models.Session
        db.Where("user_id = ?", userID).Order("created_at DESC").Limit(limit).Find(&sessions)
        // ... marshal and return
    })

    // platform.usage_stats
    srv.AddTool(&mcp.Tool{
        Name:        "platform.usage_stats",
        Description: "Get your usage summary: sessions, tokens, and cost over a time range",
        InputSchema: &statsSchema,
    }, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        var stats struct {
            Sessions int     `json:"sessions"`
            Tokens   int     `json:"tokens"`
            Cost     float64 `json:"cost"`
        }
        db.Model(&models.Session{}).Where("user_id = ? AND created_at > ?", userID, since).
            Select("COUNT(*) as sessions, SUM(input_tokens+output_tokens) as tokens, SUM(cost_usd) as cost").
            Scan(&stats)
        // ... marshal and return
    })
}
```

**Why this approach wins:**
- **No new infrastructure** — uses existing MCPFactory registration, existing MCP server, existing tool discovery
- **Works for MCP and CLI identically** — registered on the MCP server like any other tool, Claude/Cursor/`nube ask` all see it
- **User-scoped by construction** — `BuildServer()` already knows the user; platform tools get `userID` at registration time, every query is `WHERE user_id = ?`
- **Fast** — direct DB query, no HTTP round-trip, no JS VM startup
- **Read-only** — platform tools only read data; no risk of the AI modifying platform state
- **Always available** — not an installable app, always registered for every user

**What it enables for the AI:**

```
User: "how much have I spent on AI this week?"
AI: calls platform.usage_stats(days=7)
AI: "You've run 47 sessions this week, using 284k tokens across Claude and Ollama.
     Total cost: $3.42 — $3.42 from Claude, $0 from Ollama (local)."

User: "which of my tools keeps failing?"
AI: calls platform.list_sessions(status="error", limit=20)
AI: "Your last 20 errors are all from rubix.query_nodes — looks like the Rubix
     host at 192.168.1.50 is timing out. Want me to check the connection?"

User: "is there an app for PDF reports?"
AI: calls platform.search_apps(query="pdf report")
AI: "Found pdf-export — it has render and list_templates tools, rated 4.2/5.
     Want me to install it?"
```

**Later additions (once tool call tracking #6 is done):**

```
platform.tool_stats        — "Show tool usage, latency, and error rates"
  params: days (default 7), app (optional)
  returns: [{tool, calls, avg_duration_ms, error_rate, avg_output_bytes}]
```

---

## Priority ranking

| # | Idea | Effort | Impact | Status |
|---|---|---|---|---|
| 6 | Tool call tracking | Low | High | **DONE** — `ToolCallLog` on sessions, per-tool name/duration/status/error |
| 1 | Preamble composition | Low | High | **DONE** — `BuildAppContext()` on MCPFactory, injected via `AgentService` |
| 9 | GORM + SQLite/PG migration | Medium | High | **DONE** — SQLite via GORM, auto-migrate, JSON import on first startup. See [DB.md](DB.md) |
| 10 | Platform tools | Low | High | Next — DB is ready, register Go-native `platform.*` tools in MCPFactory |
| 8 | Server-side agent loop | High | High | Next — unlocks tools for all non-Claude providers |
| 5 | Prompt chaining | Medium | High | Planned — high value but get execution model right |
| 2 | Tool result caching | Medium | Medium | Phase 2 — informed by tracking data from #6 |
| 3 | Settings/secrets hardening | Low | Medium | Phase 2 — model split exists, just finish the API/encryption |
| 4 | Event hooks | High | Medium | Phase 3 — only after core loop is solid |
| 7 | App templates | Low | Low | Nice to have — do it when someone asks |

**What's next:**

With tracking (#6), preamble (#1), and the DB (#9) done, the two highest-impact remaining items are platform tools (#10) and the server-side agent loop (#8). Platform tools are low effort since GORM is in place — just register `platform.list_sessions`, `platform.usage_stats`, etc. as Go-native tools in MCPFactory. The agent loop is higher effort but unlocks tool calling for Ollama, OpenAI, Anthropic, and Gemini.
