# Multi-App Staged Workflows

Apps can define **multi-step workflows** that chain tools from multiple apps, execute stage-by-stage, and stop on failure with clear feedback to the user.

---

## How it works

A workflow is a YAML file that declares an ordered list of stages. The engine executes them top-to-bottom, piping data between stages via template variables. If a stage fails, the workflow stops (or retries/skips, depending on configuration). The user can track progress by polling a REST endpoint.

```
[research] ──ok──> [draft] ──ok──> [review] ──approved──> [render] ──ok──> [complete]
     │                 │               │                      │
     └──fail──> STOP   └──fail──> STOP └──rejected──> STOP   └──fail──> STOP
```

---

## Workflow definition

Workflows live in a `workflows/` directory inside an app:

```
apps/
  sales-brochure/
    app.yaml
    workflows/
      create-brochure.yaml
      quarterly-report.yaml
```

Each `.yaml` file defines one workflow. The server loads them automatically at startup by scanning every app directory for `workflows/*.yaml`.

### Full example

```yaml
# apps/sales-brochure/workflows/create-brochure.yaml

name: create-brochure
description: Generate a professional product sales brochure
depends: [nube-hardware, pdf-export]

inputs:
  - name: product
    description: Product name (e.g. "Rubix Compute")
    required: true
  - name: format
    description: Output format
    options: [pdf, docx]
    default: pdf

stages:
  - name: research
    description: Look up product specifications
    tool: nube-hardware.get_product
    params:
      product: "{{inputs.product}}"
    save_as: product_data
    on_fail: stop

  - name: draft
    description: Write brochure content
    prompt: |
      Write a professional sales brochure for {{product_data.name}}.

      Product specs:
      {{product_data.specs}}

      Include: hero statement, key features (3-5), technical specs table,
      use cases, and a call to action. Write in marketing tone.
    save_as: brochure_content
    on_fail: stop

  - name: review
    description: User reviews the draft
    approval: true
    show: "{{brochure_content}}"
    on_reject: stop

  - name: render
    description: Render to final format
    tool: "{{inputs.format}}-export.render"
    params:
      content: "{{brochure_content}}"
      template: brochure
    save_as: output_file
    on_fail: stop

  - name: complete
    description: Deliver the result
    output:
      message: "Brochure ready: {{output_file.url}}"
      file: "{{output_file}}"
```

---

## Stage types

| Type | Field | What it does |
|---|---|---|
| **tool** | `tool: app.tool_name` | Calls a JS tool from any installed app |
| **prompt** | `prompt: "..."` | Sends a prompt to the user's default AI provider |
| **approval** | `approval: true` | Pauses and waits for the user to approve, reject, or cancel |
| **output** | `output: { message, file, content }` | Delivers the final result |
| **conditional** | `type: conditional` | Branches on a value via `switch`/`cases` |

A stage must set exactly one of these. The engine validates this at load time.

### Common stage fields

| Field | Description |
|---|---|
| `name` | Unique name within the workflow (required) |
| `description` | Human-readable label shown in UI/CLI |
| `params` | Key-value map passed to tool stages, supports `{{templates}}` |
| `save_as` | Variable name to store this stage's output for later stages |
| `on_fail` | What to do on failure: `stop` (default), `retry`, `skip` |
| `on_reject` | For approval stages: `stop` (default) or `retry` |
| `max_retries` | Max retry attempts when `on_fail: retry` (default: 3) |
| `timeout_sec` | Per-stage timeout in seconds (0 = no timeout) |
| `show` | Template string rendered and shown to user on approval stages |

---

## Template variables

Stages reference data from earlier stages using `{{variable}}` syntax with dot-path support.

| Variable | Source |
|---|---|
| `{{inputs.product}}` | User-provided input |
| `{{product_data}}` | Output of the stage with `save_as: product_data` |
| `{{product_data.name}}` | Nested field access |
| `{{previous}}` | Output of the immediately preceding stage |
| `{{rejection_feedback}}` | User feedback from a rejected approval stage |

Templates are resolved immediately before each stage executes. A stage cannot reference a future stage. Unresolvable variables are left as-is.

---

## Failure handling

| `on_fail` | Behaviour |
|---|---|
| `stop` | Halt the workflow, mark the stage as failed, record the error (default) |
| `retry` | Re-run the stage up to `max_retries` times, then stop if still failing |
| `skip` | Log the error on the stage, mark it as skipped, continue to next stage |

When a workflow stops on failure, the run record stores `failed_at` (stage name) and `error` (message). The UI/CLI can show exactly where and why it broke.

---

## Conditional stages

Branch based on an input or a previous stage's output:

```yaml
- name: choose-format
  type: conditional
  switch: "{{inputs.format}}"
  cases:
    pdf:
      tool: pdf-export.render
      params: { content: "{{brochure_content}}" }
    docx:
      tool: docx-export.render
      params: { content: "{{brochure_content}}" }
  save_as: output_file
  on_fail: stop
```

