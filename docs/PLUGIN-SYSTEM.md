# Plugin System — Separate-Process Extensions over NATS

Plugins are independent processes that extend bizzy by connecting to its embedded NATS server. A plugin can provide **any combination** of services — tools, prompts, workflows, adapters, event handlers — declared via a `services` field in its manifest. Written in any language (Go, Node.js, Python, Rust, etc.), managed externally (systemd, Docker, supervisor, manually — bizzy does not start or stop them).

## Supported services

A plugin declares which services it provides. Each service type hooks into a different part of bizzy:

| Service | What it extends | How bizzy uses it |
|---|---|---|
| `tools` | ToolService, MCP, AI | Plugin tools appear alongside app tools. AI calls them identically. |
| `prompts` | MCPFactory, prompt listing | Prompt templates served via MCP, usable by AI and CLI. |
| `workflows` | Workflow engine | Plugin registers workflow definitions. Stages execute via NATS. |
| `adapter` | Command bus, adapters | Plugin acts as a new ingress/egress channel (like Slack, email). |
| `handler` | Event bus | Plugin subscribes to bus events and reacts (notifications, logging, sync). |

A single plugin can provide multiple services. A Python ML plugin might provide `tools` + `handler` (tools for analysis, handler to react to workflow completions). A Telegram plugin might provide `adapter` only.

```
┌─────────────────────────────────────────────────────────────────┐
│                         nube-server                             │
│                                                                 │
│  Plugin Registry reads manifest.services[] and wires each:     │
│                                                                 │
│  services: ["tools"]     → inject into ToolService + MCP        │
│  services: ["prompts"]   → inject into MCPFactory               │
│  services: ["workflows"] → register with Workflow engine        │
│  services: ["adapter"]   → register with AdapterRegistry        │
│  services: ["handler"]   → plugin self-subscribes to bus topics │
└─────────────────────────────────────────────────────────────────┘
```

---

## How it works

```
┌──────────────────────────────────────────────────────────────┐
│                        nube-server                           │
│                                                              │
│  Embedded NATS (127.0.0.1:4222, JetStream)                  │
│                                                              │
│  Plugin Registry                                             │
│    ├── watches extension.register / extension.deregister     │
│    ├── caches manifests + injects tools/prompts into MCP     │
│    ├── monitors heartbeats (extension.health.<name>)         │
│    └── proxies tool calls via NATS request/reply             │
│                                                              │
│  AI / Commands / Workflows see plugin tools as native:       │
│    MCP ──→ ToolService ──→ PluginProxy ──→ NATS req/reply   │
│    Command Router ──→ same path                              │
│    Workflow stage ──→ same path                              │
│                                                              │
│  Bus events flow to plugins automatically:                   │
│    Bus publishes workflow.> / job.> / command.>               │
│    Plugins subscribe to whatever they care about             │
└──────────────────────┬───────────────────────────────────────┘
                       │ NATS (localhost:4222 or unix socket)
                       │
     ┌─────────────────┴───────────────────────────────────────┐
     │                    Plugins                               │
     │                                                         │
     │  Python ML        Node.js scraper      Go hardware      │
     │  nats.connect()   nats.connect()       nats.Connect()   │
     └─────────────────────────────────────────────────────────┘
```

---

## NATS configuration change

The embedded NATS server currently runs with `DontListen: true` (in-process only). Plugins need a TCP listener — but only on localhost.

```go
opts := &natsserver.Options{
    DontListen: false,             // was true — open for local plugin connections
    Host:       "127.0.0.1",       // localhost only, not exposed externally
    Port:       4222,
    StoreDir:   filepath.Join(dataDir, "nats"),
    JetStream:  true,
}
```

Plugins connect to `nats://127.0.0.1:4222`. No auth initially (localhost trust), but NATS supports token/TLS auth when needed.

---

## Plugin lifecycle

### 1. Register

Plugin starts, connects to NATS, sends its manifest via NATS **request/reply** (not fire-and-forget publish):

```
Subject: extension.register  (NATS Request — plugin waits for reply)
Payload:
{
  "protocol_version": 1,
  "name": "weather-plugin",
  "version": "1.2.0",
  "description": "Weather data and forecast tools",
  "services": ["tools", "prompts", "handler"],
  "tools": [
    {
      "name": "get_forecast",
      "description": "Get weather forecast for a location",
      "parameters": {
        "type": "object",
        "properties": {
          "city": { "type": "string", "description": "City name" },
          "days": { "type": "integer", "description": "Forecast days (1-7)", "default": 3 }
        },
        "required": ["city"]
      }
    },
    {
      "name": "get_alerts",
      "description": "Get active weather alerts for a region",
      "parameters": {
        "type": "object",
        "properties": {
          "region": { "type": "string" }
        },
        "required": ["region"]
      }
    }
  ],
  "prompts": [
    {
      "name": "weather_report",
      "description": "Generate a weather report for a region",
      "template": "Analyze the weather data for {{region}} and write a summary report.",
      "arguments": [
        { "name": "region", "description": "Region to report on", "required": true }
      ]
    }
  ],
  "preamble": "This plugin provides weather data tools. Use get_forecast for specific cities and get_alerts for regional warnings."
}
```

### More manifest examples

**Adapter plugin** — adds Telegram as a command bus channel:

```json
{
  "protocol_version": 1,
  "name": "telegram-adapter",
  "version": "1.0.0",
  "services": ["adapter"],
  "adapter": {
    "channel": "telegram",
    "parse_config": {
      "bare_text_behaviour": "require_mention",
      "mention_prefix": "@bizzy"
    }
  }
}
```

**Workflow plugin** — provides workflow definitions that execute stages via NATS:

```json
{
  "protocol_version": 1,
  "name": "ml-pipeline",
  "version": "2.0.0",
  "services": ["tools", "workflows"],
  "tools": [
    { "name": "preprocess", "description": "Clean and normalise a dataset", "parameters": { "..." : "..." } },
    { "name": "train", "description": "Train a model", "parameters": { "..." : "..." } },
    { "name": "evaluate", "description": "Evaluate model accuracy", "parameters": { "..." : "..." } }
  ],
  "workflows": [
    {
      "name": "train-and-eval",
      "description": "Full ML pipeline: preprocess → train → evaluate",
      "stages": [
        { "name": "prep", "tool": "plugin.ml-pipeline.preprocess" },
        { "name": "train", "tool": "plugin.ml-pipeline.train" },
        { "name": "review", "type": "approval" },
        { "name": "eval", "tool": "plugin.ml-pipeline.evaluate" }
      ]
    }
  ]
}
```

**Handler-only plugin** — no tools, just reacts to events:

```json
{
  "protocol_version": 1,
  "name": "pagerduty-sync",
  "version": "1.0.0",
  "services": ["handler"],
  "description": "Creates PagerDuty incidents when workflows fail"
}
```

The handler plugin subscribes to `workflow.failed` / `job.failed` on NATS itself — no manifest config needed for event subscriptions, since any NATS client can subscribe to any topic.

### Manifest fields

| Field | Required | Description |
|---|---|---|
| `protocol_version` | yes | Integer. Current: `1`. Bizzy rejects plugins with an unsupported protocol version so manifest/payload format changes don't silently break plugins. |
| `name` | yes | Unique plugin name. Namespaced under `plugin.<name>.*` |
| `version` | yes | Semver. Plugin's own version. Bizzy logs on register/reload |
| `services` | yes | What this plugin provides: `["tools"]`, `["adapter"]`, `["tools", "workflows", "handler"]`, etc. |
| `description` | no | Shown in `GET /api/plugins` and tool browser |
| `tools` | no | Tool definitions (required if services includes `tools`) |
| `prompts` | no | Prompt templates with `{{key}}` substitution (required if services includes `prompts`) |
| `workflows` | no | Workflow definitions with stages (required if services includes `workflows`) |
| `adapter` | no | Adapter config: channel name, parse rules (required if services includes `adapter`) |
| `preamble` | no | Context injected into AI prompts — tells the AI what the plugin can do |
| `heartbeat_interval_sec` | no | Heartbeat publish interval in seconds (default 10, min 5, max 60). Bizzy uses `interval * 1.5` as the stale threshold |

These map to the same concepts in apps where applicable:

| App concept | Plugin equivalent |
|---|---|
| `tools/*.json` schema | `manifest.tools[].parameters` |
| `prompts/*.md` templates | `manifest.prompts[].template` |
| `workflows/*.yaml` | `manifest.workflows[]` |
| `preamble` file | `manifest.preamble` field |
| `app.yaml` name/description | `manifest.name` / `manifest.description` |

Bizzy receives this as a NATS request and **replies directly** to the plugin — no separate ack topic, no race condition:

1. Validates the manifest (name unique, services valid, required fields present, `protocol_version` compatible)
2. Stores in SQLite + in-memory cache
3. For each service in `manifest.services`:
   - `tools` → injects into ToolService + MCPFactory, namespaced `plugin.<name>.<tool>`
   - `prompts` → registers in MCPFactory, namespaced `plugin.<name>.<prompt>`
   - `workflows` → registers workflow definitions with the Workflow engine
   - `adapter` → registers with AdapterRegistry as a new channel, routes commands/replies over NATS
   - `handler` → no bizzy-side wiring needed (plugin self-subscribes to bus topics)
4. Adds preamble to prompt enrichment (if present)
5. Subscribes to `extension.health.<name>` for heartbeat monitoring
6. **Replies** to the request with success or error:

```
Success reply:
{ "status": "ok", "tools_registered": 2, "services_wired": ["tools", "prompts", "handler"] }

Error reply:
{ "status": "error", "error": "manifest validation failed: tools[1] missing 'name' field" }
```

The plugin uses `nc.Request()` (Go), `nc.request()` (Node/Python) with a timeout. If the reply is an error or times out, the plugin knows registration failed and can log/retry.

### 2. Heartbeat

Plugin publishes periodically. Default interval is 10 seconds, configurable via the manifest `heartbeat_interval_sec` field (min 5, max 60):

```
Subject: extension.health.weather-plugin
Payload: { "status": "ok" }
```

Bizzy's health monitor watches these. If 3 consecutive heartbeats are missed, the plugin is marked `crashed` and its services are unwired. The stale threshold is `heartbeat_interval * 1.5` (default: 15s).

**Startup grace period**: when bizzy restarts and reloads plugins from the DB, it sets `LastHeartbeat = time.Now()` and `HealthFailures = 0` for all reloaded plugins. This gives plugins time to reconnect without being falsely marked as crashed. The grace window is 60 seconds — after that, normal health checking applies.

### 3. Hot-reload

