# Multi-Provider AI Support

Goal: let any AI provider (Claude Code CLI, Ollama, OpenAI, Anthropic, Gemini) power the system — through the CLI, frontend, and API — using a single server-side code path.

---

## Current state (after Phase 1 + 2)

| Component | How it works | Status |
|---|---|---|
| **Runner interface** | `Runner.Run(ctx, cfg, sessionID, onEvent)` with `context.Context` for cancellation | Done |
| **Providers** | Claude (CLI), Ollama (native API), Codex, Copilot registered in `Registry` | Done |
| **WS streaming** | All providers through unified `runner.Run()` — no Claude special-case | Done |
| **Job system** | `POST /api/agents/jobs` (submit), `GET` (poll with `?after=`), `DELETE` (cancel) | Done |
| **CLI `nube ask`** | WS client to server, `--provider`, `--model`, `--direct` flags | Done |
| **CLI `nube agents`** | `submit` and `poll` subcommands for async jobs | Done |
| **Session model** | Tracks `provider`, `model`, `input_tokens`, `output_tokens`, `tool_calls` | Done |
| **Session migration** | Backfills `provider="claude"` on startup for existing sessions | Done |
| **OllamaRunner** | Native `/api/chat` with streaming, token reporting, model discovery | Done |
| **Provider discovery** | `GET /api/agents/providers` returns provider, available, type, models[] | Done |
| **User preferences** | Default provider/model per user | Not started |
| **Provider config API** | Admin CRUD for API keys, Ollama host | Not started |
| **Provider test endpoint** | `POST /api/agents/providers/:name/test` | Not started |
| **Frontend provider selector** | Dropdown in chat UI | Not started |
| **Cloud providers** | OpenAI, Anthropic, Gemini runners | Not started |
| **Agent loop** | MCP tool calling for non-Claude providers | Not started |

### Architecture

```
Three ways to run AI — all share the same Runner backend:

1. WebSocket (real-time — frontend, interactive CLI):
   CLI (nube ask)  --+
   Frontend        --+---> WS /api/agents/run ---> Server ---> Runner.Run()
   Mobile (future) --+          |
                                +-- streams events to client in real-time

2. Jobs + polling (async — cron, CI, scripts, webhooks):
   POST /api/agents/jobs         ---> returns {job_id: "job-xxx"} immediately
   GET  /api/agents/jobs/:id     ---> poll every ~3s, get latest events
   DELETE /api/agents/jobs/:id   ---> cancel a running job

3. Direct mode (no server — CLI fallback):
   nube ask --direct "question"  ---> shells out to provider directly
```

---

## Key files

| File | What it does |
|---|---|
| `pkg/airunner/runner.go` | `Runner` interface, `RunConfig`, `Event`, `RunResult` types, provider constants |
| `pkg/airunner/claude.go` | `ClaudeRunner` — wraps `pkg/claude.Run()`, passes `ResumeID` for multi-turn |
| `pkg/airunner/ollama.go` | `OllamaRunner` — native `/api/chat` streaming, `InstalledModels()` |
| `pkg/airunner/codex.go` | `CodexRunner` — OpenAI Codex CLI |
| `pkg/airunner/copilot.go` | `CopilotRunner` — GitHub Copilot CLI |
| `pkg/airunner/registry.go` | Thread-safe registry, `ModelLister` interface, `ProviderInfo` with type/models |
| `pkg/airunner/jobstore.go` | In-memory job store with event buffer, context cancellation, 10-min cleanup |
| `pkg/api/agents_ws.go` | Unified WS handler — all providers through `runner.Run()` |
| `pkg/api/agents_jobs.go` | Job submit, poll, cancel endpoints |
| `pkg/api/agents_rest.go` | Sync REST endpoint (legacy, still works) |
| `pkg/claude/runner.go` | Low-level Claude CLI spawner with stream-json parsing |
| `pkg/cli/cmd_ask.go` | `nube ask` — WS client with `--provider`, `--model`, `--direct` |
| `pkg/cli/cmd_agents.go` | `nube agents submit` and `nube agents poll` |
| `pkg/models/session.go` | Session with Provider, Model, token tracking fields |

---

## Runner interface

```go
type Runner interface {
    Name() Provider
    Available() bool
    Run(ctx context.Context, cfg RunConfig, sessionID string, onEvent func(Event)) RunResult
}
```

All runners accept `context.Context` — when the WS drops or a job is cancelled, the context is cancelled and the runner kills the underlying process or closes the HTTP connection.

### RunConfig

```go
type RunConfig struct {
    Prompt       string   // the user's message
    ResumeID     string   // resume a previous session (Claude: --resume)
    MCPURL       string   // MCP server endpoint
    MCPToken     string   // bearer token for MCP auth
    AllowedTools string   // tool pattern filter
    Model        string   // model override
    WorkDir      string   // working directory
}
```

### RunResult

```go
type RunResult struct {
    Text            string   // aggregated response text
    Provider        string   // which provider ran
    Model           string   // which model was used
    ClaudeSessionID string   // Claude-specific, for --resume
    DurationMS      int
    CostUSD         float64
    InputTokens     int
    OutputTokens    int
    ToolCalls       int
}
```

---

## Provider discovery

`GET /api/agents/providers` returns:

