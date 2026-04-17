# Command & Event Bus — Remote Control for Bizzy

A unified command syntax, internal event bus, and adapter layer that lets users start, monitor, and control workflows/tools/jobs from any interface — Slack, email, CLI, Flutter, webhooks, or future channels.

**This does not replace existing systems.** The workflow engine, AI runners, tool service, MCP server, CLI, and REST API all stay exactly as they are. This layer sits above them — a new way to invoke and observe them from any channel, with notifications and reply routing. Think of it as a universal remote control, not a new engine.

---

## Problem

Bizzy has three ways to invoke AI (WebSocket, REST, async jobs) and a workflow engine — but they all assume a direct API caller. There's no way to:

- Send a Slack message from your phone to start a workflow
- Get notified in Slack when a job finishes
- Cancel a running workflow by replying to a thread
- Have an email trigger an AI triage and auto-reply
- Wire a webhook to a tool call without writing custom glue code

Each entry point speaks a different dialect. The workflow engine has its own YAML verbs. The job store has its own API. Adding a new channel (Slack, Teams, SMS) means writing bespoke handlers for every action. That's N channels x M actions = N*M integration code.

**What's needed:** a shared command language that every channel parses into, a router that dispatches uniformly, and an event bus that closes the loop with notifications.

---

## Architecture

```
┌──────────────────────────────────────────────────────────────────────┐
│  ADAPTERS (ingress + egress)                                         │
│                                                                      │
│  Slack     CLI     Flutter    REST     Gmail    Webhook    Cron       │
│    │        │         │        │         │        │         │         │
│    └────────┴─────────┴────────┴─────────┴────────┴─────────┘         │
│                            │          ▲                               │
│                       parse │          │ reply                        │
│                            ▼          │                               │
├──────────────────────────────────────────────────────────────────────┤
│  COMMAND ROUTER                                                      │
│                                                                      │
│  Validate → Authorize → Resolve target → Dispatch to executor        │
│                                                                      │
├──────────────────────────────────────────────────────────────────────┤
│  EXECUTORS (existing services, unchanged)                            │
│                                                                      │
│  AgentService    ToolService    WorkflowRunner    JobStore            │
│       │               │              │               │               │
│       └───────────────┴──────────────┴───────────────┘               │
│                            │                                         │
│                       publish │                                       │
│                            ▼                                         │
├──────────────────────────────────────────────────────────────────────┤
│  EVENT BUS (NATS embedded / JetStream)                               │
│                                                                      │
│  Subscribers:                                                        │
│    ReplyRouter → send result back through originating adapter        │
│    Notifier    → push notifications, email alerts          (future)  │
│    Auditor     → write to audit log                                  │
│    Chainer     → trigger follow-up commands on completion  (future)  │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

**Key principle:** Commands flow down (intent), events flow up (facts). Adapters convert external signals into commands. Subscribers convert events into external replies. The bus only carries events. The router only processes commands.

---

## The Command

Every interaction — from every channel — parses into a single `Command` struct. This is the reusable syntax system.

```go
type Command struct {
    ID       string         `json:"id"`         // unique command ID (idempotency key)
    Verb     Verb           `json:"verb"`        // what to do
    Target   Target         `json:"target"`      // what to act on
    Params   map[string]any `json:"params"`      // arguments
    UserID   string         `json:"user_id"`     // resolved from auth
    ReplyTo  ReplyInfo      `json:"reply_to"`    // durable reply routing info
    IssuedAt time.Time      `json:"issued_at"`
}

type Verb string

const (
    VerbRun     Verb = "run"      // start something
    VerbAsk     Verb = "ask"      // AI prompt (natural language)
    VerbStatus  Verb = "status"   // check progress
    VerbCancel  Verb = "cancel"   // stop something
    VerbRestart Verb = "restart"  // re-run from scratch
    VerbList    Verb = "list"     // list items
    VerbApprove Verb = "approve"  // approve a waiting stage
    VerbReject  Verb = "reject"   // reject a waiting stage
    VerbHelp    Verb = "help"     // show available commands
)

type Target struct {
    Kind string `json:"kind"`  // "workflow", "tool", "job", "prompt", "session"
    Name string `json:"name"`  // "weekly-report", "rubix.query_nodes", "abc-123"
}
```

### Durable reply routing

Reply info is **data, not a live reference**. It persists with the command/workflow run so replies survive server restarts, and so a job started from the CLI can notify Slack on completion.

```go
// ReplyInfo is serialisable routing data — stored in the DB alongside the command/run.
// Adapters reconstruct a live sender from this data.
// Address is stored as JSON in the DB. Each channel type has a typed struct
// so deserialization catches missing/wrong fields early, not at send time.
type ReplyInfo struct {
    Channel string          `json:"channel"`    // "slack", "email", "http", "webhook", "cron"
    Address json.RawMessage `json:"address"`    // typed per channel (see below)
}

// Typed address structs — one per channel. Deserialized from ReplyInfo.Address.
// Using typed structs instead of map[string]any prevents panics from
// missing keys or wrong types in the reply path.

type SlackAddress struct {
    ChannelID string `json:"channel_id"`
    ThreadTS  string `json:"thread_ts"`
}

type EmailAddress struct {
    To      string `json:"to"`
    Subject string `json:"subject"`
}

type WebhookAddress struct {
    CallbackURL string `json:"callback_url,omitempty"`
}

// Examples:
// Slack:  {channel: "slack",   address: {"channel_id": "C0123ABC", "thread_ts": "1713340800.000100"}}
// Email:  {channel: "email",   address: {"to": "user@example.com", "subject": "Re: Weekly Report"}}
// HTTP:   {channel: "webhook", address: {"callback_url": "https://example.com/hook"}}

// ReplyChannel is the live sender, reconstructed from ReplyInfo at send time.
type ReplyChannel interface {
    Send(ctx context.Context, msg ReplyMessage) error
}

