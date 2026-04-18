# Flow Engine — Visual DAG Workflows

A graph-based workflow engine that replaces the linear stage pipeline with a full DAG (directed acyclic graph). Users design flows visually in a React Flow canvas — dragging nodes, connecting edges, configuring settings — and the engine executes them with parallel branches, conditional routing, loops, and approval gates.

**This builds on existing bizzy infrastructure.** The NATS event bus, plugin system, tool/prompt services, command bus, and adapters all stay unchanged. The flow engine is a new execution backend that emits the same bus events (`workflow.>`) and is invokable via the same command syntax (`run flow/my-flow`). The existing linear workflow runner continues to work for YAML-defined pipelines.

---

## Design influences

| Source | What we take |
|---|---|
| **Rubix runtime** (our existing system) | Node/port model, sequential processing per node, hot-reload, degraded nodes, port value caching, service injection via context |
| **GoFlow** | DAG definition with explicit nodes + edges, aggregator pattern for fan-in, `ConditionalBranch`, `ForEachBranch`, data as `[]byte` between nodes |
| **Floxy** | Fork/join with join strategies, human-decision nodes (approval gates), compensation/rollback on failure, step handler interface, DLQ for failed steps |
| **Bizzy existing** | NATS bus events, plugin tool proxy, command bus verbs, MCP tool serving, approval channels |

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│  React Flow Frontend                                                │
│                                                                     │
│  Node palette ──→ Canvas ──→ Save (POST /api/flows)                │
│  Live execution view ←── SSE /api/events/stream                    │
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
│                              ├── PromptExecutor   │─→ AgentService  │
│                              ├── PluginExecutor   │─→ NATS proxy    │
│                              ├── FlowControl      │─→ (internal)    │
│                              └── IntegrationExec  │─→ adapters      │
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
    Data     map[string]any `json:"data,omitempty"`       // node-specific config
    Ports    *PortsDef      `json:"ports,omitempty"`      // override default ports
}

type Position struct {
    X float64 `json:"x"`
    Y float64 `json:"y"`
}

// PortsDef allows nodes to declare custom ports beyond the defaults.
// Most nodes use the defaults from their type registration and don't need this.
type PortsDef struct {
    Inputs  []PortDef `json:"inputs,omitempty"`
    Outputs []PortDef `json:"outputs,omitempty"`
}

type PortDef struct {
    Handle      string `json:"handle"`                // port identifier (edge connects to this)
    Label       string `json:"label,omitempty"`
    Type        string `json:"type,omitempty"`         // "any", "string", "number", "bool", "object"
    Required    bool   `json:"required,omitempty"`
}
```

### FlowEdgeDef — a connection between nodes

```go
type FlowEdgeDef struct {
    ID           string `json:"id"`
    Source       string `json:"source"`                // source node ID
    SourceHandle string `json:"sourceHandle"`          // output port handle
    Target       string `json:"target"`                // target node ID
    TargetHandle string `json:"targetHandle"`          // input port handle
    Condition    string `json:"condition,omitempty"`   // for conditional edges: expr expression
    Label        string `json:"label,omitempty"`       // display label on edge
}
```

### FlowRun — a single execution

```go
type FlowRun struct {
    ID          string              `json:"id" gorm:"primaryKey"`     // "frun-" prefix
    FlowID      string              `json:"flow_id" gorm:"index"`
    FlowVersion int                 `json:"flow_version"`             // snapshot version at time of run
    FlowName    string              `json:"flow_name"`
    Status      FlowRunStatus       `json:"status" gorm:"index"`
    Inputs      map[string]any      `json:"inputs,omitempty" gorm:"serializer:json"`
    Output      any                 `json:"output,omitempty" gorm:"serializer:json"`
    NodeStates  map[string]NodeState `json:"node_states" gorm:"serializer:json"`
    Error       string              `json:"error,omitempty"`
    UserID      string              `json:"user_id" gorm:"index"`
    ReplyTo     *ReplyInfo          `json:"reply_to,omitempty" gorm:"serializer:json"`
    CreatedAt   time.Time           `json:"created_at"`
    FinishedAt  *time.Time          `json:"finished_at,omitempty"`
}

type FlowRunStatus string

const (
    FlowRunPending         FlowRunStatus = "pending"
    FlowRunRunning         FlowRunStatus = "running"
    FlowRunWaitingApproval FlowRunStatus = "waiting_approval"
    FlowRunCompleted       FlowRunStatus = "completed"
    FlowRunFailed          FlowRunStatus = "failed"
    FlowRunCancelled       FlowRunStatus = "cancelled"
)

// NodeState tracks per-node execution within a run.
type NodeState struct {
    Status     NodeStatus     `json:"status"`
    Output     any            `json:"output,omitempty"`
    Error      string         `json:"error,omitempty"`
    StartedAt  *time.Time     `json:"started_at,omitempty"`
    FinishedAt *time.Time     `json:"finished_at,omitempty"`
    DurationMS int            `json:"duration_ms,omitempty"`
    Retries    int            `json:"retries,omitempty"`
}

type NodeStatus string