```json
[
  {"provider": "claude",  "available": true,  "type": "cli", "models": []},
  {"provider": "ollama",  "available": true,  "type": "api", "models": ["gemma3", "llama3.1"]},
  {"provider": "codex",   "available": false, "type": "cli", "models": []},
  {"provider": "copilot", "available": false, "type": "cli", "models": []}
]
```

Runners that implement the `ModelLister` interface (currently just Ollama) return installed models. Ollama calls `GET /api/tags` to discover them.

---

## Job system

For async/automation use (scripts, cron, CI, webhooks). The frontend uses WS, not jobs.

```
POST /api/agents/jobs
  Body: {"prompt": "...", "provider": "ollama", "model": "gemma3", "agent": "..."}
  Returns: {"job_id": "job-xxx", "status": "running"}

GET /api/agents/jobs/:id?after=<index>
  Returns: {job_id, status, provider, model, events[], result}
  Events only with index > after (incremental polling)

DELETE /api/agents/jobs/:id
  Cancels a running job via context cancellation
```

**Server-side**: jobs store events in memory. A cleanup goroutine removes completed jobs after 10 minutes. Sessions are persisted to `sessions.json` on completion.

### CLI usage

```bash
# Interactive (WS, default):
nube ask "check offline devices"
nube ask --provider ollama --model gemma3 "hello"

# Submit as job (for scripts/cron):
nube agents submit --provider ollama "generate report"
nube agents poll <job-id>           # streams events until done
nube agents poll <job-id> --once    # one-shot status check

# Direct mode (no server):
nube ask --direct "quick question"
```

---

## Provider configuration

### Two layers

**Global (admin-only)** — provider availability and API keys. All users share these. The admin configures which providers are available.

```bash
# Claude Code CLI — just needs `claude` binary in PATH
# Ollama (local, free)
OLLAMA_HOST=http://localhost:11434    # default

# Cloud providers (Phase 3)
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-ant-...
GEMINI_API_KEY=AI...
```

**Per-user** — default provider and model preference. Each user picks which provider they default to from the admin-enabled providers.

```
User A: default_provider=ollama, default_model=gemma3
User B: default_provider=claude, default_model=  (uses whatever claude picks)
```

Users can always override per-request via `--provider`/`--model` flags or the frontend dropdown.

### Planned API (not yet built)

```
GET  /api/settings/providers          — list provider configs (admin)
PUT  /api/settings/providers          — update provider configs (admin)
GET  /users/me/preferences            — get user default provider/model
PUT  /users/me/preferences            — set user default provider/model
POST /api/agents/providers/:name/test — test a provider connection
```

### Per-user API keys (BYOK — deferred)

All users share the server's API keys. Per-user keys add complexity (encrypted storage, validation, rotation, UI). Defer to post-Phase 4.

---

## Session model

```go
type Session struct {
    ID              string    `json:"id"`
    Provider        string    `json:"provider"`                    // "claude", "ollama", etc.
    Model           string    `json:"model,omitempty"`
    ClaudeSessionID string    `json:"claude_session_id,omitempty"` // Claude-specific, for --resume
    Agent           string    `json:"agent,omitempty"`
    Prompt          string    `json:"prompt"`
    Result          string    `json:"result,omitempty"`
    Status          string    `json:"status"`
    DurationMS      int       `json:"duration_ms"`
    CostUSD         float64   `json:"cost_usd"`
    InputTokens     int       `json:"input_tokens,omitempty"`
    OutputTokens    int       `json:"output_tokens,omitempty"`
    ToolCalls       int       `json:"tool_calls,omitempty"`
    UserID          string    `json:"user_id"`
    CreatedAt       time.Time `json:"created_at"`
}
```

### Session resumption

- **Claude**: native `--resume` via `ClaudeSessionID`. Multi-turn works today.
- **All other providers**: no built-in resume. Multi-turn requires replaying full message history. Deferred to Phase 3.

### Data migration

On startup, `migrateSessionProvider()` backfills `provider="claude"` on any session that has no provider set.

---

## Remaining phases

### Phase 3 — Cloud AI providers

| Provider | API format | Runner |
|---|---|---|
| OpenAI | OpenAI API | `OpenAICompatRunner` |
| Gemini | OpenAI-compatible | `OpenAICompatRunner` (different base URL) |
| Anthropic | Messages API | `AnthropicRunner` |

Plus: message history storage (`data/session_messages/`) for multi-turn on non-Claude providers.

### Phase 4 — MCP bridge (agent loop with tool calling)

For non-Claude providers to use MCP tools, the server becomes the MCP client:

```
User prompt
  -> Load user's installed tools (from MCPFactory)
  -> Convert MCP schema -> provider's function-calling format
  -> Send prompt + tool definitions to LLM API
  -> LLM responds with tool call request
  -> Execute via existing JS/OpenAPI runtime
  -> Feed result back to LLM
  -> Loop until final text response or maxSteps hit
```

Estimated ~700-1000 lines across `agentloop.go`, `toolconvert.go`, `toolexec.go`, `messageformat.go`.

---

## What NOT to do

- **Don't pretend all providers are equal** — Claude has native MCP + session resume. Others need the agent loop + message replay. Document the differences, don't hide them.
- **Don't build the agent loop before basic text chat works** — get cloud providers streaming text first (Phase 3), add tool calling later (Phase 4).
- **Don't merge Ollama into the OpenAI compat runner** — keep it on the native `/api/chat` API for better control.
- **Don't force everything through WS** — automation, CI, webhooks need the job system.
- **Don't silently fall back to a different model** — return a clear error with available alternatives.