Plugin sends `nc.Request("extension.register", ...)` again with the same name. Bizzy treats a name collision as a reload:

1. Re-reads the manifest
2. Diffs services: removes stale tools/workflows/prompts, adds new, updates changed
3. Swaps atomically in ToolService/MCPFactory/WorkflowEngine
4. **Sets status to `active`** — this clears `crashed` status if the plugin is recovering from a crash
5. Replies with the diff summary

This is the hot-reload path. The plugin can change its tool set, add/remove services, update descriptions — all without bizzy restarting. It's also the **crash recovery** path: a plugin that crashed and restarted just re-registers and is immediately active again.

```
Plugin deploys new version (or restarts after crash) →
  nc.Request("extension.register", updatedManifest) →
    bizzy diffs, swaps services, clears crashed status →
      AI immediately sees updated tools via MCP
```

### 4. Unload

Three paths:

| Trigger | What happens |
|---|---|
| Plugin publishes `extension.deregister` | Clean unload — tools removed, subscriptions dropped |
| Heartbeat timeout (3 missed) | Auto-unload — same as above, status set to `crashed` |
| Admin calls `DELETE /api/plugins/{name}` | Force unload — tools removed even if plugin is still running |

On unload:
- Tools disappear from ToolService/MCP immediately
- In-flight tool calls get a NATS timeout error (returned to caller as a tool error)
- Plugin's event subscriptions are its own — NATS cleans them up when the connection drops

```
Subject: extension.deregister
Payload: { "name": "weather-plugin" }
```

---

## Tool calls — NATS request/reply

When the AI (or a command, or a workflow stage) calls a plugin tool, bizzy uses NATS request/reply.

Plugins subscribe to their tool subjects using a **queue group** named after the plugin. This ensures only one instance receives each request (load-balanced if multiple instances are running) and prevents other processes from intercepting tool calls meant for a specific plugin:

```
Bizzy sends:
  Subject: tool.call.weather-plugin.get_forecast
  Reply:   _INBOX.abc123
  Payload:
  {
    "params": { "city": "Sydney", "days": 5 },
    "context": {
      "user_id": "usr-123",
      "command_id": "cmd-456",
      "timeout_ms": 30000
    }
  }

Plugin receives on tool.call.weather-plugin.get_forecast, processes, replies:
  Subject: _INBOX.abc123
  Payload:
  {
    "result": {
      "city": "Sydney",
      "forecast": [
        { "day": "Mon", "high": 24, "low": 18, "condition": "Sunny" },
        { "day": "Tue", "high": 22, "low": 17, "condition": "Partly cloudy" }
      ]
    }
  }
```

Or on error:

```json
{
  "error": "Weather API rate limit exceeded",
  "retryable": true
}
```

NATS request/reply has a built-in timeout (configurable per-plugin, default 30s). No HTTP connections, no connection pooling, no retry logic — NATS handles it.

### Backpressure

Plugins should limit concurrency on their tool call subscriptions. If a plugin is slow and 50 tool calls pile up, unbounded goroutines will cause cascading timeouts. Use NATS queue subscriptions with bounded workers:

```go
// Go: bounded to 8 concurrent tool calls
nc.QueueSubscribe("tool.call.my-plugin.*", "my-plugin", handler,
    nats.MaxAckPending(8))
```

Bizzy-side, the proxy uses the NATS request timeout (default 30s). If a plugin doesn't respond in time, the caller gets a timeout error — no indefinite hangs.

### Tool call flow through the system

```
User: "what's the weather in Sydney?"
  → AI (Claude/Ollama) decides to call plugin.weather-plugin.get_forecast
    → ToolService.CallTool("plugin.weather-plugin.get_forecast", {city: "Sydney"})
      → PluginProxy.Call("weather-plugin", "get_forecast", params)
        → NATS Request to tool.call.weather-plugin.get_forecast (timeout 30s)
          → Plugin receives, calls weather API, replies
        ← NATS Reply with result
      ← return to ToolService
    ← return to AI
  → AI formats response: "Sydney forecast: Sunny, 24C..."
```

---

## Event streaming

Plugins subscribe to whatever bus topics they care about. No configuration needed — just subscribe.

```python
# Python plugin subscribing to workflow events
import asyncio, json, nats

async def main():
    nc = await nats.connect("nats://127.0.0.1:4222")

    # Simple subscribe — get all workflow completions
    async def on_workflow_done(msg):
        data = json.loads(msg.data)
        print(f"Workflow {data['target_name']} completed")

    await nc.subscribe("workflow.completed", cb=on_workflow_done)

    # Wildcard — get all job events
    await nc.subscribe("job.>", cb=on_job_event)

    # Durable — survives plugin restart, picks up missed events
    js = nc.jetstream()
    await js.subscribe("workflow.>",
        durable="weather-plugin-workflows",
        cb=on_workflow_event)
```

```javascript
// Node.js plugin subscribing to events
const { connect, JSONCodec } = require('nats');

async function main() {
  const nc = await connect({ servers: 'nats://127.0.0.1:4222' });
  const jc = JSONCodec();

  // Subscribe to all command events
  const sub = nc.subscribe('command.>');
  for await (const msg of sub) {
    const data = jc.decode(msg.data);
    console.log(`Command ${data.verb} on ${data.target_kind}/${data.target_name}`);
  }
}

main();
```

