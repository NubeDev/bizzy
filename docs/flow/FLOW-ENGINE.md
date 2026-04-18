# Flow Engine — Visual DAG Workflows

A graph-based workflow engine that executes visual DAG (directed acyclic graph) flows. Users design flows in a React Flow canvas — dragging nodes, connecting edges, configuring settings — and the engine executes them with parallel branches, conditional routing, loops, and approval gates.

**This builds on existing bizzy infrastructure.** The NATS event bus, plugin system, tool/prompt services, command bus, and adapters all stay unchanged. The flow engine is a self-contained execution backend that emits bus events (`flow.>`) and is invokable via REST API or auto-triggered via cron/interval schedules.

---

## Current status

### What's built and working

| Layer | Status | Files |
|---|---|---|
| **Data model** | Done | `pkg/flow/types.go`, `store.go` |
| **Node registry** | Done (22 built-in types) | `pkg/flow/registry.go` |
| **DAG validation** | Done (Kahn's algorithm) | `pkg/flow/validate.go` |
| **Execution engine** | Done (single-writer, parallel) | `pkg/flow/engine.go` |
| **Node executor framework** | Done (pluggable `NodeExecutor` interface) | `pkg/flow/executor.go` |
| **Built-in node executors** | Done | `pkg/flow/nodes.go` |
| **Integration node executors** | Done | `pkg/flow/integrations.go` |
| **Runtime lifecycle** | Done (Deploy/Undeploy/DeployAll) | `pkg/flow/runtime.go` |
| **Trigger handlers** | Done (cron, interval) | `pkg/flow/triggers.go` |
| **Event bus** | Done (FLOWS JetStream stream) | `pkg/flow/events.go` |
| **JS expression eval** | Done (goja via `JSRuntime.EvalExpression`) | `pkg/flow/expr.go`, `pkg/apps/jsruntime.go` |
| **JSON Schema settings** | Done (fluent builder + 22 schemas) | `pkg/flow/settings/` |
| **REST API** | Done (16 endpoints) | `pkg/api/flows_handler.go` |
| **Frontend editor** | Done (palette, canvas, config, live polling) | `frontend/src/components/flow/` |
| **Frontend pages** | Done (list + editor with polling) | `frontend/src/pages/flows.tsx`, `flow-editor.tsx` |
| **Service interfaces** | Done (canonical `ToolCaller` + `PromptRunner` in `pkg/services/`) | `pkg/services/interfaces.go` |
| **Tests** | 6 passing | `pkg/flow/engine_test.go` |

### What's next

| Item | Priority | Why |
|---|---|---|
| Wire `JSRuntimeFactory` into engine | High | Flow expression nodes (condition, transform) get user-scoped JS runtimes with secrets, config, plugins — full platform access |
| Dynamic tool registration from apps | High | Installed app tools auto-appear as draggable flow nodes in the palette |
| Frontend: JSON Schema form rendering | Medium | Replace hand-coded node config panels with auto-generated forms from schema — new node types get config UI for free |
| Frontend: trigger settings panel | Medium | Trigger node config (type, schedule) shows in right panel — partially done, needs the JSON Schema renderer |
| Command bus integration | Medium | `run flow/my-flow` from Slack/CLI/webhooks |
| Webhook trigger handler | Low | Register HTTP routes for webhook-triggered flows |
| Event trigger handler | Low | Subscribe to NATS topics for event-triggered flows |

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│  React Flow Frontend                                                │
│                                                                     │
│  Node palette ──→ Canvas ──→ Save (POST /api/flows)                │
│  Live polling  ←── GET /api/flow-runs/:id (configurable interval)  │
│  Node settings ←── JSON Schema (GET /api/flows/node-types/:type)   │
└──────────────────────────────┬──────────────────────────────────────┘
                               │ REST API
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│  nube-server                                                        │
│                                                                     │
│  Flow Store (SQLite)         Flow Engine (DAG executor)             │
│    ├── FlowDef (nodes,       ├── Topological sort                  │
│    │   edges, metadata)      ├── Parallel execution (fan-out)      │
│    └── FlowRun (state,       ├── Join/sync (fan-in)                │
│        node results)         ├── Conditional routing                │
│                              ├── ForEach iteration                  │
│                              ├── Approval gates                    │
│                              └── Error handling (stop/skip/retry)  │
│                                        │                            │
│                              ┌─────────┴─────────┐                 │
│                              │  Executor Registry │                 │
│                              │  (NodeExecutor)    │                 │
│                              ├── Built-in (22)    │                 │
│                              ├── tool:* (dynamic) │                 │
│                              └── Custom (register)│                 │
│                              └────────────────────┘                 │
│                                        │                            │
│                              ┌─────────┴─────────┐                 │
│                              │  Services          │                 │
│                              ├── ToolCaller       │─→ ToolService   │
│                              ├── PromptRunner     │─→ AgentService  │
│                              ├── JSRuntimeFactory │─→ goja VM       │
│                              ├── Agents           │─→ airunner      │
│                              └── Bus              │─→ NATS          │
│                              └────────────────────┘                 │
│                                        │                            │
│                              ┌─────────┴─────────┐                 │
│                              │  Runtime Lifecycle │                 │
│                              ├── DeployAll (boot) │                 │
│                              ├── Deploy (on save) │                 │
│                              ├── Undeploy (delete)│                 │
│                              ├── Shutdown (exit)  │                 │
│                              └────────────────────┘                 │
│                                        │                            │
│                              ┌─────────┴─────────┐                 │
│                              │  Trigger Handlers  │                 │
│                              ├── CronTrigger      │                 │
│                              ├── IntervalTrigger  │                 │
│                              ├── (webhook — TODO) │                 │
│                              └── (event — TODO)   │                 │
│                              └────────────────────┘                 │
│                                        │                            │
│                              ┌─────────┴─────────┐                 │
│                              │    NATS Event Bus  │                 │
│                              │  flow.started      │                 │
│                              │  flow.node.started  │                │
│                              │  flow.node.completed│                │
│                              │  flow.completed     │                │
│                              └────────────────────┘                 │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Key design decisions

### NodeExecutor pattern (no god object)

The engine does NOT hold references to every service (Slack, email, AI, etc). Instead:

- **`NodeExecutor` interface** — each node type registers its own executor function
- **`ExecutorRegistry`** — maps node type → executor, with prefix matching for `tool:*`
- **`Services` struct** — bundles external deps; executors take only what they need via `ExecContext`
- **Adding a new node type = one function + one `Register` call. Zero changes to Engine.**

```go
type NodeExecutor interface {
    Execute(ctx context.Context, exec *ExecContext) (any, error)
}

type ExecContext struct {
    Run      *FlowRun
    Node     *FlowNodeDef
    Def      *FlowDef
    Inputs   map[string]any
    Services *Services
    Engine   *Engine
}
```

### Trigger config lives on the node

The trigger node's `Data` map IS the trigger configuration. No separate `FlowDef.Trigger` struct to sync. The runtime reads it via `def.TriggerConfig()` which returns the trigger node's `Data` directly. Adding new trigger fields = just add to the node's JSON Schema. No backend code changes needed.

### JS expressions via goja (not expr-lang)

All expression evaluation uses `JSRuntime.EvalExpression()` — the same goja engine that app tools use. This means flow expressions have access to the full platform API (HTTP, crypto, base64, etc) and will get secrets/config/plugins when `JSRuntimeFactory` is wired in.

Two modes:
- **Simple**: `input.value > 5` — env keys injected as globals, expression evaluated directly
- **Full function**: `function handle(params) { return http.get(...).body.status === "ok" }`

### Runtime lifecycle (PLC-style)

Flows are "deployed" — their triggers run persistently like a PLC program:

```
Server boots → DeployAll() → loads all flows from DB → starts triggers
Flow saved   → Deploy()    → trigger starts immediately
Flow deleted → Undeploy()  → trigger stops
Server exits → Shutdown()  → all triggers stop cleanly
```

A flow with `trigger.type = "interval"` and `trigger.schedule = "10s"` runs every 10 seconds forever. No API calls needed after the initial save. Survives server reboots.

### Canonical service interfaces

`ToolCaller` and `PromptRunner` are defined once in `pkg/services/interfaces.go`. Both `flow` and `command` packages import from there. `ToolService` directly satisfies `ToolCaller` — no adapter shims needed.

---

## Data model

### FlowDef — the saved flow definition

```go
type FlowDef struct {
    ID          string            `json:"id" gorm:"primaryKey"`
    Name        string            `json:"name" gorm:"uniqueIndex"`
    Description string            `json:"description"`
    Version     int               `json:"version"`
    Nodes       []FlowNodeDef     `json:"nodes" gorm:"serializer:json"`
    Edges       []FlowEdgeDef     `json:"edges" gorm:"serializer:json"`
    Inputs      []FlowInputDef    `json:"inputs,omitempty" gorm:"serializer:json"`
    Trigger     *TriggerDef       `json:"trigger,omitempty" gorm:"serializer:json"`
    Settings    map[string]any    `json:"settings,omitempty" gorm:"serializer:json"`
    UserID      string            `json:"user_id" gorm:"index"`
    CreatedAt   time.Time         `json:"created_at"`
    UpdatedAt   time.Time         `json:"updated_at"`
}
```

### FlowNodeDef — a node on the canvas

```go
type FlowNodeDef struct {
    ID       string         `json:"id"`
    Type     string         `json:"type"`
    Label    string         `json:"label,omitempty"`
    Position Position       `json:"position"`
    Data     map[string]any `json:"data,omitempty"`  // node config (matches JSON Schema)
    Ports    *PortsDef      `json:"ports,omitempty"`
}
```

`Data` holds everything — node settings, trigger config, counter config, etc. The structure is defined by each node type's JSON Schema.

### FlowRun — a single execution

```go
type FlowRun struct {
    ID          string               `json:"id" gorm:"primaryKey"`
    FlowID      string               `json:"flow_id" gorm:"index"`
    FlowVersion int                  `json:"flow_version"`
    FlowName    string               `json:"flow_name"`
    Status      FlowRunStatus        `json:"status" gorm:"index"`
    Inputs      map[string]any       `json:"inputs,omitempty" gorm:"serializer:json"`
    Output      map[string]any       `json:"output,omitempty" gorm:"serializer:json"`
    NodeStates  map[string]NodeState `json:"node_states" gorm:"serializer:json"`
    Variables   map[string]any       `json:"variables,omitempty" gorm:"serializer:json"`
    Error       string               `json:"error,omitempty"`
    UserID      string               `json:"user_id" gorm:"index"`
    CreatedAt   time.Time            `json:"created_at"`
    FinishedAt  *time.Time           `json:"finished_at,omitempty"`
}
```

`NodeStates` stores per-node input/output so you can inspect what data entered and left each node. `Variables` holds flow-level state (counters, set-variable values).

---

## Node types

22 built-in types ship with the engine.

### Flow control nodes

| Type | Ports | Description |
|---|---|---|
| `trigger` | out: `output` | Entry point. Emits flow inputs. Config: `type` (manual/cron/interval), `schedule`. |
| `approval` | in: `input`, out: `approved`, `rejected` | Pauses execution, waits for user action. |
| `condition` | in: `input`, out: `true`, `false` | Evaluates JS expression, routes to matching branch. |
| `switch` | in: `input`, out: `case_*`, `default` | Multi-way branch. |
| `merge` | in: `input_1`..`input_N`, out: `output` | Fan-in join. Waits for ALL inputs. |
| `race` | in: `input_1`..`input_N`, out: `output`, `winner` | Fan-in first. Emits first arrival. |
| `foreach` | in: `items`, out: `item`, `done` | Iterates over array with configurable concurrency. |
| `delay` | in: `input`, out: `output` | Waits for configured duration. |
| `output` | in: `input` | Terminal node. Flow result. |
| `error` | in: `input` | Terminal error. Fails the flow. |

### Data nodes

| Type | Ports | Description |
|---|---|---|
| `value` | out: `output` | Emits a static JSON value. |
| `template` | in: `input`, out: `output` | Go `text/template` string interpolation. |
| `http-request` | in: `url`, `method`, `body`, `headers`, out: `output` | HTTP request, returns `{status, body}`. |
| `transform` | in: `input`, out: `output` | JS expression to reshape data. |
| `set-variable` | in: `input`, out: `output` | Stores a value in the flow's variable map. |
| `counter` | in: `input`, out: `output` | Increments/decrements/resets a counter in flow variables. |
| `log` | in: `input`, out: `output` | Logs input, passes through unchanged. |

### Integration nodes

| Type | Ports | Description |
|---|---|---|
| `ai-prompt` | in: `prompt`, `provider`, `model`; out: `result`, `error` | Single-turn AI prompt. |
| `ai-runner` | in: `prompt`, `work_dir`; out: `result`, `error` | Full AI coding session via `pkg/airunner`. |
| `slack-send` | in: `channel`, `message`, `thread_ts`; out: `result`, `error` | Send Slack message. |
| `email-send` | in: `to`, `subject`, `body`; out: `result`, `error` | Send email via SMTP. |
| `webhook-call` | in: `url`, `method`, `headers`, `body`; out: `response`, `error` | HTTP request. |

### Dynamic tool nodes

Every tool from every installed app becomes a placeable node (registered via `tool:appName.toolName` prefix). Not yet wired — see roadmap.

---

## Execution engine

### Single-writer model

One main goroutine owns all run state (`FlowRun`, `NodeStates`, DB persistence). Worker goroutines execute nodes and report results back via a channel. No race conditions on shared state.

### Execution loop

```
1. Validate graph (cycles, ports, types)
2. Seed trigger node as "ready"
3. Fire all ready nodes in parallel (up to MaxParallelNodes)
4. Wait for results on channel
5. On result: update state, propagate outputs, fire newly-ready nodes
6. Repeat until: terminal node completes OR no inflight nodes remain
```

### Error handling per node

| Strategy | Behaviour |
|---|---|
| `stop` (default) | Node failed → flow failed → remaining nodes cancelled |
| `skip` | Node marked skipped, downstream gets nil, flow continues |
| `retry` | Retry up to `max_retries` times, then fall through to `stop` |
| `fallback` | Route to `error` output port for recovery path |

### Approval gates

Approval nodes return `ErrApprovalRequired`. The main loop pauses the run, persists to DB, and exits cleanly. The approval API reloads and re-enters the execution loop. Flows survive server restarts.

---

## REST API

### Flow definitions (CRUD)

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/flows` | Create flow (auto-deploys trigger) |
| `GET` | `/api/flows` | List flows (user-scoped) |
| `GET` | `/api/flows/:id` | Get flow |
| `PUT` | `/api/flows/:id` | Update flow (re-deploys trigger) |
| `DELETE` | `/api/flows/:id` | Delete flow (undeploys trigger) |
| `POST` | `/api/flows/:id/duplicate` | Clone flow |

### Flow execution

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/flows/:id/run` | Start a flow run (manual trigger) |
| `GET` | `/api/flows/:id/runs` | List runs for a flow |
| `GET` | `/api/flow-runs/:runId` | Get run status with all node states |
| `POST` | `/api/flow-runs/:runId/approve/:nodeId` | Approve an approval gate |
| `POST` | `/api/flow-runs/:runId/reject/:nodeId` | Reject an approval gate |
| `POST` | `/api/flow-runs/:runId/cancel` | Cancel a running flow |

### Node type catalog

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/flows/node-types` | Full catalog (grouped by category, includes JSON Schema settings) |
| `GET` | `/api/flows/node-types/:type` | Detail for one type |

### Validation

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/flows/validate` | Validate without saving |

---

## Trigger system

### How it works

The trigger node's `Data` map contains the trigger configuration. When a flow is saved, the engine reads `TriggerConfig()` and starts the appropriate handler.

| Trigger type | Schedule format | Example | Behaviour |
|---|---|---|---|
| `manual` | — | — | Only runs via `POST /api/flows/:id/run` |
| `interval` | Go duration | `10s`, `5m`, `1h` | Repeating timer, fires `StartRun()` each tick |
| `cron` | 5-field cron | `0 9 * * 1-5` | Minute-granularity matching, fires on match |
| `webhook` | — | — | (TODO) Register HTTP route |
| `event` | NATS topic | `sensor.>` | (TODO) Subscribe to topic |

### Lifecycle

```
Save flow with interval trigger "10s"
  → Deploy() reads trigger node Data
  → Creates IntervalTrigger handler
  → Handler starts goroutine with 10s ticker
  → Each tick calls StartRun() → flow executes → results stored

Server reboots
  → DeployAll() loads all flows from DB
  → Re-deploys each one → triggers resume

Delete flow
  → Undeploy() stops handler → ticker goroutine exits
```

### Adding a new trigger type

1. Implement `TriggerHandler` interface (`Start` + `Stop`)
2. Register factory via `engine.RegisterTrigger("mytype", factory)`
3. Add to trigger node JSON Schema enum

---

## File layout

```
pkg/flow/
    engine.go          — Engine struct, StartRun, execute loop, dispatch
    engine_test.go     — 6 tests
    executor.go        — NodeExecutor interface, ExecContext, Services, ExecutorRegistry
    scheduling.go      — Node readiness, firing, input gathering
    propagation.go     — Output routing, port resolution, error handling
    lifecycle.go       — Approval API, cancel, recover, subgraph execution
    runtime.go         — Deploy/Undeploy/DeployAll/Shutdown lifecycle
    triggers.go        — CronTrigger, IntervalTrigger, cron matching
    nodes.go           — 17 built-in node executors + helpers
    integrations.go    — 5 integration executors + tool: prefix handler
    types.go           — FlowDef, FlowNodeDef, FlowEdgeDef, FlowRun, NodeState
    registry.go        — NodeTypeRegistry: 22 built-in types
    validate.go        — Graph validation (Kahn's algorithm, port checks)
    store.go           — FlowDef + FlowRun CRUD (SQLite via GORM)
    events.go          — FlowEvent struct, topic constants, bus publishing
    expr.go            — JS expression eval via JSRuntime.EvalExpression
    settings/
        schema.go      — JSONSchema type + fluent builder
        multi.go       — MultipleSettingsSchemas for complex nodes
        builtins.go    — JSON Schema for all 22 built-in node types

pkg/services/
    interfaces.go      — Canonical ToolCaller + PromptRunner interfaces
    prompt_runner.go   — AgentPromptRunner (bridges AgentService → PromptRunner)
    tools.go           — ToolService (directly satisfies ToolCaller)

pkg/apps/
    jsruntime.go       — EvalExpression() + NewFlowRuntime() for flow engine

pkg/api/
    flows_handler.go   — REST endpoints for flow CRUD + execution

frontend/src/
    pages/
        flows.tsx              — Flow list page
        flow-editor.tsx        — Full-screen editor with live polling
    hooks/
        use-flows.ts           — React Query hooks (CRUD, run, poll, latest run)
    components/
        flow/
            canvas.tsx         — React Flow canvas
            node-palette.tsx   — Draggable node sidebar
            node-config.tsx    — Right panel: node settings (trigger, counter, etc)
            custom-nodes/
                base-node.tsx  — Unified node renderer
            execution-overlay.tsx  — Live run state
            flow-toolbar.tsx   — Save, validate, polling interval dropdown
```

---

## Roadmap

### Phase 1: Wire JSRuntime into engine (next)

- Add `JSRuntimeFactory` to `Services` — creates user-scoped `JSRuntime` with secrets, config, plugins
- Wire factory in `main.go` using `ToolService.ResolveTool` pattern
- Condition/switch/transform nodes automatically get full platform access
- Flow expressions can call `secrets.get()`, `http.get()`, `plugins.call()`, `tools.call()`

### Phase 2: Dynamic tool registration

- On app install/uninstall, register/deregister tool node types in the flow registry
- Each app tool becomes a draggable node with auto-generated ports from tool params
- Tool nodes execute via `ToolService.CallTool` (already wired)

### Phase 3: Frontend — JSON Schema form rendering

- Replace hand-coded `node-config.tsx` with a generic JSON Schema renderer
- Use `@json-render/shadcn` (already installed) or build a lightweight renderer
- New node types get config panels for free — no frontend code changes
- Trigger settings, counter settings, etc. all render from schema

### Phase 4: Command bus integration

- `run flow/<name>` via command parser
- Register flow names as command targets
- Flows invokable from Slack, CLI, webhooks via existing command bus

### Phase 5: Additional triggers

- Webhook trigger: register `POST /hooks/flow/<name>` route, fire on request
- Event trigger: subscribe to NATS topic pattern, fire on matching event
- Both use the existing `TriggerHandler` interface — just new implementations

### Phase 6: AI coding workflow

- `ai-runner` node wired to `pkg/airunner.Runner`
- Example flow: cron trigger → list tasks → foreach → ai-runner → approve/reject
- Slack progress streaming via `flow.node.progress` events

---

## Dependencies

| Concern | Library | Notes |
|---|---|---|
| React Flow | `@xyflow/react` | Canvas, nodes, edges, minimap, controls |
| Auto-layout | `@dagrejs/dagre` | Automatic node positioning |
| JSON Schema forms | `@json-render/shadcn` | Node config panels (to be wired) |
| JS runtime | `goja` | Condition/transform/switch expressions, platform APIs |
| DAG validation | Built-in (Kahn's algorithm) | Cycle detection, topological sort |

**No new Go dependencies needed.** `goja` is already in `go.mod`. `expr-lang/expr` has been removed.

---

## Tests

6 passing tests validate the core engine:

| Test | Flow | What it proves |
|---|---|---|
| `TestSimpleValueToOutput` | trigger → value → output | Static value node, output stored on run |
| `TestConditionBranching` | trigger → condition → true/false | Only active branch fires (JS expression via goja) |
| `TestLogPassthrough` | trigger → log → output | Data flows through passthrough unchanged |
| `TestTransformNode` | trigger → transform(`input.value * 2`) → output | JS expression eval: 21 * 2 = 42 |
| `TestTemplateNode` | trigger → template → output | Go template interpolation |
| `TestErrorHandlingSkip` | trigger → bad-transform(on_error=skip) → output | Skip strategy: flow completes despite error |

Run: `go test -v ./pkg/flow/...`
