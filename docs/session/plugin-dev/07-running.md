# 07 — Running & Crash Recovery

## Running

```bash
# 1. Start bizzy
make start

# 2. Build and run the plugin
cd plugins/github-plugin
go build -o github-plugin .
GITHUB_TOKEN=ghp_yourtoken ./github-plugin

# Output:
# [github-plugin] connected to nats://127.0.0.1:4222
# [github-plugin] registered — 6 tools (reloaded=false)
# [github-plugin] ready
```

---

## Verify tools are registered

```bash
# List all tools — should see plugin.github.*
nube tools list | grep github

# Or check the API
curl http://localhost:8080/api/plugins
```

---

## Use it

```bash
nube ask "review PR #42 in NubeDev/bizzy"
nube ask "what did I work on this week in NubeDev/bizzy?"
nube ask "draft release notes for NubeDev/bizzy since last Monday"
```

---

## Crash recovery

If the plugin crashes, bizzy marks its tools unavailable after **3 missed heartbeats** (~15s). When the plugin restarts it re-registers and tools come back immediately — no bizzy restart needed.

```bash
# After a crash, just restart:
GITHUB_TOKEN=ghp_yourtoken ./github-plugin
# [github-plugin] registered — 6 tools (reloaded=true)   ← back immediately
```

The `reloaded=true` in the reply means bizzy replaced the existing registration rather than creating a new one.

---

## Hot reload

Re-run the plugin with an updated manifest (new tools, changed descriptions). Bizzy diffs the old and new manifests, swaps the tools atomically, and clears any `crashed` status. The AI sees the updated tools immediately.

```bash
# Update manifest.go, rebuild, restart — no bizzy restart needed
go build -o github-plugin . && GITHUB_TOKEN=ghp_yourtoken ./github-plugin
```

---

## Environment variables

| Variable | Default | Description |
|---|---|---|
| `GITHUB_TOKEN` | required | GitHub Personal Access Token |
| `NATS_URL` | `nats://127.0.0.1:4222` | NATS server address |