```go
// Go plugin subscribing to events
nc, _ := nats.Connect("nats://127.0.0.1:4222")
js, _ := nc.JetStream()

// Durable subscription — picks up where it left off after restart
js.Subscribe("job.>", func(msg *nats.Msg) {
    var ev EventData
    json.Unmarshal(msg.Data, &ev)
    log.Printf("Job %s: %s", ev.TargetID, ev.Status)
    msg.Ack()
}, nats.Durable("my-go-plugin-jobs"))
```

### Plugins publishing events

Plugins can emit their own events on the bus:

```
Subject: extension.event.weather-plugin.alert
Payload:
{
  "message": "Severe weather warning for Sydney region",
  "severity": "critical",
  "region": "Sydney"
}
```

Other plugins, adapters, or the notification system can subscribe to `extension.event.>` or `extension.event.weather-plugin.>`.

---

## Service types in detail

### `tools` — extend AI capabilities

Plugin tools appear in MCP, ToolService, CLI, workflows, and the command bus — identical to app tools. Namespaced as `plugin.<name>.<tool>`.

Bizzy dispatches tool calls via NATS request/reply (see "Tool calls" above). The plugin just subscribes to `tool.call.<name>.*` and responds.

### `prompts` — reusable AI prompt templates

Same as app prompts — markdown templates with `{{key}}` substitution, served via MCP. The AI or user can request them by name.

### `workflows` — register workflow definitions

A plugin can register full workflow definitions (stages, approval gates, failure handling). These work exactly like YAML-defined workflows from apps, but the definition comes from the plugin manifest instead of a file on disk.

Workflow stages that reference plugin tools (e.g. `tool: plugin.ml-pipeline.preprocess`) are executed via the normal NATS tool call path. Stages can also reference built-in app tools — a plugin workflow can mix plugin and app tools freely.

```
Plugin registers workflow "train-and-eval":
  [preprocess] → [train] → [review (approval)] → [evaluate]
       │             │                                 │
       └─────────────┴─────────────────────────────────┘
       All stages call back to the plugin via NATS tool.call.*
```

Workflows registered by plugins appear in `GET /api/workflows` and can be started the same way:

```
nube workflow run plugin.ml-pipeline.train-and-eval --dataset sales.csv
run workflow/plugin.ml-pipeline.train-and-eval --dataset sales.csv   (Slack)
POST /api/workflows/run  {"name": "plugin.ml-pipeline.train-and-eval", ...}
```

### `adapter` — new command bus channels

An adapter plugin acts as a new ingress/egress channel for the command bus. It receives messages from an external system (Telegram, Discord, SMS, etc.), converts them to commands, and sends replies back.

The adapter communicates with bizzy's command router over NATS:

```
External system → Plugin receives message
  → Plugin publishes to adapter.command.<name>:
    {
      "message_id": "tg-msg-12345-678",
      "text": "run workflow/weekly-report --site Sydney",
      "user_id": "tg-user-123",
      "reply_to": {
        "channel": "telegram",
        "address": {"chat_id": 12345, "message_id": 678}
      }
    }

  → Bizzy's command router parses, dispatches, publishes result

  → Plugin subscribes to adapter.reply.<name>:
    {
      "reply_to": {"channel": "telegram", "address": {"chat_id": 12345}},
      "text": "Workflow started (wf-7a3b)"
    }

  → Plugin sends reply back to Telegram
```

The plugin handles all external protocol details (Telegram API, Discord gateway, etc.). Bizzy only sees commands and replies on NATS. This means adding a new channel requires zero changes to bizzy — just a new plugin.

The `message_id` field is required for deduplication — external platforms often retry delivery (Telegram webhooks, Discord gateway reconnects). Bizzy's command router uses the existing dedup cache to reject duplicate `message_id` values within a TTL window.

NATS subjects for adapters:

```
adapter.command.<name>           — plugin → bizzy: parsed command
adapter.reply.<name>             — bizzy → plugin: reply to send
adapter.command.<name>.raw       — plugin → bizzy: raw text for bizzy to parse
```

### `handler` — react to bus events

A handler plugin subscribes directly to NATS bus topics. No manifest config needed — the plugin just calls `nc.subscribe()` on whatever topics it cares about. The `handler` service type is declared in the manifest so bizzy knows the plugin exists (health monitoring, admin visibility), but the actual subscriptions are managed by the plugin itself.

Use cases:
- PagerDuty integration — subscribe to `workflow.failed`, create incidents
- Audit logger — subscribe to `command.>`, write to external log
- Metrics exporter — subscribe to `job.>`, push to Prometheus/Grafana
- Cross-system sync — subscribe to `tool.completed`, update external DB

---

## Topic taxonomy — additions

```
extension.>
    extension.register                     — plugin manifest (NATS request/reply — response is success/error)
    extension.deregister                   — plugin clean shutdown
    extension.health.<name>                — heartbeat (configurable interval, default 10s)
    extension.event.<name>.*               — custom events published by plugins

tool.call.<plugin>.<tool>                  — bizzy → plugin tool invocation (NATS request/reply, queue group)

adapter.command.<name>                     — adapter plugin → bizzy: inbound command (with message_id for dedup)
adapter.command.<name>.raw                 — adapter plugin → bizzy: raw text for bizzy to parse
adapter.reply.<name>                       — bizzy → adapter plugin: outbound reply

plugin.rpc.<target>.<method>               — plugin-to-plugin direct communication (convention, not enforced)
```