Each case can define a `tool` or `prompt` with `params`. If the resolved switch value doesn't match any case, the stage fails.

---

## REST API

All endpoints require authentication (Bearer token).

### Start a workflow

The client generates a UUID and sends it as `workflow_id`. This is an idempotency key -- submitting the same ID twice returns the existing run.

```
POST /api/workflows/run
{
  "workflow_id": "wf-abc123",
  "app": "sales-brochure",
  "workflow": "create-brochure",
  "inputs": {
    "product": "Rubix Compute",
    "format": "pdf"
  }
}

-> 202 Accepted
{
  "workflow_id": "wf-abc123",
  "status": "running",
  "current_stage": "research"
}
```

### Poll status

Poll `GET /api/workflows/:id` on a 1-2s interval. The `version` counter increments on every stage transition -- skip re-renders when it hasn't changed. Stop polling when `status` is `completed`, `failed`, or `cancelled`.

```
GET /api/workflows/wf-abc123

-> 200
{
  "id": "wf-abc123",
  "app": "sales-brochure",
  "workflow": "create-brochure",
  "status": "waiting_approval",
  "version": 4,
  "current_idx": 2,
  "stages": [
    { "name": "research",  "status": "completed", "duration_ms": 1200 },
    { "name": "draft",     "status": "completed", "duration_ms": 8500 },
    { "name": "review",    "status": "waiting",   "output": "...brochure text..." },
    { "name": "render",    "status": "pending" },
    { "name": "complete",  "status": "pending" }
  ],
  "user_id": "usr-abc",
  "created_at": "2026-04-17T08:00:00Z"
}
```

### Approve or reject

```
POST /api/workflows/wf-abc123/approve
{ "action": "approve" }

POST /api/workflows/wf-abc123/approve
{ "action": "reject", "feedback": "Make the tone less formal" }

POST /api/workflows/wf-abc123/approve
{ "action": "cancel" }
```

### List runs

```
GET /api/workflows?app=sales-brochure&status=running

-> 200
{ "runs": [ ... ] }
```

### Cancel

```
POST /api/workflows/wf-abc123/cancel

-> 200
{ "workflow_id": "wf-abc123", "status": "cancelled" }
```

### List definitions

```
GET /api/workflows/definitions

-> 200
[
  {
    "app": "sales-brochure",
    "name": "create-brochure",
    "description": "Generate a professional product sales brochure",
    "depends": ["nube-hardware", "pdf-export"],
    "inputs": [ { "name": "product", "required": true }, ... ],
    "stage_count": 5
  }
]
```

---

## CLI

```bash
# Run a workflow (polls until done, interactive approval)
nube workflow run sales-brochure create-brochure -i product="Rubix Compute" -i format=pdf

# Output:
# Workflow started: wf-1713340800000000000
# Status: running
# * research -- research... done (1.2s)
# * draft -- draft... done (8.5s)
# * review -- waiting for approval
#   "The Rubix Compute is NubeIO's flagship..."
#   Approve? (y/n): y
# * render -- render... done (2.1s)
# * complete -- complete... done (0.0s)
# Workflow completed.

# List runs
nube workflow list

# Check status
nube workflow status wf-abc123

# Cancel
nube workflow cancel wf-abc123

# List available workflow definitions
nube workflow definitions
```

The CLI also supports `nube wf` as a shorthand alias.

On failure:

```bash
# * research -- done (1.2s)
# * draft -- done (8.5s)
# * review -- waiting for approval
#   Approve? (y/n): y
# x render -- FAILED
#   Error: pdf-export app returned: "template 'brochure' not found"
```

---

## Data model

Persisted to `data/workflow_runs.json` via jsondb.

```go
type WorkflowRun struct {
    ID         string         `json:"id"`          // client-generated UUID (idempotency key)
    AppName    string         `json:"app"`
    Workflow   string         `json:"workflow"`
    Inputs     map[string]any `json:"inputs"`
    Status     WorkflowStatus `json:"status"`      // running, waiting_approval, completed, failed, cancelled
    Version    int            `json:"version"`      // increments on every stage transition
    Stages     []StageResult  `json:"stages"`
    CurrentIdx int            `json:"current_idx"`  // index of currently executing stage
    FailedAt   string         `json:"failed_at"`    // stage name that failed
    Error      string         `json:"error"`
    UserID     string         `json:"user_id"`
    CreatedAt  time.Time      `json:"created_at"`
    FinishedAt *time.Time     `json:"finished_at"`
}

type StageResult struct {
    Name       string     `json:"name"`
    Status     StageStatus `json:"status"`   // pending, running, completed, failed, skipped, waiting
    Output     any        `json:"output"`    // result data (available to later stages via save_as)
    Error      string     `json:"error"`
    DurationMS int        `json:"duration_ms"`
    StartedAt  *time.Time `json:"started_at"`
}
```

