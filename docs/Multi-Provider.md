# Multi-Provider AI Support

Goal: let any AI provider (Claude Code CLI, Ollama, OpenAI, Anthropic, Gemini) power the system — through the CLI, frontend, and API — using a single server-side code path.

---

## Current state

| Component | How it works today | Problem |
|---|---|---|
| **CLI `nube ask`** | Shells out to `claude` CLI binary directly, bypasses server | No session persistence, no streaming through server, Claude-only |
| **Frontend chat** | Connects to `WS /api/agents/run`, server shells out to `claude` | Works but has Claude special-case in `agents_ws.go:134` |
| **REST sync** | `POST /api/agents/run/sync`, uses `airunner.Runner` interface | Blocks until done, no streaming, but provider-agnostic |
| **Session model** | `Session` has `ClaudeSessionID` field | No `provider` field, no `model` field, Claude-specific |

### Two code paths for the same thing

```
Frontend:
  browser -> WS /api/agents/run -> server -> claude CLI -> stream events back

CLI:
  nube ask -> directly spawns claude CLI -> prints to terminal
  (server is never involved, no session saved)
```

This is the core problem. The CLI bypasses the server, so it gets no session history, no provider switching, and will never support Ollama/OpenAI without duplicating all that logic client-side.

---

## Architecture: WS for interactive, Jobs for automation

**Three ways to run AI — all share the same `Runner` backend.**

```
1. WebSocket (real-time push — frontend, interactive CLI):
   CLI (nube ask)  --+
   Frontend        --+---> WS /api/agents/run ---> Server ---> Provider
   Mobile (future) --+          |
                                +-- streams events to client in real-time

2. Jobs + polling (async REST — cron, CI, scripts, webhooks):
   POST /api/agents/jobs         ---> returns {job_id: "job-xxx"} immediately
   GET  /api/agents/jobs/:id     ---> poll every ~3s, get latest events
   DELETE /api/agents/jobs/:id   ---> cancel a running job

3. Direct mode (no server — CLI fallback):
   nube ask --direct "question"  ---> shells out to provider directly
```

### Why WS + Jobs, not one or the other

**WS for interactive use:**
- AI responses take 5-60 seconds. Users need to see tokens as they arrive.
- Tool calls show up in real-time ("calling device_summary...")
- Frontend and interactive CLI both use the same WS protocol.

**Jobs for everything else:**
- AI is slow. A blocking REST call that hangs for 30 seconds is terrible for cron, CI, webhooks, and scripts.
- Jobs solve this: submit, get a UUID, poll for updates. Client can disconnect and reconnect.
- Works behind load balancers with timeout limits (no long-lived HTTP connections).
- Natural fit for automation: submit job, poll until `status: "done"`, get result.

The current `POST /api/agents/run/sync` blocks until done — replace it with the job system.

### Job system design

```
POST /api/agents/jobs
  Body: {"prompt": "...", "provider": "ollama", "model": "gemma4", "agent": "..."}
  Returns: {"job_id": "job-xxx", "status": "running"}

GET /api/agents/jobs/:id
  Returns: {
    "job_id": "job-xxx",
    "status": "running" | "done" | "error" | "cancelled",
    "provider": "ollama",
    "model": "gemma4",
    "events": [                        // all events since job start (or since ?after=<index>)
      {"type": "connected", "model": "gemma4", "index": 0},
      {"type": "text", "content": "The device...", "index": 1},
      {"type": "tool_call", "name": "device_summary", "index": 2},
      {"type": "text", "content": "Here are the results...", "index": 3},
      {"type": "done", "duration_ms": 4500, "cost_usd": 0.00, "index": 4}
    ],
    "result": "full text (only set when status=done)"
  }

GET /api/agents/jobs/:id?after=2
  Returns: only events with index > 2 (incremental polling)

DELETE /api/agents/jobs/:id
  Cancels a running job (context cancellation)
```

**Polling with `?after=<index>`** — client tracks the last event index it saw, requests only new events. Efficient, no duplicate data. Typical poll interval: 2-3 seconds.

**Server-side**: the job runner stores events in memory (and persists final result to sessions.json on completion). Events are append-only during the run. A cleanup goroutine removes completed jobs after 10 minutes.

**CLI usage:**

