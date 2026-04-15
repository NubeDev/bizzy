# New Session Checklist

**Give this to Claude at the start of each coding session**

---

## Context to Load

1. **Read**: `/home/user/code/go/nube/rubix/docs/OVERVIEW.md` (this provides project structure)
2. **Check memory**: `~/.claude/projects/-home-user-code-go-nube-rubix/memory/MEMORY.md`
3. **Review recent commits**: `git log --oneline -10` to see what was done recently

## Environment Check

```bash
# Verify you're in the right directory
pwd  # Should be: /home/user/code/go/nube/rubix

# Check current branch
git branch --show-current

# Check server status
curl http://localhost:9000/healthz 2>/dev/null || echo "Server not running"
```

## Key Paths Reference

- **Documentation**: `docs/system/v1/`
- **Frontend**: `frontend/` (see `frontend/CLAUDE.md`)
- **Backend**: `internal/`, `cmd/rubix/`
- **Plugins**: `nodes/rubix/v2/plugins_manager/`
- **API definitions**: `configs/ras/`
- **Database**: `bin/dev/data/db/rubix.db`
- **Logs**: `bin/dev/logs/rubix.log`

## Coding Guidelines

**DO:**
- Read files before editing
- Use Edit tool (not Write) for existing files
- Test compilation after changes
- Check docs before asking questions
- Write clear, concise comments (only when needed)

**DON'T:**
- No emojis unless requested
- Don't create unnecessary files
- Don't over-engineer solutions

**CODE COMMENTS - CRITICAL:**
- ❌ **NO checkmarks** (✅/❌) in code comments
- ❌ **NO "STEP-1", "STEP-2"** markers in code
- ❌ **NO debugging steps** in comments
- ❌ **NO AI-style** generic comments ("Initialize variable", "Call function")
- ✅ **ONLY write comments** when the code is genuinely complex or non-obvious
- ✅ **Comment the WHY**, not the WHAT (code shows what it does)

**Bad examples:**
```go
// ✅ Create new user - NO!
// STEP-1: Initialize the struct - NO!
// TODO: Add error handling later - NO!
user := &User{Name: name}  // Create user struct - NO!
```

**Good examples:**
```go
// Retry 3 times because the external API is flaky
for i := 0; i < 3; i++ { ... }

// Use pointer to avoid copying large struct
func Process(data *LargeData) { ... }

// No comment needed - code is self-explanatory
user := &User{Name: name}
```

## Quick Start Commands

```bash
# Build and run
make build && make dev

# Build plugin (if working on plugins)
cd nodes/rubix/v2/plugins_manager && ./build-plugin.sh plm

# Login for API testing
curl -X POST http://localhost:9000/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@rubix.io","password":"admin@rubix.io"}'
```

## What to Ask User

- What are we working on today?
- Should I check any specific documentation?
- Are there any recent issues or blockers?

---

**Now ask the user what they want to work on.**