// AdapterRegistry maps channel names to factories that rebuild ReplyChannel from stored ReplyInfo.
// If an adapter was disabled or deregistered after a command started (e.g., Slack disabled
// mid-workflow), BuildReply returns an error. The reply router logs this so it appears
// in the audit trail rather than silently dropping the notification.
type AdapterRegistry interface {
    BuildReply(info ReplyInfo) (ReplyChannel, error)
}
```

This solves the "workflow runs for 10 minutes and the server restarts" problem. The `ReplyInfo` is persisted with the workflow run. On restart, the reply router reconstructs a live `SlackReply` from `{channel_id, thread_ts}` and delivers the result. No lost notifications.

It also enables cross-channel notifications: start a job from the CLI, configure `reply_to: {channel: "slack", address: {channel_id: "C0123ABC"}}` to get notified in Slack when it finishes.

### Text syntax

A consistent text format that works identically in Slack, CLI, or any text-based channel:

```
[verb] [kind/name] [--param value ...]
```

Examples:

```
run workflow/weekly-report --site Sydney
run tool/rubix.query_nodes --floor 3
status workflow/wf-abc123
cancel job/job-xyz789
list workflows --status running
approve workflow/wf-abc123
reject workflow/wf-abc123 --feedback "Make it less formal"
restart workflow/wf-abc123
ask "check which devices are offline on floor 3"
help
```

The `ask` verb is special — the target is implicit (AI), and the rest is natural language routed through `AgentService`.

### Shorthand

For convenience, common patterns have shorthand:

```
run weekly-report --site Sydney       → infers kind=workflow (searches workflows, then tools)
status wf-abc123                      → infers kind from ID prefix
cancel abc123                         → infers kind from ID lookup
```

The parser resolves ambiguity by searching in order: exact match on workflows, then tools, then prompts. If still ambiguous, it picks the first match. Use `kind/name` for explicit disambiguation.

---

## Command parser

```go
// Parse converts raw text into a Command.
// Used by every adapter — Slack messages, CLI input, email subject lines.
func Parse(text string, userID string, replyTo ReplyInfo) (Command, error)
```

Parsing rules:

1. **Starts with a known verb** → structured command: `run workflow/weekly-report --site Sydney`
2. **Starts with `/`** → slash-command shorthand: `/weekly-report Sydney` → `run workflow/weekly-report --params`
3. **No verb recognized** → depends on adapter config (see "ask fallback" below)
4. **Bare ID** → status lookup: `wf-abc123` → `status workflow/wf-abc123`

### Ask fallback is per-adapter

Unrecognised text becoming an AI call makes sense in some channels but not others:

| Adapter | Bare text behaviour | Why |
|---|---|---|
| CLI | `ask` fallback | User is at a terminal, intent is clear |
| Flutter | `ask` fallback | Chat interface, user expects AI |
| Slack | **Require bot mention** (`@bizzy check devices`) | Shared channels — "hello", "lol" shouldn't trigger AI calls |
| Email | **Ignore** unless subject contains a command prefix | Spam/noise risk too high |
| Webhook | **Reject** — require structured JSON | Programmatic callers should be explicit |

```go
type ParseConfig struct {
    BareTextBehaviour string // "ask" | "require_mention" | "ignore" | "reject"
    MentionPrefix     string // "@bizzy" — stripped before parsing
}
```

The Slack adapter passes `ParseConfig{BareTextBehaviour: "require_mention", MentionPrefix: "@bizzy"}`. The CLI passes `ParseConfig{BareTextBehaviour: "ask"}`.

---

## Command router

The router validates, authorizes, and dispatches commands to the right executor.

```go
type Router struct {
    agents    *services.AgentService
    tools     *services.ToolService
    workflows *workflow.Runner
    jobs      *airunner.JobStore
    bus       *Bus
    parser    *Parser
    dedup     *DedupCache  // command ID → TTL window
    limiter   *RateLimiter // per-user token bucket
    adapters  AdapterRegistry
}

func (r *Router) Execute(ctx context.Context, cmd Command) {
    // 1. Deduplication — reject duplicate command IDs within TTL window
    if r.dedup.Seen(cmd.ID) {
        return // already processed (Slack retry, webhook retry, etc.)
    }
    r.dedup.Mark(cmd.ID, 5*time.Minute)

    // 2. Rate limit — per-user token bucket
    if !r.limiter.Allow(cmd.UserID) {
        r.reply(ctx, cmd, ReplyMessage{Text: "Rate limited — try again shortly."})
        return
    }

    // 3. Publish command accepted event
    r.bus.Publish("command.received", CommandEvent{Command: cmd})

    // 4. Dispatch based on verb + target kind
    var result Result
    var err error

    switch cmd.Verb {
    case VerbRun:
        result, err = r.executeRun(ctx, cmd)
    case VerbAsk:
        result, err = r.executeAsk(ctx, cmd)
    case VerbStatus:
        result, err = r.executeStatus(ctx, cmd)
    case VerbCancel:
        result, err = r.executeCancel(ctx, cmd)
    case VerbRestart:
        result, err = r.executeRestart(ctx, cmd)
    case VerbList:
        result, err = r.executeList(ctx, cmd)
    case VerbApprove, VerbReject:
        result, err = r.executeApproval(ctx, cmd)
    case VerbHelp:
        result, err = r.executeHelp(ctx, cmd)
    }

    // 5. Publish result
    //    For sync targets (status, list, tool call): command.completed with result
    //    For async targets (workflow, job): command.accepted with ID only
    //    The real result comes later via workflow.completed / job.completed
    if err != nil {
        r.bus.Publish("command.failed", CommandResultEvent{Command: cmd, Error: err})
    } else {
        topic := "command.completed"
        if result.Async {
            topic = "command.accepted"  // result will arrive later on workflow.*/job.*
        }
        r.bus.Publish(topic, CommandResultEvent{Command: cmd, Result: result})
    }
}

func (r *Router) reply(ctx context.Context, cmd Command, msg ReplyMessage) {
    ch, err := r.adapters.BuildReply(cmd.ReplyTo)
    if err != nil {
        return
    }
    ch.Send(ctx, msg)
}
```

### Run dispatch

```go
func (r *Router) executeRun(ctx context.Context, cmd Command) (Result, error) {
    switch cmd.Target.Kind {
    case "workflow":
        // Persist reply info with the workflow run so completion notifications
        // survive restarts and can be routed back to the originating channel
        runID, err := r.workflows.Start(cmd.UserID, cmd.Target.Name, cmd.Params,
            workflow.WithReplyTo(cmd.ReplyTo))
        return Result{ID: runID, Message: "Workflow started", Async: true}, err

    case "tool":
        output, err := r.tools.CallTool(cmd.UserID, cmd.Target.Name, cmd.Params)
        return Result{Output: output}, err

    case "ai":
        jobID := r.agents.RunJob(cmd.UserID, cmd.Params["prompt"].(string),
            airunner.WithReplyTo(cmd.ReplyTo))
        return Result{ID: jobID, Message: "Job started", Async: true}, nil

    default:
        return Result{}, fmt.Errorf("unknown target kind: %s", cmd.Target.Kind)
    }
}
```

---

## Event bus — NATS embedded

NATS embedded server running in-process. Same binary, no separate service, no ops overhead — but with real persistence (JetStream), wildcard subscriptions, at-least-once delivery, and a clear upgrade path to standalone NATS if bizzy ever goes multi-server.

### Why NATS, not a hand-rolled bus

A mutex + map bus works until it doesn't:

| Concern | Hand-rolled | NATS embedded |
|---|---|---|
| Persistence across restart | Lost | JetStream durable streams |
| Subscriber backpressure | Build it yourself | Built-in (MaxAckPending, rate limits) |
| Wildcard subscriptions | Build it yourself | Native (`workflow.>`, `job.*`) |
| At-least-once delivery | Not possible | JetStream consumer acks |
| Bounded workers | Build it yourself | MaxAckPending + AckWait |
| Upgrade to multi-server | Rewrite everything | Point at external NATS, zero code change |
| Dependency | None | `github.com/nats-io/nats-server/v2/server` (embedded) |

NATS embedded is a single Go import. It starts in-process on a random port (or Unix socket), stores streams in a directory, and shuts down with the server. No Docker, no config file, no separate process.

### Implementation

```go
type Bus struct {
    server *natsserver.Server  // embedded NATS server
    conn   *nats.Conn          // client connection to embedded server
    js     nats.JetStreamContext
}