```bash
# Interactive (WS, default):
nube ask "check offline devices"

# Submit as job (for scripts/cron):
JOB=$(nube agents submit --prompt "generate report" --provider ollama -o json | jq -r .job_id)
# Poll until done:
nube agents poll $JOB          # blocks, prints events as they arrive (polls internally)
# Or one-shot check:
nube agents poll $JOB --once   # print current state and exit
```

**curl usage (CI/webhooks):**

```bash
# Submit
JOB_ID=$(curl -s -X POST http://localhost:8090/api/agents/jobs \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"prompt":"generate weekly report","provider":"ollama","model":"gemma4"}' \
  | jq -r .job_id)

# Poll until done
while true; do
  RESULT=$(curl -s http://localhost:8090/api/agents/jobs/$JOB_ID \
    -H "Authorization: Bearer $TOKEN")
  STATUS=$(echo $RESULT | jq -r .status)
  if [ "$STATUS" = "done" ] || [ "$STATUS" = "error" ]; then
    echo $RESULT | jq .result
    break
  fi
  sleep 3
done
```

### CLI change

`nube ask` default mode: connect to the server via WS for streaming.

```go
// Default: WS through server (streaming, session persistence, any provider)
ws := connectWS(serverURL + "/api/agents/run?token=" + token)
ws.WriteJSON(wsRequest{Prompt: prompt, Provider: provider, Model: model})
for event := range ws.ReadEvents() {
    printEvent(event)  // stream to terminal
}
```

**Fallback: `--direct` mode** when the server isn't running.

```go
// Fallback: direct provider invocation (no server needed)
// Claude: shell out to claude binary (current behavior)
// Ollama: direct HTTP to localhost:11434
// Others: direct API call with local env key
```

This keeps the CLI functional when the server is down — critical for single-user laptop use. If the server isn't reachable and `--direct` isn't set, show a clear error: `"server not running at localhost:8090 — start with 'make server' or use --direct"`.

**`nube ask` flags after migration:**

```bash
nube ask "check offline devices"                           # WS to server, default provider
nube ask --provider ollama --model gemma4 "check devices"  # WS to server, ollama
nube ask --provider openai --model gpt-4.1 "summarize"    # WS to server, openai
nube ask --direct "quick question"                         # bypass server, direct to claude CLI
nube ask --direct --provider ollama "hello"                # bypass server, direct to ollama
```

---

## OpenAPI spec sync (manual + validation test)

The CLI auto-generates commands from `cmd/nube/openapi.yaml`. The server routes live in `pkg/api/router.go`. These can drift apart.

### Strategy: validation test

A Go test that loads both and asserts every route has a matching spec entry:

```go
// TestOpenAPIMatchesRouter loads the embedded openapi.yaml and the router,
// then asserts every registered route has a corresponding spec path+method.
func TestOpenAPIMatchesRouter(t *testing.T) {
    spec := loadEmbeddedSpec()
    router := buildTestRouter()

    for _, route := range router.Routes() {
        // Skip WS-only and MCP routes (not in spec by design)
        if isExcluded(route.Path) {
            continue
        }
        assertSpecHasRoute(t, spec, route.Method, route.Path)
    }
}
```

**Excluded routes** (intentionally not in the spec):
- `GET /api/agents/run` -- WebSocket upgrade, not REST
- `GET /api/agents/qa` -- WebSocket upgrade, not REST
- `ANY /mcp`, `/mcp/*path` -- MCP protocol, not REST

### Rules for AI and developers

When adding a new route:

1. Add the handler in `pkg/api/`
2. Register it in `router.go`
3. Add the matching entry in `cmd/nube/openapi.yaml` with `x-cli.command`
4. Run `go test ./tests/ -run TestOpenAPIMatchesRouter` to verify
5. Build CLI: `go build ./cmd/nube/` -- verify the new command appears

When removing or changing a route, update both files and run the test.

---

## Session model changes

The `Session` model needs to track which provider and model were used, and support multi-turn conversation for non-Claude providers.

### Current (`pkg/models/session.go`)

```go
type Session struct {
    ID              string    `json:"id"`
    ClaudeSessionID string    `json:"claude_session_id,omitempty"`
    Agent           string    `json:"agent,omitempty"`
    Prompt          string    `json:"prompt"`
    Result          string    `json:"result,omitempty"`
    Status          string    `json:"status"`
    DurationMS      int       `json:"duration_ms"`
    CostUSD         float64   `json:"cost_usd"`
    UserID          string    `json:"user_id"`
    CreatedAt       time.Time `json:"created_at"`
}
```

