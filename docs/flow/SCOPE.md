# Flow Engine — Scope & Long-Term Roadmap

This document captures the full state of the flow engine, known issues, and the phased plan to get to a production-quality system. Each phase has clear deliverables and acceptance criteria.

---

## Current state (as of April 2026)

### What's built and working

| Area | Status | Details |
|---|---|---|
| **Execution engine** | Complete | Single-writer DAG executor with parallel fan-out, fan-in (merge/race), conditional routing, foreach loops, approval gates, cancel, recover |
| **Node types** | 23 built-in | 10 flow-control, 8 data (including function node), 5 integration |
| **Node-RED msg convention** | Complete | All nodes emit/consume `{payload, topic, _msgid}`. Settings-as-defaults, msg-properties-override pattern. `resolveFromMsg` helper. Backward-compatible expression env (`input` alias for `payload`) |
| **Function node** | Complete | JS code with `msg`, `node.log/warn/error`, `flow.get/set`, `tools.call`. Full platform APIs via JSRuntime |
| **Triggers** | 4 types | manual, cron, interval, webhook, event — all implemented and registered |
| **Dynamic tool nodes** | Complete | App tools auto-register as `tool:appName.toolName` with single-input port and auto-generated JSON Schema config |
| **JSRuntimeFactory** | Wired | User-scoped runtimes with secrets, config, plugins for expression eval |
| **Deploy/undeploy lifecycle** | Complete | DeployAll on boot, Deploy on save, Undeploy on delete, Shutdown on exit |
| **NATS events** | 12 topics | `flow.started` through `flow.cancelled`, persisted in FLOWS JetStream stream (7-day retention) |
| **REST API** | 16 endpoints | Full CRUD, run, cancel, approve/reject, validate, duplicate, node-type catalog, webhook trigger |
| **Command bus integration** | Complete | `run flow/<name>` from Slack, CLI, webhooks via command router |
| **Frontend editor** | Complete | Palette, canvas, drag-and-drop, auto-layout, schema-driven config panel, execution overlay with approval buttons, live polling |
| **JSON Schema config** | Complete | All 23 node types have schemas. SchemaForm renders them automatically. New node types get config UI for free |
| **Tests** | 9 passing | Value, condition, log, transform, template, error-skip, function, function-flow-state, function-passthrough |

### What's broken or incomplete