func New(dataDir string) (*Bus, error) {
    // Start embedded NATS with JetStream
    opts := &natsserver.Options{
        DontListen: true,  // no external TCP — in-process only
        StoreDir:   filepath.Join(dataDir, "nats"),
        JetStream:  true,
    }
    srv, _ := natsserver.NewServer(opts)
    go srv.Start()
    srv.ReadyForConnections(5 * time.Second)

    // Connect in-process (no network hop)
    conn, _ := nats.Connect("", nats.InProcessServer(srv))
    js, _ := conn.JetStream()

    // Create streams for each event domain
    js.AddStream(&nats.StreamConfig{
        Name:     "COMMANDS",
        Subjects: []string{"command.>"},
        MaxAge:   24 * time.Hour,  // retain for audit
    })
    js.AddStream(&nats.StreamConfig{
        Name:     "WORKFLOWS",
        Subjects: []string{"workflow.>"},
        MaxAge:   7 * 24 * time.Hour,
    })
    js.AddStream(&nats.StreamConfig{
        Name:     "JOBS",
        Subjects: []string{"job.>"},
        MaxAge:   24 * time.Hour,
    })
    js.AddStream(&nats.StreamConfig{
        Name:     "TOOLS",
        Subjects: []string{"tool.>"},
        MaxAge:   24 * time.Hour,
    })

    return &Bus{server: srv, conn: conn, js: js}, nil
}

// Publish sends an event. JetStream persists it.
func (b *Bus) Publish(topic string, data any) error {
    payload, _ := json.Marshal(data)
    _, err := b.js.Publish(topic, payload)
    return err
}

// Subscribe creates a push-based consumer.
// NATS handles wildcard matching, backpressure, and redelivery.
func (b *Bus) Subscribe(pattern string, handler func(msg *nats.Msg)) (*nats.Subscription, error) {
    return b.js.Subscribe(pattern, handler,
        nats.MaxAckPending(64),    // bounded concurrency — no goroutine explosion
        nats.AckWait(30*time.Second),
    )
}

// SubscribeDurable survives restarts — picks up where it left off.
func (b *Bus) SubscribeDurable(pattern, name string, handler func(msg *nats.Msg)) (*nats.Subscription, error) {
    return b.js.Subscribe(pattern, handler,
        nats.Durable(name),
        nats.MaxAckPending(64),
        nats.AckWait(30*time.Second),
    )
}

func (b *Bus) Close() {
    b.conn.Close()
    b.server.Shutdown()
}
```

### Topic taxonomy

```
command.>
    command.received            — a command was parsed and dispatched
    command.accepted            — async command acknowledged (workflow/job started, result coming later)
    command.completed           — sync command finished with result
    command.failed              — command failed

job.>
    job.started                 — async AI job began
    job.progress                — streaming text/tool events
    job.completed               — job finished with result
    job.failed                  — job errored
    job.cancelled               — job was cancelled

workflow.>
    workflow.started            — workflow run began
    workflow.stage.started      — a stage began executing
    workflow.stage.completed    — a stage finished
    workflow.stage.failed       — a stage failed
    workflow.waiting_approval   — approval gate reached, waiting on user
    workflow.approved           — user approved
    workflow.rejected           — user rejected
    workflow.completed          — all stages done
    workflow.failed             — workflow stopped on error
    workflow.cancelled          — workflow was cancelled

tool.>
    tool.called                 — a tool was invoked
    tool.completed              — tool returned result
    tool.failed                 — tool errored
```

### Event data

Every event carries enough context to route a reply:

```go
type EventData struct {
    CommandID  string    `json:"command_id,omitempty"`
    UserID     string    `json:"user_id"`
    TargetKind string    `json:"target_kind,omitempty"`  // "workflow", "job", "tool"
    TargetName string    `json:"target_name,omitempty"`
    TargetID   string    `json:"target_id,omitempty"`    // run ID, job ID
    ReplyTo    ReplyInfo `json:"reply_to"`               // durable — survives restart
    Status     string    `json:"status,omitempty"`
    Output     any       `json:"output,omitempty"`
    Error      string    `json:"error,omitempty"`
}
```

---

## Adapters

An adapter does two things: parse inbound messages into commands, and send outbound replies. Each adapter implements both directions.

```go
type Adapter interface {
    Name() string
    Start(ctx context.Context, router *Router) error
    Stop() error
    // BuildReply reconstructs a live ReplyChannel from stored ReplyInfo.
    // Called by the reply router when delivering async results.
    BuildReply(info ReplyInfo) (ReplyChannel, error)
}
```

Adapters don't contain business logic. They translate between external protocols and the Command/Event system.

### Slack adapter

Bidirectional — receives Slack messages, sends Slack replies. Maintains a thread-to-workflow mapping so contextual commands like `approve` resolve to the right run.

```go
type SlackAdapter struct {
    botToken string
    appToken string
    client   *slack.Client
    router   *Router
    // Thread context: maps Slack thread_ts → active workflow/job ID.
    // Persisted in SQLite so it survives restarts.
    threads  *ThreadStore
}

func (s *SlackAdapter) Start(ctx context.Context, router *Router) error {
    s.router = router

    // 1. Listen for messages via Socket Mode (no public URL needed)
    socketClient := socketmode.New(s.client, socketmode.OptionAppToken(s.appToken))

    // 2. Subscribe to bus events for reply routing
    router.Bus().SubscribeDurable("workflow.waiting_approval", "slack-approvals", s.handleApprovalNeeded)
    router.Bus().SubscribeDurable("workflow.completed", "slack-wf-done", s.handleWorkflowDone)
    router.Bus().SubscribeDurable("workflow.failed", "slack-wf-failed", s.handleWorkflowFailed)
    router.Bus().SubscribeDurable("job.completed", "slack-job-done", s.handleJobDone)

    // 3. Process incoming messages
    go func() {
        for evt := range socketClient.Events {
            switch evt.Type {
            case socketmode.EventTypeEventsAPI:
                s.handleIncoming(ctx, evt)
            }
        }
    }()

    return socketClient.RunContext(ctx)
}

func (s *SlackAdapter) handleIncoming(ctx context.Context, evt socketmode.Event) {
    msg := extractMessage(evt)

    // Require bot mention in shared channels
    if !strings.Contains(msg.Text, "<@"+s.botUserID+">") {
        return  // not addressed to us
    }
    text := stripMention(msg.Text, s.botUserID)

    // Build durable reply info (stored with the command/workflow)
    addrJSON, _ := json.Marshal(SlackAddress{
        ChannelID: msg.Channel,
        ThreadTS:  msg.ThreadTS,
    })
    replyTo := ReplyInfo{
        Channel: "slack",
        Address: addrJSON,
    }

    // For contextual commands (bare "approve"/"reject" in a thread),
    // look up which workflow this thread is tracking
    cmd, err := s.router.Parser().Parse(text, resolveUser(msg.User), replyTo,
        ParseConfig{BareTextBehaviour: "require_mention"})
    if err != nil {
        s.reply(ctx, replyTo, "Could not understand: "+err.Error())
        return
    }

    // If approve/reject with no target, resolve from thread context
    if (cmd.Verb == VerbApprove || cmd.Verb == VerbReject) && cmd.Target.Name == "" {
        runID, ok := s.threads.Lookup(msg.ThreadTS)
        if !ok {
            s.reply(ctx, replyTo, "No active workflow in this thread.")
            return
        }
        cmd.Target = Target{Kind: "workflow", Name: runID}
    }

    // Dispatch
    s.router.Execute(ctx, cmd)
}