### After

```go
type Session struct {
    ID              string    `json:"id"`
    Provider        string    `json:"provider"`                    // NEW: "claude", "ollama", "openai", "anthropic", "gemini"
    Model           string    `json:"model,omitempty"`             // NEW: "claude-sonnet-4-20250514", "gemma4", "gpt-4.1"
    ClaudeSessionID string    `json:"claude_session_id,omitempty"` // KEPT: Claude-specific, used for --resume
    Agent           string    `json:"agent,omitempty"`
    Prompt          string    `json:"prompt"`
    Result          string    `json:"result,omitempty"`
    Status          string    `json:"status"`
    DurationMS      int       `json:"duration_ms"`
    CostUSD         float64   `json:"cost_usd"`
    InputTokens     int       `json:"input_tokens,omitempty"`      // NEW: token tracking
    OutputTokens    int       `json:"output_tokens,omitempty"`     // NEW: token tracking
    ToolCalls       int       `json:"tool_calls,omitempty"`        // NEW: how many tools were called
    UserID          string    `json:"user_id"`
    CreatedAt       time.Time `json:"created_at"`
}
```

### Session resumption is provider-specific

**Claude CLI**: pass `--resume <session_id>` — the CLI manages conversation state internally. This is the only provider with native resume. Keep `ClaudeSessionID` as-is.

**All other providers**: no built-in resume. Multi-turn requires replaying the full message history. This needs a separate message store:

```
data/
  sessions.json              # session metadata (existing)
  session_messages/          # NEW: message history per session
    ses-abc123.json          # [{role, content, tool_calls, tool_results}]
```

For non-Claude providers, "resuming" means:
1. Load messages from `session_messages/ses-xxx.json`
2. Append the new user message
3. Send the full array to the provider API
4. Append the assistant response
5. Save back to disk

**This is deferred to Phase 3/4.** Phases 1-2 are single-turn only for non-Claude providers. Claude keeps its native resume.

### Data migration

Existing `data/sessions.json` records have `claude_session_id` but no `provider` or `model`. On startup, backfill:
- If `claude_session_id` is set: `provider = "claude"`
- Otherwise: `provider = "claude"` (all existing sessions are Claude)
- `model`: leave empty for historical sessions (not available retroactively)

No field renames needed — `claude_session_id` stays as-is since it IS Claude-specific.

---

## Runner interface changes

### Add context.Context for cancellation

The current `Runner.Run()` signature has no cancellation mechanism. If a user closes the WS connection mid-stream, the AI process keeps running.

```go
// Before:
Run(cfg RunConfig, sessionID string, onEvent func(Event)) RunResult

// After:
Run(ctx context.Context, cfg RunConfig, sessionID string, onEvent func(Event)) RunResult
```

When the WS connection drops, cancel the context. The runner:
- Claude CLI: `cmd.Process.Kill()`
- Ollama/OpenAI/Anthropic: close the HTTP response body (stops streaming)

### Expand RunResult

```go
type RunResult struct {
    Text            string  `json:"text"`
    Provider        string  `json:"provider"`
    Model           string  `json:"model"`
    ClaudeSessionID string  `json:"claude_session_id,omitempty"` // Claude-specific
    DurationMS      int     `json:"duration_ms"`
    CostUSD         float64 `json:"cost_usd"`
    InputTokens     int     `json:"input_tokens"`
    OutputTokens    int     `json:"output_tokens"`
    ToolCalls       int     `json:"tool_calls"`
}
```

---

## Provider implementation plan

### Phase 1 -- Unify code paths

Split into sub-phases to manage dependencies:

**Phase 1a -- Data models (no behavioral changes)**

| Task | What changes |
|---|---|
| Add `context.Context` to `Runner.Run()` | Update interface + all existing runners |
| Expand `RunResult` | Add Provider, Model, ClaudeSessionID, token fields |
| Update `Session` model | Add Provider, Model, InputTokens, OutputTokens, ToolCalls |
| Add startup migration | Backfill `provider = "claude"` on existing sessions |

