# Git Pull Request Prompt

Use this prompt to generate a pull request description for the current branch.

## Goal

Describe only the work introduced by the current branch compared with the chosen base branch.

## Branch Scope Rule

Only include commits and changes that are unique to the current branch.

Do not include:

- commits already in the base branch
- merged history from unrelated branches
- unrelated work from other features

## Required Workflow

1. Identify the current branch
2. Identify the correct base branch for the repo
3. Compare the current branch to that base branch
4. Review both commit history and code diff
5. Summarize only the net-new work

Example commands:

```bash
git branch --show-current
git log <base-branch>...HEAD --oneline
git diff <base-branch>...HEAD --stat
git status --short
```

## Default Output

By default, produce a concise summary:

```markdown
## Summary
<1-3 sentence overview>

## Changes
- key change
- key change
```

## Detailed Mode

If the user asks for full detail, include commit hashes and more granular grouping.

## Output Rules

- Return the result inside a fenced `markdown` code block
- Group changes logically
- Focus on what changed and why it matters
- Keep the tone clear and practical
- Do not include AI attribution lines unless the user asks

## Verification Checks

Before finalizing:

- Does the commit count make sense for one branch?
- Do the changes match the branch purpose?
- Are you sure merged history was not included?