// BuildReply reconstructs a live Slack reply sender from stored ReplyInfo.
// Called when a workflow completes minutes later — the original connection is gone,
// but the channel ID and thread TS are enough to post a reply.
func (s *SlackAdapter) BuildReply(info ReplyInfo) (ReplyChannel, error) {
    var addr SlackAddress
    if err := json.Unmarshal(info.Address, &addr); err != nil {
        return nil, fmt.Errorf("invalid slack address: %w", err)
    }
    if addr.ChannelID == "" {
        return nil, fmt.Errorf("slack address missing channel_id")
    }
    return &SlackReply{
        client:   s.client,
        channel:  addr.ChannelID,
        threadTS: addr.ThreadTS,
    }, nil
}

// When a workflow starts, record which thread it's in
func (s *SlackAdapter) onWorkflowStarted(data EventData) {
    if data.ReplyTo.Channel == "slack" {
        var addr SlackAddress
        if err := json.Unmarshal(data.ReplyTo.Address, &addr); err != nil {
            return
        }
        s.threads.Set(addr.ThreadTS, data.TargetID)  // thread_ts → workflow run ID
    }
}
```

**Slack reply channel:**

```go
type SlackReply struct {
    client   *slack.Client
    channel  string
    threadTS string
}

func (r *SlackReply) Send(ctx context.Context, msg ReplyMessage) error {
    _, _, err := r.client.PostMessageContext(ctx, r.channel,
        slack.MsgOptionText(msg.Text, false),
        slack.MsgOptionTS(r.threadTS),  // reply in thread
    )
    return err
}
```

**Thread context table:**

```sql
CREATE TABLE slack_threads (
    thread_ts   TEXT PRIMARY KEY,
    run_id      TEXT NOT NULL,      -- workflow run ID or job ID
    run_type    TEXT NOT NULL,      -- "workflow" or "job"
    channel_id  TEXT NOT NULL,
    created_at  DATETIME NOT NULL
);

CREATE INDEX idx_slack_threads_run ON slack_threads(run_id);
```

### Gmail adapter

Polls for new emails, parses subject/body into commands, replies via SMTP. Sender identity is verified before executing commands.

```go
type GmailAdapter struct {
    gmailSvc       *gmail.Service
    mailer         *gomail.Client
    pollInterval   time.Duration
    query          string            // e.g. "is:unread label:bizzy"
    lastSeen       string
    allowedDomains []string          // e.g. ["nube-io.com"] — reject unknown senders
    router         *Router
}

func (g *GmailAdapter) Start(ctx context.Context, router *Router) error {
    g.router = router

    // Subscribe to events for email replies
    router.Bus().SubscribeDurable("command.completed", "gmail-replies", g.handleReply)

    // Poll loop
    ticker := time.NewTicker(g.pollInterval)
    for {
        select {
        case <-ctx.Done():
            return nil
        case <-ticker.C:
            g.poll(ctx)
        }
    }
}

func (g *GmailAdapter) poll(ctx context.Context) {
    messages := g.fetchNew(g.query, g.lastSeen)
    for _, msg := range messages {
        // Security: verify sender domain
        fromDomain := extractDomain(msg.From)
        if !g.isAllowed(fromDomain) {
            continue  // ignore unknown senders — don't even error-reply
        }

        // Verify DKIM/SPF passed (Gmail provides this in headers)
        if !msg.AuthResults.DKIMPass || !msg.AuthResults.SPFPass {
            continue  // spoofed sender — drop silently
        }

        addrJSON, _ := json.Marshal(EmailAddress{
            To:      msg.From,
            Subject: "Re: " + msg.Subject,
        })
        replyTo := ReplyInfo{
            Channel: "email",
            Address: addrJSON,
        }

        // Parse email body as a command (ignore = skip unparseable emails)
        cmd, err := g.router.Parser().Parse(msg.Body, resolveEmailUser(msg.From), replyTo,
            ParseConfig{BareTextBehaviour: "ignore"})
        if err != nil {
            continue
        }

        g.router.Execute(ctx, cmd)
        g.lastSeen = msg.ID
    }
}
```

### Webhook adapter

Receives HTTP POSTs from external services. Requires structured JSON — no natural language fallback.

```go
type WebhookAdapter struct {
    secret string
    router *Router
}

func (w *WebhookAdapter) Handler() http.HandlerFunc {
    return func(rw http.ResponseWriter, r *http.Request) {
        if !verifySignature(r, w.secret) {
            http.Error(rw, "unauthorized", 401)
            return
        }

        var req struct {
            Text   string         `json:"text"`    // command text
            UserID string         `json:"user_id"` // caller identity
            Params map[string]any `json:"params"`  // optional structured params
        }
        json.NewDecoder(r.Body).Decode(&req)

        replyTo := ReplyInfo{Channel: "webhook"}
        if callbackURL := r.Header.Get("X-Callback-URL"); callbackURL != "" {
            addrJSON, _ := json.Marshal(WebhookAddress{CallbackURL: callbackURL})
            replyTo.Address = addrJSON
        }

        cmd, err := w.router.Parser().Parse(req.Text, req.UserID, replyTo,
            ParseConfig{BareTextBehaviour: "reject"})
        if err != nil {
            http.Error(rw, err.Error(), 400)
            return
        }

        w.router.Execute(r.Context(), cmd)
        json.NewEncoder(rw).Encode(map[string]string{"command_id": cmd.ID, "status": "accepted"})
    }
}
```

### Cron adapter

Fires commands on a schedule. Commands are configured in the database and reloaded periodically so new entries take effect without a server restart.

```go
type CronAdapter struct {
    scheduler *gocron.Scheduler
    db        *gorm.DB
    router    *Router
}

func (c *CronAdapter) Start(ctx context.Context, router *Router) error {
    c.router = router

    // Initial load
    c.reload(ctx)

    // Re-sync from DB every 60s — picks up new/changed/disabled entries
    // without requiring a server restart.
    ticker := time.NewTicker(60 * time.Second)
    go func() {
        for {
            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
                c.reload(ctx)
            }
        }
    }()

    c.scheduler.StartAsync()
    return nil
}

func (c *CronAdapter) reload(ctx context.Context) {
    var entries []CronCommand
    c.db.Where("enabled = ?", true).Find(&entries)

    // Clear and rebuild — gocron supports removing all jobs
    c.scheduler.Clear()

    for _, entry := range entries {
        cmd, _ := c.router.Parser().Parse(entry.Command, entry.UserID,
            ReplyInfo{Channel: "cron"}, ParseConfig{BareTextBehaviour: "reject"})

        c.scheduler.Cron(entry.Schedule).Tag(entry.ID).Do(func() {
            c.router.Execute(ctx, cmd)
        })
    }
}
```

### Existing entry points

The current REST API, WebSocket handlers, and CLI continue to work as-is. They can gradually be refactored to emit bus events for visibility, but there's no requirement to route them through the command parser. They already call the services directly.

The only change: workflows and jobs start publishing events to the bus (a few `bus.Publish()` calls in the existing runners). This makes their progress visible to all subscribers.

---

## Reply routing

The `ReplyRouter` subscriber watches for completion events and sends results back through the originating adapter's reply channel.

```go
type ReplyRouter struct {
    bus      *Bus
    adapters AdapterRegistry
}