| Issue | Severity | Detail |
|---|---|---|
| **Approval resume launches duplicate execute goroutine** | Critical | `ApproveNode`/`RejectNode` call `go e.execute()` which starts a second concurrent execution goroutine for the same run. The engine claims single-writer semantics but has no guard preventing two loops from mutating `run.NodeStates` (a plain Go map) simultaneously. This causes panics or silent data corruption under real approval workflows |
| **Foreach subgraph has concurrent map writes** | Critical | `executeForeach` spawns concurrent goroutines that call `executeSubgraph`, which writes to `run.NodeStates` without synchronization. Multiple foreach iterations racing on the same map will panic (`concurrent map writes`) or corrupt the run record |
| **Approval resume loses delivered inputs** | High | When `ApproveNode` calls `go e.execute()`, the new execute loop creates a fresh `deliveredInputs` map. Fan-in nodes (merge/race) downstream of an approval gate lose the inputs accumulated by the pre-approval loop and may never fire |
| **No flow definition snapshot for in-progress runs** | High | Approval-paused runs load the flow definition fresh from the store on resume. If the flow was edited while paused, the run resumes with a different graph than what was executing. No versioning or snapshot strategy exists |
| **Cron parser only handles `*` and bare integers** | High | Ranges (`1-5`), lists (`1,3,5`), and steps (`*/5`) silently never match. The schema example `"0 9 * * 1-5"` (weekdays) doesn't work. Users will create cron schedules that never fire with no error |
| **3 events defined but never published** | Medium | `flow.node.skipped`, `flow.approved`, `flow.rejected` — downstream listeners (SSE, WebSocket, notifications) will never receive these |
| **AI prompt/runner schema-executor mismatch** | Medium | `AIRunnerSchema` has `timeout_mins` field the executor ignores. `AIPromptSchema` has `provider`/`model` fields that `PromptRunner.RunPrompt(ctx, userID, prompt)` can't accept |
| **Flow state is in-memory only** | Medium | `flow.get/set` values (counters, function node state) are lost on server restart. No persist-to-DB option |
| **HTTP client per invocation** | Low | `executeHTTPRequest` and `executeWebhookCallNode` create a new `http.Client` each call — no connection pooling. Will exhaust file descriptors under high-frequency flows |
| **Foreach subgraph execution order** | Low | Uses Go map iteration (random order) for subgraph node sequence. Could execute nodes before their upstream dependencies in the subgraph |
| **No retry backoff** | Low | `on_error=retry` re-queues immediately with no delay. Transient errors (rate limits, network) will burn through retries instantly |
| **Validate mutates FlowDef** | Low | `validateForEachSubgraphs` writes `_subgraph_nodes` into `node.Data` as a side effect. Hidden coupling between validation and runtime |
| **SSE default topics wrong** | Low | `events_sse.go` defaults to `workflow.>` which doesn't exist — should be `flow.>` |
| **Deploy errors not surfaced** | Low | `createFlow`/`updateFlow` API handlers ignore the error from `engine.Deploy()`. Bad trigger config (e.g. invalid cron) saves but silently fails to deploy |
| **Webhook path not sanitized** | Low | Flow names with spaces/special chars become malformed webhook paths |

### Test coverage gaps

Untested node types: approval, merge, race, foreach, delay, switch, http-request, counter, set-variable, error, all 5 integration nodes, all tool nodes.

Untested features: all trigger types, `on_error=retry`, `on_error=fallback`, concurrent run limits, cycle detection, port validation, cancel, approve/reject lifecycle.

### Doc staleness

`FLOW-ENGINE.md` says 22 node types (should be 23), 6 tests (should be 9), and marks webhook/event triggers, JSRuntimeFactory, and JSON Schema rendering as TODO — all are done. Integration node port definitions show multi-port inputs that are now single-input.

---

## Phase 1: Fix bugs and harden (immediate)

Goal: fix the broken things before building new things. Items 1.1–1.3 are crash-level concurrency bugs and must land first.

### 1.1 Run-level mutex for NodeStates

Add a `sync.RWMutex` to `FlowRun` that protects `NodeStates` and `Variables`. All reads/writes to these maps go through accessor methods that hold the lock. The main execute loop, approval resume, foreach subgraph iterations, and persist all use these accessors.

**Files:** `pkg/flow/engine.go`, `pkg/flow/lifecycle.go`, `pkg/flow/propagation.go`, `pkg/flow/nodes.go`

### 1.2 Approval resume — eliminate duplicate execute goroutine

Replace the current pattern (`ApproveNode` calls `go e.execute()`) with a resume channel. The original `execute` loop blocks on a `resumeCh` when the run enters `WaitingApproval` status. `ApproveNode`/`RejectNode` write the approval result to the channel, and the **same** goroutine continues. This preserves single-writer semantics and keeps `deliveredInputs` intact.

**Files:** `pkg/flow/engine.go`, `pkg/flow/lifecycle.go`

### 1.3 Foreach subgraph state isolation

`executeSubgraph` must not write directly to `run.NodeStates` from concurrent goroutines. Instead, each iteration builds a local `map[string]NodeState` for its subgraph nodes. After `wg.Wait()`, the parent goroutine merges all iteration states into `run.NodeStates` under the run mutex. This eliminates the concurrent map write panic.

**Files:** `pkg/flow/lifecycle.go`, `pkg/flow/nodes.go`

### 1.4 Shared HTTP client

