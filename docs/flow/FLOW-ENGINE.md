# Flow Engine — Visual DAG Workflows

A graph-based workflow engine that replaces the linear stage pipeline with a full DAG (directed acyclic graph). Users design flows visually in a React Flow canvas — dragging nodes, connecting edges, configuring settings — and the engine executes them with parallel branches, conditional routing, loops, and approval gates.

**This builds on existing bizzy infrastructure.** The NATS event bus, plugin system, tool/prompt services, command bus, and adapters all stay unchanged. The flow engine is a new execution backend that emits the same bus events (`workflow.>`) and is invokable via the same command syntax (`run flow/my-flow`). The existing linear workflow runner continues to work for YAML-defined pipelines.

---

## Current status

### What's built and working

| Layer | Status | Files |
|---|---|---|
| **Data model** | Done | `pkg/flow/types.go`, `store.go` |
| **Node registry** | Done (23 built-in types) | `pkg/flow/registry.go` |
| **DAG validation** | Done (Kahn's algorithm) | `pkg/flow/validate.go` |
| **Execution engine** | Done (single-writer, parallel) | `pkg/flow/engine.go` |
| **Built-in nodes** | Done | `pkg/flow/nodes.go` |
| **Integration nodes** | Done | `pkg/flow/integrations.go` |
| **Event bus** | Done (FLOWS JetStream stream) | `pkg/flow/events.go` |
| **Expression eval** | Done (expr-lang — **to be replaced**) | `pkg/flow/expr.go` |
| **REST API** | Done (16 endpoints) | `pkg/api/flows_handler.go` |
| **Frontend editor** | Done (palette, canvas, toolbar, overlay) | `frontend/src/components/flow/` |
| **Frontend pages** | Done (list + editor) | `frontend/src/pages/flows.tsx`, `flow-editor.tsx` |
| **Tests** | 6 passing | `pkg/flow/engine_test.go` |

### What's next (see [Roadmap](#roadmap))

| Item | Why |
|---|---|
| Replace `expr-lang/expr` with `goja` | We already have a full JS runtime (`JSRuntime`) with HTTP, secrets, plugins, tools — adding a second expression language was a mistake |
| JSON Schema node settings | Current config panel is hand-coded per node type. Port the Rubix `JSONSchema` builder + `MultipleSettingsSchemas` pattern |
| Wire `AppRegistry` + `JSRuntime` into engine | Nodes that eval user code need full platform access, not a toy expression evaluator |
| Render settings with `@json-render/shadcn` | Auto-generated forms from schema, same approach as Rubix |

---

## Design influences

| Source | What we take |
|---|---|
| **Rubix runtime** (our existing system) | Node/port model, sequential processing per node, hot-reload, degraded nodes, port value caching, service injection via context, **JSON Schema settings + multiple schemas** |
| **GoFlow** | DAG definition with explicit nodes + edges, aggregator pattern for fan-in, `ConditionalBranch`, `ForEachBranch`, data as `[]byte` between nodes |
| **Floxy** | Fork/join with join strategies, human-decision nodes (approval gates), compensation/rollback on failure, step handler interface, DLQ for failed steps |
| **Bizzy existing** | NATS bus events, plugin tool proxy, command bus verbs, MCP tool serving, approval channels, **`JSRuntime` (goja) with full API surface** |

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│  React Flow Frontend                                                │
│                                                                     │
│  Node palette ──→ Canvas ──→ Save (POST /api/flows)                │
│  Live execution view ←── SSE /api/events/stream                    │
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
│                              │  Node Executors    │                 │
│                              ├── ToolExecutor     │─→ ToolService   │
│                              ├── ScriptExecutor   │─→ JSRuntime     │
│                              ├── PluginExecutor   │─→ NATS proxy    │
│                              ├── FlowControl      │─→ (internal)    │
│                              └── IntegrationExec  │─→ adapters      │
│                              └────────────────────┘                 │
│                                        │                            │
│                              ┌─────────┴─────────┐                 │
│                              │  Runtime Context   │                 │
│                              ├── AppRegistry      │                 │
│                              ├── JSRuntime (goja) │                 │
│                              ├── PluginRegistry   │                 │
│                              ├── ToolService      │                 │
│                              ├── AgentService     │                 │
│                              └── SecretStore      │                 │
│                              └────────────────────┘                 │
│                                        │                            │
│                                   bus.Publish()                     │
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

## Data model

### FlowDef — the saved flow definition

```go
type FlowDef struct {
    ID          string            `json:"id" gorm:"primaryKey"`       // "flow-" prefix
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
    ID       string         `json:"id"`                   // unique within flow
    Type     string         `json:"type"`                 // see node type registry
    Label    string         `json:"label,omitempty"`      // display name
    Position Position       `json:"position"`             // {x, y} for React Flow
    Data     map[string]any `json:"data,omitempty"`       // node-specific config (matches JSON Schema)
    Ports    *PortsDef      `json:"ports,omitempty"`      // override default ports
}
```

`Data` holds the node's configuration — the values that the settings form edits. The structure is defined by the node type's JSON Schema (see [Node settings](#node-settings)).

### FlowEdgeDef — a connection between nodes

```go
type FlowEdgeDef struct {
    ID           string `json:"id"`
    Source       string `json:"source"`                // source node ID
    SourceHandle string `json:"sourceHandle"`          // output port handle
    Target       string `json:"target"`                // target node ID
    TargetHandle string `json:"targetHandle"`          // input port handle
    Condition    string `json:"condition,omitempty"`   // for conditional edges: JS expression
    Label        string `json:"label,omitempty"`       // display label on edge
}
```

### FlowRun — a single execution

```go
type FlowRun struct {
    ID          string               `json:"id" gorm:"primaryKey"`     // "frun-" prefix
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
    ReplyTo     *ReplyInfo           `json:"reply_to,omitempty" gorm:"serializer:json"`
    CreatedAt   time.Time            `json:"created_at"`
    FinishedAt  *time.Time           `json:"finished_at,omitempty"`
}
```

`NodeStates` stores per-node execution state including **input and output values** — so you can inspect what data entered and left each node after a run completes:

```go
type NodeState struct {
    Status     NodeStatus `json:"status"`
    Input      any        `json:"input,omitempty"`      // what the node received (port values)
    Output     any        `json:"output,omitempty"`      // what the node produced
    Error      string     `json:"error,omitempty"`
    StartedAt  *time.Time `json:"started_at,omitempty"`
    FinishedAt *time.Time `json:"finished_at,omitempty"`
    DurationMS int        `json:"duration_ms,omitempty"`
    Retries    int        `json:"retries,omitempty"`
}
```

### Data flow between nodes

```
trigger outputs run.Inputs (map[string]any)
    │
    ▼ edge: sourceHandle="output" → targetHandle="input"
    │
    ▼ downstream node receives: inputs["input"] = the whole map
    │
    ▼ expressions access nested keys: input.value, input.name
```

- **Single-output nodes** (trigger, value, log, transform): any edge handle gets the full output
- **Multi-output nodes** (condition, switch, approval): only the active port's edge fires, inactive branches stay pending
- **Merge nodes**: wait for ALL connected input ports before firing
- **Race nodes**: fire when ANY input arrives

---

## Node types

Every node type is registered in a **node type registry** — a catalog of what's available to place on the canvas. 23 built-in types ship with the engine.

### 1. Built-in flow control nodes

| Type | Ports | Description |
|---|---|---|
| `trigger` | out: `output` | Entry point. Emits flow inputs. Every flow has exactly one. |
| `approval` | in: `input`, out: `approved`, `rejected` | Pauses execution, waits for user action. |
| `condition` | in: `input`, out: `true`, `false` | Evaluates expression, routes to matching branch. |
| `switch` | in: `input`, out: `case_*`, `default` | Multi-way branch. |
| `merge` | in: `input_1`..`input_N`, out: `output` | Fan-in join. Waits for ALL inputs. |
| `race` | in: `input_1`..`input_N`, out: `output`, `winner` | Fan-in first. Emits first arrival. |
| `foreach` | in: `items`, out: `item`, `done` | Iterates over array. Collects results into `done`. |
| `delay` | in: `input`, out: `output` | Waits for configured duration. |
| `output` | in: `input` | Terminal node. Flow result. |
| `error` | in: `input` | Terminal error. Fails the flow. |

### 2. Data nodes

| Type | Ports | Description |
|---|---|---|
| `value` | out: `output` | Emits a static JSON value. No inputs needed — great for testing. |
| `template` | in: `input`, out: `output` | Go `text/template` string interpolation. |
| `http-request` | in: `url`, `method`, `body`, `headers`, out: `output` | HTTP request, returns `{status, body}`. No external service needed. |
| `transform` | in: `input`, out: `output` | Expression evaluation to reshape data. |
| `set-variable` | in: `input`, out: `output` | Stores a value in the flow's variable map. |
| `log` | in: `input`, out: `output` | Logs input, passes through unchanged. |

### 3. Tool nodes (from apps)

Every tool from every installed app becomes a placeable node. Generated dynamically from `AppInstall` records.

| Type pattern | Ports | Source |
|---|---|---|
| `tool:<appName>.<toolName>` | in: one port per tool param, out: `result`, `error` | App JS/OpenAPI tools |
| `tool:plugin.<name>.<tool>` | in: per param, out: `result`, `error` | Plugin tools via NATS |

### 4. Integration nodes

| Type | Ports | Description |
|---|---|---|
| `ai-prompt` | in: `prompt`, `provider`, `model`; out: `result`, `error` | Single-turn AI prompt |
| `ai-runner` | in: `prompt`, `work_dir`; out: `result`, `error` | Full AI coding session via `pkg/airunner` |
| `slack-send` | in: `channel`, `message`, `thread_ts`; out: `result`, `error` | Send Slack message |
| `email-send` | in: `to`, `subject`, `body`; out: `result`, `error` | Send email via SMTP |
| `webhook-call` | in: `url`, `method`, `headers`, `body`; out: `response`, `error` | HTTP request |

### Node type registry

```go
type NodeTypeDef struct {
    Type        string    `json:"type"`
    Label       string    `json:"label"`
    Description string    `json:"description,omitempty"`
    Category    string    `json:"category"`      // "flow-control", "tool", "integration", "data"
    Icon        string    `json:"icon,omitempty"`
    Source      string    `json:"source"`         // "builtin", "app", "plugin"
    Ports       PortsDef  `json:"ports"`          // default ports
    Settings    any       `json:"settings,omitempty"` // JSON Schema for node config panel
}
```

Built-in types are registered at startup. App/plugin tool types are rebuilt when apps are installed/uninstalled or plugins register/deregister.

---

## Node settings (JSON Schema)

> **Status: designed, not yet implemented.** Current config panel is hand-coded. This section describes the target architecture.

Every node type declares its settings as a **JSON Schema**. The frontend renders the schema into a form using `@json-render/shadcn` (same pattern as Rubix). The user's settings are stored in `FlowNodeDef.Data`.

### Why JSON Schema

- **One source of truth** — schema defines the form, validation, and defaults
- **Already proven** — Rubix uses this exact pattern for 50+ node types
- **No hand-coded config panels** — add a new node type, its settings form appears automatically
- **Frontend validation** — JSON Schema validates before submission
- **Multiple schemas** — complex nodes can show different settings based on use case

### Schema structure

Each node type can provide a settings schema via the registry:

```go
type NodeTypeDef struct {
    // ...existing fields...
    Settings    *JSONSchema              `json:"settings,omitempty"`
    Schemas     *MultipleSettingsSchemas `json:"schemas,omitempty"`
}
```

### JSONSchema type

Ported from Rubix `internal/services/flowruntime/v2/shared/schema.go`:

```go
type JSONSchema struct {
    Title       string                  `json:"title,omitempty"`
    Description string                  `json:"description,omitempty"`
    Type        string                  `json:"type"`
    Properties  map[string]JSONSchema   `json:"properties,omitempty"`
    Required    []string                `json:"required,omitempty"`
    Default     any                     `json:"default,omitempty"`
    Enum        []any                   `json:"enum,omitempty"`
    Minimum     *float64                `json:"minimum,omitempty"`
    Maximum     *float64                `json:"maximum,omitempty"`
    MinLength   *int                    `json:"minLength,omitempty"`
    MaxLength   *int                    `json:"maxLength,omitempty"`
    Pattern     string                  `json:"pattern,omitempty"`
    Format      string                  `json:"format,omitempty"`
    Items       *JSONSchema             `json:"items,omitempty"`
    ReadOnly    bool                    `json:"readOnly,omitempty"`
    UIWidget    string                  `json:"ui:widget,omitempty"`
    UIHidden    bool                    `json:"ui:hidden,omitempty"`

    // Conditional (if/then/else)
    If   *JSONSchema `json:"if,omitempty"`
    Then *JSONSchema `json:"then,omitempty"`
    Else *JSONSchema `json:"else,omitempty"`
}
```

With a fluent builder:

```go
// Example: condition node settings schema
schema.Object().
    Title("Condition Settings").
    Property("expression", schema.String().
        Title("Expression").
        Description("JavaScript expression that returns true/false").
        UIWidget("code").
        MinLength(1).
        Build()).
    Property("on_error", schema.String().
        Title("On Error").
        Default("stop").
        Enum("stop", "skip", "retry", "fallback").
        Build()).
    Required("expression").
    Build()
```

### Multiple settings schemas

For nodes with distinct use cases (e.g. `ai-runner` can be configured for Claude, OpenCode, or Codex — each needs different fields):

```go
type MultipleSettingsSchemas struct {
    SupportsMultiple bool                  `json:"supportsMultiple"`
    Schemas          []NamedSettingsSchema `json:"schemas"`
    DefaultSchema    string               `json:"defaultSchema"`
}

type NamedSettingsSchema struct {
    Name        string     `json:"name"`
    DisplayName string     `json:"displayName"`
    Description string     `json:"description"`
    Schema      JSONSchema `json:"schema"`
}
```

Frontend flow:
1. User drops node on canvas
2. If `schemas.supportsMultiple`, show selection dialog first
3. Load the selected schema
4. Render form from schema
5. User fills in settings → stored in `node.Data`

### Example: HTTP Request node schema

```go
func httpRequestSchema() JSONSchema {
    return schema.Object().
        Title("HTTP Request").
        Property("url", schema.String().
            Title("URL").
            URL().
            Default("https://").
            Build()).
        Property("method", schema.String().
            Title("Method").
            Default("GET").
            Enum("GET", "POST", "PUT", "PATCH", "DELETE").
            Build()).
        Property("headers", schema.Object().
            Title("Headers").
            Description("Key-value pairs").
            Build()).
        Property("timeout", schema.Integer().
            Title("Timeout (seconds)").
            Default(int64(30)).
            Range(1, 300).
            Build()).
        Property("on_error", schema.String().
            Title("On Error").
            Default("stop").
            Enum("stop", "skip", "retry", "fallback").
            Build()).
        Required("url").
        Build()
}
```

---

## Scripting: goja, not expr

> **Status: designed, not yet implemented.** Current code uses `expr-lang/expr`. This section describes the migration to `goja`.

### Why replace expr with goja

`expr-lang/expr` is a sandboxed expression evaluator. It can do `input > 5` and `len(items) > 0`. That's it. Meanwhile, we already have:

- **`goja`** — Full ES5+ JavaScript engine, already a dependency
- **`JSRuntime`** — Rich runtime built on goja with HTTP, secrets, config, plugins, tool calling, files, crypto, base64, URL, env
- **App tools** — Users already write JavaScript for their app tools

Adding a second language (`expr`) for flow conditions means:
- Users learn two languages
- Condition nodes can't call tools or make HTTP requests
- Transform nodes can't use any platform APIs
- The flow engine is weaker than a simple app tool

### Target: unified JS execution

Every node that evaluates user code uses `JSRuntime` (goja):

| Node | Current (expr) | Target (goja) |
|---|---|---|
| `condition` | `input > 5` | `function handle(params) { return params.input > 5 }` or shorthand: `params.input > 5` |
| `switch` | `status` | `function handle(params) { return params.status }` |
| `transform` | `input * 2` | `function handle(params) { return params.input * 2 }` or full: `function handle(params) { return http.get(...) }` |
| Edge condition | `value > 0` | Same — JS expression |

### Shorthand expressions

For simple cases, we auto-wrap. If the user's code doesn't contain `function handle`, wrap it:

```go
// User writes: params.input > 5
// Engine wraps: function handle(params) { return params.input > 5 }
```

This keeps simple conditions easy while allowing full JS when needed.

### What JSRuntime gives flow nodes

Everything that app tools already have:

```js
// HTTP
let resp = http.get("https://api.example.com/data")
let data = http.post("https://...", { body: JSON.stringify(payload) })

// Secrets (from app install)
let apiKey = secrets.get("api_key")

// Config (from app settings)
let baseUrl = config.get("base_url")

// Plugins
if (plugins.exists("ml-service")) {
    let result = plugins.call("ml-service", "analyze", { text: params.input })
}

// Other tools in the same app
let nodes = tools.call("query_nodes", { filter: "type=sensor" })

// Files (read/write within app directory)
let template = files.read("templates/report.md")

// Crypto, base64, URL parsing
let hash = crypto.sha256(params.input)
let encoded = base64.encode(params.input)
```

### Runtime context per node

When a flow node needs to execute JS, the engine creates a `JSRuntime` scoped to the user:

```go
func (e *Engine) createNodeRuntime(userID string) *apps.JSRuntime {
    // Get user's app installs for secrets/config
    // Wire in plugin query, tool caller
    // Return configured JSRuntime
}
```

This means a condition node in a flow can do things like:

```js
// Check if a sensor value is above threshold
function handle(params) {
    let threshold = config.get("alert_threshold")
    return params.input.value > parseFloat(threshold)
}
```

Or a transform node can enrich data:

```js
function handle(params) {
    let weather = http.get(`https://api.weather.com/current?city=${params.input.city}`)
    return {
        ...params.input,
        temperature: weather.body.temp,
        conditions: weather.body.description
    }
}
```

---

## Execution engine

### Design principles

The engine follows a **single-writer** model: one main goroutine owns all run state (`FlowRun`, `NodeStates`, DB persistence). Worker goroutines execute nodes and report results back via a channel. This avoids race conditions on shared state and ensures DB writes never overlap.

The engine is also **resumable**: when a flow pauses (approval gate, server restart), the run is fully persisted. Resuming loads the run from DB and re-enters the main loop.

### DAG resolution

When a flow run starts (or resumes), the engine:

1. **Validates** the graph — no cycles, all required ports connected, all node types registered
2. **Topologically sorts** nodes to determine execution order
3. **Identifies independent groups** — nodes with no dependency can run in parallel
4. **Initializes `NodeState`** for every node as `pending`

### Execution loop

```go
func (e *Engine) execute(ctx context.Context, run *FlowRun, def *FlowDef) {
    ctx, cancel := context.WithCancel(ctx)
    defer cancel()

    results := make(chan nodeResult, len(def.Nodes))
    inflight := 0

    // Seed the trigger node
    e.markReady(run, triggerNodeID)
    inflight += e.fireReadyNodes(ctx, run, def, results)

    for inflight > 0 {
        select {
        case <-ctx.Done():
            run.Status = FlowRunCancelled
            return

        case res := <-results:
            inflight--

            if res.Error != nil {
                cont := e.handleNodeError(run, def, res)
                if !cont { cancel(); return }
            } else {
                // Store output + input in NodeState
                // Propagate outputs to downstream nodes (only active ports)
                // Fire newly-ready nodes
            }

            // Check for terminal completion or approval pause
        }
    }
}
```

### Output propagation — active ports only

After a node completes, the engine routes its output to downstream nodes. Multi-output nodes (condition, switch, approval) only fire the **active** port — inactive branches stay pending:

```go
func resolvePortValue(output any, sourceHandle string) (value any, active bool) {
    switch o := output.(type) {
    case PortOutput:
        if o.Port == sourceHandle {
            return o.Value, true   // this port fired
        }
        return nil, false          // wrong port — skip this edge
    default:
        return output, true        // single-output: all edges fire
    }
}
```

This prevents both branches of a condition from executing.

### Error handling

Each node can configure error behaviour via `data.on_error`:

| Strategy | Behaviour |
|---|---|
| `stop` (default) | Node failed, flow failed, remaining nodes cancelled |
| `skip` | Node marked skipped, downstream gets nil, flow continues |
| `retry` | Retry up to `data.max_retries` times, then fall through to `stop` |
| `fallback` | Route to `error` output port. Downstream error path handles recovery. |

### Approval gates

Approval nodes **do not block a goroutine**. The node returns `ErrApprovalRequired`, the main loop sets `FlowRunWaitingApproval`, persists to DB, and exits cleanly. The approval API reloads the run and re-enters the execution loop. Flows survive server restarts.

### Resource limits

| Limit | Default |
|---|---|
| Max parallel nodes per run | 50 |
| Max foreach iterations | 1000 (hard cap 10000) |
| Max foreach concurrency | 10 |
| Max flow run duration | 1 hour |
| Max node execution time | 5 minutes |
| Max concurrent runs per user | 10 |

---

## REST API

### Flow definitions (CRUD)

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/flows` | Create flow |
| `GET` | `/api/flows` | List flows (user-scoped) |
| `GET` | `/api/flows/:id` | Get flow |
| `PUT` | `/api/flows/:id` | Update flow (increments version) |
| `DELETE` | `/api/flows/:id` | Delete flow |
| `POST` | `/api/flows/:id/duplicate` | Clone flow |

### Flow execution

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/flows/:id/run` | Start a flow run |
| `GET` | `/api/flows/:id/runs` | List runs for a flow |
| `GET` | `/api/flow-runs/:runId` | Get run status with all node states (includes input/output per node) |
| `POST` | `/api/flow-runs/:runId/approve/:nodeId` | Approve an approval gate |
| `POST` | `/api/flow-runs/:runId/reject/:nodeId` | Reject an approval gate |
| `POST` | `/api/flow-runs/:runId/cancel` | Cancel a running flow |

### Node type catalog

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/flows/node-types` | Full catalog (grouped by category) |
| `GET` | `/api/flows/node-types/:type` | Detail for one type (ports, settings schema) |

### Validation

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/flows/validate` | Validate without saving |

---

## Event bus topics

```
flow.>
    flow.started
    flow.node.started
    flow.node.completed
    flow.node.failed
    flow.node.skipped
    flow.node.progress              — streaming from long-running nodes (ai-runner)
    flow.waiting_approval
    flow.approved
    flow.rejected
    flow.completed
    flow.failed
    flow.cancelled
```

---

## AI Runner node

The `ai-runner` node runs a full AI coding session using `pkg/airunner`. Unlike `ai-prompt` (single-turn), `ai-runner` starts an agentic session that can edit files, run commands, use MCP tools, and create commits.

### Node definition

| Field | Type | Description |
|---|---|---|
| `provider` | config | `claude`, `opencode`, `codex`, `copilot`, `ollama` |
| `model` | config | Model override — optional |
| `work_dir` | input or config | Working directory (the repo) |
| `prompt` | input | The coding task |
| `resume_session` | config | Resume previous session for this node+run |
| `thinking_budget` | config | `"low"`, `"medium"`, `"high"` |
| `allowed_tools` | config | MCP tool filter pattern |
| `timeout_mins` | config | Per-session timeout, default 30 |

**Result output:**

```json
{
  "text": "Fixed the login timeout...",
  "provider": "opencode",
  "model": "claude-sonnet-4-20250514",
  "session_id": "sess_abc123",
  "cost_usd": 0.042,
  "duration_ms": 45200,
  "input_tokens": 12400,
  "output_tokens": 3200,
  "tool_calls": 8
}
```

---

## File layout

```
pkg/flow/
    engine.go          — DAG executor (749 lines)
    engine_test.go     — 6 tests: value flow, condition branching, passthrough,
                         expression eval, template, error handling
    store.go           — FlowDef + FlowRun CRUD (SQLite via GORM)
    types.go           — FlowDef, FlowNodeDef, FlowEdgeDef, FlowRun, NodeState
    registry.go        — NodeTypeRegistry: 23 built-in types + dynamic tool/plugin types
    validate.go        — Graph validation (Kahn's algorithm, port checks, foreach subgraphs)
    nodes.go           — Built-in node executors (condition, switch, merge, foreach, delay,
                         value, template, http-request, transform, set-variable, log)
    integrations.go    — Integration executors (slack-send, email-send, webhook-call,
                         ai-prompt, ai-runner)
    events.go          — FlowEvent struct, topic constants, bus publishing
    expr.go            — Expression eval (expr-lang — TO BE REPLACED with goja)
    settings/          — JSON Schema builder + MultipleSettingsSchemas (TO BE BUILT)

pkg/api/
    flows_handler.go   — REST endpoints for flow CRUD + execution + node types

frontend/src/
    pages/
        flows.tsx              — Flow list page (create, duplicate, delete)
        flow-editor.tsx        — Full-screen editor (palette + canvas + config + overlay)
    hooks/
        use-flows.ts           — React Query hooks (CRUD, run, approve, reject, validate)
    components/
        flow/
            canvas.tsx         — React Flow canvas (drag-drop, auto-layout, port connections)
            node-palette.tsx   — Draggable node sidebar grouped by category
            node-config.tsx    — Right panel: node settings (HAND-CODED — to be replaced
                                 with JSON Schema forms)
            custom-nodes/
                base-node.tsx  — Unified node renderer (category colors, ports, exec state)
            execution-overlay.tsx  — Live run state with expandable input/output per node,
                                     inline approve/reject buttons
            flow-toolbar.tsx   — Save, validate, run, auto-layout buttons
```

---

## Roadmap

### Phase 1: JSON Schema settings (`pkg/flow/settings/`)

Port from Rubix:
- `JSONSchema` struct with fluent builder (`String()`, `Integer()`, `Object()`, `.Enum()`, `.Range()`, etc.)
- `MultipleSettingsSchemas` + `NamedSettingsSchema` types
- Each built-in node type declares its schema
- Serve schemas via `GET /api/flows/node-types/:type`
- Frontend renders with `@json-render/shadcn` instead of hand-coded inputs

### Phase 2: Wire app ecosystem into engine

- Add `AppRegistry`, `PluginRegistry`, `JSRuntime` factory to `Engine` struct
- Tool nodes get full `JSRuntime` context (secrets, config, plugin access)
- Node type registry dynamically registers tools from installed apps
- `ToolService.CallTool` used for `tool:` nodes (already done)

### Phase 3: Replace expr with goja

- Condition/switch/transform nodes use `JSRuntime.ExecuteScript()`
- Auto-wrap simple expressions: `input > 5` → `function handle(params) { return params.input > 5 }`
- Edge conditions use same mechanism
- Remove `expr-lang/expr` dependency from `go.mod`
- Delete `expr.go`

### Phase 4: Command bus + triggers

- `run flow/<name>` via command parser
- Cron triggers → auto-register with cron adapter
- Webhook triggers → register path under `/hooks/flow/<name>`
- Event triggers → NATS subscription

### Phase 5: Coding workflow

- `ai-runner` node wired to `pkg/airunner.Runner` (structure exists, needs testing)
- `coding-tasks` app — task CRUD + select/list-selected tools
- Slack progress streaming — `flow.node.progress` → Slack thread
- Example flow: daily cron → list tasks → foreach → ai-runner → approve/reject

---

## Dependencies

| Concern | Library | Notes |
|---|---|---|
| React Flow | `@xyflow/react` | Canvas, nodes, edges, minimap, controls |
| Auto-layout | `@dagrejs/dagre` | Automatic node positioning |
| JSON Schema forms | `@json-render/shadcn` | Node config panels (same as Rubix) |
| JS runtime | `goja` (already a dep) | Condition/transform/switch expressions, tool scripts |
| DAG validation | Built-in (Kahn's algorithm) | Cycle detection, topological sort |

**No new Go dependencies needed.** `goja` is already in `go.mod`. `expr-lang/expr` will be removed.

---

## Tests

6 passing tests validate the core engine:

| Test | Flow | What it proves |
|---|---|---|
| `TestSimpleValueToOutput` | trigger → value → output | Static value node, output stored on run |
| `TestConditionBranching` | trigger → condition → true/false | Only active branch fires (false stays pending) |
| `TestLogPassthrough` | trigger → log → output | Data flows through passthrough unchanged |
| `TestTransformNode` | trigger → transform(`input.value * 2`) → output | Expression eval: 21 * 2 = 42 |
| `TestTemplateNode` | trigger → template → output | Go template interpolation |
| `TestErrorHandlingSkip` | trigger → bad-transform(on_error=skip) → output | Skip strategy: flow completes despite error |

Run: `go test -v ./pkg/flow/...`