func (r *ReplyRouter) Start() {
    // Sync commands — immediate result
    r.bus.SubscribeDurable("command.completed", "reply-router-sync", func(msg *nats.Msg) {
        var ev CommandResultEvent
        json.Unmarshal(msg.Data, &ev)

        ch, err := r.adapters.BuildReply(ev.Command.ReplyTo)
        if err != nil {
            msg.Ack()
            return
        }
        ch.Send(context.Background(), ReplyMessage{Text: formatResult(ev.Result)})
        msg.Ack()
    })

    // Async commands — ack immediately, real result comes later
    r.bus.SubscribeDurable("command.accepted", "reply-router-async", func(msg *nats.Msg) {
        var ev CommandResultEvent
        json.Unmarshal(msg.Data, &ev)

        ch, _ := r.adapters.BuildReply(ev.Command.ReplyTo)
        if ch != nil {
            ch.Send(context.Background(), ReplyMessage{
                Text: fmt.Sprintf("Started %s (%s)", ev.Result.Message, ev.Result.ID),
            })
        }
        msg.Ack()
    })

    // Workflow/job completion — the real result for async commands
    r.bus.SubscribeDurable("workflow.completed", "reply-router-wf", func(msg *nats.Msg) {
        var ev EventData
        json.Unmarshal(msg.Data, &ev)

        ch, err := r.adapters.BuildReply(ev.ReplyTo)
        if err != nil {
            // Adapter was disabled/deregistered after the workflow started.
            // Log it so it appears in the audit trail.
            log.Warn().Str("channel", ev.ReplyTo.Channel).Str("target_id", ev.TargetID).
                Err(err).Msg("reply dropped: adapter unavailable")
            msg.Ack()
            return
        }
        ch.Send(context.Background(), ReplyMessage{
            Text: fmt.Sprintf("Workflow %s completed: %s", ev.TargetName, ev.Output),
        })
        msg.Ack()
    })

    r.bus.SubscribeDurable("command.failed", "reply-router-err", func(msg *nats.Msg) {
        var ev CommandResultEvent
        json.Unmarshal(msg.Data, &ev)

        ch, _ := r.adapters.BuildReply(ev.Command.ReplyTo)
        if ch != nil {
            ch.Send(context.Background(), ReplyMessage{
                Text: "Error: " + ev.Error,
            })
        }
        msg.Ack()
    })
}
```

### Notification preferences (future)

Users can configure where they want notifications beyond the originating channel:

```go
type NotifyPrefs struct {
    OnWorkflowDone    []string `json:"on_workflow_done"`     // ["slack", "email"]
    OnJobFailed       []string `json:"on_job_failed"`        // ["slack"]
    OnApprovalNeeded  []string `json:"on_approval_needed"`   // ["slack", "push"]
}
```

The `Notifier` subscriber reads these prefs and fans out to the right adapters. This is a later addition — start with reply-to-origin routing.

### Command chaining (future)

The `Chainer` subscriber watches for completion events and triggers follow-up commands. Example: when `workflow/weekly-report` completes, automatically run `tool/email-report --to ops@nube-io.com`.

This is powerful but dangerous — it creates feedback loops if misconfigured (A completes → triggers B → B completes → triggers A). Implementation must include:

- **Max chain depth** (e.g., 5) — reject commands that exceed the depth
- **Loop detection** — track the chain of command IDs and reject if a target appears twice
- **Admin-only configuration** — regular users can't create chains

This is a later addition. The bus and command infrastructure support it naturally — the Chainer is just another subscriber that calls `router.Execute()` — but the safety guardrails need careful design.

---

## Wiring

All the pieces come together at server startup:

```go
// In cmd/nube-server/main.go

// 1. Create NATS bus (embedded, in-process, JetStream persistence)
eventBus, _ := bus.New(filepath.Join(dataDir, "nats"))
defer eventBus.Close()

// 2. Create command parser (knows about available workflows/tools)
parser := command.NewParser(workflowStore, toolService)

// 3. Create adapter registry
adapterRegistry := adapters.NewRegistry()

// 4. Create router
router := command.NewRouter(command.RouterConfig{
    Parser:    parser,
    Agents:    agentService,
    Tools:     toolService,
    Workflows: workflowRunner,
    Jobs:      jobStore,
    Bus:       eventBus,
    Adapters:  adapterRegistry,
})

// 5. Create reply router
replyRouter := notify.NewReplyRouter(eventBus, adapterRegistry)
replyRouter.Start()

// 6. Start adapters (only enabled ones)
var enabledAdapters []adapters.Adapter
if slackCfg != nil {
    sa := slack.New(slackCfg)
    enabledAdapters = append(enabledAdapters, sa)
    adapterRegistry.Register("slack", sa)
}
if gmailCfg != nil {
    ga := gmail.New(gmailCfg)
    enabledAdapters = append(enabledAdapters, ga)
    adapterRegistry.Register("email", ga)
}
// ... webhook, cron

for _, a := range enabledAdapters {
    go a.Start(ctx, router)
}

// 7. Existing HTTP/WS handlers continue to work unchanged
api.RegisterRoutes(mux, app)
```

---

## Phone scenario — full walkthrough

A user on their phone, managing work through Slack:

### Start a workflow

```
Phone → Slack #ops: "@bizzy run weekly-report --site Sydney"

Bizzy → Slack #ops (thread): "Started workflow weekly-report (wf-7a3b).
  Stages: devices → alarms → write → review → deliver"
```

### Check status

```
Phone → Slack #ops: "@bizzy status wf-7a3b"

Bizzy → Slack #ops (thread): "weekly-report (wf-7a3b) — running
  [done] devices (1.2s)
  [done] alarms (0.8s)
  [running] write...
  [pending] review
  [pending] deliver"
```

### Get notified for approval

```
Bizzy → Slack #ops (thread): "Approval needed for weekly-report (wf-7a3b):

  Weekly Operations Report — Sydney
  - 3 devices offline (Floor 2 controller, Floor 5 sensor, Roof RTU)
  - 12 alarms this week, 4 critical
  - Temperature exceedance on Floor 5 needs investigation

  Reply: 'approve' or 'reject [feedback]'"

Phone → Slack #ops (in thread): "approve"
```

The Slack adapter sees `approve` in a thread, looks up thread → wf-7a3b, and dispatches `Command{Verb: Approve, Target: workflow/wf-7a3b}`.

### Get notified on completion

```
Bizzy → Slack #ops (thread): "Workflow weekly-report (wf-7a3b) completed.
  Weekly report for Sydney ready. Duration: 14.2s"