**Phase 1b -- Job system + WS refactoring (depends on 1a)**

| Task | What changes |
|---|---|
| `pkg/api/agents_jobs.go` | New: job submit, poll, cancel endpoints |
| `pkg/airunner/jobstore.go` | New: in-memory job store with event buffer, cleanup goroutine |
| Refactor `agents_ws.go` | Remove the `if provider == ProviderClaude` special-case. All providers through `runner.Run()`. |
| Update `ClaudeRunner.Run()` | Return `ClaudeSessionID` in `RunResult` instead of using a separate `claude.RunResult` type |
| Thread context through WS + jobs | Cancel on WS disconnect or `DELETE /api/agents/jobs/:id` |
| Replace `POST /api/agents/run/sync` | Deprecate in favor of job system (or keep as convenience alias that submits + polls internally) |
| Update `cmd/nube/openapi.yaml` | Add job endpoints, update CLI commands |

**Phase 1c -- CLI migration (depends on 1b)**

| Task | What changes |
|---|---|
| Rewrite `nube ask` as WS client | Connect to server, stream events to terminal |
| Add `--provider` and `--model` flags | Provider selection |
| Add `--direct` flag | Fallback to direct provider invocation without server |
| `nube agents submit` | Submit a job, return job ID |
| `nube agents poll <job-id>` | Poll a job (--once for single check, default: poll until done) |
| Server-down detection | Clear error message if server unreachable and no --direct |

**Phase 1d -- OpenAPI sync test (independent, parallel with anything)**

| Task | What changes |
|---|---|
| Write `TestOpenAPIMatchesRouter` | Validate spec matches router, catch drift |
| Run in CI | Fail build if spec and router diverge |

### Phase 2 -- Add Ollama

Use Ollama's **native `/api/chat` endpoint**, not the OpenAI-compat `/v1` endpoint. The native API gives better control, better error reporting, and avoids compat-layer quirks.

| Task | What changes |
|---|---|
| `pkg/airunner/ollama.go` | New runner: HTTP calls to `POST http://localhost:11434/api/chat` with streaming |
| Register in `registry.go` | `r.Register(&OllamaRunner{})` |
| `Available()` check | HTTP GET to `http://localhost:11434/api/tags` -- also returns installed model list |
| Config | `OLLAMA_HOST` env var (default `http://localhost:11434`) |
| Streaming | Ollama returns newline-delimited JSON, parse into `Event` structs |
| Token reporting | Map `eval_count` / `prompt_eval_count` to InputTokens/OutputTokens |

**Ollama runner sketch (~150 lines):**

```go
type OllamaRunner struct {
    Host string // default http://localhost:11434
}

func (r *OllamaRunner) Run(ctx context.Context, cfg RunConfig, sessionID string, onEvent func(Event)) RunResult {
    // POST /api/chat with {"model": cfg.Model, "messages": [...], "stream": true}
    // Read response line by line (context-aware, cancel closes body)
    // Parse each JSON line: {"message": {"content": "..."}, "done": false}
    // Emit Event{Type: "text", Content: chunk}
    // On final line (done: true): extract eval_count, prompt_eval_count
    // Emit Event{Type: "done", ...}
}
```

**No MCP tool calling in Phase 2** -- Ollama just does text chat. Tools come in Phase 4.

### Phase 3 -- Cloud AI providers

| Provider | API format | Go library |
|---|---|---|
| **OpenAI** | OpenAI API | `github.com/sashabaranov/go-openai` |
| **Anthropic** | Anthropic Messages API | `github.com/anthropics/anthropic-sdk-go` |
| **Gemini** | OpenAI-compatible | `github.com/sashabaranov/go-openai` (Gemini supports OpenAI format) |

**Strategy**: `OpenAICompatRunner` for OpenAI + Gemini (same API shape, different base URL). Separate `AnthropicRunner` for Anthropic (different API format). Keep `OllamaRunner` on native API (Phase 2) — don't merge it into the compat runner.

```
Runners after Phase 3:
  +-- ClaudeCliRunner        (shells out to claude binary)
  +-- OllamaRunner           (native /api/chat -- Phase 2)
  +-- OpenAICompatRunner     (OpenAI + Gemini via /v1/chat/completions)
  +-- AnthropicRunner        (Anthropic Messages API)
  +-- (codex, copilot -- existing, keep or deprecate)
```

