# 03 — The Manifest

`plugins/github-plugin/manifest.go` — data only, no logic.

The manifest is what bizzy reads when the plugin registers. It declares every tool and prompt. Bizzy validates it and rejects registration if anything is missing or malformed.

---

## Structure

```go
var pluginManifest = plugin.Manifest{
    Name:        "github",           // → tools namespaced as plugin.github.*
    Version:     "1.0.0",
    Description: "GitHub API tools — PRs, commits, diffs, issues, comments",
    Services:    []plugin.ServiceType{plugin.ServiceTools, plugin.ServicePrompts},
    Preamble:    "Use plugin.github.* tools to interact with GitHub repositories. Always pass owner and repo as separate parameters.",
    Tools:       githubTools,
    Prompts:     githubPrompts,
}
```

---

## Tools

Each tool needs: `Name`, `Description`, and a JSON Schema `Parameters` block.

```go
var githubTools = []plugin.ToolSpec{
    {
        Name:        "list_prs",
        Description: "List pull requests for a repo. Filter by state (open/closed/all), author, or label.",
        Parameters: map[string]any{
            "type": "object",
            "properties": map[string]any{
                "owner":  map[string]any{"type": "string", "description": "GitHub org or username"},
                "repo":   map[string]any{"type": "string", "description": "Repository name"},
                "state":  map[string]any{"type": "string", "description": "open | closed | all", "default": "open"},
                "author": map[string]any{"type": "string", "description": "Filter by PR author (optional)"},
            },
            "required": []string{"owner", "repo"},
        },
    },
    {
        Name:        "get_pr",
        Description: "Get full PR details: description, diff, review comments, CI checks.",
        Parameters: map[string]any{
            "type": "object",
            "properties": map[string]any{
                "owner":  map[string]any{"type": "string"},
                "repo":   map[string]any{"type": "string"},
                "number": map[string]any{"type": "integer", "description": "PR number"},
            },
            "required": []string{"owner", "repo", "number"},
        },
    },
    {
        Name:        "get_diff",
        Description: "Get the raw unified diff for a PR or a specific commit.",
        Parameters: map[string]any{
            "type": "object",
            "properties": map[string]any{
                "owner":  map[string]any{"type": "string"},
                "repo":   map[string]any{"type": "string"},
                "number": map[string]any{"type": "integer", "description": "PR number"},
                "sha":    map[string]any{"type": "string", "description": "Commit SHA (alternative to number)"},
            },
            "required": []string{"owner", "repo"},
        },
    },
    {
        Name:        "post_comment",
        Description: "Post a review comment on a PR.",
        Parameters: map[string]any{
            "type": "object",
            "properties": map[string]any{
                "owner":  map[string]any{"type": "string"},
                "repo":   map[string]any{"type": "string"},
                "number": map[string]any{"type": "integer"},
                "body":   map[string]any{"type": "string", "description": "Comment body (markdown)"},
                "path":   map[string]any{"type": "string", "description": "File path for inline comment (optional)"},
                "line":   map[string]any{"type": "integer", "description": "Line number for inline comment (optional)"},
            },
            "required": []string{"owner", "repo", "number", "body"},
        },
    },
    {
        Name:        "list_commits",
        Description: "List commits on a branch. Returns message, author, date, sha.",
        Parameters: map[string]any{
            "type": "object",
            "properties": map[string]any{
                "owner":  map[string]any{"type": "string"},
                "repo":   map[string]any{"type": "string"},
                "branch": map[string]any{"type": "string", "description": "Branch name (optional)"},
                "since":  map[string]any{"type": "string", "description": "ISO8601 date — only commits after this"},
                "limit":  map[string]any{"type": "integer", "description": "Max commits to return (default 20)"},
            },
            "required": []string{"owner", "repo"},
        },
    },
    {
        Name:        "list_issues",
        Description: "List issues for a repo. Filter by state, label, or assignee.",
        Parameters: map[string]any{
            "type": "object",
            "properties": map[string]any{
                "owner":    map[string]any{"type": "string"},
                "repo":     map[string]any{"type": "string"},
                "state":    map[string]any{"type": "string", "default": "open"},
                "label":    map[string]any{"type": "string", "description": "Filter by label (optional)"},
                "assignee": map[string]any{"type": "string", "description": "Filter by assignee (optional)"},
            },
            "required": []string{"owner", "repo"},
        },
    },
}
```

---

## Prompts

```go
var githubPrompts = []plugin.PromptSpec{
    {
        Name:        "summarise_changes",
        Description: "Summarise recent commits for a standup or release note",
        Template:    "Summarise the recent changes in {{owner}}/{{repo}} since {{since}} in a clear standup format: what was done, what is in progress, any notable issues.",
        Arguments: []plugin.PromptArg{
            {Name: "owner", Description: "GitHub org or username", Required: true},
            {Name: "repo",  Description: "Repository name", Required: true},
            {Name: "since", Description: "Date or relative time, e.g. 'yesterday', '2026-04-10'", Required: true},
        },
    },
}
```

---

## Validation rules (enforced by bizzy)

- `name` must match `^[a-z][a-z0-9_-]{0,62}$`
- `services` must be non-empty and only contain known service types
- If `services` includes `"tools"`, at least one tool must be defined
- If `services` includes `"prompts"`, at least one prompt must be defined
- Every tool must have a `name` matching the same pattern
