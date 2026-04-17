# Multi-App Staged Workflows

Goal: let apps define **multi-step workflows** that chain tools from multiple apps, execute stage-by-stage, and stop on failure with clear feedback to the user.

---

## The problem

Right now an app has tools and prompts. The AI figures out which tools to call by reading the preamble. This works for simple tasks but breaks down when:

- A task has a **strict order** (you can't render a PDF before writing the content)
- A step **must succeed** before the next one runs (don't email a brochure that failed to render)
- The user needs to **see progress** stage by stage, not just a final blob of text
- Multiple apps need to **cooperate** and the AI doesn't know the correct sequence

Workflows solve this by making the stages explicit, executable, and observable.

---

## Example: sales brochure workflow

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
    on_reject: retry   # go back to draft stage with user feedback

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

## Core concepts

### Stages

A workflow is an ordered list of stages. Each stage does exactly one thing:

| Stage type | What it does |
|---|---|
| `tool` | Calls an MCP tool from any installed app |
| `prompt` | Sends a prompt to the AI and captures the response |
| `approval` | Pauses and shows the user a result, waits for approve/reject |
| `output` | Delivers the final result to the user |
| `conditional` | Branches based on a value (e.g. pick PDF vs DOCX path) |

### Stage execution

```
[research] ──ok──> [draft] ──ok──> [review] ──approved──> [render] ──ok──> [complete]
     │                 │               │                      │
     └──fail──> STOP   └──fail──> STOP └──rejected──> [draft] └──fail──> STOP
```

Every stage produces a result. The result is saved to a named variable (`save_as`) and available to all later stages via `{{variable_name}}`. If a stage fails and `on_fail: stop` is set, the workflow halts and the user sees exactly which stage failed and why.

### Failure handling

Each stage declares what happens on failure:

| `on_fail` | Behaviour |
|---|---|
| `stop` | Halt workflow, show error to user (default) |
| `retry` | Re-run this stage (with optional max retries) |
| `skip` | Log warning, continue to next stage |
| `fallback` | Run a named fallback stage instead |

### Approval gates

The `approval` stage type pauses the workflow and presents output to the user. The user can:

- **Approve** — continue to next stage
- **Reject with feedback** — go back to a previous stage (defined by `on_reject`) with the user's feedback injected into the prompt
- **Cancel** — stop the workflow entirely

This is critical for document generation, email sending, anything where the user needs to see before committing.

---

## Data flow between stages

Every stage has access to:

| Variable | Source |
|---|---|
| `{{inputs.*}}` | User-provided inputs from workflow start |
| `{{stage_name}}` or `{{save_as_name}}` | Output of a completed stage |
| `{{previous}}` | Output of the immediately preceding stage |
| `{{memory.server}}` | Server memory (same as AI conversations) |
| `{{memory.user}}` | User memory |

Results flow forward only — a stage cannot reference a future stage. The runtime resolves templates before executing each stage.

---

## Workflow file format

```
apps/
  sales-brochure/
    app.yaml                    # declares depends: [nube-hardware, pdf-export]
    workflows/
      create-brochure.yaml      # the workflow definition
      quarterly-report.yaml     # another workflow
    preamble                    # optional: AI context when running prompts in this workflow
```

Workflows live in a `workflows/` directory inside the app. Each `.yaml` file is one workflow. The app's `app.yaml` declares dependencies so the system knows which apps must be installed.

### app.yaml additions

```yaml
name: sales-brochure
version: 1.0.0
description: Product brochure generator
depends:
  - nube-hardware
  - pdf-export
  - docx-export
```

The `depends` field lists app names. At install time, the system checks all dependencies are available. Missing deps block installation with a clear message.

---

## API

### Run a workflow

The client generates a UUID and sends it as `workflow_id`. This serves as an idempotency key — if the same ID is submitted twice, the server returns the existing run instead of creating a duplicate.

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

→ 202 Accepted
{
  "workflow_id": "wf-abc123",
  "status": "running",
  "current_stage": "research"
}
```

### Approve / reject a stage

```
POST /api/workflows/wf-abc123/approve
{ "action": "approve" }

POST /api/workflows/wf-abc123/approve
{ "action": "reject", "feedback": "Make the tone less formal" }
```

### List workflow runs

```
GET /api/workflows?app=sales-brochure&status=running

→ 200
{
  "runs": [
    { "workflow_id": "wf-abc123", "app": "sales-brochure", "workflow": "create-brochure", "status": "running", "current_stage": "draft", "created_at": "..." },
    ...
  ]
}
```

### Cancel a workflow

```
POST /api/workflows/wf-abc123/cancel

→ 200
{ "workflow_id": "wf-abc123", "status": "cancelled" }
```

### Polling pattern

The frontend polls `GET /api/workflows/:id` on an interval (e.g. 1-2s) to track progress. Each response includes a `version` counter that increments on every stage transition — the client can skip re-renders when the version hasn't changed.

```
GET /api/workflows/wf-abc123

→ 200
{
  "workflow_id": "wf-abc123",
  "version": 4,
  "status": "waiting_approval",
  "current_stage": "review",
  "stages": [
    { "name": "research",  "status": "completed", "duration_ms": 1200 },
    { "name": "draft",     "status": "completed", "duration_ms": 8500 },
    { "name": "review",    "status": "waiting",   "output": "...brochure text..." },
    { "name": "render",    "status": "pending" },
    { "name": "complete",  "status": "pending" }
  ]
}
```

The UI renders a progress stepper from the `stages` array. Poll while `status` is `running` or `waiting_approval`. Stop polling on `completed`, `failed`, or `cancelled`.

---

## CLI

```bash
nube workflow run sales-brochure create-brochure --product "Rubix Compute" --format pdf

# Output:
# ● research — Look up product specifications... done (1.2s)
# ● draft — Write brochure content... done (8.5s)
# ● review — User reviews the draft
#   [draft content shown here]
#   Approve? (y/n/feedback): y
# ● render — Render to PDF... done (2.1s)
# ✓ complete — Brochure ready: /tmp/rubix-compute-brochure.pdf

nube workflow list                    # list available workflows
nube workflow status wf-abc123        # check running workflow
nube workflow cancel wf-abc123        # cancel running workflow
```

On failure:

```bash
# ● research — Look up product specifications... done (1.2s)
# ● draft — Write brochure content... done (8.5s)
# ● review — Approved
# ✗ render — Render to PDF... FAILED
#   Error: pdf-export app returned: "template 'brochure' not found"
#   Workflow stopped at stage 4/5.
```

---

## UI (frontend)

The workflow maps naturally to a **stepper component**:

```
┌─────────────────────────────────────────────────────────┐
│  Create Sales Brochure                                  │
│                                                         │
│  ✓ Research ──── ✓ Draft ──── ● Review ──── ○ Render    │
│                                                         │
│  ┌─────────────────────────────────────────────────┐    │
│  │  Draft Brochure                                 │    │
│  │                                                 │    │
│  │  The Rubix Compute is NubeIO's flagship...      │    │
│  │  ...                                            │    │
│  │                                                 │    │
│  └─────────────────────────────────────────────────┘    │
│                                                         │
│  [Approve]   [Reject with feedback]   [Cancel]          │
└─────────────────────────────────────────────────────────┘
```

On failure, the failed stage is highlighted red with the error message. Completed stages stay green. The user can see exactly where it broke.

---

## Data model

### WorkflowRun (persisted to workflows.json)

```go
type WorkflowRun struct {
    ID         string          `json:"id"`          // client-generated UUID, also serves as idempotency key
    AppName    string          `json:"app"`
    Workflow   string          `json:"workflow"`
    Inputs     map[string]any  `json:"inputs"`
    Status     string          `json:"status"`      // running, waiting_approval, completed, failed, cancelled
    Version    int             `json:"version"`     // increments on every stage transition, for efficient polling
    Stages     []StageResult   `json:"stages"`
    FailedAt   string          `json:"failed_at"`   // stage name that failed
    Error      string          `json:"error"`
    UserID     string          `json:"user_id"`
    CreatedAt  time.Time       `json:"created_at"`
    FinishedAt *time.Time      `json:"finished_at"`
}

type StageResult struct {
    Name       string     `json:"name"`
    Status     string     `json:"status"`    // pending, running, completed, failed, skipped, waiting
    Output     any        `json:"output"`    // result data (saved for template resolution)
    Error      string     `json:"error"`
    DurationMS int        `json:"duration_ms"`
    StartedAt  *time.Time `json:"started_at"`
}
```

---

## Conditional stages

For branching logic based on inputs or previous results:

```yaml
stages:
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

---

## Relationship to existing features

| Existing feature | Workflow equivalent | Difference |
|---|---|---|
| Single tool call | One `tool` stage | Workflow adds ordering + failure handling |
| Prompt template | One `prompt` stage | Workflow adds data piping between stages |
| Agent run (`nube ask`) | Free-form AI decides | Workflow is structured, deterministic order |
| QA mode tool | `approval` stage | Workflow integrates approval into a pipeline |

Workflows don't replace `nube ask`. They complement it. Use `nube ask` when the AI should figure things out. Use workflows when the steps are known and must execute reliably.

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
    on_fail: skip    # alarms are optional, continue without them

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
    on_reject: retry

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
    on_fail: stop    # no point continuing if device is unreachable

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

## Implementation plan

### Phase 1: Workflow engine (core)

- Parse `workflows/*.yaml` files during app loading
- `WorkflowRunner` that executes stages sequentially
- Template resolution (`{{variable}}` substitution)
- `on_fail: stop` halts and reports which stage failed
- Persist `WorkflowRun` to `workflows.json`
- Client-generated UUID as idempotency key (prevent duplicate runs)
- REST API: `POST /api/workflows/run`, `GET /api/workflows/:id`, `POST /api/workflows/:id/cancel`

### Phase 2: Approval gates + polling

- `approval` stage type — pauses and waits for user input
- `POST /api/workflows/:id/approve` for approve/reject
- `version` counter on `WorkflowRun` for efficient polling
- `GET /api/workflows` list endpoint with status/app filters
- CLI: interactive approval prompt

### Phase 3: App dependencies

- `depends` field in `app.yaml`
- Install-time dependency validation
- Uninstall warning when dependents exist
- Store UI shows dependency chain

### Phase 4: Conditional stages + retry

- `conditional` stage with `switch/cases`
- `on_fail: retry` with max retries and backoff
- `on_fail: fallback` to named fallback stage
- Parallel stages (stages that don't depend on each other)

---

## What NOT to do

- **Don't make workflows Turing-complete** — no loops, no arbitrary branching. Stages run top-to-bottom with optional conditionals. If you need complex logic, write a tool in JS.
- **Don't mix workflow and free-form AI** — a workflow's `prompt` stage uses the AI, but the workflow *structure* is deterministic. The AI doesn't decide which stage runs next.
- **Don't auto-retry without limits** — always require `max_retries` on retry stages. Infinite retry loops are a cost and rate-limit disaster.
- **Don't skip approval for destructive actions** — any stage that writes data, sends emails, or modifies devices should have an approval gate (or at least be flagged in the workflow definition).