**Known compat-layer issues to handle:**
- Streaming chunk format: Ollama/Gemini sometimes omit `finish_reason`, handle gracefully
- Error format: OpenAI returns `{error: {message, type, code}}`, Gemini returns Google-style errors -- normalize both
- Token usage: different field locations across providers -- map to `InputTokens`/`OutputTokens` in `RunResult`
- Model naming: user passes exact model string, no mapping layer needed

**Multi-turn for API providers:**
- Add message history storage (`data/session_messages/`)
- On resume: load previous messages, append new, send full array
- Token budget awareness: warn if message history approaches context window limit

### Phase 4 -- MCP bridge (agent loop with tool calling)

This is the big one. For non-Claude providers to use MCP tools, the server becomes the MCP client.

**Realistic estimate: 700-1000 lines**, broken down:

| File | Lines (est.) | What it does |
|---|---|---|
| `agentloop.go` | ~150 | Core loop: prompt -> LLM -> tool calls -> execute -> repeat |
| `toolconvert.go` | ~200 | Convert MCP tool schemas to OpenAI/Anthropic function-calling format |
| `toolexec.go` | ~100 | Bridge to existing JS runtime / OpenAPI executor |
| `messageformat.go` | ~150 | Provider-specific message formatting (tool results differ per provider) |
| Error handling, context mgmt | ~200 | Hallucinated tool names, timeouts, loops, malformed JSON |

```
User prompt
  -> Server loads user's installed tools (from MCPFactory)
  -> toolconvert: MCP schema -> provider's function-calling format
  -> Sends prompt + tool definitions to LLM API
  -> LLM responds with tool call request
  -> toolexec: execute via existing JS/OpenAPI runtime
  -> messageformat: serialize result in provider's expected format
  -> Feed result back to LLM
  -> Loop until LLM gives final text response or maxSteps hit
```

**What makes this hard (beyond the pseudocode):**

1. **Tool schema conversion** -- MCP tools are JSON Schema. OpenAI wants `parameters` in a specific format. Anthropic wants `input_schema`. Complex schemas (oneOf, $ref, nested objects from OpenAPI specs) are lossy to convert.

2. **Streaming within the loop** -- each `ChatWithTools` call is a streaming response. Must stream text tokens to the client while detecting tool call requests mid-stream. OpenAI tool calls arrive as deltas that need accumulating.

3. **Error recovery** -- LLM hallucinates a tool name (return error to LLM, let it retry). Tool call times out (configurable per-app, default 5s). LLM stuck in loop calling same tool (detect and break). Malformed tool call JSON from smaller models (parse best-effort or error).

4. **Context window management** -- each loop iteration adds messages. After 5-10 tool calls, smaller models (8k context) are exhausted. Need truncation or early termination.

**Safety controls:**

```go
type AgentLoop struct {
    Runner     Runner
    Tools      []ToolDefinition
    MaxSteps   int              // hard limit on tool calls
    MaxCost    float64          // token cost budget
    DryRun     bool             // log tool calls without executing
    OnEvent    func(Event)
    Ctx        context.Context  // cancellation
}
```

**Model capability for tool calling:**

| Model | Tool calling support | Quality | Recommendation |
|---|---|---|---|
| Claude (via CLI) | Native MCP -- already works | Excellent | Use Path 1 (no agent loop needed) |
| GPT-4.1 / GPT-4o | OpenAI function calling | Excellent | Full tool support |
| Gemma 4 (Ollama) | Ollama function calling | Good | Test thoroughly |
| Llama 3.x (Ollama) | Ollama function calling | Good | Test thoroughly |
| Small models (<7B) | Limited / unreliable | Poor | Text-only, disable tools |

**Prototype first**: get one OpenAI function call working end-to-end with one hardcoded tool before building the full framework. This validates the concept cheaply.

---

## Provider configuration

### Server-side config

Providers are configured via environment variables on the server:

```bash
# Claude Code CLI (Path 1 -- already works)
# Just needs `claude` binary in PATH

# Ollama (local, free)
OLLAMA_HOST=http://localhost:11434    # default

# OpenAI
OPENAI_API_KEY=sk-...

# Anthropic (server-side API, not CLI)
ANTHROPIC_API_KEY=sk-ant-...

# Gemini
GEMINI_API_KEY=AI...
```

