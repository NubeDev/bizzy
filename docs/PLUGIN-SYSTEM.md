# Plugin System — Separate-Process Extensions over NATS

Plugins are independent processes that connect to bizzy's embedded NATS server. They can be written in any language (Go, Node.js, Python, Rust, etc.), run as their own process, and communicate entirely over NATS subjects. Bizzy does not start or stop plugins — that's handled externally (systemd, Docker, supervisor, or manually).

**This does not replace apps.** Apps are bundles of tools/prompts/workflows that run inside bizzy (JS sandbox, OpenAPI proxy). Plugins are for heavier workloads that need their own runtime — a Python ML pipeline, a Node.js scraper, a Go service that talks to proprietary hardware.

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
│    ├── caches manifests + injects tools into ToolService     │
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

Plugin starts, connects to NATS, publishes its manifest:

```
Subject: extension.register
Payload:
{
  "name": "weather-plugin",
  "version": "1.2.0",
  "description": "Weather data and forecast tools",
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
  ]
}
```

Bizzy receives this on its `extension.register` subscription and:

1. Validates the manifest (name unique, tools have valid schemas)
2. Stores in SQLite + in-memory cache
3. Injects tools into ToolService, namespaced as `plugin.<name>.<tool>` (e.g. `plugin.weather-plugin.get_forecast`)
4. Subscribes to `extension.health.<name>` for heartbeat monitoring
5. Publishes acknowledgement:

```
Subject: extension.registered.<name>
Payload: { "status": "ok", "tools_registered": 2 }
```

### 2. Heartbeat

Plugin publishes periodically (every 10 seconds):

```
Subject: extension.health.weather-plugin
Payload: { "status": "ok" }
```

Bizzy's health monitor watches these. If 3 consecutive heartbeats are missed (30s), the plugin is marked `crashed` and its tools are removed from ToolService.

### 3. Hot-reload

Plugin publishes to `extension.register` again with the same name. Bizzy treats a name collision as a reload:

1. Re-reads the manifest
2. Diffs tools: removes stale, adds new, updates changed
3. Updates ToolService atomically
4. Publishes `extension.registered.<name>` with the diff summary

This is the hot-reload path. The plugin can change its tool set, update descriptions, add/remove tools — all without bizzy restarting.

```
Plugin deploys new version →
  publishes to extension.register with updated manifest →
    bizzy diffs, swaps tools →
      AI immediately sees new tools via MCP
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

When the AI (or a command, or a workflow stage) calls a plugin tool, bizzy uses NATS request/reply:

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

## Topic taxonomy — additions

```
extension.>
    extension.register                     — plugin manifest submission
    extension.registered.<name>            — bizzy ack after successful register
    extension.deregister                   — plugin clean shutdown
    extension.health.<name>               — heartbeat (every 10s)
    extension.event.<name>.*               — custom events published by plugins

tool.call.<plugin>.<tool>                  — bizzy → plugin tool invocation (request/reply)
```

These sit alongside the existing bus topics (`command.>`, `workflow.>`, `job.>`, `tool.>`).

---

## Data model

```sql
CREATE TABLE plugins (
    name            TEXT PRIMARY KEY,
    version         TEXT NOT NULL,
    description     TEXT,
    manifest        TEXT NOT NULL,              -- full JSON manifest
    status          TEXT NOT NULL DEFAULT 'active',  -- active, crashed, disabled
    registered_at   DATETIME NOT NULL,
    last_heartbeat  DATETIME,
    health_failures INTEGER DEFAULT 0
);
```

The manifest JSON is the source of truth for what tools the plugin provides. On bizzy restart, it reloads all `active` plugins from the DB and waits for their heartbeats to resume.

---

## REST API

For admin visibility and control. Plugins themselves never use these — they only speak NATS.

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/plugins` | List all plugins (name, version, status, tools, last heartbeat) |
| `GET` | `/api/plugins/{name}` | Plugin detail + full manifest |
| `DELETE` | `/api/plugins/{name}` | Force unload |
| `POST` | `/api/plugins/{name}/disable` | Disable (tools removed, plugin can still connect) |
| `POST` | `/api/plugins/{name}/enable` | Re-enable (re-reads manifest, restores tools) |

---

## Plugin implementation — what you need

A minimal plugin in any language:

1. Connect to NATS
2. Publish manifest to `extension.register`
3. Subscribe to `tool.call.<name>.*` for tool calls
4. Publish heartbeat to `extension.health.<name>` every 10s
5. On shutdown, publish to `extension.deregister`

### Go example

```go
package main

import (
    "encoding/json"
    "time"
    "github.com/nats-io/nats.go"
)

func main() {
    nc, _ := nats.Connect("nats://127.0.0.1:4222")
    defer nc.Close()

    // 1. Register
    manifest, _ := json.Marshal(map[string]any{
        "name":    "my-plugin",
        "version": "0.1.0",
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
    nc.Publish("extension.register", manifest)

    // 2. Handle tool calls
    nc.Subscribe("tool.call.my-plugin.ping", func(msg *nats.Msg) {
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

    // Block forever (or until signal)
    select {}
}
```

### Node.js example

