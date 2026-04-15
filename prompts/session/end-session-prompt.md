# End of Session Handoff

Use this prompt at the end of a coding session to generate a concise handoff for the next session.

If the user explicitly says not to generate docs or handoff notes, skip this.

## Goal

Create a short, actionable handoff document that helps a fresh session continue the work quickly.

## Output Structure

```markdown
# Session Handoff

## Session Summary
- What was worked on
- What changed
- What was accomplished

## Current Status
- What's working
- What's partially done
- What's not working

## Blocking Issues
- Include only if there are actual blockers

## Next Steps
- Immediate priorities
- Suggested approach
- Files or areas that need attention

## Context for Next Session
- Important decisions
- Gotchas
- Assumptions to keep in mind
```

## Writing Rules

- Keep it concise and practical
- Focus on facts, not narrative
- Only include blockers if they are real
- When showing code, comment the why, not the what
- Do not add generic AI-style code comments

## Save Location

Use the repository's existing session-notes convention if one exists.

If no convention exists, use a simple path like:

- `docs/sessions/{topic}/{date}-{short-description}.md`

If the repo does not use a `docs/` folder, choose an equivalent handoff or notes location already used by the project.