const (
    NodePending   NodeStatus = "pending"
    NodeReady     NodeStatus = "ready"      // all inputs satisfied, queued for execution
    NodeRunning   NodeStatus = "running"
    NodeCompleted NodeStatus = "completed"
    NodeFailed    NodeStatus = "failed"
    NodeSkipped   NodeStatus = "skipped"
    NodeWaiting   NodeStatus = "waiting"    // approval gate
)
```

### TriggerDef — what starts the flow

```go
type TriggerDef struct {
    Type     string         `json:"type"`      // "manual", "cron", "webhook", "event"
    Schedule string         `json:"schedule,omitempty"`  // cron: "0 9 * * *"
    Event    string         `json:"event,omitempty"`     // event: "workflow.completed", "job.failed"
    Filter   map[string]any `json:"filter,omitempty"`    // event: match fields
}
```

When a flow has a trigger, saving it auto-registers with the appropriate adapter:
- `cron` → creates a `cron_commands` entry
- `webhook` → registers a path under `/hooks/flow/<flow-name>`
- `event` → creates a NATS subscription on the bus

`manual` (or no trigger) means the flow is only started via API/CLI/Slack.

---

## Node types

Every node type is registered in a **node type registry** — a catalog of what's available to place on the canvas. Node types come from four sources:

### 1. Built-in flow control nodes

These are always available.

| Type | Ports | Description |
|---|---|---|
| `trigger` | out: `output` | Entry point of the flow. Emits the flow inputs. Every flow has exactly one. |
| `approval` | in: `input`, out: `approved`, `rejected` | Pauses execution, waits for user action. Rejection can route to a different branch. |
| `condition` | in: `input`, out: `true`, `false` | Evaluates an expr expression against the input. Routes to `true` or `false` output. |
| `switch` | in: `input`, out: `case_*`, `default` | Multi-way branch. Evaluates expression, routes to matching case output port. |
| `merge` | in: `input_1`..`input_N`, out: `output` | Fan-in join. Waits for **all** connected inputs, then emits merged result. |
| `race` | in: `input_1`..`input_N`, out: `output`, `winner` | Fan-in first. Emits the first input that arrives, cancels remaining branches via context cancellation. |
| `foreach` | in: `items`, out: `item` (per iteration), `done` (all complete) | Iterates over array input. Executes downstream subgraph once per item. Collects results into `done`. |
| `delay` | in: `input`, out: `output` | Waits for configured duration, then passes input through. |
| `transform` | in: `input`, out: `output` | Applies an expr expression or JSONPath transformation to reshape data between nodes. |
| `set-variable` | in: `input`, out: `output` | Stores a value into the flow's variable map. Downstream nodes can reference it via `{{vars.name}}`. |
| `log` | in: `input`, out: `output` | Logs the input value to the flow run record (passthrough — output equals input). |
| `output` | in: `input` | Terminal node. Marks the final result of the flow. |
| `error` | in: `input` | Terminal error node. Marks the flow as failed with the input as the error message. |

### 2. Tool nodes (from apps)

Every tool from every app the user has installed becomes a placeable node. Generated dynamically from the user's `AppInstall` records.

| Type pattern | Ports | Source |
|---|---|---|
| `tool:<appName>.<toolName>` | in: one port per tool param, out: `result`, `error` | App JS/OpenAPI tools |

Example: `tool:rubix.query_nodes` has input ports `filter`, `limit` and output ports `result`, `error`.

### 3. Plugin tool nodes

Same as app tools, but from registered plugins.

| Type pattern | Ports | Source |
|---|---|---|
| `tool:plugin.<name>.<tool>` | in: per param, out: `result`, `error` | Plugin tools via NATS |

### 4. Integration action nodes

Built-in nodes that use the adapter infrastructure to send messages outward. These are the adapters used as **egress actions**, not ingress triggers.

| Type | Ports | Description |
|---|---|---|
| `slack-send` | in: `channel`, `message`, `thread_ts`; out: `result`, `error` | Send a Slack message |
| `email-send` | in: `to`, `subject`, `body`; out: `result`, `error` | Send an email via SMTP |
| `webhook-call` | in: `url`, `method`, `headers`, `body`; out: `response`, `error` | Make an HTTP request |
| `ai-prompt` | in: `prompt`, `provider`, `model`; out: `result`, `error` | Run an AI prompt through AgentService (single-turn, no tools) |
| `ai-runner` | in: `prompt`, `work_dir`; out: `result`, `error` | Run a full AI coding session via `pkg/airunner` — supports Claude, OpenCode, Codex, Copilot. Has MCP tool access, can edit files, run commands, create commits. See [AI Runner node](#ai-runner-node). |

### Node type registry

```go
type NodeTypeRegistry struct {
    mu    sync.RWMutex
    types map[string]NodeTypeDef
}

type NodeTypeDef struct {
    Type        string    `json:"type"`
    Label       string    `json:"label"`
    Description string    `json:"description,omitempty"`
    Category    string    `json:"category"`     // "flow-control", "tool", "integration", "data"
    Icon        string    `json:"icon,omitempty"`
    Source      string    `json:"source"`        // "builtin", "app", "plugin"
    Ports       PortsDef  `json:"ports"`         // default ports
    Settings    any       `json:"settings,omitempty"` // JSON Schema for node config panel
}
```

Built-in types are registered at startup. App/plugin tool types are rebuilt when apps are installed/uninstalled or plugins register/deregister.

---

## Execution engine

### Design principles

The engine follows a **single-writer** model: one main goroutine owns all run state (`FlowRun`, `NodeStates`, DB persistence). Worker goroutines execute nodes and report results back via a channel. This avoids race conditions on shared state and ensures DB writes never overlap.

The engine is also **resumable**: when a flow pauses (approval gate, server restart), the run is fully persisted. Resuming loads the run from DB and re-enters the main loop — no in-memory channels or goroutines need to survive across restarts.

### DAG resolution

When a flow run starts (or resumes), the engine:

1. **Validates** the graph — no cycles (except inside `foreach` subgraphs), all required ports connected, all node types registered
2. **Topologically sorts** nodes to determine execution order
3. **Identifies independent groups** — nodes with no dependency between them can run in parallel
4. **Initializes `NodeState`** for every node as `pending`

### Execution loop (channel-based, single-writer)

```go
// nodeResult is sent by worker goroutines back to the main loop.
type nodeResult struct {
    NodeID     string
    Output     any
    Error      error
    StartedAt  time.Time
    FinishedAt time.Time
}

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
            e.persistRun(run)
            return

        case res := <-results:
            inflight--

            // --- single writer: only the main loop touches run state ---
            if res.Error != nil {
                cont := e.handleNodeError(run, def, res)
                if !cont {
                    // flow failed — cancel all in-flight nodes
                    cancel()
                    run.Status = FlowRunFailed
                    e.persistRun(run)
                    return
                }
            } else {
                state := &run.NodeStates[res.NodeID]
                state.Status = NodeCompleted
                state.Output = res.Output
                state.StartedAt = &res.StartedAt
                state.FinishedAt = &res.FinishedAt
                state.DurationMS = int(res.FinishedAt.Sub(res.StartedAt).Milliseconds())

                e.publishNodeCompleted(run, def.GetNode(res.NodeID))
                e.propagateOutputs(run, def, res.NodeID, res.Output)
            }

            // Persist after every state change (single writer, no overlap)
            e.persistRun(run)

            // Check for flow-level completion
            if e.allTerminalsDone(run, def) {
                run.Status = FlowRunCompleted
                now := time.Now()
                run.FinishedAt = &now
                e.persistRun(run)
                return
            }

            // Check for pause (approval gate hit)
            if run.Status == FlowRunWaitingApproval {
                // Run is persisted, main loop exits.
                // Approval API will call ResumeRun() to re-enter.
                return
            }

            // Fire any newly-ready nodes
            inflight += e.fireReadyNodes(ctx, run, def, results)
        }
    }

    // No in-flight nodes and no terminals done — deadlock or empty graph
    if run.Status == FlowRunRunning {
        run.Status = FlowRunFailed
        run.Error = "flow deadlocked: no runnable nodes and no terminals completed"
        e.persistRun(run)
    }
}

