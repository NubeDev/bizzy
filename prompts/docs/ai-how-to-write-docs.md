# AI Documentation Writing Guide

Guidelines for writing clear, useful documentation that can work in any repository.

## Before Writing

- Confirm where the docs should live
- Check whether the repo has a docs convention or manifest
- Read related docs first so style and structure stay consistent
- Do not assume the user wants public-facing docs unless they say so

## What Not To Do

- Do not write internal implementation notes as end-user docs
- Do not create docs in a protected or published area unless the user asks
- Do not link to files that do not exist
- Do not include local-only paths or temporary workspace references in docs

## Core Principles

### Be Direct

- Lead with what it is and why it matters
- Skip long introductions
- Start with the answer, then the detail

### Respect Reader Time

- Keep paragraphs short
- Use bullets when listing things
- Cover one concept per section

### Show, Don't Lecture

- Prefer concrete examples over long explanations
- Use realistic examples
- Include code only when it helps

## Recommended Structure

```markdown
# Title
Short summary.

## Quick Start

## Core Concepts

## Examples

## Reference
```

Use only the sections that make sense for the doc.

## Content Guidelines

### Include

- what it does
- why it exists
- how to use it
- common patterns
- parameters, return values, errors, and edge cases when relevant
- links to related docs

### Exclude

- unnecessary history
- obvious steps
- internal implementation detail unless the doc is explicitly for developers
- multiple equivalent solutions when one good path is enough

## Style

- Use active voice
- Use present tense
- Prefer clarity over cleverness
- Keep headings descriptive and scannable

## Quality Checklist

- Is the first sentence useful?
- Can the reader complete the task quickly?
- Are examples copy-paste ready when applicable?
- Do the headings make sense on their own?
- Can any section be cut without losing value?