---

## Architecture

```
cmd/nube-server/main.go          -- loads workflow_runs.json, scans app dirs, wires up runner
pkg/workflow/types.go             -- WorkflowDef, StageDef, InputDef (YAML schema)
pkg/workflow/loader.go            -- Store: loads workflows/*.yaml from app dirs, validation
pkg/workflow/template.go          -- {{variable}} resolution with dot-path support
pkg/workflow/runner.go            -- Runner: executes stages, manages active runs, persistence
pkg/models/workflow.go            -- WorkflowRun, StageResult (persisted types)
pkg/api/workflows.go              -- REST handlers + ToolCaller/PromptRunner bridges
pkg/cli/cmd_workflow.go           -- CLI: run, list, status, cancel, definitions
cmd/nube/openapi.yaml             -- OpenAPI spec for all workflow endpoints
```

The runner uses two interfaces to call into the rest of the system:

- **ToolCaller** -- resolves and executes JS tools via the existing `resolveJSTool` path
- **PromptRunner** -- sends prompts to the user's default AI provider via the existing `airunner.Runner` interface

Both are implemented as bridges in `pkg/api/workflows.go` that delegate to the `API` struct.

---

## UI integration

The workflow maps to a stepper component. Poll `GET /api/workflows/:id` and render from the `stages` array:

```
+-----------------------------------------------------------+
|  Create Sales Brochure                                    |
|                                                           |
|  [check] Research -- [check] Draft -- [*] Review -- [ ] Render  |
|                                                           |
|  +-----------------------------------------------------+ |
|  |  Draft Brochure                                      | |
|  |                                                      | |
|  |  The Rubix Compute is NubeIO's flagship...           | |
|  +-----------------------------------------------------+ |
|                                                           |
|  [Approve]   [Reject with feedback]   [Cancel]            |
+-----------------------------------------------------------+
```

- `completed` stages: green check
- `running` stage: spinner
- `waiting` stage: show output + approval buttons
- `failed` stage: red with error message
- `pending` stages: grey

---

## More examples

### Weekly site report

```yaml
name: weekly-report
description: Generate weekly operations report for management
depends: [rubix]

inputs:
  - name: site
    description: Site name
    required: true

stages:
  - name: devices
    description: Get device status summary
    tool: rubix.query_nodes
    params: { site: "{{inputs.site}}", summary: true }
    save_as: device_summary
    on_fail: stop

  - name: alarms
    description: Get alarm history for the week
    tool: rubix.get_alarms
    params: { site: "{{inputs.site}}", period: "7d" }
    save_as: alarm_data
    on_fail: skip

  - name: write
    description: Write the report
    prompt: |
      Write a concise weekly operations report for {{inputs.site}}.

      Device summary:
      {{device_summary}}

      Alarms (last 7 days):
      {{alarm_data}}

      Format: executive summary (3 bullets), then detail sections.
      Flag anything that needs immediate attention.
    save_as: report
    on_fail: stop

  - name: review
    approval: true
    show: "{{report}}"
    on_reject: stop

  - name: deliver
    output:
      message: "Weekly report for {{inputs.site}} ready."
      content: "{{report}}"
```

### Onboard new device

```yaml
name: onboard-device
description: Add a new device with validation
depends: [rubix, nube-admin]

inputs:
  - name: device_ip
    required: true
  - name: floor
    required: true

stages:
  - name: ping
    description: Verify device is reachable
    tool: nube-admin.ping_host
    params: { host: "{{inputs.device_ip}}" }
    on_fail: stop

  - name: discover
    description: Discover device type and points
    tool: rubix.discover_device
    params: { ip: "{{inputs.device_ip}}" }
    save_as: device_info
    on_fail: stop

  - name: confirm
    description: Confirm device details before adding
    approval: true
    show: |
      Found: {{device_info.type}} at {{inputs.device_ip}}
      Points: {{device_info.point_count}}
      Floor: {{inputs.floor}}

      Add this device?
    on_reject: stop

  - name: register
    description: Register device in Rubix
    tool: rubix.create_node
    params:
      ip: "{{inputs.device_ip}}"
      type: "{{device_info.type}}"
      floor: "{{inputs.floor}}"
    save_as: node
    on_fail: stop

  - name: complete
    output:
      message: "Device {{node.name}} added to floor {{inputs.floor}}."
```

---

## Design constraints

- **Not Turing-complete.** No loops, no arbitrary branching. Stages run top-to-bottom with optional conditionals. Complex logic belongs in a JS tool.
- **AI doesn't control flow.** A `prompt` stage uses the AI for content generation, but the workflow structure is deterministic. The AI never decides which stage runs next.
- **Retries have limits.** `on_fail: retry` always caps at `max_retries` (default 3). No infinite retry loops.
- **Approval gates for destructive actions.** Any stage that writes data, sends emails, or modifies devices should have an approval stage before it.