// fireReadyNodes launches goroutines for all ready nodes. Returns count launched.
func (e *Engine) fireReadyNodes(ctx context.Context, run *FlowRun, def *FlowDef, results chan<- nodeResult) int {
    ready := e.getReadyNodes(run)
    for _, nodeID := range ready {
        run.NodeStates[nodeID] = NodeState{Status: NodeRunning}
        e.publishNodeStarted(run, def.GetNode(nodeID))

        go e.runNode(ctx, run, def, nodeID, results)
    }
    return len(ready)
}
```

A node becomes **ready** when all its connected input ports have received values. The main loop re-evaluates readiness after *each individual* node completion, so downstream nodes fire as soon as possible rather than waiting for a full batch.

### Node execution (worker goroutine)

Worker goroutines are pure: they read inputs (gathered before dispatch), call the executor, and send the result. They never touch `run.NodeStates` or call `persistRun`.

```go
func (e *Engine) runNode(ctx context.Context, run *FlowRun, def *FlowDef, nodeID string, results chan<- nodeResult) {
    nodeDef := def.GetNode(nodeID)
    started := time.Now()

    // Gather inputs from upstream edges (read-only on completed node outputs)
    inputs := e.gatherInputs(run, def, nodeID)

    // Execute based on node type
    output, err := e.dispatch(ctx, run, nodeDef, inputs)

    results <- nodeResult{
        NodeID:     nodeID,
        Output:     output,
        Error:      err,
        StartedAt:  started,
        FinishedAt: time.Now(),
    }
}
```

### Dispatching by node type

The dispatch function is called by worker goroutines. It must not mutate `run` directly — it returns values via the `nodeResult` channel.

```go
func (e *Engine) dispatch(ctx context.Context, run *FlowRun, node *FlowNodeDef, inputs map[string]any) (any, error) {
    // Apply per-node timeout if configured
    if timeout := getNodeTimeout(node); timeout > 0 {
        var cancel context.CancelFunc
        ctx, cancel = context.WithTimeout(ctx, timeout)
        defer cancel()
    }

    switch {
    case node.Type == "trigger":
        return run.Inputs, nil

    case node.Type == "approval":
        return nil, ErrApprovalRequired // sentinel — main loop pauses the run

    case node.Type == "condition":
        return e.executeCondition(node, inputs)

    case node.Type == "switch":
        return e.executeSwitch(node, inputs)

    case node.Type == "merge":
        return e.executeMerge(inputs)

    case node.Type == "race":
        return e.executeRace(ctx, run, node, inputs)

    case node.Type == "foreach":
        return e.executeForeach(ctx, run, node, inputs)

    case node.Type == "delay":
        return e.executeDelay(ctx, node, inputs)

    case node.Type == "transform":
        return e.executeTransform(node, inputs)

    case node.Type == "set-variable":
        return e.executeSetVariable(node, inputs)

    case node.Type == "log":
        return e.executeLog(run, node, inputs)

    case node.Type == "ai-prompt":
        return e.executePrompt(ctx, run, node, inputs)

    case node.Type == "ai-runner":
        return e.executeAIRunner(ctx, run, node, inputs)

    case strings.HasPrefix(node.Type, "tool:"):
        toolName := strings.TrimPrefix(node.Type, "tool:")
        return e.tools.CallTool(ctx, run.UserID, toolName, inputs)

    case node.Type == "slack-send":
        return e.executeSlackSend(ctx, node, inputs)

    case node.Type == "email-send":
        return e.executeEmailSend(ctx, node, inputs)

    case node.Type == "webhook-call":
        return e.executeWebhookCall(ctx, node, inputs)

    case node.Type == "output":
        return PortOutput{Port: "output", Value: inputs["input"]}, nil // main loop stores to run.Output

    case node.Type == "error":
        return nil, fmt.Errorf("%v", inputs["input"]) // terminal error

    default:
        return nil, fmt.Errorf("unknown node type: %s", node.Type)
    }
}
```

### Output propagation and conditional edges

After a node completes, the engine routes its output to downstream nodes via edges. Edges can be **unconditional** (always fire) or **conditional** (fire only when expression matches):

```go
func (e *Engine) propagateOutputs(run *FlowRun, def *FlowDef, nodeID string, output any) {
    for _, edge := range def.EdgesFrom(nodeID) {
        // For nodes with multiple output ports (condition: true/false, approval: approved/rejected),
        // the output is a map keyed by port handle. Only propagate the active port.
        portValue := resolvePortValue(output, edge.SourceHandle)

        // Evaluate edge condition if present
        if edge.Condition != "" {
            if !evalCondition(edge.Condition, portValue) {
                continue
            }
        }

        // Deliver value to target node's input port
        e.deliverInput(run, edge.Target, edge.TargetHandle, portValue)

        // Check if target node is now ready (all required inputs satisfied)
        if e.isNodeReady(run, def, edge.Target) {
            run.NodeStates[edge.Target] = NodeState{Status: NodeReady}
        }
    }
}
```

### Condition and switch nodes

Condition and switch nodes don't propagate to all outputs — they activate only the matching output port.

Expressions use [`expr-lang/expr`](https://github.com/expr-lang/expr) — a sandboxed, type-safe expression language. Unlike Go's `text/template`, expr cannot call arbitrary methods, access the filesystem, or escape its sandbox. Expressions are compiled at flow-save time (validation catches syntax errors early) and evaluated at runtime against a controlled environment containing only the node's input values and flow variables.

```go
// Condition node: evaluates expression, activates "true" or "false" output port
func (e *Engine) executeCondition(node *FlowNodeDef, inputs map[string]any) (any, error) {
    exprStr, ok := node.Data["expression"].(string) // e.g. "input > 100"
    if !ok {
        return nil, fmt.Errorf("condition node %s: missing 'expression' in data", node.ID)
    }

    env := buildExprEnv(inputs)
    result, err := expr.Eval(exprStr, env)
    if err != nil {
        return nil, fmt.Errorf("condition node %s: expression error: %w", node.ID, err)
    }

    if boolVal, ok := result.(bool); ok && boolVal {
        return PortOutput{Port: "true", Value: inputs["input"]}, nil
    }
    return PortOutput{Port: "false", Value: inputs["input"]}, nil
}

// Switch node: evaluates expression, activates matching case port
func (e *Engine) executeSwitch(node *FlowNodeDef, inputs map[string]any) (any, error) {
    exprStr, ok := node.Data["expression"].(string)
    if !ok {
        return nil, fmt.Errorf("switch node %s: missing 'expression' in data", node.ID)
    }

    env := buildExprEnv(inputs)
    result, err := expr.Eval(exprStr, env)
    if err != nil {
        return nil, fmt.Errorf("switch node %s: expression error: %w", node.ID, err)
    }

    value := fmt.Sprintf("%v", result)
    cases, _ := node.Data["cases"].(map[string]any)
    for caseName := range cases {
        if caseName == value {
            return PortOutput{Port: "case_" + caseName, Value: inputs["input"]}, nil
        }
    }
    return PortOutput{Port: "default", Value: inputs["input"]}, nil
}