These sit alongside the existing bus topics (`command.>`, `workflow.>`, `job.>`, `tool.>`).

---

## Data model

```sql
CREATE TABLE plugins (
    name            TEXT PRIMARY KEY,
    version         TEXT NOT NULL,
    description     TEXT,
    services        TEXT NOT NULL,              -- JSON array: ["tools", "adapter", "handler"]
    manifest        TEXT NOT NULL,              -- full JSON manifest
    status          TEXT NOT NULL DEFAULT 'active',  -- active, crashed, disabled
    registered_at   DATETIME NOT NULL,
    last_heartbeat  DATETIME,
    health_failures INTEGER DEFAULT 0
);
```

The `services` column is a JSON array. Query with SQLite's `json_each()`:

```sql
-- Find all plugins that provide the 'adapter' service
SELECT p.* FROM plugins p, json_each(p.services) AS s WHERE s.value = 'adapter';

-- Find all active tool providers
SELECT p.* FROM plugins p, json_each(p.services) AS s
WHERE s.value = 'tools' AND p.status = 'active';
```

The full manifest JSON is the source of truth. On bizzy restart, it reloads all `active` plugins from the DB, re-wires their services, and applies a startup grace period for health checks (see "Heartbeat" above).

---

## REST API

For admin visibility and control. Plugins themselves never use these — they only speak NATS.

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/plugins` | List all plugins (name, version, status, services, last heartbeat). Filter: `?service=tools` |
| `GET` | `/api/plugins/{name}` | Plugin detail + full manifest |
| `DELETE` | `/api/plugins/{name}` | Force unload |
| `POST` | `/api/plugins/{name}/disable` | Disable (tools removed, plugin can still connect) |
| `POST` | `/api/plugins/{name}/enable` | Re-enable (re-reads manifest, restores tools) |

---

## Plugin implementation — what you need

Every plugin, regardless of service type:

1. Connect to NATS
2. Publish manifest to `extension.register` (with `services` declaring what you provide)
3. Subscribe to the relevant NATS subjects for your service type(s):
   - `tools` → subscribe to `tool.call.<name>.*`, respond to requests
   - `adapter` → subscribe to `adapter.reply.<name>`, publish to `adapter.command.<name>`
   - `handler` → subscribe to whatever bus topics you care about (`workflow.>`, `job.>`, etc.)
   - `workflows` → no subscriptions needed (bizzy handles execution, calls your tools via NATS)
   - `prompts` → no subscriptions needed (bizzy serves them from the manifest)
4. Publish heartbeat to `extension.health.<name>` every 10s
5. On shutdown, publish to `extension.deregister`

### Go example

```go
package main

import (
    "encoding/json"
    "log"
    "time"
    "github.com/nats-io/nats.go"
)

func main() {
    nc, _ := nats.Connect("nats://127.0.0.1:4222")
    defer nc.Close()

    // 1. Register (request/reply — get success or error back)
    manifest, _ := json.Marshal(map[string]any{
        "protocol_version": 1,
        "name":             "my-plugin",
        "version":          "0.1.0",
        "services":         []string{"tools"},
        "tools": []map[string]any{
            {
                "name":        "ping",
                "description": "Ping a host",
                "parameters": map[string]any{
                    "type": "object",
                    "properties": map[string]any{
                        "host": map[string]any{"type": "string"},
                    },
                    "required": []string{"host"},
                },
            },
        },
    })
    reply, err := nc.Request("extension.register", manifest, 5*time.Second)
    if err != nil {
        log.Fatalf("registration failed: %v", err)
    }
    log.Printf("registered: %s", reply.Data)

    // 2. Handle tool calls (queue group = only this plugin gets its own calls)
    nc.QueueSubscribe("tool.call.my-plugin.*", "my-plugin", func(msg *nats.Msg) {
        var req struct {
            Params map[string]any `json:"params"`
        }
        json.Unmarshal(msg.Data, &req)

        host := req.Params["host"].(string)
        // ... do the ping ...

        result, _ := json.Marshal(map[string]any{
            "result": map[string]any{"host": host, "latency_ms": 12},
        })
        msg.Respond(result)
    })

    // 3. Heartbeat
    ticker := time.NewTicker(10 * time.Second)
    go func() {
        for range ticker.C {
            nc.Publish("extension.health.my-plugin", []byte(`{"status":"ok"}`))
        }
    }()

    select {}
}
```

### Node.js example

```javascript
const { connect, JSONCodec } = require('nats');

async function main() {
  const nc = await connect({ servers: 'nats://127.0.0.1:4222' });
  const jc = JSONCodec();

  // 1. Register (request/reply)
  const reply = await nc.request('extension.register', jc.encode({
    protocol_version: 1,
    name: 'my-node-plugin',
    version: '0.1.0',
    services: ['tools'],
    tools: [{
      name: 'scrape',
      description: 'Scrape a webpage and return text content',
      parameters: {
        type: 'object',
        properties: { url: { type: 'string', description: 'URL to scrape' } },
        required: ['url']
      }
    }]
  }), { timeout: 5000 });
  console.log('registered:', jc.decode(reply.data));

  // 2. Handle tool calls (queue group)
  const sub = nc.subscribe('tool.call.my-node-plugin.*', { queue: 'my-node-plugin' });
  (async () => {
    for await (const msg of sub) {
      const { params } = jc.decode(msg.data);
      try {
        const text = await scrape(params.url);
        msg.respond(jc.encode({ result: { text } }));
      } catch (err) {
        msg.respond(jc.encode({ error: err.message }));
      }
    }
  })();

  // 3. Heartbeat
  setInterval(() => {
    nc.publish('extension.health.my-node-plugin', jc.encode({ status: 'ok' }));
  }, 10_000);
}