```

Even if the server restarted between approval and completion — the `ReplyInfo` was persisted with the workflow run, NATS redelivers the `workflow.completed` event, and the Slack adapter reconstructs the reply from `{channel_id, thread_ts}`.

### Cancel

```
Phone → Slack #ops: "@bizzy cancel wf-7a3b"

Bizzy → Slack #ops (thread): "Cancelled workflow wf-7a3b."
```

### Natural language (with mention)

```
Phone → Slack #ops: "@bizzy which devices are offline right now?"

Bizzy → Slack #ops (thread): "3 devices offline:
  - Floor 2 Controller (192.168.1.50) — last seen 2h ago
  - Floor 5 Temp Sensor (192.168.1.87) — last seen 45m ago
  - Roof RTU (192.168.1.102) — last seen 3h ago"
```

---

## How this relates to the job engine

The [JOB-ENGINE.md](JOB-ENGINE.md) doc describes triggers, handlers, and job tracking. This design absorbs and extends it:

| Job engine concept | Where it lives now |
|---|---|
| **Triggers** (Cron, Slack, Gmail, Webhook) | **Adapters** — same external integrations, but they produce Commands instead of raw Events |
| **Handlers** | **Router + Executors** — the router dispatches commands to existing services; no custom handler-per-trigger |
| **Engine.On("slack", handler)** | **Eliminated** — the N*M problem goes away because every adapter produces the same Command |
| **Job tracking** (SQLite jobs table) | **Unchanged** — jobs table still tracks execution history; now populated by the bus |
| **Manual fire** | **Command with verb=run** — `router.Execute(cmd)` replaces `engine.Fire(event)` |

The job engine becomes a subscriber of the bus rather than the orchestrator. It records executions, tracks status, and serves the `/api/jobs` query API — but it doesn't route or dispatch.

---

## How this relates to workflows

The existing workflow engine ([MULTI-APP-WORKFLOW.md](MULTI-APP-WORKFLOW.md)) is **unchanged**. It continues to execute YAML-defined stage pipelines. What changes:

1. **Workflows emit bus events** — `workflow.started`, `workflow.stage.completed`, `workflow.waiting_approval`, etc. Currently status is only visible by polling the REST API.
2. **Approval via any channel** — the `approve` and `reject` verbs in the command syntax map to `workflows.Approve(runID)`. A Slack reply, an email, or a CLI command all work.
3. **Workflows can be started from any adapter** — `run workflow/weekly-report` works from Slack, cron, email, webhook, CLI, Flutter, or REST.
4. **Workflows persist ReplyInfo** — so completion notifications route back to the originating channel even minutes later, even after restarts.

---

## How this relates to MCP and CLI

The MCP server and CLI are **unchanged**. They are separate concerns:

| System | Purpose | Users |
|---|---|---|
| **MCP server** | Tool discovery + execution for AI clients | Claude, Cursor, MCP-compatible clients |
| **CLI** | Human command interface at a terminal | Developers, admins |
| **REST API** | Programmatic access | Flutter app, scripts, integrations |
| **Command/Bus** | Remote invocation + notifications from external channels | Slack users, email, webhooks, cron |

The command/bus layer doesn't replace any of these — it adds new entry points (Slack, email, cron, webhook) that call the **same services** the REST API and CLI already call. The bus adds **observability** (events) and **notifications** (reply routing) that benefit all entry points.

```
MCP ─────────────────────┐
CLI ─────────────────────┤
REST API ────────────────┤──→ AgentService / ToolService / WorkflowRunner
Slack adapter ───────────┤                        │
Gmail adapter ───────────┤                   bus.Publish()
Cron adapter ────────────┘                        │
                                             Notifications,
                                             audit, metrics
```

---

## REST API additions

The command system adds these endpoints alongside the existing per-resource APIs:

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/command` | Execute a command (text or structured) |
| `GET` | `/api/command/help` | List available verbs, targets, and syntax |
| `GET` | `/api/events/stream` | SSE stream of bus events (user-scoped, admin `?all=true`) |
| `POST` | `/hooks/command` | Inbound webhook (HMAC or Bearer auth) |
| `GET` | `/api/adapters` | List adapter configs from DB |
| `GET` | `/api/cron` | List scheduled command entries |
| `POST` | `/api/cron` | Create a scheduled command |
| `DELETE` | `/api/cron/:id` | Delete a scheduled command |
| `PATCH` | `/api/cron/:id` | Enable/disable a scheduled command |
| `GET` | `/api/notifications/prefs` | Get notification preferences for current user |
| `PUT` | `/api/notifications/prefs` | Set notification preferences |
| `GET` | `/api/webhooks/logs` | Webhook audit log (admin) |

### Command endpoint

```
POST /api/command
{
  "text": "run workflow/weekly-report --site Sydney"
}

→ 202 Accepted
{
  "command_id": "cmd-abc123",
  "verb": "run",
  "target": {"kind": "workflow", "name": "weekly-report"},
  "result": {
    "id": "wf-7a3b",
    "message": "Workflow started",
    "async": true
  }
}
```

Or structured:

```
POST /api/command
{
  "verb": "run",
  "target": "workflow/weekly-report",
  "params": {"site": "Sydney"}
}
```

### Event stream

Server-sent events for real-time monitoring. Events are **user-scoped** — the authenticated user only sees events for their own commands, workflows, and jobs. Admins can pass `?all=true` to see everything.

```
GET /api/events/stream?topics=workflow.>,job.>

data: {"topic":"workflow.started","data":{"run_id":"wf-7a3b","name":"weekly-report"}}
data: {"topic":"workflow.stage.completed","data":{"run_id":"wf-7a3b","stage":"research","duration_ms":1200}}
data: {"topic":"workflow.stage.completed","data":{"run_id":"wf-7a3b","stage":"draft","duration_ms":8500}}
data: {"topic":"workflow.waiting_approval","data":{"run_id":"wf-7a3b","stage":"review"}}
```

Scoping is done server-side: the SSE handler subscribes to the requested topics on the bus but filters events by `user_id` before sending them to the client. Every `EventData` carries `user_id`, so no subject-hierarchy changes are needed.

---

## Configuration

Adapters are configured via the database (admin-managed), not config files. This allows runtime enable/disable without restarts.

```sql
CREATE TABLE adapters (
    name        TEXT PRIMARY KEY,          -- "slack", "gmail", "cron", "webhook"
    type        TEXT NOT NULL,             -- adapter type
    enabled     BOOLEAN NOT NULL DEFAULT false,
    config      TEXT,                      -- JSON config (tokens, schedules, etc.)
    created_at  DATETIME NOT NULL,
    updated_at  DATETIME NOT NULL
);

CREATE TABLE cron_commands (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,       -- "daily-report"
    schedule    TEXT NOT NULL,              -- "0 9 * * *"
    command     TEXT NOT NULL,              -- "run workflow/weekly-report --site Sydney"
    user_id     TEXT NOT NULL,             -- run as this user
    enabled     BOOLEAN NOT NULL DEFAULT true,
    created_at  DATETIME NOT NULL
);
```

### Adapter config examples

