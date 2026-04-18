# 06 — Consumer Apps

These apps have **no JS tools** — just an `app.yaml` with a preamble that tells the AI which plugin tools to use and how.

---

## git-review

`data/apps/git-review/app.yaml`

```yaml
name: git-review
version: 1.0.0
description: AI-powered PR review assistant
author: Admin
permissions:
  allowedHosts: []
  defaultToolClass: read-only
settings: []
tags:
  - github
  - code-review
preamble: |
  You are an expert code reviewer. Use the plugin.github.* tools to assist with PR reviews.

  Workflow:
  1. Call plugin.github.list_prs to find open PRs if not given a specific one
  2. Call plugin.github.get_diff to fetch the full diff
  3. Review for: correctness, security, performance, style, missing tests
  4. Use plugin.github.post_comment to leave inline comments on specific lines
  5. Summarise your overall verdict at the end

  Be constructive and specific. Reference file paths and line numbers.
```

---

## git-standup

`data/apps/git-standup/app.yaml`

```yaml
name: git-standup
version: 1.0.0
description: Generate standup updates from GitHub activity
author: Admin
permissions:
  allowedHosts: []
  defaultToolClass: read-only
settings: []
tags:
  - github
  - productivity
preamble: |
  You help generate standup updates from GitHub activity.
  Use plugin.github.list_commits and plugin.github.list_prs to find what happened.

  Format:
  ✅ Done: (completed work)
  🔄 In progress: (open PRs, ongoing work)
  ⚠️ Blockers: (anything stuck or waiting for review)

  Keep it concise — one line per item.
```

---

## git-release

`data/apps/git-release/app.yaml`

```yaml
name: git-release
version: 1.0.0
description: Draft release notes from merged PRs
author: Admin
permissions:
  allowedHosts: []
  defaultToolClass: read-only
settings: []
tags:
  - github
  - releases
preamble: |
  You draft release notes from merged pull requests.
  Use plugin.github.list_prs (state=closed) to find recently merged PRs.

  Group changes into:
  ## ✨ New Features
  ## 🐛 Bug Fixes
  ## ⚠️ Breaking Changes
  ## 🔧 Internal / Chores

  Use the PR title and description — one line per entry.
  Format as markdown suitable for a GitHub Release or CHANGELOG.md.
```
