package bootstrap

var promptFlowEngine = Prompt{
	Name:        "flow_engine",
	Description: "Flow engine reference — visual DAG workflows, Node-RED msg convention, node types, triggers, REST API, and how to build flows",
	Body: `# Flow Engine — Visual DAG Workflows

The flow engine executes visual DAG (directed acyclic graph) workflows. Users design flows in a React Flow canvas — dragging nodes, connecting edges, configuring settings — and the engine runs them with parallel branches, conditional routing, loops, and approval gates.

## Core concepts

**Flows** are graphs of nodes connected by edges. Each node does one thing (call a tool, make an HTTP request, evaluate a condition). Edges carry data between nodes.

**The msg convention** (copied from Node-RED): every wire carries a msg object:

` + "```" + `json
{
  "payload": { "temperature": 22, "unit": "C" },
  "topic": "sensor/living-room",
  "_msgid": "a1b2c3d4e5f6"
}
` + "```" + `

- ` + "`" + `msg.payload` + "`" + ` — the main data. Nodes read from and write to this.
- ` + "`" + `msg.topic` + "`" + ` — optional label/category for routing.
- ` + "`" + `msg._msgid` + "`" + ` — unique trace ID, auto-generated.
- Other properties — nodes can add custom keys (e.g. ` + "`" + `msg.statusCode` + "`" + `, ` + "`" + `msg.headers` + "`" + `). They flow through unless explicitly removed.

**Settings override pattern**: every node has a config panel (JSON Schema driven). Settings are defaults. Properties on the incoming msg override them. For example, an HTTP Request node configured with ` + "`" + `url: "https://api.example.com"` + "`" + ` can be overridden by sending ` + "`" + `msg.url = "https://other.com"` + "`" + `.

**Single input port**: all nodes (except merge/race/foreach) have one ` + "`" + `input` + "`" + ` port. Data arrives as a msg. Multiple parameters are just keys in the msg or the payload — not separate ports. This keeps the graph clean.

## Node types (24 built-in)

### Flow control

| Type | Description | Ports |
|---|---|---|
| ` + "`" + `trigger` + "`" + ` | Entry point. Config: type (manual/cron/interval/webhook/event), schedule | out: output |
| ` + "`" + `condition` + "`" + ` | JS expression → routes to true or false | in: input, out: true/false |
| ` + "`" + `switch` + "`" + ` | JS expression → routes to case_<value> or default | in: input, out: case_*/default |
| ` + "`" + `merge` + "`" + ` | Fan-in: waits for ALL connected inputs | in: input_1..N, out: output |
| ` + "`" + `race` + "`" + ` | Fan-in: emits the first input that arrives | in: input_1..N, out: output |
| ` + "`" + `foreach` + "`" + ` | Iterates an array with configurable concurrency | in: items, out: item/done |
| ` + "`" + `delay` + "`" + ` | Waits for a duration, passes msg through | in: input, out: output |
| ` + "`" + `approval` + "`" + ` | Pauses execution, waits for user approve/reject | in: input, out: approved/rejected |
| ` + "`" + `output` + "`" + ` | Terminal node. Extracts payload as the flow result | in: input |
| ` + "`" + `error` + "`" + ` | Terminal error. Fails the flow with payload as error message | in: input |

### Data

| Type | Description |
|---|---|
| ` + "`" + `function` + "`" + ` | Write JavaScript. Full access to msg, node.log(), flow.get/set(), tools.call() |
| ` + "`" + `debug` + "`" + ` | Passthrough that captures msg for the debug panel. Config: label, output (payload/full), active |
| ` + "`" + `value` + "`" + ` | Emits a static JSON value |
| ` + "`" + `transform` + "`" + ` | JS expression to reshape payload (result becomes new payload) |
| ` + "`" + `template` + "`" + ` | Go text/template interpolation. Payload fields are top-level: ` + "`" + `{{.name}}` + "`" + ` |
| ` + "`" + `http-request` + "`" + ` | HTTP request. msg.url/method/headers override settings. msg.payload = body. Output: payload=response, statusCode, headers |
| ` + "`" + `set-variable` + "`" + ` | Stores payload in a named flow variable. Passthrough |
| ` + "`" + `counter` + "`" + ` | Increment/decrement/reset a persistent counter (survives across runs) |
| ` + "`" + `log` + "`" + ` | Logs payload to stdout. Passthrough |

### Integration

| Type | Description |
|---|---|
| ` + "`" + `ai-prompt` + "`" + ` | Single-turn AI prompt. msg.payload or msg.prompt = prompt text |
| ` + "`" + `ai-runner` + "`" + ` | Full AI coding session via airunner. msg.prompt, msg.work_dir, msg.provider |
| ` + "`" + `slack-send` + "`" + ` | Send Slack message. msg.payload = message text, msg.channel |
| ` + "`" + `email-send` + "`" + ` | Send email. msg.payload = body, msg.to, msg.subject |
| ` + "`" + `webhook-call` + "`" + ` | HTTP request (same as http-request, kept for backward compat) |

### Dynamic tool nodes

Every tool from every installed app becomes a placeable node: ` + "`" + `tool:appName.toolName` + "`" + `. These have a single ` + "`" + `input` + "`" + ` port and ` + "`" + `result` + "`" + `/` + "`" + `error` + "`" + ` outputs. Settings panel is auto-generated from the tool's parameters.

Tool param resolution: node settings (config panel) are defaults → msg-level properties override → msg.payload keys override (highest priority).

## Function node

The most powerful node. Write JavaScript that receives ` + "`" + `msg` + "`" + ` and returns the modified ` + "`" + `msg` + "`" + `:

` + "```" + `js
// Double the payload value
msg.payload = msg.payload * 2;
return msg;

// Call a tool
var result = tools.call("rubix.query_nodes", {filter: "type=sensor"});
msg.payload = result;
return msg;

// Use persistent flow state
var count = flow.get("counter") || 0;
count++;
flow.set("counter", count);
msg.payload = { count: count };
return msg;

// Drop the message (don't send to output)
if (msg.payload.skip) return null;
return msg;
` + "```" + `

Available in JS scope:
- ` + "`" + `msg` + "`" + ` — the message (read/write payload, topic, custom props)
- ` + "`" + `node.log(s)` + "`" + `, ` + "`" + `node.warn(s)` + "`" + `, ` + "`" + `node.error(s)` + "`" + ` — logging
- ` + "`" + `flow.get(key)` + "`" + ` / ` + "`" + `flow.set(key, value)` + "`" + ` — persistent state (survives across runs)
- ` + "`" + `tools.call(name, params)` + "`" + ` — call any installed app tool
- Plus all platform APIs: ` + "`" + `http.get/post` + "`" + `, ` + "`" + `base64` + "`" + `, ` + "`" + `crypto` + "`" + `, ` + "`" + `env.get` + "`" + `

## Expression nodes (condition, switch, transform)

Expressions evaluate against the msg with these variables:
- ` + "`" + `payload` + "`" + ` — msg.payload (the main data)
- ` + "`" + `input` + "`" + ` — alias for payload (backward compat)
- ` + "`" + `msg` + "`" + ` — the full msg object
- ` + "`" + `topic` + "`" + ` — msg.topic
- ` + "`" + `vars` + "`" + ` — flow-level variables

Examples:
- Condition: ` + "`" + `payload.temperature > 30` + "`" + `
- Transform: ` + "`" + `payload.items.map(i => i.name)` + "`" + `
- Switch: ` + "`" + `payload.status` + "`" + ` (routes to ` + "`" + `case_ok` + "`" + `, ` + "`" + `case_error` + "`" + `, ` + "`" + `default` + "`" + `)

## Triggers

| Type | Config | Behavior |
|---|---|---|
| ` + "`" + `manual` + "`" + ` | — | Only via POST /api/flows/:id/run |
| ` + "`" + `cron` + "`" + ` | 5-field cron (` + "`" + `0 9 * * 1-5` + "`" + `) | Minute-granularity, supports ranges/steps/lists |
| ` + "`" + `interval` + "`" + ` | Go duration (` + "`" + `10s` + "`" + `, ` + "`" + `5m` + "`" + `) | Repeating timer |
| ` + "`" + `webhook` + "`" + ` | path (default: flow name) | POST /hooks/flow/:path → fires the flow, body becomes msg.payload |
| ` + "`" + `event` + "`" + ` | NATS topic pattern | Subscribes to bus, fires on matching events |

Flows are "deployed" — triggers run persistently. Save a flow → trigger starts. Delete → trigger stops. Server reboots → triggers resume (DeployAll on boot).

## Error handling

Per-node ` + "`" + `on_error` + "`" + ` setting:

| Strategy | Behavior |
|---|---|
| ` + "`" + `stop` + "`" + ` (default) | Node fails → flow fails → remaining nodes cancelled |
| ` + "`" + `skip` + "`" + ` | Node marked skipped, downstream gets nil, flow continues |
| ` + "`" + `retry` + "`" + ` | Retry up to max_retries times, then stop |
| ` + "`" + `fallback` + "`" + ` | Route to error output port for recovery path |

## Debug node

Place a debug node after any node to capture messages in the debug panel:

` + "```" + `
[trigger] → [http-request] → [debug "api response"] → [transform] → [output]
` + "```" + `

Config:
- ` + "`" + `label` + "`" + ` — display name in the debug panel
- ` + "`" + `output` + "`" + ` — what to capture: ` + "`" + `payload` + "`" + ` (default) or ` + "`" + `full` + "`" + ` (entire msg)
- ` + "`" + `active` + "`" + ` — set false to mute without removing from the graph

Debug entries appear in the right sidebar debug tab with:
- Color-coded by source node
- Collapsible JSON tree with syntax highlighting
- Copy-to-clipboard per entry
- Filter by debug node
- History persists across runs until manually cleared

## REST API

### Flow definitions (CRUD)

| Method | Path | Description |
|---|---|---|
| POST | /api/flows | Create flow (auto-deploys trigger) |
| GET | /api/flows | List flows |
| GET | /api/flows/:id | Get flow |
| PUT | /api/flows/:id | Update flow (re-deploys trigger) |
| DELETE | /api/flows/:id | Delete flow (undeploys trigger) |
| POST | /api/flows/:id/duplicate | Clone flow |

### Flow execution

| Method | Path | Description |
|---|---|---|
| POST | /api/flows/:id/run | Start a run (returns immediately, async execution) |
| GET | /api/flows/:id/runs | List runs for a flow |
| GET | /api/flow-runs/:runId | Get run with all node states |
| POST | /api/flow-runs/:runId/approve/:nodeId | Approve an approval gate |
| POST | /api/flow-runs/:runId/reject/:nodeId | Reject an approval gate |
| POST | /api/flow-runs/:runId/cancel | Cancel a running flow |

### Node type catalog

| Method | Path | Description |
|---|---|---|
| GET | /api/flows/node-types | All types (grouped by category, includes JSON Schema settings) |
| GET | /api/flows/node-types/:type | Detail for one type |

### Webhook trigger

| Method | Path | Description |
|---|---|---|
| POST | /hooks/flow/:path | Fire a webhook-triggered flow (unauthenticated) |

### Validation

| Method | Path | Description |
|---|---|---|
| POST | /api/flows/validate | Validate a flow graph without saving |

## Command bus integration

Flows can be triggered from Slack, CLI, or webhooks via the command bus:

` + "```" + `
run flow/daily-report
run flow/sensor-check --threshold 30
` + "```" + `

## Real-time events

The engine publishes events to the NATS bus under ` + "`" + `flow.>` + "`" + `:

` + "`" + `flow.started` + "`" + `, ` + "`" + `flow.node.started` + "`" + `, ` + "`" + `flow.node.completed` + "`" + `, ` + "`" + `flow.node.failed` + "`" + `, ` + "`" + `flow.node.skipped` + "`" + `, ` + "`" + `flow.debug` + "`" + `, ` + "`" + `flow.waiting_approval` + "`" + `, ` + "`" + `flow.approved` + "`" + `, ` + "`" + `flow.rejected` + "`" + `, ` + "`" + `flow.completed` + "`" + `, ` + "`" + `flow.failed` + "`" + `, ` + "`" + `flow.cancelled` + "`" + `

The frontend connects via ` + "`" + `WS /api/events/ws` + "`" + ` and subscribes to ` + "`" + `flow.>` + "`" + ` filtered by flow_id. Node states update in real-time on the canvas — no polling.

## Example flow: API → Transform → Slack

1. **Trigger** (cron: ` + "`" + `0 9 * * 1-5` + "`" + `) — weekdays at 9am
2. **HTTP Request** (url: ` + "`" + `https://api.weather.com/current` + "`" + `) — fetch weather
3. **Debug** (label: "weather response") — inspect the API response
4. **Transform** (expression: ` + "`" + `"Temperature: " + payload.body.temp + "°C"` + "`" + `) — format message
5. **Slack Send** (channel: #general) — post to Slack

Each wire carries a msg. The trigger emits ` + "`" + `{payload: null}` + "`" + `. The HTTP node emits ` + "`" + `{payload: responseBody, statusCode: 200}` + "`" + `. The transform sets ` + "`" + `payload` + "`" + ` to the formatted string. Slack reads ` + "`" + `msg.payload` + "`" + ` as the message text.

## File layout

` + "```" + `
pkg/flow/
    engine.go          — Engine struct, StartRun, execute loop
    executor.go        — NodeExecutor interface, ExecContext, Services
    nodes.go           — Built-in executors (17 types)
    integrations.go    — Integration executors (5 types + tool:*)
    function_node.go   — Function node (JS with flow/tools API)
    debug_node.go      — Debug node (capture to debug panel)
    msg.go             — Node-RED msg helpers (NewMsg, MsgPayload, MsgSet, etc.)
    types.go           — FlowDef, FlowRun, NodeState, DebugEntry
    registry.go        — NodeTypeRegistry (24 built-in types)
    validate.go        — DAG validation (Kahn's algorithm)
    store.go           — SQLite CRUD
    events.go          — NATS event publishing
    runtime.go         — Deploy/Undeploy lifecycle
    triggers.go        — Cron, interval, webhook, event triggers
    propagation.go     — Output routing, error handling
    scheduling.go      — Node readiness, firing
    lifecycle.go       — Approval, cancel, recover, subgraph
    expr.go            — JS expression evaluation
    settings/          — JSON Schema builder + all 24 node schemas
` + "```" + `
`,
}