### Provider discovery

`GET /api/agents/providers` already exists. Update it to return richer info:

```json
[
  {"provider": "claude",    "available": true,  "type": "cli",   "models": ["claude-sonnet-4-20250514"]},
  {"provider": "ollama",    "available": true,  "type": "api",   "models": ["gemma4", "llama3.1"]},
  {"provider": "openai",    "available": true,  "type": "api",   "models": ["gpt-4.1", "gpt-4o-mini"]},
  {"provider": "anthropic", "available": false, "type": "api",   "models": []},
  {"provider": "gemini",    "available": false, "type": "api",   "models": []}
]
```

For Ollama, `Available()` calls `GET /api/tags` to list installed models.
For API providers, `Available()` checks if the API key env var is set.

### Per-user API keys (deferred)

Currently all users share the server's API keys. Per-user keys (BYOK -- bring your own key) is a valid future feature but adds complexity:
- Where to store keys (not plaintext in JSON DB)
- Key validation on save
- Key rotation / expiry handling
- UI for key management

Defer to post-Phase 4. For now, the server admin configures keys via env vars.

---

## Frontend changes

The frontend chat component already sends `provider` in the WS request. Updates needed:

- Provider selector dropdown in chat UI (populated from `GET /api/agents/providers`)
- Model selector (populated from provider's model list)
- Session history shows provider + model badge
- Cost display adapts (Ollama shows $0.00, cloud shows actual cost)

---

## Operational concerns

### Rate limiting

- **Provider-side**: OpenAI has TPM/RPM limits. If hit mid-agent-loop, back off and retry once, then surface the error to the user. Ollama has no limits (local).
- **Server-side**: limit concurrent AI sessions per user (default: 3). Prevents one user from consuming all server resources. Implemented as a per-user semaphore in the WS handler.
- **Cost controls**: `MaxCost` per session in the agent loop. No per-user budget system for now -- defer to post-Phase 4.

### Concurrent session safety

`jsondb.Collection` uses a mutex for writes -- safe for concurrent access within one process. Multiple sessions writing to `sessions.json` simultaneously is fine. The bottleneck is AI provider concurrency, not DB writes.

### Model fallback

If a requested model isn't available (wrong name, no API access), return a clear error immediately:
```json
{"error": "model gpt-4.1 not available for provider openai", "available_models": ["gpt-4o", "gpt-4o-mini"]}
```
No silent fallback to a different model -- that would surprise the user.

---

## What NOT to do

- **Don't fork picoclaw or pi-mono** -- they solve different problems. Multi-provider LLM calling is ~300 lines of Go per provider. The app store + MCP server + JS sandbox is the hard part, and it's already built.
- **Don't force everything through WS** -- automation, CI, webhooks, and cron need the job system (REST poll). WS is for interactive use.
- **Don't pretend all providers are equal** -- Claude has native MCP + session resume. Others need the agent loop + message replay. Document the differences, don't hide them.
- **Don't build the agent loop before basic text chat works** -- get Ollama streaming text first (Phase 2), add tool calling later (Phase 4).
- **Don't merge Ollama into the OpenAI compat runner** -- keep it on the native `/api/chat` API for better control. Merge later only if it's genuinely identical in practice.

---

## Recommended prototype order

Before committing to the full phased plan, validate the two biggest unknowns:

**Day 1-2**: OllamaRunner with text streaming through the job system. Submit job, poll for events. This validates both the Runner interface for HTTP-based providers and the job polling pattern.

**Day 3-4**: One manual OpenAI function call. Hardcode a single tool, send to GPT-4.1, get tool call back, execute it, feed result back. This validates the Phase 4 concept before building the full framework.

**Day 5**: WS handler refactoring -- remove the Claude special-case, test that Claude still works through the unified path.

This order de-risks the two biggest unknowns (HTTP provider streaming and tool calling) before committing to architectural changes.

---

## Implementation order

```
Phase 1a -- Data models (parallel-safe, no behavioral changes)
  +-- Add context.Context to Runner.Run() interface
  +-- Expand RunResult (Provider, Model, tokens)
  +-- Update Session model (Provider, Model, tokens)
  +-- Backfill provider="claude" on existing sessions

Phase 1b -- Job system + WS refactoring (depends on 1a)
  +-- Job store (in-memory event buffer, cleanup goroutine)
  +-- POST /api/agents/jobs (submit), GET (poll with ?after=), DELETE (cancel)
  +-- Remove Claude special-case in agents_ws.go
  +-- All providers through runner.Run()
  +-- Thread context.Context for cancellation (WS disconnect + job cancel)
  +-- Deprecate POST /api/agents/run/sync
  +-- Update openapi.yaml with job endpoints
  +-- Test: submit job via REST, poll events, frontend WS still works

Phase 1c -- CLI migration (depends on 1b)
  +-- Rewrite nube ask as WS client (interactive default)
  +-- Add --provider, --model, --direct flags
  +-- nube agents submit (submit job, return ID)
  +-- nube agents poll <job-id> (poll until done, or --once)
  +-- --direct fallback for when server is down
  +-- Test: nube ask works, nube agents submit/poll works

Phase 1d -- OpenAPI sync test (independent)
  +-- Write TestOpenAPIMatchesRouter
  +-- Add to CI

Phase 2 -- Ollama (local, free, easy to test)
  +-- OllamaRunner on native /api/chat
  +-- Register in registry
  +-- Available() returns installed models
  +-- Test: nube ask --provider ollama --model gemma4 "hello"

Phase 3 -- Cloud APIs (can overlap with Phase 2)
  +-- OpenAICompatRunner (OpenAI + Gemini)
  +-- AnthropicRunner
  +-- Message history storage for multi-turn
  +-- Provider discovery returns model lists
  +-- Test: nube ask --provider openai --model gpt-4.1 "hello"

Phase 4 -- Agent loop with tool calling (~700-1000 lines)
  +-- agentloop.go: core loop
  +-- toolconvert.go: MCP schema -> provider function format
  +-- toolexec.go: bridge to existing JS/OpenAPI runtime
  +-- messageformat.go: provider-specific result serialization
  +-- Error handling: hallucinated tools, timeouts, loops, malformed JSON
  +-- Context window management for smaller models
  +-- Safety: maxSteps, maxCost, dryRun
  +-- Test: nube ask --provider ollama "check offline devices" -> calls tools
```

---

## Files that change

| File | Phase | Change |
|---|---|---|
| `pkg/airunner/runner.go` | 1a | Add context.Context to Run(), expand RunResult |
| `pkg/airunner/claude.go` | 1a | Accept context, return ClaudeSessionID in RunResult |
| `pkg/airunner/codex.go` | 1a | Accept context |
| `pkg/airunner/copilot.go` | 1a | Accept context |
| `pkg/models/session.go` | 1a | Add Provider, Model, InputTokens, OutputTokens, ToolCalls |
| `pkg/airunner/jobstore.go` | 1b | New: in-memory job store with event buffer + cleanup |
| `pkg/api/agents_jobs.go` | 1b | New: submit, poll, cancel job endpoints |
| `pkg/api/agents_ws.go` | 1b | Remove Claude special-case, thread context, cancel on disconnect |
| `pkg/api/agents_rest.go` | 1b | Deprecate sync endpoint or alias to job submit+poll |
| `cmd/nube/openapi.yaml` | 1b | Add job endpoints |
| `pkg/cli/cmd_ask.go` | 1c | Rewrite as WS client, add --provider/--model/--direct flags |
| `pkg/cli/cmd_agents.go` | 1c | New: submit and poll commands |
| `tests/openapi_sync_test.go` | 1d | New: validate spec matches router |
| `pkg/airunner/ollama.go` | 2 | New: OllamaRunner on native API |
| `pkg/airunner/registry.go` | 2-3 | Register new runners |
| `pkg/airunner/openai_compat.go` | 3 | New: OpenAI-compatible runner (OpenAI + Gemini) |
| `pkg/airunner/anthropic.go` | 3 | New: Anthropic API runner |
| `pkg/models/session_messages.go` | 3 | New: message history for multi-turn |
| `pkg/airunner/agentloop.go` | 4 | New: tool-calling loop |
| `pkg/airunner/toolconvert.go` | 4 | New: MCP schema -> provider format |
| `pkg/airunner/toolexec.go` | 4 | New: tool execution bridge |
| `pkg/airunner/messageformat.go` | 4 | New: provider-specific message formatting |