main();
```

### Python example

```python
import asyncio, json, nats

async def main():
    nc = await nats.connect("nats://127.0.0.1:4222")

    # 1. Register (request/reply)
    reply = await nc.request("extension.register", json.dumps({
        "protocol_version": 1,
        "name": "my-python-plugin",
        "version": "0.1.0",
        "services": ["tools"],
        "tools": [{
            "name": "analyze",
            "description": "Run ML analysis on a dataset",
            "parameters": {
                "type": "object",
                "properties": {
                    "dataset": {"type": "string"},
                    "model": {"type": "string", "default": "linear"}
                },
                "required": ["dataset"]
            }
        }]
    }).encode(), timeout=5)
    print(f"registered: {json.loads(reply.data)}")

    # 2. Handle tool calls (queue group)
    async def on_call(msg):
        req = json.loads(msg.data)
        params = req["params"]
        result = run_analysis(params["dataset"], params.get("model", "linear"))
        await msg.respond(json.dumps({"result": result}).encode())

    await nc.subscribe("tool.call.my-python-plugin.*",
                       queue="my-python-plugin", cb=on_call)

    # 3. Heartbeat
    async def heartbeat():
        while True:
            await nc.publish("extension.health.my-python-plugin",
                           b'{"status":"ok"}')
            await asyncio.sleep(10)

    asyncio.create_task(heartbeat())
    await asyncio.Event().wait()

asyncio.run(main())
```

---

## Bizzy-side implementation

### Plugin registry (`pkg/extensions/registry.go`)

```go
type Registry struct {
    mu      sync.RWMutex
    plugins map[string]*Plugin      // name → plugin
    db      *gorm.DB
    nc      *nats.Conn
    tools   *services.ToolService   // inject/remove tools
}

type Plugin struct {
    Name           string
    Version        string
    Description    string
    Manifest       Manifest
    Status         string    // "active", "crashed", "disabled"
    RegisteredAt   time.Time
    LastHeartbeat  time.Time
    HealthFailures int
}

func (r *Registry) Start() {
    // Subscribe to registration
    r.nc.Subscribe("extension.register", r.handleRegister)
    r.nc.Subscribe("extension.deregister", r.handleDeregister)

    // Reload active plugins from DB on startup
    r.reloadFromDB()

    // Start health monitor
    go r.healthLoop()
}

func (r *Registry) handleRegister(msg *nats.Msg) {
    var m Manifest
    if err := json.Unmarshal(msg.Data, &m); err != nil {
        msg.Respond(errorReply("invalid JSON: " + err.Error()))
        return
    }

    // Protocol version check
    if m.ProtocolVersion != SupportedProtocolVersion {
        msg.Respond(errorReply(fmt.Sprintf(
            "unsupported protocol_version %d (server supports %d)",
            m.ProtocolVersion, SupportedProtocolVersion)))
        return
    }

    if err := m.Validate(); err != nil {
        msg.Respond(errorReply("manifest validation failed: " + err.Error()))
        return
    }

    r.mu.Lock()
    defer r.mu.Unlock()

    existing, isReload := r.plugins[m.Name]
    if isReload {
        r.diffAndSwapServices(existing.Manifest, m)
    } else {
        r.wireServices(m)
    }

    plugin := &Plugin{
        Name:         m.Name,
        Version:      m.Version,
        Description:  m.Description,
        Manifest:     m,
        Status:       "active",  // clears "crashed" if plugin re-registers after a crash
        RegisteredAt: time.Now(),
        LastHeartbeat: time.Now(),
        HealthFailures: 0,
    }
    r.plugins[m.Name] = plugin
    r.saveToDB(plugin)

    // Reply directly to the requesting plugin
    ack, _ := json.Marshal(map[string]any{
        "status":         "ok",
        "services_wired": m.Services,
        "reloaded":       isReload,
    })
    msg.Respond(ack)
}
```

### Tool proxy (`pkg/extensions/proxy.go`)

```go
type Proxy struct {
    nc      *nats.Conn
    timeout time.Duration // default 30s
}