// buildExprEnv creates the sandboxed environment for expression evaluation.
// Only input port values and flow variables are accessible — no methods, no side effects.
func buildExprEnv(inputs map[string]any) map[string]any {
    return inputs // extend with vars map when set-variable is implemented
}
```

### Merge (fan-in) node

Waits for all connected inputs before firing:

```go
func (e *Engine) executeMerge(inputs map[string]any) (any, error) {
    // inputs is already the merged map of all input port values
    // e.g. {"input_1": resultA, "input_2": resultB}
    return inputs, nil
}
```

The engine only marks a merge node as `ready` when every connected input port has a value. This is the fan-in synchronization point.

### ForEach node

Spawns parallel sub-executions for each item in the input array.

#### Subgraph boundary detection

A foreach node's **subgraph** is the set of nodes reachable from its `item` output port that are *not* reachable from any other path outside the foreach. The boundary is determined at validation time using reachability analysis:

1. Walk forward from the foreach's `item` port, collecting all reachable nodes
2. The subgraph ends at nodes that connect back to nodes *outside* the reachable set (these are the "collector" edges that feed into the `done` port)
3. Store the subgraph node IDs in `node.Data["_subgraph_nodes"]` during validation so the engine doesn't recompute at runtime

**Constraint:** Subgraph nodes cannot have inputs from outside the foreach (except the foreach's `item` port). This is enforced at validation time. If a node inside the subgraph needs external data, it must be passed through the foreach's `items` input as part of each item object.

#### Namespaced node states per iteration

Each iteration gets its own set of `NodeState` entries, namespaced by iteration index:

```
NodeStates["foreach-1:iter-0:transform-3"] = NodeState{...}
NodeStates["foreach-1:iter-1:transform-3"] = NodeState{...}
```

This allows the live execution view to show per-iteration progress and avoids state collisions between parallel iterations.

#### Execution

```go
func (e *Engine) executeForeach(ctx context.Context, run *FlowRun, node *FlowNodeDef, inputs map[string]any) (any, error) {
    items, ok := inputs["items"].([]any)
    if !ok {
        return nil, fmt.Errorf("foreach node %s: 'items' input must be an array", node.ID)
    }

    // Enforce max iterations to prevent runaway loops
    maxIter := getMaxIterations(node) // default 1000, configurable via node.Data["max_iterations"]
    if len(items) > maxIter {
        return nil, fmt.Errorf("foreach node %s: %d items exceeds max_iterations (%d)", node.ID, len(items), maxIter)
    }

    // Get precomputed subgraph node IDs (set during validation)
    subgraphNodeIDs := node.Data["_subgraph_nodes"].([]string)

    // Execute subgraph once per item, with bounded concurrency
    concurrency := getConcurrency(node) // default 10, configurable via node.Data["concurrency"]
    results := make([]any, len(items))
    sem := make(chan struct{}, concurrency)
    var mu sync.Mutex
    var firstErr error
    var wg sync.WaitGroup

    for i, item := range items {
        wg.Add(1)
        sem <- struct{}{}
        go func(idx int, val any) {
            defer wg.Done()
            defer func() { <-sem }()

            // Each iteration gets a namespaced mini-run of the subgraph
            iterPrefix := fmt.Sprintf("%s:iter-%d", node.ID, idx)
            result, err := e.executeSubgraph(ctx, run, subgraphNodeIDs, iterPrefix, val)
            if err != nil {
                mu.Lock()
                if firstErr == nil {
                    firstErr = fmt.Errorf("foreach node %s iteration %d: %w", node.ID, idx, err)
                }
                mu.Unlock()
                return
            }
            results[idx] = result
        }(i, item)
    }
    wg.Wait()

    if firstErr != nil {
        return nil, firstErr
    }

    return PortOutput{Port: "done", Value: results}, nil
}
```

### Approval gate (non-blocking, resumable)

Approval gates **do not block a goroutine**. Instead, the node signals the main loop to pause the run. The run is fully persisted to SQLite, and the main loop exits cleanly. When the user approves or rejects via the API, the engine reloads the run from DB and resumes execution.

This design means:
- No leaked goroutines waiting on channels
- Flows survive server restarts — a run paused at an approval gate before a restart resumes correctly after
- Multiple approval gates in a flow work naturally (each one pauses and resumes independently)

**Dispatch returns a sentinel to signal the pause:**

```go
// ErrApprovalRequired is returned by the approval node executor to signal the
// main loop to pause the run. It is not a real error — the main loop handles it
// by setting the run status and exiting cleanly.
var ErrApprovalRequired = errors.New("approval required")

func (e *Engine) executeApproval(ctx context.Context, run *FlowRun, node *FlowNodeDef, inputs map[string]any) (any, error) {
    // Return sentinel — the main loop will pause the run
    return nil, ErrApprovalRequired
}
```

**The main loop handles it in the error path:**

```go
// Inside the main loop's error handling:
if errors.Is(res.Error, ErrApprovalRequired) {
    state := &run.NodeStates[res.NodeID]
    state.Status = NodeWaiting
    run.Status = FlowRunWaitingApproval

    e.publish("flow.waiting_approval", FlowEvent{
        RunID:  run.ID,
        NodeID: res.NodeID,
        Output: inputs,
    })
    e.persistRun(run)
    // Main loop will exit on the next iteration (sees FlowRunWaitingApproval)
    return
}
```

**Approval API resumes the run:**

```go
// Called by POST /api/flow-runs/:runId/approve/:nodeId
func (e *Engine) ApproveNode(ctx context.Context, runID, nodeID string) error {
    run, def := e.loadRun(runID) // reload from DB

    state := &run.NodeStates[nodeID]
    state.Status = NodeCompleted
    state.Output = PortOutput{Port: "approved", Value: state.Input}
    run.Status = FlowRunRunning

    e.propagateOutputs(run, def, nodeID, state.Output)
    e.persistRun(run)
    e.publish("flow.approved", FlowEvent{RunID: run.ID, NodeID: nodeID})

    // Re-enter the execution loop
    go e.execute(ctx, run, def)
    return nil
}

