# Job Engine — Trigger-driven job library

A standalone, reusable library in `pkg/jobengine/` for running jobs from external triggers. Decoupled from the rest of bizzy — no dependency on apps, MCP, AI runners, or HTTP handlers. Handlers bridge to those at wiring time.

---

## Problem

Bizzy can run AI tools via REST, WebSocket, and workflows — but only when a user explicitly asks. There's no way to say "when a Slack message arrives, run this tool" or "every morning, email me a site report". We need a generic trigger-to-action engine that the existing system can plug into.

---

## Core concepts

```
Trigger ──fires──▶ Event ──routed──▶ Handler ──produces──▶ Result
                                                              │
                                                         stored in
                                                          JobStore
```

| Concept | What it is |
|---|---|
| **Trigger** | Something that fires: a cron tick, a Slack message, a new Gmail, a webhook POST, a manual API call |
| **Event** | The payload a trigger produces: who sent it, what channel, what text, what email, etc. |
| **Handler** | What runs when a trigger fires: call a tool, run an AI prompt, send a reply, start a workflow |
| **Job** | A tracked execution: trigger event in, result out, with status, duration, errors |
| **Engine** | The orchestrator: registers triggers + handlers, routes events, tracks jobs |

---

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                       Engine                            │
│                                                         │
│   Triggers                         Handlers             │
│   ┌──────────┐                    ┌──────────────────┐  │
│   │ Cron     │──▶ Event ────────▶ │ ToolCallHandler  │  │
│   │ Slack    │──▶ Event ────────▶ │ AIPromptHandler  │  │
│   │ Gmail    │──▶ Event ────────▶ │ WebhookHandler   │  │
│   │ Webhook  │──▶ Event ────────▶ │ CustomFunc       │  │
│   │ Manual   │──▶ Event ────────▶ │                  │  │
│   └──────────┘                    └──────────────────┘  │
│         │                                │              │
│         └────────── Job Store ───────────┘              │
│                   (SQLite table)                        │
└─────────────────────────────────────────────────────────┘
         │                                    │
         ▼                                    ▼
   External services                  Bizzy internals
   (Slack API, Gmail,                 (ToolService, AgentService,
    SMTP, webhooks)                    WorkflowRunner)
```

---

## Interfaces

```go
// Trigger watches an external source and fires events.
type Trigger interface {
    Name() string
    Start(ctx context.Context, fire func(Event)) error
    Stop() error
}

// Event is the payload a trigger produces.
type Event struct {
    ID          string         `json:"id"`
    TriggerName string         `json:"trigger"`
    Kind        string         `json:"kind"`         // "cron.tick", "slack.message", "gmail.received"
    Data        map[string]any `json:"data"`         // trigger-specific payload
    ReceivedAt  time.Time      `json:"received_at"`
}

// Handler processes an event and returns a result.
type Handler interface {
    Name() string
    Handle(ctx context.Context, event Event) (Result, error)
}

// HandlerFunc adapts a plain function to the Handler interface.
type HandlerFunc func(ctx context.Context, event Event) (Result, error)

// Result is what a handler returns.
type Result struct {
    Output  map[string]any `json:"output,omitempty"`
    Message string         `json:"message,omitempty"`
}
```

---

## Engine API

```go
engine := jobengine.New(db)

// Register triggers
engine.AddTrigger(cron.New("daily-report", "0 9 * * *"))
engine.AddTrigger(slack.New(slackCfg))
engine.AddTrigger(gmail.New(gmailCfg))

// Route: when trigger fires, run handler
engine.On("cron:daily-report", reportHandler)
engine.On("slack", slackRouter)
engine.On("gmail", emailTriageHandler)

// Start all triggers (blocks until ctx cancelled)
engine.Start(ctx)

// Manual fire (for API/testing)
engine.Fire(Event{TriggerName: "manual", Kind: "manual.fire", Data: params})