Create a package-level `var httpClient = &http.Client{...}` with a connection-pooling transport. Use it in `executeHTTPRequest` and `executeWebhookCallNode`.

**Files:** `pkg/flow/nodes.go`, `pkg/flow/integrations.go`

### 1.5 Cron parser

Replace the `matchesCron`/`fieldMatches` functions in `triggers.go` with a proper cron field parser that handles:
- Ranges: `1-5`
- Lists: `1,3,5`
- Steps: `*/5`, `1-30/2`
- Combinations: `1-5,10,15-20`

No external dependency needed — this is ~60 lines of parsing. Add tests for each pattern.

**Files:** `pkg/flow/triggers.go`, new `pkg/flow/triggers_test.go`

### 1.6 Publish missing events

Add `eventEmitter` calls for the 3 unpublished events:
- `nodeSkipped` — in `handleNodeError` skip branch (propagation.go)
- `flowApproved` — in `ApproveNode` (lifecycle.go)
- `flowRejected` — in `RejectNode` (lifecycle.go)

**Files:** `pkg/flow/propagation.go`, `pkg/flow/lifecycle.go`

### 1.7 Fix AI schema-executor gaps

- `executeAIRunnerNode`: read `timeout_mins` from node data (or rename schema field to match what the executor reads)
- `executeAIPrompt`: either add provider/model to `PromptRunner` interface or remove those fields from the schema. Removing from schema is the simpler path since `PromptRunner` is a shared interface

**Files:** `pkg/flow/integrations.go`, `pkg/flow/settings/builtins.go`, possibly `pkg/services/interfaces.go`

### 1.8 Surface deploy errors

Return deploy errors from `createFlow`/`updateFlow` API handlers. If deploy fails, respond with a warning (flow is saved but trigger is not active).

**Files:** `pkg/api/flows_handler.go`

### 1.9 Fix SSE default topics

Change `workflow.>` to `flow.>` in `events_sse.go`.

**Files:** `pkg/api/events_sse.go`

### 1.10 Update docs

Rewrite `FLOW-ENGINE.md` to reflect current state: 23 node types, 9 tests, all triggers implemented, single-input port model, Node-RED msg convention, function node.

**Files:** `docs/flow/FLOW-ENGINE.md`

---

## Phase 2: Real-time events via WebSocket (next)

Goal: replace polling with a single unified WebSocket for all real-time events.

### 2.1 Unified event WebSocket

New endpoint: `WS /api/events/ws?token=<auth>`

Protocol:
```
Client → Server (subscribe):
  {"subscribe": "flow.>", "flow_id": "flow-abc123"}
  {"subscribe": "job.>"}
  {"subscribe": "command.>"}

Client → Server (unsubscribe):
  {"unsubscribe": "flow.>"}

Server → Client (events):
  {"topic": "flow.node.completed", "data": {"run_id":"...","node_id":"counter-1","output":{...}}}
  {"topic": "flow.completed", "data": {"run_id":"...","status":"completed"}}

Client → Server (commands — future):
  {"action": "flow.run", "flow_id": "flow-abc123", "inputs": {...}}
  {"action": "flow.cancel", "run_id": "frun-xyz"}
  {"action": "flow.approve", "run_id": "frun-xyz", "node_id": "approval-1"}
```

Implementation:
- Server creates ephemeral NATS subscriptions per client subscription request
- Filters events by authenticated `user_id` + optional scope (flow_id, job_id)
- Fan-out to all matching WS clients
- Unsubscribes NATS on WS close
- Uses gorilla/websocket (already a dependency)

**Files:** new `pkg/api/events_ws.go`, `pkg/api/router.go`

### 2.2 Frontend event hook

New `useEventWS(topics, filter?)` hook that:
- Opens a single WS connection (shared via React context or singleton)
- Sends subscribe messages for requested topics
- Returns a stream of typed events
- Auto-reconnects with exponential backoff
- Unsubscribes on unmount

