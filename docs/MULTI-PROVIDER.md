# Multi-Provider AI System

The server supports multiple AI providers through a single unified interface. Any provider can power any entry point — WebSocket, REST, async jobs, or CLI — using the same code path.

---

## Architecture

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
   nube ask --direct "question"  ---> shells out to Claude CLI directly
```

Every provider goes through `Runner.Run()`. The server doesn't know or care whether the backend is a CLI process, a local API, or a cloud service.

---

## Registered providers

Four providers are registered at startup in `NewRegistry()`:

| Provider | Type | Backend | Tool calling | Session resume |
|---|---|---|---|---|
| **Claude** | CLI | Shells out to `claude` binary | Yes (native MCP) | Yes (`--resume`) |
| **Ollama** | API | HTTP to `/api/chat` (local) | No | No |
| **Codex** | CLI | Shells out to `codex` binary | No | No |
| **Copilot** | CLI | Shells out to `gh copilot` | No | No |

Seven provider constants are defined (claude, ollama, openai, anthropic, gemini, codex, copilot) but only the four above have runner implementations. OpenAI, Anthropic, and Gemini are defined as constants for future use.

**Implementation:** [pkg/airunner/registry.go](../pkg/airunner/registry.go)

```go
func NewRegistry() *Registry {
    r := &Registry{runners: make(map[Provider]Runner)}
    r.Register(&ClaudeRunner{})
    r.Register(&OllamaRunner{})
    r.Register(&CodexRunner{})
    r.Register(&CopilotRunner{})
    return r
}
```

---

## Runner interface

Every provider implements this interface:

```go
type Runner interface {
    Name() Provider
    Available() bool
    Run(ctx context.Context, cfg RunConfig, sessionID string, onEvent func(Event)) RunResult
}
```

- `Available()` checks whether the provider is reachable (CLI in PATH, API responding)
- `Run()` blocks until done or cancelled, streaming events via `onEvent`
- `ctx` cancellation kills the underlying process/connection

**Implementation:** [pkg/airunner/runner.go](../pkg/airunner/runner.go)

### RunConfig

```go
type RunConfig struct {
    Prompt       string   // the user's message (with memory prefix already prepended)
    ResumeID     string   // resume a previous session (Claude: --resume)
    MCPURL       string   // MCP server endpoint for tool access
    MCPToken     string   // bearer token for MCP auth
    AllowedTools string   // tool pattern filter (e.g. "mcp__nube__*")
    Model        string   // model override (e.g. "gemma3", "gpt-4.1")
    WorkDir      string   // working directory for CLI processes
}
```

### Event

All providers emit the same normalised event types:

```go
type Event struct {
    Type       string   // "connected", "tool_call", "text", "error", "done"
    Provider   string   // "claude", "ollama", "codex", "copilot"
    SessionID  string
    Model      string   // set on "connected"
    Name       string   // tool name on "tool_call"
    Content    string   // text chunk on "text"
    Error      string   // message on "error"
    DurationMS int      // set on "done"
    CostUSD    float64  // set on "done"
}
```

### RunResult

```go
type RunResult struct {
    Text            string   // aggregated response text
    Provider        string
    Model           string
    ClaudeSessionID string   // Claude-specific, for --resume
    DurationMS      int
    CostUSD         float64
    InputTokens     int
    OutputTokens    int
    ToolCalls       int
}
```

---

## Provider details

### Claude

Shells out to the Claude Code CLI with stream-json output. The only provider with native MCP tool calling and session resume.

**Availability check:** `exec.LookPath("claude")`

**How it runs:**
1. Writes a temporary MCP config file pointing to the server's `/mcp` endpoint
2. Spawns: `claude -p --output-format stream-json --verbose --allowedTools "mcp__nube__*" "ToolSearch" --mcp-config <file>`
3. Optionally adds `--resume <id>` for multi-turn
4. Pipes the user prompt via stdin
5. Parses newline-delimited JSON events from stdout

**Implementation:** [pkg/airunner/claude.go](../pkg/airunner/claude.go) wraps [pkg/claude/runner.go](../pkg/claude/runner.go)

### Ollama

Calls the local Ollama server's native `/api/chat` endpoint with streaming.

**Availability check:** `GET /api/tags` responds with 200 (2s timeout)

**Host resolution (priority order):**
1. Admin config via `Configure(host, _)` from provider settings
2. `OLLAMA_HOST` env var
3. Default: `http://localhost:11434`