// Query job history
jobs, _ := engine.Jobs(JobFilter{Trigger: "slack", Status: "error", Limit: 50})
```

---

## Triggers

### Cron

Fires on a schedule. Uses cron expressions or simple intervals.

```go
cron.New("daily-report", "0 9 * * *")       // 9am daily
cron.NewInterval("health-check", 5*time.Minute) // every 5 min
```

Event:
```json
{"trigger": "cron:daily-report", "kind": "cron.tick", "data": {"scheduled_at": "..."}}
```

### Slack

Listens for Slack events via Socket Mode (no public URL needed) or Events API.

```go
slack.New(slack.Config{
    BotToken: "xoxb-...",
    AppToken: "xapp-...",   // for socket mode
    Events:   []string{"message", "app_mention", "slash_command"},
})
```

Event:
```json
{
  "trigger": "slack",
  "kind": "slack.message",
  "data": {
    "channel": "C0123ABC",
    "user": "U0123XYZ",
    "text": "check device status for floor 3",
    "thread_ts": "1713340800.000100"
  }
}
```

The handler can reply back via the Slack API — the event `Data` includes enough context to post a response.

### Gmail

Polls for new emails matching a query. Tracks last-seen message ID to avoid duplicates.

```go
gmail.New(gmail.Config{
    CredentialsJSON: []byte("..."),   // OAuth2 service account or user creds
    PollInterval:    2 * time.Minute,
    Query:           "is:unread label:support",
    MarkRead:        true,            // mark as read after processing
})
```

Event:
```json
{
  "trigger": "gmail",
  "kind": "gmail.received",
  "data": {
    "message_id": "msg-abc123",
    "from": "client@example.com",
    "subject": "Alarm on Floor 5",
    "body": "We're seeing high temps...",
    "labels": ["support", "unread"]
  }
}
```

### Webhook

Exposes an HTTP endpoint. Any external service (GitHub, PagerDuty, MQTT bridge, custom) can POST to it.

```go
webhook.New(webhook.Config{
    Path:         "/hooks/jobengine",
    SecretHeader: "X-Hook-Secret",   // optional signature verification
    Secret:       "whsec_...",
})
```

Event:
```json
{
  "trigger": "webhook",
  "kind": "webhook.post",
  "data": {"headers": {...}, "body": {...}}
}
```

### Manual

No background listener. Fired programmatically via `engine.Fire()`. Used for API-triggered jobs and testing.

---

## Job tracking

Every trigger fire creates a Job record persisted to SQLite.

```go
type Job struct {
    ID          string     `json:"id"`
    TriggerName string     `json:"trigger"`
    HandlerName string     `json:"handler"`
    EventKind   string     `json:"event_kind"`
    Status      string     `json:"status"`       // pending, running, done, error
    Input       string     `json:"input"`         // JSON of Event.Data
    Output      string     `json:"output"`        // JSON of Result
    Error       string     `json:"error"`
    DurationMS  int        `json:"duration_ms"`
    CreatedAt   time.Time  `json:"created_at"`
    FinishedAt  *time.Time `json:"finished_at"`
}
```

### SQLite table

```sql
CREATE TABLE jobs (
    id          TEXT PRIMARY KEY,
    trigger     TEXT NOT NULL,
    handler     TEXT NOT NULL,
    event_kind  TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'pending',
    input       TEXT,          -- JSON
    output      TEXT,          -- JSON
    error       TEXT,
    duration_ms INTEGER DEFAULT 0,
    created_at  DATETIME NOT NULL,
    finished_at DATETIME
);

CREATE INDEX idx_jobs_trigger ON jobs(trigger);
CREATE INDEX idx_jobs_status  ON jobs(status);
CREATE INDEX idx_jobs_created ON jobs(created_at);
```

---

## File layout

```
pkg/jobengine/
    engine.go          -- Engine: register triggers/handlers, route events, start/stop
    trigger.go         -- Trigger interface
    handler.go         -- Handler interface, HandlerFunc adapter
    job.go             -- Job model, status constants
    store.go           -- JobStore: persist/query jobs via gorm

    triggers/
        cron.go        -- Cron trigger (go-co-op/gocron)
        slack.go       -- Slack trigger (slack-go/slack socket mode)
        gmail.go       -- Gmail trigger (Google Gmail API polling)
        webhook.go     -- Inbound webhook trigger (http handler)
        manual.go      -- Manual/programmatic trigger
