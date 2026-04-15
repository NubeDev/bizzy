# AI Documentation Cleanup Guide

Use this prompt after a task to clean up temporary documentation the AI created during the work.

## Goal

Remove disposable notes while preserving docs the user is likely to want.

## Step 1: Find Docs Created During This Work

Check new markdown files in the current branch or working tree.

Examples:

```bash
git diff --name-status <base-branch>...HEAD --diff-filter=A | grep '\.md$'
```

or:

```bash
git status --short | grep '^\?\?.*\.md$'
```

Use the repo's actual base branch instead of assuming a fixed name.

## Step 2: Categorize

### Safe To Auto-Delete

- temporary plans
- progress notes
- scratch files
- phase or rollout notes
- task-local checklists
- one-off investigation docs that are clearly disposable

### Ask Before Deleting

- user-facing docs
- architecture docs
- readmes
- decision records
- onboarding docs
- any doc that looks reusable or intentional

## Step 3: Ask For Approval Briefly

Use a short format like:

```text
I created these docs during this task:

AUTO-DELETE:
- path/to/temp-plan.md
- path/to/scratch-notes.md

KEEP OR DELETE?
- docs/architecture/feature-x.md - feature overview
- docs/api/new-endpoint.md - endpoint docs

Proceed with cleanup?
```

## Rules

- Only clean up files created during the current task or branch work
- Do not delete existing maintained docs unless the user approves
- Keep approval requests short
- Default temporary docs to deletion