**Files:** new `frontend/src/hooks/use-event-ws.ts`

### 2.3 Replace flow polling

- `useLatestFlowRun` switches from polling to WS subscription on `flow.>` scoped to the flow ID
- Remove poll interval dropdown from `FlowToolbar`
- `ExecutionOverlay` receives live node state updates
- Canvas node status rings update in real-time

**Files:** `frontend/src/hooks/use-flows.ts`, `frontend/src/components/flow/flow-toolbar.tsx`, `frontend/src/pages/flow-editor.tsx`

### 2.4 Deprecate SSE

Mark `/api/events/stream` (SSE) as deprecated. Keep it working for backward compat but add a deprecation header. New consumers use the WS endpoint.

**Files:** `pkg/api/events_sse.go`

---

## Phase 3: Test coverage and reliability

Goal: confidence to refactor and ship.

### 3.1 Node type tests

One test per node type that exercises its core behavior through the engine (not unit-testing the executor in isolation — test the full flow: trigger → node → output).

Priority order:
1. foreach (most complex — subgraph execution, concurrency)
2. merge + race (fan-in behavior)
3. switch (multi-way routing)
4. counter (persistent state)
5. delay (context cancellation)
6. http-request (mock HTTP server)
7. approval (approve/reject lifecycle)
8. error node (explicit failure)

### 3.2 Error strategy tests

- `on_error=retry` with max_retries=2 — verify 2 retries then fail
- `on_error=fallback` — verify error port receives the error string
- `on_error=stop` (implicit) — verify flow fails

### 3.3 Trigger tests

- Cron: mock time, verify `matchesCron` for all field patterns
- Interval: verify ticker fires and calls StartRun
- Webhook: verify `HandleWebhook` dispatches correctly
- Event: verify NATS subscription and event parsing

### 3.4 Validation tests

- Cycle detection (graph with a loop → error)
- Missing trigger node → error
- Unconnected required port → error
- Unknown node type → error (except `tool:` prefix)

**Files:** new `pkg/flow/triggers_test.go`, expand `pkg/flow/engine_test.go`

---

## Phase 4: Persistent flow state

Goal: `flow.get/set` values survive server restarts.

### 4.1 SQLite-backed flow state

Add a `flow_state` table: `(flow_id, key, value_json, updated_at)`. Replace the in-memory `deployment.state` map with DB reads/writes. Use a write-through cache (in-memory map backed by DB) so hot-path reads don't hit SQLite.

### 4.2 State TTL and cleanup

Add configurable TTL per flow state key. Stale state is cleaned up on a background ticker. Prevents unbounded growth from flows that set many keys.

**Files:** `pkg/flow/runtime.go`, `pkg/flow/store.go`, `pkg/database/database.go` (migration)

---

## Phase 5: Frontend polish

Goal: the flow editor feels professional.

### 5.1 Conditional schema rendering

SchemaForm supports `if/then/else` from the JSON Schema. The trigger node's config shows `schedule` only when `type=cron|interval`, `webhook_path` only when `type=webhook`, `event` only when `type=event`.

**Files:** `frontend/src/components/flow/schema-form.tsx`, `pkg/flow/settings/builtins.go` (add if/then/else to TriggerSchema)

### 5.2 Code editor widget

Replace the plain `<textarea>` for `ui:widget=code` with a lightweight code editor (Monaco or CodeMirror). Syntax highlighting for JS expressions in condition, transform, switch, and function nodes.

**Files:** `frontend/src/components/flow/schema-form.tsx`, `package.json`

### 5.3 Run button in editor

Add a "Run" button to `FlowToolbar` that triggers `POST /api/flows/:id/run` with optional inputs dialog. See the result live via the WS connection (Phase 2).

**Files:** `frontend/src/components/flow/flow-toolbar.tsx`

### 5.4 Validation highlighting