```javascript
const { connect, JSONCodec } = require('nats');

async function main() {
  const nc = await connect({ servers: 'nats://127.0.0.1:4222' });
  const jc = JSONCodec();

  // 1. Register
  nc.publish('extension.register', jc.encode({
    name: 'my-node-plugin',
    version: '0.1.0',
    tools: [{
      name: 'scrape',
      description: 'Scrape a webpage and return text content',
      parameters: {
        type: 'object',
        properties: {
          url: { type: 'string', description: 'URL to scrape' }
        },
        required: ['url']
      }
    }]
  }));

  // 2. Handle tool calls
  const sub = nc.subscribe('tool.call.my-node-plugin.scrape');
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

    # 1. Register
    await nc.publish("extension.register", json.dumps({
        "name": "my-python-plugin",
        "version": "0.1.0",
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
    }).encode())

    # 2. Handle tool calls
    async def on_call(msg):
        req = json.loads(msg.data)
        params = req["params"]
        result = run_analysis(params["dataset"], params.get("model", "linear"))
        await msg.respond(json.dumps({"result": result}).encode())

    await nc.subscribe("tool.call.my-python-plugin.analyze", cb=on_call)

    # 3. Heartbeat
    async def heartbeat():
        while True:
            await nc.publish("extension.health.my-python-plugin",
                           b'{"status":"ok"}')
            await asyncio.sleep(10)

    asyncio.create_task(heartbeat())
    await asyncio.Event().wait()  # block forever

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
    json.Unmarshal(msg.Data, &m)

    r.mu.Lock()
    defer r.mu.Unlock()

    existing, isReload := r.plugins[m.Name]
    if isReload {
        // Diff tools, remove stale, add new
        r.diffAndSwapTools(existing.Manifest, m)
    } else {
        // First registration — add all tools
        r.addTools(m)
    }

    plugin := &Plugin{
        Name:         m.Name,
        Version:      m.Version,
        Description:  m.Description,
        Manifest:     m,
        Status:       "active",
        RegisteredAt: time.Now(),
    }
    r.plugins[m.Name] = plugin
    r.saveToDB(plugin)

    // Ack
    ack, _ := json.Marshal(map[string]any{
        "status":           "ok",
        "tools_registered": len(m.Tools),
        "reloaded":         isReload,
    })
    r.nc.Publish("extension.registered."+m.Name, ack)
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
        // Extract plugin name from subject: extension.health.<name>
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
            if time.Since(p.LastHeartbeat) > 15*time.Second {
                p.HealthFailures++
                if p.HealthFailures >= 3 {
                    p.Status = "crashed"
                    r.removeTools(p.Manifest)
                    r.saveToDB(p)
                    log.Warn().Str("plugin", name).Msg("plugin crashed — tools removed")
                }
            }
        }
        r.mu.Unlock()
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

```
                          ToolService.CallTool()
                                  │
                    ┌─────────────┼──────────────┐
                    │             │              │
               JS tools     OpenAPI proxy   Plugin proxy
            (Goja sandbox)  (HTTP to app)   (NATS req/reply)
                    │             │              │
                Built-in apps  External APIs  Separate processes
```

Plugin tools are registered in ToolService with a `plugin.*` namespace prefix. The AI, commands, workflows, and MCP all see them as regular tools. No special handling needed.

| System | How it sees plugin tools |
|---|---|
| MCP | Listed alongside app tools, namespaced `plugin.<name>.<tool>` |
| CLI (`nube tools`) | Shows up in the tool list |
| Workflows | Can be used as a stage tool: `tool: plugin.weather-plugin.get_forecast` |
| Command bus | `run tool/plugin.weather-plugin.get_forecast --city Sydney` |
| Flutter app | Appears in tool browser |

---

## Comparison: apps vs plugins

| | Apps | Plugins |
|---|---|---|
| Runtime | In-process (JS sandbox / OpenAPI proxy) | Separate process (any language) |
| Lifecycle | Managed by bizzy (load from disk) | Self-managed (you start/stop) |
| Communication | Direct function call / HTTP | NATS request/reply |
| Hot-reload | `POST /admin/reload-apps` | Re-publish manifest to NATS |
| Event access | Bus publish only (one-way) | Full NATS subscribe (bidirectional) |
| Use case | Lightweight tools, prompts, workflows | Heavy workloads, custom runtimes, ML, hardware |
| Install | App store, per-user | System-wide, admin-managed |
| Isolation | Goja sandbox (CPU/memory limits) | OS process isolation |

---

## Security considerations

- **Localhost only** — NATS listens on `127.0.0.1`, not `0.0.0.0`. Plugins must run on the same machine (or use SSH tunnels / overlay networks for remote).
- **No auth initially** — localhost trust model. When needed, NATS supports token auth, TLS client certs, and NKey authentication. Add it without changing the plugin protocol.
- **Tool namespacing** — plugins can only register tools under `plugin.<their-name>.*`. They cannot shadow built-in app tools.
- **Resource limits** — NATS `MaxAckPending` prevents a misbehaving plugin from flooding the bus. Tool call timeouts prevent hung plugins from blocking the AI.
- **No filesystem access** — plugins don't get access to bizzy's data directory. They only interact through NATS messages.

---

## Future

- **Plugin marketplace** — plugins packaged as Docker images or binaries, installable from a registry
- **Remote plugins** — NATS leaf nodes or gateway for plugins running on other machines
- **Plugin permissions** — restrict which bus topics a plugin can subscribe to or publish on
- **Plugin config** — plugins declare settings in their manifest, configurable via REST API, delivered at registration time
- **Streaming tool results** — for long-running tools, plugin publishes progress on `tool.progress.<plugin>.<tool>` before sending the final reply