// Called by POST /api/flow-runs/:runId/reject/:nodeId
func (e *Engine) RejectNode(ctx context.Context, runID, nodeID, feedback string) error {
    run, def := e.loadRun(runID)

    state := &run.NodeStates[nodeID]
    state.Status = NodeCompleted
    state.Output = PortOutput{Port: "rejected", Value: feedback}
    run.Status = FlowRunRunning

    e.propagateOutputs(run, def, nodeID, state.Output)
    e.persistRun(run)
    e.publish("flow.rejected", FlowEvent{RunID: run.ID, NodeID: nodeID})

    go e.execute(ctx, run, def)
    return nil
}
```

**Startup recovery:** When the server starts, query for runs with `status = "waiting_approval"`. These are correctly persisted and will resume when the user acts on them via the API. No recovery action needed — they're just waiting.

### Error handling

Each node can configure error behaviour via its `data.on_error` field:

| Strategy | Behaviour |
|---|---|
| `stop` (default) | Node marked failed, flow marked failed, remaining nodes cancelled |
| `skip` | Node marked skipped, downstream nodes receive nil, flow continues |
| `retry` | Retry up to `data.max_retries` times with exponential backoff, then fall through to `stop` |
| `fallback` | Route to `error` output port instead of failing. Downstream `error` path handles recovery. |

Error handling runs in the **main loop** (single writer), not in worker goroutines. The main loop receives a `nodeResult` with an error and applies the strategy:

```go
// handleNodeError processes a failed node result. Returns true if the flow should continue, false to stop.
func (e *Engine) handleNodeError(run *FlowRun, def *FlowDef, res nodeResult) bool {
    node := def.GetNode(res.NodeID)
    strategy := getErrorStrategy(node) // "stop", "skip", "retry", "fallback"
    state := &run.NodeStates[res.NodeID]

    switch strategy {
    case "retry":
        max := getMaxRetries(node)
        if state.Retries < max {
            state.Retries++
            state.Status = NodeReady // will be picked up by fireReadyNodes on next iteration
            return true
        }
        // exhausted retries, fall through to stop

    case "skip":
        state.Status = NodeSkipped
        state.Error = res.Error.Error()
        e.propagateOutputs(run, def, res.NodeID, nil)
        return true

    case "fallback":
        state.Status = NodeCompleted
        state.Error = res.Error.Error()
        e.propagateOutputs(run, def, res.NodeID, PortOutput{Port: "error", Value: res.Error.Error()})
        return true
    }

    // "stop" — fail the whole flow
    state.Status = NodeFailed
    state.Error = res.Error.Error()
    run.Error = fmt.Sprintf("node %s failed: %s", res.NodeID, res.Error.Error())
    e.publishFlowFailed(run, node, res.Error)
    return false // caller will cancel context and set FlowRunFailed
}
```

### Cancellation

Every flow run gets its own `context.WithCancel`. Cancellation propagates to all in-flight nodes:

- **Cancel API** (`POST /api/flow-runs/:runId/cancel`): Calls the cancel func, which causes all in-flight `dispatch` calls to return `ctx.Err()`. The main loop sees `<-ctx.Done()` and sets `FlowRunCancelled`.
- **Stop strategy**: When a node fails with `on_error: "stop"`, the main loop calls `cancel()` before exiting. In-flight nodes on other branches receive context cancellation.
- **Race node**: When the first branch completes, its sub-context is cancelled for the other branches (see race node section).

Node executors (tool calls, AI prompts, webhook calls) **must** respect `ctx.Done()` by passing the context through to HTTP clients, NATS requests, etc. This ensures cancellation actually stops work rather than just abandoning goroutines.

### Race node (fan-in first)

The `race` node fans out to multiple branches and takes the first result. It requires per-branch cancellation:

```go
func (e *Engine) executeRace(ctx context.Context, run *FlowRun, node *FlowNodeDef, inputs map[string]any) (any, error) {
    // Each input port represents a branch. The race node is "ready" when at least
    // one input arrives. The engine marks it ready on first input, not all inputs.

    // Find which input arrived first
    var winnerPort string
    var winnerValue any
    for port, val := range inputs {
        if val != nil {
            winnerPort = port
            winnerValue = val
            break
        }
    }

    // Cancel in-flight branches for the other inputs.
    // The main loop holds a per-node cancel func map. When the race node completes,
    // it signals the main loop to cancel all nodes that feed into the race's other
    // input ports. Those nodes' contexts are derived from a branch-specific sub-context.
    return RaceOutput{
        Port:   "output",
        Value:  winnerValue,
        Winner: winnerPort,
    }, nil
}
```

**Implementation detail:** The main loop creates a `context.WithCancel` per fan-out branch leading into a race node. When the race completes, the main loop cancels the remaining branches' contexts. This is tracked in `Engine.branchContexts` — a map of `raceNodeID -> []context.CancelFunc`.

### Resource limits

The engine enforces configurable resource bounds to prevent runaway flows:

| Limit | Default | Configurable via |
|---|---|---|
| Max parallel nodes per run | 50 | `Engine.MaxParallelNodes` |
| Max foreach iterations | 1000 | `node.Data["max_iterations"]` (hard cap: 10000) |
| Max foreach concurrency | 10 | `node.Data["concurrency"]` |
| Max flow run duration | 1 hour | `FlowDef.Settings["timeout"]` |
| Max node execution time | 5 minutes | `node.Data["timeout"]` |
| Max concurrent runs per user | 10 | `Engine.MaxRunsPerUser` |

The engine checks `MaxParallelNodes` in `fireReadyNodes` — if the in-flight count would exceed the limit, excess ready nodes stay in `NodeReady` state and are picked up when slots free.

Flow-level and node-level timeouts are implemented via `context.WithTimeout` wrapping the run context and per-node dispatch context respectively.

### Startup recovery

On server start, the engine queries for in-progress runs and handles them:

| Status | Recovery action |
|---|---|
| `running` | These were interrupted mid-execution. Mark as `failed` with error "interrupted by server restart". A future enhancement could resume from the last persisted state. |
| `waiting_approval` | No action needed — these are correctly paused. The approval API will resume them. |
| `pending` | Re-queue for execution (these were accepted but hadn't started). |

---

## AI Runner node

The `ai-runner` node runs a full AI coding session using the existing `pkg/airunner` infrastructure. Unlike `ai-prompt` (which is a single-turn LLM call), `ai-runner` starts an agentic session that can edit files, run commands, use MCP tools, and create commits — the same thing as running Claude Code, OpenCode, Codex, or Copilot from the CLI.

### Why this matters

This node turns the flow engine into a **remote coding orchestrator**. You can build flows that:
- Queue up coding tasks during the day, execute them sequentially at night
- Fan out independent tasks to parallel AI sessions across different repos
- Gate each task with approval — get a Slack message on failure, reply to continue or stop
- Chain coding sessions — one session's output (a commit, a branch, test results) feeds the next

### Node definition

| Field | Type | Description |
|---|---|---|
| `provider` | config | AI runner provider: `claude`, `opencode`, `codex`, `copilot`, `ollama` |
| `model` | config | Model override (e.g. `claude-sonnet-4-20250514`, `gpt-4.1`) — optional, uses provider default |
| `work_dir` | input or config | Working directory for the session (the repo to work in) |
| `prompt` | input | The coding task description |
| `resume_session` | config | If true, resume the previous session for this node+run (multi-turn) |
| `thinking_budget` | config | `"low"`, `"medium"`, `"high"` — controls extended thinking |
| `allowed_tools` | config | MCP tool filter pattern (e.g. `"*"` or `"nube.*"`) |
| `timeout_mins` | config | Per-session timeout, default 30 |

**Ports:**
- Inputs: `prompt` (string, required), `work_dir` (string, optional — falls back to config)
- Outputs: `result` (object), `error` (string)

**Result output shape:**

```json
{
  "text": "Fixed the login timeout by increasing the session TTL...",
  "provider": "opencode",
  "model": "claude-sonnet-4-20250514",
  "session_id": "sess_abc123",
  "cost_usd": 0.042,
  "duration_ms": 45200,
  "input_tokens": 12400,
  "output_tokens": 3200,
  "tool_calls": 8,
  "tool_call_log": ["read_file", "edit_file", "bash", "edit_file", "bash", "git_commit", ...]
}
```

### Implementation

The node calls `Runner.Run()` synchronously from its worker goroutine. The flow engine handles async/parallel at the DAG level — the node itself is a blocking call.

```go
func (e *Engine) executeAIRunner(ctx context.Context, run *FlowRun, node *FlowNodeDef, inputs map[string]any) (any, error) {
    provider := resolveString(node.Data, inputs, "provider")
    model := resolveString(node.Data, inputs, "model")
    prompt := resolveString(node.Data, inputs, "prompt")
    workDir := resolveString(node.Data, inputs, "work_dir")

    runner, err := e.agents.GetRunner(airunner.Provider(provider))
    if err != nil {
        return nil, fmt.Errorf("ai-runner: provider %q not available: %w", provider, err)
    }

    // Build session ID for resume support
    sessionID := fmt.Sprintf("flow:%s:%s", run.ID, node.ID)

    cfg := airunner.RunConfig{
        Prompt:         prompt,
        SystemPrompt:   e.agents.BuildSystemPrompt(run.UserID),
        Model:          model,
        WorkDir:        workDir,
        MCPURL:         e.mcpURL,
        MCPToken:       e.mcpToken,
        AllowedTools:   getStringOrDefault(node.Data, "allowed_tools", "*"),
        ThinkingBudget: getStringOrDefault(node.Data, "thinking_budget", "medium"),
    }

    // Resume previous session if configured
    if resumeSession, _ := node.Data["resume_session"].(bool); resumeSession {
        if prev, ok := run.NodeStates[node.ID]; ok && prev.Output != nil {
            if prevResult, ok := prev.Output.(map[string]any); ok {
                cfg.ResumeID = prevResult["session_id"].(string)
            }
        }
    }

    // Stream progress events to the bus — Slack adapter can subscribe to these
    // for real-time updates while the session runs
    result := runner.Run(ctx, cfg, sessionID, func(ev airunner.Event) {
        e.publish("flow.node.progress", FlowEvent{
            RunID:    run.ID,
            NodeID:   node.ID,
            NodeType: "ai-runner",
            Output:   ev,
        })
    })

    if result.Error != "" {
        return nil, fmt.Errorf("ai-runner: %s", result.Error)
    }

    return map[string]any{
        "text":          result.Text,
        "provider":      string(result.Provider),
        "model":         result.Model,
        "session_id":    result.ClaudeSessionID,
        "cost_usd":      result.CostUSD,
        "duration_ms":   result.DurationMS,
        "input_tokens":  result.InputTokens,
        "output_tokens": result.OutputTokens,
        "tool_calls":    result.ToolCalls,
        "tool_call_log": result.ToolCallLog,
    }, nil
}
```

### Streaming progress to Slack

While an `ai-runner` node executes, it publishes `flow.node.progress` events to the NATS bus. Each event contains the raw `airunner.Event` — text chunks, tool calls, errors. The Slack adapter can subscribe to these and forward them to a thread:

```
flow.node.progress  →  Slack adapter  →  #dev-automation thread
```

This gives you live visibility into what the AI is doing without polling. The existing `onEvent` callback in `Runner.Run()` maps directly — no new infrastructure needed.

---

## Example: Automated coding workflow

This is the primary use case for `ai-runner` — queue coding tasks, execute them on a schedule, interact via Slack.

### Task management

A simple `coding-tasks` app provides task CRUD. Tasks live in SQLite:

```go
type CodingTask struct {
    ID          string    `json:"id" gorm:"primaryKey"`
    Title       string    `json:"title"`
    Description string    `json:"description"`        // the prompt for the AI session
    Repo        string    `json:"repo"`                // repo path, e.g. "/home/user/code/bizzy"
    Branch      string    `json:"branch,omitempty"`    // branch to work on
    Status      string    `json:"status" gorm:"index"` // "pending", "selected", "running", "done", "failed"
    Priority    int       `json:"priority"`             // execution order within a run
    Result      string    `json:"result,omitempty" gorm:"serializer:json"`
    UserID      string    `json:"user_id" gorm:"index"`
    CreatedAt   time.Time `json:"created_at"`
}
```

**Tools exposed by the app:**

| Tool | Description |
|---|---|
| `coding-tasks.add` | Add a new task (`title`, `description`, `repo`, `branch`) |
| `coding-tasks.select` | Mark tasks as selected for next run (`ids[]`) |
| `coding-tasks.list-selected` | Fetch all selected tasks ordered by priority |
| `coding-tasks.mark-done` | Update task status + store result |

These are usable from Slack, CLI, or the flow canvas — they're just app tools like any other.

### The flow definition

```
┌─────────────────────────────────────────────────┐
│ Trigger: cron "0 20 * * *"  (8pm daily)         │
└──────────────────┬──────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────┐
│ Tool: coding-tasks.list-selected                │
│ (returns array of tasks, ordered by priority)   │
└──────────────────┬──────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────┐
│ Condition: len(tasks) > 0                       │
├──────────┬──────────────────────────────────────┘
│ true     │ false
│          ▼
│   [Output: "no tasks selected"]
▼
┌─────────────────────────────────────────────────┐
│ Slack: "Starting coding run — N tasks queued"   │
└──────────────────┬──────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────┐
│ ForEach: task  (concurrency: 1 — sequential)    │
└──────────────────┬──────────────────────────────┘
                   │ item
                   ▼
