# Ideas for Improving the Framework

A collection of concrete improvements to bizzy, prioritised by impact.

---

## 1. Preamble Composition (high priority, low effort)

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
  Tools: get_product, query_nodes, get_alarms
- pdf-export: Render documents and reports to PDF format.
  Tools: render, list_templates
- rubix: Interact with the Rubix BMS runtime.
  Tools: query_nodes, write_value, discover_device

[System Prompt]
You are a helpful AI assistant...
```

The AI now knows what it can do without the user picking an agent. Each app already has a `description` field and a `preamble` file — this just composes them.

**What changes:**
- Add a `BuildAppContext(userID string) string` method to `MCPFactory` or `Registry`
- Reads installed apps for the user, collects descriptions + tool names
- Inject after memory prefix in agents_ws.go, agents_rest.go, agents_jobs.go
- Keep it short — one line per app + tool list, not full preambles (context budget)

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

---

## 3. App Settings / Secrets Separation

**Problem:** `AppInstall.Settings` holds both config and secrets in one `map[string]string`. Secrets (API keys, tokens) get returned in API responses, logged, and serialized to JSON.

**Idea:** Split settings into config (visible) and secrets (write-only).

```yaml
# app.yaml
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

**What changes:**
- `AppInstall` gets a `Secrets map[string]string` field
- API responses mask secrets: `"api_key": "sk-...***"`
- `GET /app-installs` never returns raw secret values
- JS tools access secrets via `ctx.secret("api_key")` — same as settings but stored separately
- Secrets encrypted at rest (AES-GCM with a server key) or at minimum excluded from logs

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
- `HookRunner` that executes JS hooks with the same runtime as tools
- Hook invocation points in install/uninstall handlers and agent run flow
- Hooks are fire-and-forget (don't block the main flow) except `on_session_start` which injects context

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

---

## 6. Observability: Tool Call Tracking

**Problem:** Sessions track `ToolCalls` as a count but not which tools were called, how long each took, or which ones failed. Can't answer "which tools are slow?" or "which tools fail most?"

**Idea:** Store a `ToolCallLog` array on each session.

```go
type ToolCallEntry struct {
    Name       string `json:"name"`        // "rubix.query_nodes"
    DurationMS int    `json:"duration_ms"`
    Status     string `json:"status"`      // "ok", "error"
    Error      string `json:"error,omitempty"`
}
```

**What it unlocks:**
- Dashboard: "pdf-export.render fails 30% of the time"
- Dashboard: "rubix.query_nodes averages 4.2s — the slowest tool"
- Per-session view: exactly which tools were called and in what order
- Debugging: when a session gives a bad answer, see what tool data the AI received

**What changes:**
- Add `ToolCallLog []ToolCallEntry` to `models.Session`
- Parse tool call events from the AI stream (already emitted as `tool_call` events)
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

Low effort, saves friction for app authors.

---

## Priority ranking

| # | Idea | Effort | Impact | Priority |
|---|---|---|---|---|
| 1 | Preamble composition | Low | High | Do first |
| 5 | Prompt chaining | Medium | High | Do second |
| 6 | Tool call tracking | Low | Medium | Do third |
| 2 | Tool result caching | Medium | Medium | Phase 2 |
| 3 | Settings/secrets split | Medium | Medium | Phase 2 |
| 4 | Event hooks | High | Medium | Phase 3 |
| 7 | App templates | Low | Low | Nice to have |

Preamble composition and prompt chaining are the biggest wins because they solve the core gap: **teaching the AI when and how to use the tools it already has access to**, without the user being the orchestrator.