```json
// Slack
{
  "bot_token": "xoxb-...",
  "app_token": "xapp-...",
  "channels": ["C0123ABC"],
  "require_mention": true
}

// Gmail
{
  "credentials": "...",
  "poll_interval_sec": 120,
  "query": "is:unread label:bizzy",
  "mark_read": true,
  "allowed_domains": ["nube-io.com"]
}

// Webhook
{
  "path": "/hooks/command",
  "secret": "whsec_..."
}
```

---

## File layout

Five packages, each with a single responsibility and clean dependency boundaries. Adapters communicate through interfaces defined in `pkg/command/`.

```
pkg/
    bus/                        ← layer 1: pure infrastructure, zero bizzy imports
    command/                    ← layer 2: types + parsing + routing, imports bus/
    adapters/                   ← layer 3: external channels, imports command/
    notify/                     ← layer 3: reply routing + notifications, imports command/ + bus/
    api/                        ← layer 4: HTTP wiring (existing), imports all above
```

### Dependency flow

```
                    ┌───────────────────────────┐
                    │         pkg/api/           │  ← wires everything together
                    │  command_bridge.go         │
                    │  command_handler.go        │
                    │  events_sse.go             │
                    │  adapters_handler.go       │
                    │  notify_handler.go         │
                    └─────┬──────────┬───────────┘
                          │          │
              ┌───────────┘          └───────────┐
              ▼                                  ▼
    ┌──────────────────┐              ┌──────────────────┐
    │  pkg/adapters/   │              │   pkg/notify/    │
    │  (slack, gmail,  │              │  reply_router.go │
    │   webhook, cron) │              │  notifier.go     │
    └────────┬─────────┘              └────────┬─────────┘
             │                                 │
             └──────────┬──────────────────────┘
                        ▼
              ┌──────────────────┐
              │   pkg/command/   │     ← types, parser, router, interfaces
              └────────┬─────────┘
                       ▼
              ┌──────────────────┐
              │    pkg/bus/      │     ← NATS embedded, no bizzy imports
              └──────────────────┘

    (existing, modified to emit bus events)
    pkg/services/       ← AgentService, ToolService (called by router via interfaces)
    pkg/workflow/       ← WorkflowRunner (publishes lifecycle events to bus)
    pkg/airunner/       ← Runners, JobStore (publishes job events to bus)
    pkg/models/         ← AdapterConfig, NotifyPrefs models added
```

### Actual files

```
pkg/bus/                            2 files
    bus.go                  — Bus struct: embedded NATS server, JetStream setup, Publish, Subscribe, SubscribeDurable, Close
    event.go                — EventData struct, topic constants (command.>, workflow.>, job.>, tool.>)

pkg/command/                        3 files
    command.go              — Command, Verb, Target, ReplyInfo, typed address structs (SlackAddress, EmailAddress, WebhookAddress),
                              Result, ReplyMessage, ReplyChannel interface, AdapterRegistry interface, Adapter interface
    parser.go               — Parser: text → Command, tokenizer (respects quotes), shorthand resolution, ParseConfig
    router.go               — Router: dedup cache, per-user rate limiter, dispatch to executors, publish bus events.
                              Executor interfaces: WorkflowStarter, ToolExecutor, AgentExecutor, ToolLister

pkg/adapters/                       1 + 4 sub-packages
    registry.go             — Registry: implements command.AdapterRegistry, Register/BuildReply/Get/List

    slack/                  3 files
        slack.go            — Adapter: Socket Mode listener, bot mention filter, DM support,
                              thread context resolution for approve/reject, slash commands
        reply.go            — SlackReply: implements command.ReplyChannel via slack-go PostMessage
        threads.go          — ThreadStore: thread_ts → run_id mapping (SQLite, survives restarts)

    gmail/                  2 files
        gmail.go            — Adapter: poll loop, DKIM/SPF + domain whitelist verification,
                              user lookup by email, audit logging, mark-as-read
        reply.go            — EmailReply: implements command.ReplyChannel via go-mail SMTP

    webhook/                1 file
        webhook.go          — Adapter: HTTP handler, HMAC-SHA256 or Bearer token auth,
                              text + structured JSON input, callback URL replies,
                              WebhookLog audit table, noopReply + callbackReply channels

    cron/                   1 file
        cron.go             — Adapter: DB-driven CronCommand table, 60s reload, minute-granularity
                              cron matching, CRUD helpers (ListEntries, CreateEntry, DeleteEntry, ToggleEntry)

pkg/notify/                 2 files
    reply_router.go         — ReplyRouter: subscribes to command.completed/accepted/failed,
                              workflow.completed/failed/waiting_approval. Reconstructs ReplyChannel
                              from stored ReplyInfo, delivers results. Logs dropped replies.
    notifier.go             — Notifier: per-user notification fan-out. Reads NotifyPrefs from DB,
                              sends to configured channels on workflow.completed/failed,
                              job.completed/failed, workflow.waiting_approval.

pkg/api/                    5 new files
    command_bridge.go       — CommandAgentBridge (implements command.AgentExecutor),
                              CommandToolLister (implements command.ToolLister)
    command_handler.go      — POST /api/command (text + structured), GET /api/command/help
    events_sse.go           — GET /api/events/stream (SSE, user-scoped, admin ?all=true)
    adapters_handler.go     — GET /api/adapters, GET/POST/DELETE/PATCH /api/cron
    notify_handler.go       — GET/PUT /api/notifications/prefs, GET /api/webhooks/logs

pkg/models/                 2 new files
    adapter.go              — AdapterConfig DB model (name, type, enabled, JSON config)
    notify_prefs.go         — NotifyPrefs DB model (per-user, per-event-type channel lists)
```

### Modified existing files

```
pkg/workflow/
    runner.go   (EDIT)      — Added EventPublisher interface, SetBus(), publish() helper.
                              Emits events: workflow.started, stage.started, stage.completed,
                              stage.failed, waiting_approval, approved, rejected,
                              completed, failed, cancelled
    events.go   (NEW)       — RunEvent struct, topic constants

pkg/airunner/
    jobstore.go (EDIT)      — Added EventPublisher interface, SetBus(), publish() helper.
                              Emits events: job.started, job.completed, job.cancelled

pkg/api/
    router.go   (EDIT)      — Added CmdRouter and WebhookHandler fields to API struct.
                              Registered command bus routes: /api/command, /api/command/help,
                              /api/events/stream, /hooks/command, /api/adapters, /api/cron,
                              /api/notifications/prefs, /api/webhooks/logs

pkg/database/
    database.go (EDIT)      — Added AdapterConfig and NotifyPrefs to allModels auto-migration

cmd/nube-server/
    main.go     (EDIT)      — Wires NATS bus, command parser, command router, reply router,
                              notifier, cron adapter, webhook adapter. Conditionally starts
                              Slack (NUBE_SLACK_BOT_TOKEN) and Gmail (NUBE_GMAIL_ENABLED) adapters.
```

### Import rules

| Package | Can import | Cannot import |
|---|---|---|
| `pkg/bus/` | stdlib, nats | anything in bizzy |
| `pkg/command/` | `pkg/bus/`, `pkg/models/`, stdlib | services, workflow, adapters, api |
| `pkg/adapters/*` | `pkg/command/`, `pkg/models/`, stdlib, external libs | other adapters, services, workflow |
| `pkg/notify/` | `pkg/bus/`, `pkg/command/`, `pkg/models/`, stdlib | adapters, services, workflow |
| `pkg/api/` | everything | — (top-level wiring) |

