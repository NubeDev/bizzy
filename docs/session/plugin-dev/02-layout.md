# 02 — Directory Layout

## Plugin source

```
plugins/
  github-plugin/
    go.mod          # module: github.com/NubeDev/bizzy-github-plugin
    main.go         # entry point: connect, register, serve, heartbeat
    manifest.go     # plugin manifest — tools, prompts, parameters
    tools.go        # GitHub API client + tool implementations
    dispatch.go     # routes tool.call.github.* NATS messages to tool fns
```

Each file has one job:
- `manifest.go` — data only, no logic
- `tools.go` — GitHub API only, no NATS
- `dispatch.go` — NATS only, no GitHub API
- `main.go` — wires everything together, lifecycle only

---

## Consumer apps

```
data/apps/
  git-review/
    app.yaml              # preamble: "use plugin.github.* to review PRs"
    prompts/
      review-pr.md        # structured review prompt template

  git-standup/
    app.yaml              # preamble: standup format instructions

  git-release/
    app.yaml              # preamble: release notes format instructions
    prompts/
      changelog.md        # changelog template
```

No `tools/` directories — these apps have zero JS. All capability comes from the plugin.

---

## go.mod

```
module github.com/NubeDev/bizzy-github-plugin

go 1.23

require (
    github.com/NubeDev/bizzy v0.0.0
    github.com/nats-io/nats.go v1.51.0
)

replace github.com/NubeDev/bizzy => ../../
```

The plugin only imports `pkg/plugin` for shared types (`ToolCallRequest`, `ToolCallResponse`, `Manifest`, subject constants). No dependency on the HTTP server or database.