After validation, highlight invalid nodes on the canvas (red border, tooltip with error). Show unconnected required ports.

**Files:** `frontend/src/components/flow/canvas.tsx`, `frontend/src/components/flow/custom-nodes/base-node.tsx`

### 5.5 Run history panel

Slide-out panel showing recent runs for the current flow. Click a run to see its node states in the execution overlay.

**Files:** new `frontend/src/components/flow/run-history.tsx`, `frontend/src/pages/flow-editor.tsx`

---

## Phase 6: Engine improvements

Goal: handle real-world workloads.

### 6.1 Retry with backoff

`on_error=retry` supports exponential backoff. New schema fields: `retry_delay` (initial delay, default 1s), `retry_backoff` (multiplier, default 2). The retry goroutine sleeps before re-queuing.

**Files:** `pkg/flow/propagation.go`, `pkg/flow/settings/builtins.go`

### 6.2 Node timeout enforcement

Per-node timeouts already exist in `dispatch()` but are only applied if `timeout > 0` in node data. Ensure all integration/tool nodes have sensible default timeouts in their schemas (not 0 = unlimited).

**Files:** `pkg/flow/settings/builtins.go`

### 6.3 Subgraph execution ordering

Fix `executeSubgraph` to respect topological order within the subgraph, not random map iteration order. Use the existing `TopologicalSort` function.

**Files:** `pkg/flow/lifecycle.go`

### 6.4 Validate tool existence

When validating a flow with `tool:appName.toolName` nodes, check that the tool exists in the app registry. Warn (not error) if it doesn't — the tool might be installed later.

**Files:** `pkg/flow/validate.go`

### 6.5 Run cleanup

Add `DELETE /api/flow-runs/:runId` and a configurable auto-cleanup (delete runs older than N days). Without this, runs accumulate forever.

**Files:** `pkg/flow/store.go`, `pkg/api/flows_handler.go`, `pkg/api/router.go`

---

## Phase 7: AI coding workflow

Goal: the end-to-end workflow from the original vision.

This is the capstone: `cron → list tasks → foreach → ai-runner → approve/reject`.

Prerequisites: Phases 1-3 (bugs fixed, tests pass, real-time events work).

### 7.1 Example flow template

A pre-built flow template that demonstrates:
1. Cron trigger (daily at 9am)
2. Function node that lists open tasks (from an API or tool)
3. ForEach iterates tasks
4. AI-runner processes each task
5. Approval gate for human review
6. Slack notification on completion

### 7.2 Flow templates API

`GET /api/flows/templates` — returns pre-built flow templates. `POST /api/flows/from-template/:name` — creates a flow from a template with user-configurable variables.

### 7.3 Progress streaming

Pipe `flow.node.progress` events (from ai-runner) through the WS connection so the frontend shows live AI session output within the flow editor.

---

## Architecture principles

These apply to all phases:

1. **One wire, one msg.** Every node has a single `input` port (except merge/race/foreach). Settings are defaults, msg properties override. This is the Node-RED convention and it's non-negotiable.

2. **No hacks in the engine.** The engine doesn't know about specific node types. `ExecutorRegistry` dispatches to pluggable executors. Adding a node type is one function + one `Register` call. Zero changes to the engine.

3. **Schema-driven config.** Every node type has a JSON Schema. The frontend renders it automatically. No hand-coded config panels. Adding a field to a node = adding a line to its schema builder.

4. **Events everywhere.** Every state change publishes to NATS. The WS connection delivers them to the frontend. No polling. No special-case REST endpoints for state updates.

5. **Test through the engine.** Tests create flows, run them, and check the result. No unit-testing executors in isolation — that misses integration bugs. The `testEngine` + `waitRun` pattern makes this easy.

6. **Msg flows through.** Passthrough nodes preserve the msg (including `_msgid`, `topic`, custom properties). Transform nodes set a new payload but preserve everything else via `MsgSet`. This enables tracing and debugging.
