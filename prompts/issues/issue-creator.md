# Issue Creator

Reusable instructions for turning rough task notes into clean, actionable issues.

## When To Use

Use this when the input is messy, informal, or incomplete and needs to be turned into issues that work for both humans and AI agents.

## Input

The user may provide:

- rough notes
- bullet lists
- chat messages
- file paths
- screenshots
- references to docs or code

## Process

### Step 1: Discover Context

For each task:

1. Find the relevant source files
2. Look for existing documentation in the repo that covers the same area
3. Read enough of the source to understand the current implementation
4. Read enough of the docs to confirm relevance

### Step 2: Write Each Issue

Use this template:

```markdown
## TASK-XX: [Short title]

**Size**: Small | Medium | Large

**Problem**: [1-2 concise sentences]

**Source files**:
- [path/to/file.ext](path/to/file.ext) - what it does
- [path/to/dir/](path/to/dir/) - what it contains

**Requirements**:
- concrete, testable requirement
- another concrete requirement

**Docs**:
- [path/to/doc.md](path/to/doc.md) - update what specifically
```

### Step 3: Size It

- Small: clear change, narrow scope, usually one file or one focused fix
- Medium: several files, integration work, or moderate refactor
- Large: broad scope, new subsystem, major UX change, or work that should be split

### Step 4: Flag Doc Updates

- If the task changes documented behavior, add a `Docs` section
- If a new doc is needed, flag it clearly with a suggested path
- If docs are unaffected, omit the `Docs` section

## Rules

- Be concise
- Do not invent requirements the user did not imply
- Split unrelated problems into separate tasks
- Use repo-relative paths
- Keep requirements concrete and testable
- Continue numbering tasks from the existing file if applicable

## Output Location

If the repo already has a task-tracking document, append to it.

If not, write to a simple location such as:

- `docs/tasks.md`

or another issue-tracking file already used by the repo.