The router in `pkg/command/` never imports `pkg/services/` or `pkg/workflow/`. It defines executor interfaces (`WorkflowStarter`, `ToolExecutor`, `AgentExecutor`, `ToolLister`) that the services implement. Wiring happens in `pkg/api/` at startup via bridge types:

```go
// In cmd/nube-server/main.go:
cmdRouter := command.NewRouter(command.RouterConfig{
    Parser:    command.NewParser(),
    Workflows: a.Workflows,                                    // workflow.Runner implements WorkflowStarter
    Tools:     a.ToolSvc,                                      // services.ToolService implements ToolExecutor
    Agents:    &api.CommandAgentBridge{AgentSvc: agentSvc, Jobs: agentSvc.Jobs},
    Lister:    &api.CommandToolLister{ToolSvc: toolSvc},
    Bus:       eventBus,
    Adapters:  adapterRegistry,
})
```

---

## Build order

| Phase | What | Status | Deliverable |
|---|---|---|---|
| **1** | NATS event bus | **Done** | `pkg/bus/bus.go`, `event.go` — embedded NATS, JetStream streams, publish/subscribe. |
| **2** | Command types + parser | **Done** | `pkg/command/command.go`, `parser.go` — Command struct, text parser, per-adapter parse config. |
| **3** | Command router | **Done** | `pkg/command/router.go` — dispatch to executors, dedup, rate limiting, bus events. |
| **4** | Reply router + adapter registry | **Done** | `pkg/notify/reply_router.go`, `pkg/adapters/registry.go` — reconstruct reply channels, send results. |
| **5** | Workflows/jobs emit bus events | **Done** | `pkg/workflow/runner.go` + `events.go`, `pkg/airunner/jobstore.go` — lifecycle events on the bus. |
| **6** | Slack adapter | **Done** | `pkg/adapters/slack/` — socket mode, bot mention, thread context, reply builder. |
| **7** | Cron adapter | **Done** | `pkg/adapters/cron/cron.go` — DB-driven scheduled commands, 60s reload, CRUD helpers. |
| **8** | Webhook adapter | **Done** | `pkg/adapters/webhook/webhook.go` — HMAC + Bearer auth, text + structured JSON, callback replies, audit log. |
| **9** | Gmail adapter | **Done** | `pkg/adapters/gmail/` — polling, DKIM/SPF + domain whitelist, SMTP replies, audit log. |
| **10** | SSE event stream | **Done** | `pkg/api/events_sse.go` — `GET /api/events/stream`, user-scoped, admin `?all=true`. |
| **11** | Notification preferences | **Done** | `pkg/notify/notifier.go`, `pkg/models/notify_prefs.go` — per-user fan-out to multiple channels. |

---

## Dependencies

| Concern | Library | Notes |
|---|---|---|
| Event bus | `nats-io/nats-server/v2` + `nats-io/nats.go` | Embedded server, JetStream for persistence |
| Slack | `slack-go/slack` | Socket mode, events API, message posting |
| Gmail | `google.golang.org/api/gmail/v1` | Official Google client, OAuth2 |
| Email sending | `wneessen/go-mail` | SMTP, TLS |
| Rate limiting | Built-in (token bucket in `router.go`) | No external dependency |
| Cron matching | Built-in (minute-granularity matcher in `cron.go`) | No external dependency |
| Webhook verify | Built-in (`crypto/hmac` + `crypto/sha256`) | HMAC-SHA256 signature verification |
| DB (existing) | `gorm.io/gorm` + `gorm.io/driver/sqlite` | Cron entries, thread mappings, webhook logs, notify prefs |

---

## Future: graph/flow engine

The current workflow engine is a linear pipeline with conditionals. A graph engine would add:

- **Parallel branches** — fan-out to multiple stages, fan-in when all complete
- **Loops** — retry-with-backoff, iterate over collections
- **Dynamic routing** — AI decides the next node based on results
- **Sub-workflows** — a node that runs another workflow
- **Visual editor** — Flutter canvas for drag-and-drop flow building

This is a separate effort. The command/bus layer is prerequisite infrastructure — the graph engine would emit the same bus events (`workflow.stage.>`) and be invokable via the same command syntax (`run flow/my-flow`). Build the command/bus layer first; it makes the graph engine a natural extension rather than a parallel system.

```
                    Today                          Future
              ┌─────────────────┐          ┌─────────────────────┐
              │  Linear YAML    │          │  DAG / Graph        │
              │  Pipeline       │          │  Engine             │
              │                 │          │                     │
              │  A → B → C → D │          │  A ──┬── B ──┐     │
              │                 │          │      └── C ──┤     │
              │                 │          │              ▼     │
              │                 │          │         D (fan-in)  │
              │                 │          │              │     │
              │                 │          │         ◄── loop ──│
              └────────┬────────┘          └────────┬────────────┘
                       │                            │
                       └────────────┬───────────────┘
                                    │
                            Same Command/Bus layer
                            Same events, same verbs
                            Same adapters
```

---

## Design constraints

- **NATS embedded, not external.** Runs in-process, stores in `data/nats/`. No separate service to manage. Upgrade to standalone NATS by changing one connection string.
- **Adapters are optional.** The system works without any adapters configured — the REST/WS/CLI/MCP paths still work exactly as they do today. Adapters are additive.
- **Reply info is durable.** `ReplyInfo` is data (channel type + addressing), persisted with commands and workflow runs. Replies survive restarts.
- **Commands are deduplicated.** Command IDs prevent duplicate execution from Slack retries, webhook retries, or network issues.
- **Per-user rate limiting.** Token bucket in the router prevents webhook floods or misconfigured cron from overwhelming the system.
- **Slack requires bot mention.** `@bizzy` prefix prevents casual channel messages from triggering AI calls.
- **Gmail verifies sender identity.** Domain whitelist + DKIM/SPF checks prevent spoofed emails from executing commands.
- **`command.accepted` vs `command.completed`.** Async targets (workflows, jobs) get `accepted` immediately; the real result arrives later on `workflow.completed` / `job.completed`. No ambiguous double-done signals.
- **Reply addresses are typed.** Each channel has a typed address struct (`SlackAddress`, `EmailAddress`, etc.) instead of `map[string]any`. Missing or malformed fields are caught at deserialization, not at send time.
- **Dropped replies are logged.** If an adapter is disabled after a command starts, the reply router logs the failure with the target ID so it appears in the audit trail.
- **SSE events are user-scoped.** The event stream filters by authenticated user. Admins can opt into the full stream.
- **Cron entries reload from DB.** The cron adapter re-syncs every 60s so new/changed/disabled entries take effect without a restart.
- **Chainer has safety guardrails (future).** Max chain depth, loop detection, admin-only config. Not built in phase 1.
- **Existing APIs are unchanged.** The current REST endpoints, MCP server, and CLI continue to work. The command layer is a new, parallel entry point — not a replacement.
- **Parser is forgiving.** Shorthand resolution, natural language fallback (where configured), helpful error messages. The goal is zero-friction from a phone keyboard.