┌─────────────────────────────────────────────────┐
│ Slack: "Working on: {{item.title}}"             │
└──────────────────┬──────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────┐
│ AI Runner                                       │
│  provider: opencode  (or claude)                │
│  prompt: {{item.description}}                   │
│  work_dir: {{item.repo}}                        │
│  timeout_mins: 30                               │
│  on_error: fallback                             │
└──────┬──────────────────────┬───────────────────┘
    result                  error
       │                      │
       ▼                      ▼
┌──────────────┐   ┌─────────────────────────────┐
│ Tool: coding │   │ Slack: "Task failed:        │
│ -tasks.mark  │   │  {{item.title}}             │
│ -done        │   │  Error: {{error}}"          │
└──────┬───────┘   └──────────┬──────────────────┘
       │                      │
       ▼                      ▼
┌──────────────┐   ┌─────────────────────────────┐
│ Slack: "Done │   │ Approval: "Continue to      │
│ {{item.title}│   │  next task?"                │
│ cost: $X"    │   └─────┬──────────┬────────────┘
└──────┬───────┘      approved   rejected
       │                  │         │
       ▼                  ▼         ▼
  (next iter)       (next iter)  ┌────────────────┐
                                 │ Slack: "Run    │
                                 │ stopped by     │
                                 │ user"          │
                                 └───┬────────────┘
                                     ▼
                                 [Output: stopped]
```

### The daily workflow in practice

**During the day** — add tasks from Slack or CLI:

```
> add coding-task "Fix login timeout" --repo /home/user/code/bizzy --branch fix/login \
    --description "The session TTL in auth middleware is hardcoded to 5 minutes. \
    Change it to use the configurable value from app settings. \
    The setting already exists in the DB as 'session_ttl_minutes', \
    it's just not being read. Add a test."

> add coding-task "Add retry to webhook calls" --repo /home/user/code/bizzy --branch feat/retry \
    --description "webhook-call in pkg/adapters/webhook.go has no retry logic. \
    Add exponential backoff with max 3 retries. Use the backoff package already in go.mod."

> select coding-tasks 1,2
```

**8pm — cron fires:**

```
[8:00pm] bot: Starting coding run — 2 tasks queued
[8:00pm] bot: Working on: "Fix login timeout"
[8:11pm] bot: ✓ Done: "Fix login timeout" — committed a1b2c3 on fix/login
         Cost: $0.03 | Duration: 11m | Tool calls: 6
[8:11pm] bot: Working on: "Add retry to webhook calls"
[8:24pm] bot: ✗ Task failed: "Add retry to webhook calls"
         Error: "test TestWebhookRetry timed out after 30s"
         Continue to next task? [Approve] [Reject]
```

**You reply in Slack:**
```
> reject --feedback "stop for tonight, I'll look at the test tomorrow"
```

```
[8:24pm] bot: Run stopped by user. 1/2 tasks completed.
```

---

## REST API

### Flow definitions (CRUD)

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/flows` | Create a new flow definition |
| `GET` | `/api/flows` | List flow definitions (user-scoped) |
| `GET` | `/api/flows/:id` | Get flow definition |
| `PUT` | `/api/flows/:id` | Update flow definition (increments version) |
| `DELETE` | `/api/flows/:id` | Delete flow definition |
| `POST` | `/api/flows/:id/duplicate` | Clone a flow |

### Flow execution

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/flows/:id/run` | Start a flow run |
| `GET` | `/api/flows/:id/runs` | List runs for a flow |
| `GET` | `/api/flow-runs/:runId` | Get run status with all node states |
| `POST` | `/api/flow-runs/:runId/approve/:nodeId` | Approve an approval gate |
| `POST` | `/api/flow-runs/:runId/reject/:nodeId` | Reject an approval gate |
| `POST` | `/api/flow-runs/:runId/cancel` | Cancel a running flow |

### Node type catalog

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/flows/node-types` | Full catalog of available node types (built-in + user's tools + plugins) |
| `GET` | `/api/flows/node-types/:type` | Detail for one node type (ports, settings schema, help) |

### Validation

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/flows/validate` | Validate a flow definition without saving (cycle detection, missing connections, unknown types) |

---

## Event bus topics

The flow engine publishes to a new `flow.>` stream on NATS, following the same pattern as `workflow.>`:

```
flow.>
    flow.started                    — flow run began
    flow.node.started               — a node began executing
    flow.node.completed             — a node finished (with output)
    flow.node.failed                — a node failed (with error)
    flow.node.skipped               — a node was skipped
    flow.node.progress              — streaming progress from long-running nodes (ai-runner text, tool calls)
    flow.waiting_approval           — approval gate reached
    flow.approved                   — user approved
    flow.rejected                   — user rejected
    flow.completed                  — all terminal nodes done
    flow.failed                     — flow stopped on error
    flow.cancelled                  — flow was cancelled
```

Event data follows the existing `EventData` shape with additional `NodeID` field:

```go
type FlowEvent struct {
    RunID      string    `json:"run_id"`
    FlowID     string    `json:"flow_id"`
    FlowName   string    `json:"flow_name"`
    NodeID     string    `json:"node_id,omitempty"`
    NodeType   string    `json:"node_type,omitempty"`
    UserID     string    `json:"user_id"`
    Status     string    `json:"status"`
    Output     any       `json:"output,omitempty"`
    Error      string    `json:"error,omitempty"`
    DurationMS int       `json:"duration_ms,omitempty"`
    ReplyTo    ReplyInfo `json:"reply_to,omitempty"`
}
```

The SSE endpoint (`/api/events/stream`) already supports topic filtering — the frontend subscribes to `flow.>` topics for live execution visualization.

---

## Command bus integration

Flows are invokable via the same command syntax as workflows:

```
run flow/my-flow --site Sydney
status frun-abc123
cancel frun-abc123
approve frun-abc123                      (approves the currently waiting node)
approve frun-abc123 --node approval-1    (approve a specific node)
reject frun-abc123 --feedback "too verbose"
list flows
list flow-runs --status running
```

The command parser's `executeRun` learns a new target kind: `flow`. The router dispatches to the flow engine instead of the workflow runner.

---

## React Flow frontend

### Node palette

The left sidebar shows available node types grouped by category, populated by `GET /api/flows/node-types`:

```
Flow Control
  ├── Trigger
  ├── Condition
  ├── Switch
  ├── Merge
  ├── ForEach
  ├── Delay
  ├── Approval
  └── Output

Tools (from installed apps)
  ├── rubix.query_nodes
  ├── rubix.control_point
  └── weather.get_weather

Plugins
  ├── plugin.ml-service.analyze
  └── plugin.scraper.scrape

Integrations
  ├── AI Prompt
  ├── AI Runner
  ├── Slack Send
  ├── Email Send
  └── Webhook Call

Data
  ├── Transform
  ├── Set Variable
  └── Log
```

### Canvas

Standard React Flow canvas with:
- **Drag-and-drop** from palette to canvas
- **Edge drawing** by dragging from output port to input port
- **Node config panel** — click a node to open its settings in a right sidebar (rendered from JSON Schema, like Rubix RJSF)
- **Minimap** for large flows
- **Undo/redo** (local state)
- **Auto-layout** button (dagre or elk layout algorithm)

### Node rendering

Each node type has a custom React Flow node component:

```
┌──────────────────────┐
│  ⚙ query_nodes       │  ← label + icon
│  rubix               │  ← source app
├──────────────────────┤
│  ● filter            │  ← input ports (left side)
│  ● limit             │
├──────────────────────┤
│           result  ●  │  ← output ports (right side)
│           error   ●  │
└──────────────────────┘
```

Flow control nodes have distinct visual styling (colour-coded by category). Condition/switch nodes show the expression. Approval nodes show the prompt text.

### Live execution view

When a flow is running, the canvas shows real-time state:
- **Pending** nodes: grey
- **Running** nodes: blue pulse animation
- **Completed** nodes: green with checkmark
- **Failed** nodes: red with error tooltip
- **Waiting** nodes: amber with "Approve / Reject" buttons inline
- **Edges** animate (dashed moving line) when data is flowing through them

Powered by SSE subscription to `flow.>` events. The frontend maps `flow.node.started` / `flow.node.completed` events to node state colours.

---

## Validation rules

The `POST /api/flows/validate` and pre-save validation check:

| Rule | Description |
|---|---|
| Exactly one trigger | Every flow must have exactly one `trigger` node |
| No cycles | Edges must form a DAG (exception: `foreach` subgraphs are internally cyclic but bounded) |
| All required ports connected | If a port is marked `required`, it must have an incoming edge |
| All node types registered | Every node's `type` must exist in the registry |
| Edge type compatibility | Source port type must be compatible with target port type (or either is `any`) |
| At least one terminal | Flow must have at least one `output` or `error` terminal node |
| Expressions compile | All `expr` expressions in condition/switch/transform nodes must compile without error (caught at save time, not runtime) |
| Trigger config valid | If trigger type is `cron`, schedule must be valid cron syntax |
| ForEach subgraph boundaries | Nodes reachable from a foreach `item` port must not have external inputs (other than from the foreach itself). Subgraph node IDs are computed and stored in `node.Data["_subgraph_nodes"]` during validation. |
| Resource limits | ForEach `max_iterations` must be ≤ 10000. Engine-level `max_parallel_nodes` is enforced at runtime (default 50). |

---

## File layout

```
pkg/flow/
    engine.go          — DAG executor: topological sort, parallel execution, node dispatch
    store.go           — FlowDef CRUD (SQLite via GORM)
    types.go           — FlowDef, FlowNodeDef, FlowEdgeDef, FlowRun, NodeState
    registry.go        — NodeTypeRegistry: built-in types + dynamic tool/plugin types
    validate.go        — Graph validation (cycles, ports, types)
    nodes.go           — Built-in node executors (condition, switch, merge, foreach, delay, etc.)
    integrations.go    — Integration node executors (slack-send, email-send, webhook-call, ai-prompt, ai-runner)
    events.go          — FlowEvent struct, topic constants, bus publishing
    expr.go            — Sandboxed expression evaluation (wraps expr-lang/expr) for conditions, switches, and transforms

pkg/api/
    flows_handler.go   — REST endpoints for flow CRUD + execution + node types
    flows_bridge.go    — Bridge types connecting flow engine to command router

pkg/models/
    flow.go            — FlowDef, FlowRun DB models

frontend/src/
    pages/
        flow-editor.tsx        — Main flow editor page
    components/
        flow/
            canvas.tsx         — React Flow canvas wrapper
            node-palette.tsx   — Draggable node type sidebar
            node-config.tsx    — Right panel: node settings (JSON Schema form)
            custom-nodes/      — Per-type React Flow node components
                tool-node.tsx
                condition-node.tsx
                approval-node.tsx
                merge-node.tsx
                trigger-node.tsx
                integration-node.tsx
                ai-runner-node.tsx
            execution-overlay.tsx  — Live run state visualisation
            flow-toolbar.tsx       — Save, validate, run, auto-layout buttons
```

---

## How flows relate to existing workflows

Both coexist. Linear YAML workflows continue to work unchanged.

| | YAML Workflows | Visual Flows |
|---|---|---|
| **Definition** | YAML files on disk | JSON in database (React Flow format) |
| **Editor** | Text editor | React Flow canvas |
| **Execution** | Sequential pipeline (`workflow.Runner`) | DAG engine with parallel execution (`flow.Engine`) |
| **Data passing** | `{{previous}}`, `{{save_as}}` template vars | Port-to-port via edges |
| **Branching** | Single conditional stage (switch/case) | Full condition/switch nodes with multiple output paths |
| **Parallelism** | None | Fan-out to independent nodes, merge for fan-in |
| **Iteration** | None | ForEach node |
| **Bus events** | `workflow.>` | `flow.>` |
| **Command syntax** | `run workflow/<name>` | `run flow/<name>` |
| **Triggers** | External only (cron adapter, etc.) | Embedded in definition |

A future migration path could convert YAML workflows to flow definitions automatically (each stage becomes a node, linear edges between them). No rush — both systems work.

---

## Build order

| Phase | What | Dependencies |
|---|---|---|
| **1** | `pkg/flow/types.go` + `store.go` + `pkg/models/flow.go` | None |
| **2** | `pkg/flow/registry.go` + `validate.go` | Types |
| **3** | `pkg/api/flows_handler.go` — CRUD + node-types + validate endpoints | Store, registry |
| **4** | Frontend: `flow-editor.tsx`, palette, canvas, node components | API endpoints |
| **5** | `pkg/flow/engine.go` + `nodes.go` — core DAG execution (trigger, tool, condition, merge) | Store |
| **6** | `pkg/flow/events.go` + bus integration | Engine, NATS bus |
| **7** | `pkg/flow/integrations.go` — slack-send, email-send, webhook-call, ai-prompt | Engine, adapters |
| **8** | Frontend: execution overlay (live node state via SSE) | Events |
| **9** | Approval gate (engine + frontend inline buttons) | Engine |
| **10** | ForEach, delay, transform nodes | Engine |
| **11** | Trigger definitions (auto-register with cron/webhook/event adapters) | Engine, adapters |
| **12** | Command bus integration (`run flow/<name>`) | Engine, command router |
| **13** | `ai-runner` integration node — wires `pkg/airunner.Runner` into the flow engine | Engine, airunner, AgentService |
| **14** | `coding-tasks` app — task CRUD + select/list-selected tools | App system |
| **15** | Slack progress streaming — subscribe `flow.node.progress` → Slack thread | Slack adapter, events |

Phases 1-4 deliver a **working visual editor** — users can design and save flows. Phase 5 makes them executable. Everything after is incremental capability. Phases 13-15 enable the automated coding workflow.

---

## Dependencies

| Concern | Library | Notes |
|---|---|---|
| React Flow | `@xyflow/react` | Canvas, nodes, edges, minimap, controls |
| Auto-layout | `@dagrejs/dagre` or `elkjs` | Automatic node positioning |
| JSON Schema forms | `@rjsf/core` + `@rjsf/shadcn` | Node config panels (same approach as Rubix) |
| Expression eval | `github.com/expr-lang/expr` | Sandboxed expression evaluation in condition/switch/transform nodes |
| DAG validation | Built-in (Kahn's algorithm) | Cycle detection, topological sort |

One new Go dependency (`expr-lang/expr`) — the engine otherwise uses existing NATS, GORM, and stdlib.