**How it runs:**
1. Sends `POST /api/chat` with `{model, messages: [{role: "user", content: prompt}], stream: true}`
2. Reads newline-delimited JSON chunks from the response body
3. Each chunk has `message.content` (text) and a final `done: true` chunk with token counts
4. Reports `prompt_eval_count` as input tokens, `eval_count` as output tokens
5. Cost is always $0 (local inference)

**Default model:** `gemma3` (used when no model specified)

**Model discovery:** Implements `ModelLister` interface — calls `GET /api/tags` to return installed model names.

**Implementation:** [pkg/airunner/ollama.go](../pkg/airunner/ollama.go)

### Codex

Shells out to the OpenAI Codex CLI in quiet, full-auto mode.

**Availability check:** `exec.LookPath("codex")`

**How it runs:**
1. Spawns: `codex --quiet --full-auto [--model <model>] <prompt>`
2. Reads stdout line by line
3. No structured event format — each line is emitted as a text event

**Requires:** `OPENAI_API_KEY` env var, `npm install -g @openai/codex`

**Implementation:** [pkg/airunner/codex.go](../pkg/airunner/codex.go)

### Copilot

Shells out to GitHub Copilot via the `gh` CLI extension.

**Availability check:** `exec.LookPath("gh")` + `gh extension list` contains "copilot"

**How it runs:**
1. Spawns: `gh copilot suggest -t shell <prompt>`
2. Reads stdout line by line
3. Same line-by-line text event approach as Codex

**Requires:** `gh auth login`, `gh extension install github/gh-copilot`

**Implementation:** [pkg/airunner/copilot.go](../pkg/airunner/copilot.go)

---

## Provider resolution

When a request arrives without an explicit provider, the system resolves one using this priority:

1. Request-level `provider` param (from WS message, REST body, or CLI flag)
2. User's `DefaultProvider` preference (from `UserPreferences`)
3. Fallback: `claude`

Same logic for model — request param, then user default, then provider's own default.

**Implementation:** [pkg/api/settings.go](../pkg/api/settings.go)

```go
func resolveProvider(reqProvider, reqModel string, user models.User) (Provider, string) {
    provider := Provider(reqProvider)
    if provider == "" {
        if user.Preferences != nil && user.Preferences.DefaultProvider != "" {
            provider = Provider(user.Preferences.DefaultProvider)
        } else {
            provider = ProviderClaude
        }
    }
    // ...
}
```

---

## Provider discovery API

Lists all registered providers with live availability status and installed models.

```
GET /api/agents/providers

→ 200
[
  {"provider": "claude",  "available": true,  "type": "cli", "models": []},
  {"provider": "ollama",  "available": true,  "type": "api", "models": ["gemma3", "llama3.1"]},
  {"provider": "codex",   "available": false, "type": "cli", "models": []},
  {"provider": "copilot", "available": false, "type": "cli", "models": []}
]
```

Providers that implement `ModelLister` (currently Ollama) include their installed models. The `type` field is `"cli"` for subprocess-based providers and `"api"` for HTTP-based ones.

**Implementation:** `Registry.Available()` in [pkg/airunner/registry.go](../pkg/airunner/registry.go)

### Provider test endpoint

Tests connectivity to a specific provider and reports latency.

```
POST /api/agents/providers/:name/test

→ 200
{
  "provider": "ollama",
  "available": true,
  "models": ["gemma3", "llama3.1"],
  "latency_ms": 12
}
```

On failure, returns a human-readable reason ("claude CLI not found in PATH", "ollama not reachable — is it running?", etc.).

**Implementation:** `testProvider()` in [pkg/api/settings.go](../pkg/api/settings.go)

---

## Provider configuration

### Global config (admin)

Stored in `data/provider_config.json`. Controls which providers are enabled and their connection settings.

```go
type ProviderConfig struct {
    Providers map[string]ProviderSettings
}

type ProviderSettings struct {
    Enabled bool
    APIKey  string   // for cloud providers
    Host    string   // for Ollama (default http://localhost:11434)
}
```

**Defaults** (set in `DefaultProviderConfig()`):

| Provider | Enabled | Host |
|---|---|---|
| claude | true | — (CLI, no host) |
| ollama | true | http://localhost:11434 |
| openai | false | — |
| anthropic | false | — |
| gemini | false | — |

**API:**

```
GET /api/settings/providers      → list with live availability merged in (admin)
PUT /api/settings/providers      → update config, applies to runners immediately (admin)
```