// Call sends a tool request to a plugin and waits for the reply.
// This implements the same interface as JS/OpenAPI tool execution,
// so ToolService doesn't know or care that it's a plugin.
func (p *Proxy) Call(pluginName, toolName string, params map[string]any, ctx ToolContext) (any, error) {
    subject := fmt.Sprintf("tool.call.%s.%s", pluginName, toolName)

    payload, _ := json.Marshal(ToolRequest{
        Params:  params,
        Context: ctx,
    })

    msg, err := p.nc.Request(subject, payload, p.timeout)
    if err != nil {
        return nil, fmt.Errorf("plugin %s tool %s: %w", pluginName, toolName, err)
    }

    var resp ToolResponse
    json.Unmarshal(msg.Data, &resp)
    if resp.Error != "" {
        return nil, fmt.Errorf("plugin %s tool %s: %s", pluginName, toolName, resp.Error)
    }
    return resp.Result, nil
}
```

### Health monitor (`pkg/extensions/health.go`)

```go
func (r *Registry) healthLoop() {
    // Subscribe to all heartbeats
    r.nc.Subscribe("extension.health.*", func(msg *nats.Msg) {
        parts := strings.Split(msg.Subject, ".")
        name := parts[2]

        r.mu.Lock()
        if p, ok := r.plugins[name]; ok {
            p.LastHeartbeat = time.Now()
            p.HealthFailures = 0
        }
        r.mu.Unlock()
    })

    // Check for missed heartbeats every 10s
    ticker := time.NewTicker(10 * time.Second)
    for range ticker.C {
        r.mu.Lock()
        for name, p := range r.plugins {
            if p.Status != "active" {
                continue
            }
            staleThreshold := time.Duration(float64(p.HeartbeatInterval) * 1.5)
            if time.Since(p.LastHeartbeat) > staleThreshold {
                p.HealthFailures++
                if p.HealthFailures >= 3 {
                    p.Status = "crashed"
                    r.removeServices(p.Manifest)
                    r.saveToDB(p)
                    log.Warn().Str("plugin", name).Msg("plugin crashed — services unwired")
                }
            }
        }
        r.mu.Unlock()
    }
}

// reloadFromDB sets grace period so plugins aren't falsely marked crashed on bizzy restart
func (r *Registry) reloadFromDB() {
    var plugins []PluginModel
    r.db.Where("status = ?", "active").Find(&plugins)

    now := time.Now()
    for _, pm := range plugins {
        p := pluginFromModel(pm)
        p.LastHeartbeat = now   // grace period — don't penalise for stale DB timestamp
        p.HealthFailures = 0
        r.plugins[p.Name] = p
        r.wireServices(p.Manifest)
    }
}
```

---

## File layout

```
pkg/extensions/
    registry.go        — Plugin registry: register, unload, reload, manifest cache
    proxy.go           — NATS request/reply tool call proxy
    health.go          — Heartbeat monitor, crash detection, auto-unload
    events.go          — (future) Plugin event routing rules if needed
    models.go          — Plugin, Manifest, ToolSpec, ToolRequest, ToolResponse structs

pkg/api/
    plugins_handler.go — REST endpoints for admin visibility/control

pkg/models/
    plugin.go          — Plugin DB model (for GORM)
