# Plugin Development — github-plugin

Building a real plugin end-to-end. This series uses `github-plugin` as the example — a Go plugin that exposes GitHub tools to the AI, consumed by thin apps like `git-review`, `git-standup`, and `git-release`.

---

## Documents

| File | What it covers |
|---|---|
| [01-concept.md](01-concept.md) | The pattern — plugin as shared service layer, why it works |
| [02-layout.md](02-layout.md) | Directory structure for the plugin and consumer apps |
| [03-manifest.md](03-manifest.md) | Writing the plugin manifest — tools, prompts, parameters |
| [04-tools.md](04-tools.md) | Tool implementations — GitHub API client |
| [05-main.md](05-main.md) | Main: NATS connect, register, serve, heartbeat |
| [06-apps.md](06-apps.md) | Consumer apps — git-review, git-standup, git-release |
| [07-running.md](07-running.md) | Running, verifying, crash recovery |

---

## Quick start

```bash
cd plugins/github-plugin
go build -o github-plugin .
GITHUB_TOKEN=ghp_yourtoken ./github-plugin
```

Then ask the AI:

```
nube ask "review PR #42 in NubeDev/bizzy"
nube ask "what did I work on today in NubeDev/bizzy?"
```