The `PUT` endpoint merges the request into existing config (only updates fields that are present) and calls `ApplyProviderConfig()` which pushes host/API key values into runners that implement the `Configurable` interface.

**Implementation:** [pkg/api/settings.go](../pkg/api/settings.go), [pkg/models/provider_config.go](../pkg/models/provider_config.go)

### User preferences

Each user can set a default provider and model. Stored on the `User` model.

```go
type UserPreferences struct {
    DefaultProvider string   // "claude", "ollama", etc.
    DefaultModel    string   // "gemma3", "gpt-4.1", etc.
}
```

**API:**

```
GET /users/me/preferences        → current defaults
PUT /users/me/preferences        → set defaults (validates provider exists)
```

Users can always override per-request via `--provider`/`--model` flags or the request body.

**Implementation:** `getUserPreferences()` / `updateUserPreferences()` in [pkg/api/settings.go](../pkg/api/settings.go)

---

## Job system

For async/automation use cases (scripts, cron, CI, webhooks). The frontend and CLI use WebSocket for real-time streaming; jobs are for fire-and-forget execution with polling.

**Implementation:** [pkg/airunner/jobstore.go](../pkg/airunner/jobstore.go), [pkg/api/agents_jobs.go](../pkg/api/agents_jobs.go)

### How it works

1. Client submits a job → server returns `job_id` immediately
2. Server runs `Runner.Run()` in a background goroutine
3. Events are buffered in memory with sequential indices
4. Client polls with `?after=<index>` for incremental event delivery
5. Real-time subscribers (WS handler) get events via a channel instead of polling

### API

```
POST /api/agents/jobs
  Body: {"prompt": "...", "provider": "ollama", "model": "gemma3"}
  → 202 {"job_id": "job-xxx", "status": "running"}

GET /api/agents/jobs/:id?after=<index>
  → 200 {job_id, status, provider, model, events[], result}

GET /api/agents/jobs
  → 200 [...all jobs for current user...]

DELETE /api/agents/jobs/:id
  → 200 {"status": "cancelled"}
```

### Job lifecycle

```
running → done       (normal completion)
running → error      (provider error)
running → cancelled  (user cancels via DELETE, or WS disconnects)
```

Cancellation works through `context.Context` — calling `Cancel()` cancels the context, which kills the CLI subprocess or closes the HTTP connection to Ollama.

### Cleanup

A background goroutine runs every minute and removes completed jobs older than 10 minutes. Jobs are in-memory only — session records are persisted to `sessions.json` on completion.

### WebSocket integration

The WS handler (`runAgentWS`) also uses the job store internally. It submits a job, subscribes to its event channel, and forwards events to the WebSocket. If the WebSocket disconnects, the job is cancelled. This means the WS handler and the job API share the same execution path.

---

## Session tracking

Every AI run (WS, REST, or job) persists a session record to `data/sessions.json`.

```go
type Session struct {
    ID              string     // "ses-xxx"
    Provider        string     // "claude", "ollama", "codex", "copilot"
    Model           string     // "gemma3", "claude-sonnet-4-20250514", etc.
    ClaudeSessionID string     // Claude-specific, for --resume
    Agent           string     // app name if specified
    Prompt          string     // original user prompt
    Result          string     // full response text
    Status          string     // "done", "error", "cancelled"
    DurationMS      int
    CostUSD         float64
    InputTokens     int        // Ollama: prompt_eval_count; Claude: from stream
    OutputTokens    int        // Ollama: eval_count; Claude: from stream
    ToolCalls       int        // count of tool_call events
    UserID          string
    CreatedAt       time.Time
}
```

**API:**

```
GET /api/agents/sessions       → list sessions for current user (summary view, no result text)
GET /api/agents/sessions/:id   → single session with full result text
```

### Session resume

Only Claude supports session resume. When Claude completes a run, it returns a `ClaudeSessionID` which is stored on the session. To resume:

1. Client sends `session_id` in the WS request
2. Server adds `--resume <ClaudeSessionID>` to the Claude CLI invocation
3. Claude picks up from where the previous conversation ended

Other providers have no built-in resume. Multi-turn for non-Claude providers would require message history replay (not yet implemented).

### Data migration

On startup, `migrateSessionProvider()` backfills `provider="claude"` on any session created before multi-provider support was added.

**Implementation:** [pkg/models/session.go](../pkg/models/session.go), session handlers in [pkg/api/agents_ws.go](../pkg/api/agents_ws.go)

---

## CLI