```

---

## How plugins fit with existing systems

### Tool execution — three backends, one interface

```
                     MCPFactory.BuildServer()
                              │
                    registers tools from all sources
                              │
              ┌───────────────┼───────────────┐
              │               │               │
         JS tools       OpenAPI proxy    Plugin proxy
       (Goja sandbox)   (HTTP to API)   (NATS req/reply)
              │               │               │
       apps/<name>/     apps/<name>/    separate process
       tools/*.js       openapi.yaml    (any language)
```

When MCPFactory builds a per-user MCP server, it already iterates the user's installed apps and registers JS + OpenAPI tools. Plugin tools are added the same way — the Plugin Registry injects them into MCPFactory's tool list so they appear alongside app tools.

From the AI's perspective, all three tool types are identical MCP tools with a name, description, and JSON schema. The execution backend is an implementation detail.

### MCPFactory integration

```go
// In MCPFactory.BuildServer(installs):

// 1. Register app tools (existing)
for _, install := range installs {
    app := registry.Get(install.AppName)
    registerOpenAPITools(server, app, install)  // OpenAPI → HTTP proxy
    registerJSTools(server, app, install)       // JS → Goja sandbox
    registerPrompts(server, app, install)       // Markdown templates
}

// 2. Register plugin tools (new)
for _, plugin := range pluginRegistry.ActivePlugins() {
    for _, tool := range plugin.Manifest.Tools {
        server.RegisterTool(
            "plugin."+plugin.Name+"."+tool.Name,  // same namespace pattern
            tool.Description,
            tool.Parameters,
            func(params map[string]any) (any, error) {
                return pluginProxy.Call(plugin.Name, tool.Name, params)
            },
        )
    }
}

// 3. Register plugin prompts (new, optional)
for _, plugin := range pluginRegistry.ActivePlugins() {
    for _, prompt := range plugin.Manifest.Prompts {
        server.RegisterPrompt(
            "plugin."+plugin.Name+"."+prompt.Name,
            prompt.Description,
            prompt.Template,
            prompt.Arguments,
        )
    }
}
```

### What the AI sees

After a plugin registers, the AI's tool list includes plugin tools alongside app tools with no distinction:

```
Tools available:
  rubix.query_nodes          — Query BACnet devices         (JS tool, rubix app)
  rubix.control_point        — Set a BACnet point value     (JS tool, rubix app)
  weather.get_weather        — Get current weather          (OpenAPI tool, weather app)
  plugin.ml-service.analyze  — Run ML analysis on dataset   (plugin tool, Python process)
  plugin.scraper.scrape      — Scrape a webpage             (plugin tool, Node.js process)
```

The AI calls `plugin.ml-service.analyze` exactly like it calls `rubix.query_nodes`. The MCP protocol, tool schemas, and calling convention are identical.

### All entry points work

| System | How it sees plugin tools |
|---|---|
| MCP (Claude, Cursor) | Listed alongside app tools, callable natively |
| CLI (`nube tools`) | Shows in the tool list, callable via `nube ask` |
| Workflows | Stage tool: `tool: plugin.ml-service.analyze` |
| Command bus | `run tool/plugin.ml-service.analyze --dataset sales.csv` |
| Flutter app | Appears in tool browser |
| REST API | `POST /api/agents/tools/plugin.ml-service.analyze` |
| Async jobs | AI calls plugin tools during job execution |

---

## Comparison: apps vs plugins

Both extend the same system. Apps run inside bizzy, plugins run outside it.

| | Apps | Plugins |
|---|---|---|
| **What they provide** | Tools, prompts, workflows, preamble | Tools, prompts, workflows, adapters, event handlers, preamble |
| **Runtime** | In-process (JS sandbox / OpenAPI proxy) | Separate process (any language) |
| **Lifecycle** | Managed by bizzy (load from disk) | Self-managed (you start/stop) |
| **Communication** | Direct function call / HTTP | NATS |
| **Hot-reload** | `POST /admin/reload-apps` | Re-publish manifest to NATS |
| **Event access** | Bus publish only (one-way) | Full NATS subscribe (bidirectional) |
| **Namespace** | `appName.*` | `plugin.pluginName.*` |
| **Use case** | Lightweight tools, config-driven integrations | Heavy workloads, custom runtimes, new channels, background services |
| **Install** | App store, per-user | System-wide, admin-managed |
| **Isolation** | Goja sandbox (CPU/memory limits) | OS process isolation |

### When to use which

- **App**: you need a tool that calls a REST API, runs some JS logic, or wraps an OpenAPI spec. No custom runtime needed.
- **Plugin with `tools`**: you need Python, Go, Rust, etc. — anything that doesn't fit in a 5-second JS sandbox.
- **Plugin with `adapter`**: you want to add a new channel (Telegram, Discord, SMS, custom webhook format) without touching bizzy code.
- **Plugin with `workflows`**: you want to register workflow definitions dynamically from an external system.
- **Plugin with `handler`**: you want to react to system events (PagerDuty alerts, metrics export, audit logging, cross-system sync).

---

## Plugin-to-plugin communication

Plugins can communicate directly over NATS without going through bizzy:

- **Events**: Plugin A publishes to `extension.event.plugin-a.something`, Plugin B subscribes to `extension.event.plugin-a.>`.
- **Request/reply**: Plugin A sends a NATS request to a subject Plugin B subscribes to. No bizzy involvement — it's just NATS.
- **Via tool calls**: Plugin A can ask the AI to call Plugin B's tools (indirect, AI-mediated).

For direct plugin-to-plugin request/reply, use a convention:

```
Subject: plugin.rpc.<target-plugin>.<method>
```

This is not enforced or managed by bizzy — it's a convention plugins agree on. Bizzy doesn't need to know about it.

---

## Security considerations

- **Localhost only** — NATS listens on `127.0.0.1`, not `0.0.0.0`. Plugins must run on the same machine (or use SSH tunnels / overlay networks for remote).
- **No auth initially** — localhost trust model. This is a known limitation for v1. Any local process can connect to NATS and register/subscribe. Acceptable for single-machine deployments where you control what runs on the box. For production hardening, NATS supports token auth, TLS client certs, and NKey authentication — add it without changing the plugin protocol. See "Future" for the auth roadmap.
- **Queue groups for tool calls** — plugins subscribe to their tool subjects via queue groups (`nc.QueueSubscribe("tool.call.my-plugin.*", "my-plugin", ...)`). This ensures only one subscriber handles each request and provides basic isolation between plugins. It does not prevent a rogue process from joining the queue group (that requires NATS auth).
- **Tool namespacing** — plugins can only register tools under `plugin.<their-name>.*`. Bizzy rejects manifests that try to register tools without the `plugin.` prefix. Plugins cannot shadow built-in app tools.
- **Protocol version** — bizzy rejects plugins with an unsupported `protocol_version`, preventing silent breakage when the manifest/payload format changes.
- **Adapter deduplication** — adapter plugins must include a `message_id` on inbound commands. Bizzy deduplicates via the existing command dedup cache, preventing duplicate execution from platform retries.
- **Resource limits** — NATS `MaxAckPending` prevents a misbehaving plugin from flooding the bus. Tool call timeouts (default 30s) prevent hung plugins from blocking the AI.
- **No filesystem access** — plugins don't get access to bizzy's data directory. They only interact through NATS messages.

---

## Future

- **NATS authentication** — per-plugin NKey or token auth. Each plugin gets a credential on registration (via REST API or config file). NATS authorization rules restrict which subjects each plugin can publish/subscribe to (e.g., `weather-plugin` can only subscribe to `tool.call.weather-plugin.*`, not `tool.call.other-plugin.*`). This closes the name-squatting and interception vectors flagged in the security section.
- **Plugin marketplace** — plugins packaged as Docker images or binaries, installable from a registry
- **Remote plugins** — NATS leaf nodes or gateway for plugins running on other machines
- **Plugin permissions** — restrict which bus topics a plugin can subscribe to or publish on, which services it can register
- **Plugin config** — plugins declare settings in their manifest, configurable via REST API, delivered in the registration reply
- **Streaming tool results** — for long-running tools, plugin publishes progress on `tool.progress.<plugin>.<tool>` before sending the final reply
- **Service type: `runner`** — plugin provides an AI runner (custom LLM backend), registered alongside Claude/Ollama/OpenAI
- **Service type: `storage`** — plugin provides a storage backend (S3, GCS, etc.) for workflow artifacts
- **Plugin dependencies** — manifest declares required plugins/apps, bizzy validates on registration
