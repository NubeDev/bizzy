# 01 — The Concept

## Plugin as shared service layer

Apps own **intent** (what the AI should do). Plugins own **capability** (how to do it).

```
github-plugin (Go, NATS)
  ├── plugin.github.list_prs
  ├── plugin.github.get_pr
  ├── plugin.github.get_diff
  ├── plugin.github.post_comment
  └── plugin.github.list_commits
          ↑
          │  all three apps share the same plugin tools
          │
  ┌───────┼───────────────────────────────────┐
  │       │                                   │
git-review app        git-standup app        git-release app
preamble:             preamble:              preamble:
"use plugin.github.*  "use plugin.github.*   "use plugin.github.*
 to review PRs"        to summarise my week"  to draft release notes"
```

The plugin is installed **once**. Apps that consume it are tiny — just an `app.yaml` with a preamble. No duplicated API code, no repeated auth setup.

---

## Why this split?

| Concern | Where it lives |
|---|---|
| GitHub API auth, rate limits, pagination | `github-plugin` |
| What the AI *does* with GitHub data | App preamble |
| User's PAT / token | Plugin env var (set once) |
| Specific workflow logic (review style, standup format) | App preamble + prompts |

---

## Tool naming

Plugin tools are namespaced automatically by bizzy:

```
plugin manifest name: "github"
tool name:            "list_prs"
                          ↓
MCP tool name:        "plugin.github.list_prs"
```

The AI sees and calls `plugin.github.list_prs` — it knows it's a plugin tool, not an app tool. Apps reference them in preambles by their full namespaced name.

---

## How bizzy wires it

```
github-plugin starts
  → nc.Request("extension.register", manifest)
    → bizzy validates manifest
    → injects plugin.github.* into ToolService + MCPFactory
    → replies: {"status":"ok","tools_registered":6}
  → plugin subscribes to tool.call.github.*
  → heartbeat ticker starts

User: "review PR #42 in NubeDev/bizzy"
  → git-review preamble injected into AI context
  → AI calls plugin.github.get_diff {owner:"NubeDev", repo:"bizzy", number:42}
    → ToolService → PluginProxy → NATS Request → github-plugin
    → github-plugin calls GitHub API, replies
  → AI reads diff, forms review comments
  → AI calls plugin.github.post_comment with inline comments
```