```

---

## Dependencies

No new heavy frameworks. One library per concern, all actively maintained:

| Concern | Library | Why |
|---|---|---|
| Cron expressions | `go-co-op/gocron` v2 | ~7k stars, active, fluent API, built on `robfig/cron` |
| Slack events | `slack-go/slack` | ~5k stars, active, full API: socket mode, events, slash commands |
| Gmail polling | `google.golang.org/api/gmail/v1` | Official Google client, OAuth2, labels, push support |
| Email sending | `wneessen/go-mail` | ~1.3k stars, active, modern SMTP, TLS, attachments |
| Webhook verify | `standard-webhooks` Go SDK | Signature verification for inbound hooks |
| Job persistence | `gorm` (already in project) | Already a dependency — reuse for the jobs table |

### What we're NOT adding

- **No Redis / task queue** (asynq, machinery, river). The engine runs in-process. SQLite + goroutines are enough for the scale we need. If we outgrow it later, swap the store.
- **No heavy bot framework** (slacker). `slack-go/slack` gives us what we need directly.

---

## Integration with Bizzy

The engine itself knows nothing about bizzy. Handlers are the bridge:

```go
// In cmd/nube-server/main.go or a setup function:

engine := jobengine.New(db)

// Handler that calls a tool
engine.On("slack", jobengine.HandlerFunc(func(ctx context.Context, ev Event) (Result, error) {
    result, err := toolService.CallTool(userID, ev.Data["tool"].(string), ev.Data["params"])
    return Result{Output: result}, err
}))

// Handler that runs an AI prompt
engine.On("gmail", jobengine.HandlerFunc(func(ctx context.Context, ev Event) (Result, error) {
    resp := agentService.RunSync(userID, "summarise this email: "+ev.Data["body"].(string))
    // reply via go-mail
    return Result{Message: "replied"}, nil
}))

// Handler that starts a workflow
engine.On("cron:weekly-report", jobengine.HandlerFunc(func(ctx context.Context, ev Event) (Result, error) {
    workflowRunner.Start("sales-brochure", "weekly-report", map[string]any{"site": "Sydney"})
    return Result{Message: "workflow started"}, nil
}))
```

---

## REST API (future, wired in pkg/api)

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/jobs` | List jobs (filter by trigger, status, date range) |
| `GET` | `/api/jobs/:id` | Get job detail |
| `POST` | `/api/jobs/fire` | Manually fire a trigger |
| `GET` | `/api/jobs/triggers` | List registered triggers and their status |
| `POST` | `/api/jobs/triggers/:name/enable` | Enable/disable a trigger |

---

## Example flows

### Slack message triggers a tool call

```
User posts in #ops: "check device status floor 3"
       │
       ▼
  SlackTrigger (socket mode) fires Event
       │
       ▼
  Engine routes to slack handler
       │
       ▼
  Handler calls toolService.CallTool("rubix.query_nodes", {floor: "3"})
       │
       ▼
  Handler posts result back to Slack thread
       │
       ▼
  Job recorded: trigger=slack, status=done, duration=1.2s
```

### Gmail triggers AI triage + reply

```
New email arrives: "Alarm on Floor 5 — high temps"
       │
       ▼
  GmailTrigger (poll every 2min) fires Event
       │
       ▼
  Handler sends prompt to AI: "triage this alarm email..."
       │
       ▼
  AI responds with severity + recommended action
       │
       ▼
  Handler sends reply email via go-mail
       │
       ▼
  Job recorded: trigger=gmail, status=done
```

### Cron triggers a workflow

```
9:00 AM tick
       │
       ▼
  CronTrigger fires Event
       │
       ▼
  Handler starts workflow: weekly-report for site "Sydney"
       │
       ▼
  Workflow runs through its stages (tool calls, AI prompt, approval)
       │
       ▼
  Job recorded: trigger=cron:weekly-report, status=done
```

---

## Build order

| Phase | What | Deliverable |
|---|---|---|
| **1** | Core engine + cron + manual triggers | `pkg/jobengine/` with engine, interfaces, store, cron, manual triggers. Fully testable. |
| **2** | Slack trigger | Socket mode listener, message/command routing, reply helper |
| **3** | Gmail trigger | OAuth2 setup, poll loop, duplicate tracking, email parsing |
| **4** | Webhook trigger | HTTP handler, signature verification, generic payload parsing |
| **5** | REST API + wiring | `/api/jobs/*` endpoints, wire handlers to ToolService/AgentService |
| **6** | Email sending (actions) | `go-mail` integration for handlers that need to send email replies |