### Interactive (WebSocket)

```bash
nube ask "check offline devices"                            # default provider
nube ask --provider ollama --model gemma3 "hello"           # explicit provider
nube ask --session ses-abc123 "what about floor 3?"         # resume (Claude only)
nube ask --direct "quick question"                          # bypass server, Claude CLI directly
```

**Implementation:** [pkg/cli/cmd_ask.go](../pkg/cli/cmd_ask.go)

### Async jobs

```bash
nube agents submit "generate weekly report"                 # submit job
nube agents submit --provider ollama "check devices"        # with provider
nube agents poll job-abc123                                 # stream events until done
nube agents poll job-abc123 --once                          # one-shot status check
```

**Implementation:** [pkg/cli/cmd_agents.go](../pkg/cli/cmd_agents.go)

### Provider info

```bash
nube providers                    # list all providers with status
nube providers ollama             # test a specific provider
```

**Implementation:** [pkg/cli/cmd_tools.go](../pkg/cli/cmd_tools.go) (`NewProvidersCmd()`)

---

## Optional runner interfaces

Runners can implement additional interfaces for extra capabilities:

| Interface | Method | Who implements | Purpose |
|---|---|---|---|
| `ModelLister` | `InstalledModels() ([]string, error)` | Ollama | Report installed models in discovery API |
| `Configurable` | `Configure(host, apiKey string)` | Ollama | Accept runtime config from admin settings |

**Implementation:** [pkg/airunner/registry.go](../pkg/airunner/registry.go)

---

## File map

| File | Purpose |
|---|---|
| [pkg/airunner/runner.go](../pkg/airunner/runner.go) | `Runner` interface, `RunConfig`, `Event`, `RunResult` types, provider constants |
| [pkg/airunner/registry.go](../pkg/airunner/registry.go) | Thread-safe registry, `ModelLister`, `Configurable` interfaces, `ProviderInfo` |
| [pkg/airunner/claude.go](../pkg/airunner/claude.go) | `ClaudeRunner` — wraps `pkg/claude` |
| [pkg/airunner/ollama.go](../pkg/airunner/ollama.go) | `OllamaRunner` — native `/api/chat` with streaming and model discovery |
| [pkg/airunner/codex.go](../pkg/airunner/codex.go) | `CodexRunner` — OpenAI Codex CLI |
| [pkg/airunner/copilot.go](../pkg/airunner/copilot.go) | `CopilotRunner` — GitHub Copilot via `gh` CLI |
| [pkg/airunner/jobstore.go](../pkg/airunner/jobstore.go) | In-memory job store with event buffering, cancellation, 10-min cleanup |
| [pkg/claude/runner.go](../pkg/claude/runner.go) | Low-level Claude CLI spawner with stream-json parsing |
| [pkg/api/agents_ws.go](../pkg/api/agents_ws.go) | WebSocket handler — all providers via `Runner.Run()` |
| [pkg/api/agents_rest.go](../pkg/api/agents_rest.go) | Sync REST endpoint |
| [pkg/api/agents_jobs.go](../pkg/api/agents_jobs.go) | Job submit, poll, list, cancel endpoints |
| [pkg/api/settings.go](../pkg/api/settings.go) | Provider resolution, admin config API, user preferences, provider test |
| [pkg/models/session.go](../pkg/models/session.go) | Session model with provider/model/token fields |
| [pkg/models/provider_config.go](../pkg/models/provider_config.go) | `ProviderConfig` / `ProviderSettings` types, defaults |
| [pkg/cli/cmd_ask.go](../pkg/cli/cmd_ask.go) | `nube ask` — WS client with `--provider`, `--model`, `--direct` |
| [pkg/cli/cmd_agents.go](../pkg/cli/cmd_agents.go) | `nube agents submit` / `nube agents poll` |
| `data/provider_config.json` | Persisted admin provider settings |
| `data/sessions.json` | Persisted session history |

---

## Limitations

- **Tool calling is Claude-only.** Claude has native MCP support. Other providers receive the prompt but cannot call tools. Adding tool calling for non-Claude providers requires a server-side agent loop (send prompt + tool definitions, execute tool calls, feed results back).
- **Session resume is Claude-only.** Other providers would need message history replay.
- **Cloud providers (OpenAI, Anthropic, Gemini) have no runners yet.** The constants and config slots exist, but no implementation.
- **No per-user API keys (BYOK).** All users share the server's provider config. Per-user keys would need encrypted storage and a UI for key management.
