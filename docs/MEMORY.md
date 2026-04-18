# Memory System

Persistent AI context that carries across conversations. The AI starts every session already knowing what it learned before — site details, user preferences, past discoveries — without re-asking.

---

## How it works

Memory is plain markdown text that gets prepended to every AI prompt before it reaches the provider. Two scopes:

| Scope | Who sees it | Who writes it | File |
|---|---|---|---|
| **Server** | All users, every conversation | Admin only | `data/memory/server.md` |
| **User** | That user only | The user themselves | `data/memory/users/<user-id>.md` |

When the AI receives a prompt, it actually sees:

```
[Server Memory]
This is a NubeIO deployment at Sydney Office. The BMS runs on 192.168.1.100.
We use metric units. Temperatures in Celsius.

[User Memory]
I prefer detailed technical responses, not summaries.
My team manages floors 5-8.

<the user's actual prompt starts here>
```

The user doesn't see this prefix — it's injected server-side before the prompt hits the AI provider.

---

## Storage

```
data/
  memory/
    server.md                    # server-wide memory
    users/
      usr-abc123.md              # user memory for usr-abc123
      usr-def456.md              # user memory for usr-def456
```

Each file is plain markdown. No special format, no schema. The `memory.Store` handles thread-safe reads/writes with atomic file operations (write to `.tmp`, then rename).

**Implementation:** [pkg/memory/store.go](../pkg/memory/store.go)

### Size budget

Memory consumes context window on every conversation. Keep it short.

| Scope | Recommended max |
|---|---|
| Server memory | ~2000 words |
| User memory | ~1000 words |
| **Total** | **~3000 words (~4000 tokens)** |

---

## Where memory is injected

Memory is prepended to the prompt in all four AI entry points. The injection is server-side — clients send a bare prompt, the server adds memory before passing it to the runner.

| Entry point | File | How |
|---|---|---|
| WebSocket `GET /api/agents/run` | [agents_ws.go](../pkg/api/agents_ws.go) | `a.Memory.BuildPromptPrefix(user.ID)` prepended to `req.Prompt` |
| REST `POST /api/agents/run/sync` | [agents_rest.go](../pkg/api/agents_rest.go) | Same |
| Async jobs `POST /api/agents/jobs` | [agents_jobs.go](../pkg/api/agents_jobs.go) | Same |
| Workflows `POST /api/workflows/run` | [workflows.go](../pkg/api/workflows.go) | Same (injected into prompt stages) |
| CLI `nube ask --direct` | N/A | **No memory** — bypasses server |

### BuildPromptPrefix

The `BuildPromptPrefix(userID)` method on `memory.Store` combines both scopes:

```go
func (s *Store) BuildPromptPrefix(userID string) string {
    server := s.GetServer()
    user := s.GetUser(userID)

    if server == "" && user == "" {
        return ""
    }

    var b strings.Builder
    if server != "" {
        b.WriteString("[Server Memory]\n")
        b.WriteString(strings.TrimSpace(server))
        b.WriteString("\n\n")
    }
    if user != "" {
        b.WriteString("[User Memory]\n")
        b.WriteString(strings.TrimSpace(user))
        b.WriteString("\n\n")
    }
    return b.String()
}
```

Returns empty string if no memory exists (zero overhead when unused).

---

## API

All endpoints require authentication. Server memory endpoints require admin role.

### Server memory (admin only)

**Read:**

```
GET /api/memory/server

→ 200 { "content": "We use metric units. Temperatures in Celsius." }
→ 200 { "content": "" }    ← no memory set yet
```

**Replace:**

```
PUT /api/memory/server
{ "content": "This is a NubeIO deployment at Sydney Office.\nWe use metric units." }

→ 200 { "content": "..." }
```

**Implementation:** [pkg/api/memory.go](../pkg/api/memory.go)

### User memory

**Read:**

```
GET /api/memory/me

→ 200 { "content": "I prefer detailed responses." }
```

**Replace:**

```
PUT /api/memory/me
{ "content": "I prefer detailed technical responses.\nMy team manages floors 5-8." }

→ 200 { "content": "..." }
```

**Append:**

```
POST /api/memory/me
{ "content": "I'm working on a dashboard for energy monitoring." }

→ 200 { "content": "...full memory including new line..." }
```

### Route registration

From [router.go](../pkg/api/router.go):

```go
// Memory API.
authed.GET("/api/memory/me", a.getMyMemory)
authed.PUT("/api/memory/me", a.setMyMemory)
authed.POST("/api/memory/me", a.appendMyMemory)
admin.GET("/api/memory/server", a.getServerMemory)
admin.PUT("/api/memory/server", a.setServerMemory)
```

---

## CLI

```bash
nube memory                           # show both server + user memory
nube memory server                    # show server memory (admin)
nube memory server set "new content"  # replace server memory (admin)
nube memory me                        # show my memory
nube memory me set "new content"      # replace my memory
nube memory me add "I prefer Celsius" # append to my memory
```

**Implementation:** [pkg/cli/cmd_memory.go](../pkg/cli/cmd_memory.go)

### Examples

```bash
# Admin sets up server context
nube memory server set "This is a NubeIO deployment at Sydney Office.
The BMS runs on 192.168.1.100. We use metric units.
The Rubix runtime has 847 nodes across 11 floors."

# User sets their preferences
nube memory me set "I prefer detailed technical responses, not summaries.
My team manages floors 5-8."

# User adds a note
nube memory me add "Last issue: BACnet gateway on floor 3 was offline"

# Review everything
nube memory
# === Server Memory ===
# This is a NubeIO deployment at Sydney Office.
# The BMS runs on 192.168.1.100. We use metric units.
# The Rubix runtime has 847 nodes across 11 floors.
#
# === My Memory ===
# I prefer detailed technical responses, not summaries.
# My team manages floors 5-8.
# Last issue: BACnet gateway on floor 3 was offline
```

All commands support `-o json` for machine-readable output.

---

## Server initialisation

The memory store is created in [cmd/nube-server/main.go](../cmd/nube-server/main.go):

```go
memStore := memory.NewStore(dataDir)
```

This creates `data/memory/` and `data/memory/users/` directories if they don't exist, then passes the store to the API struct:

```go
a := &api.API{
    // ...
    Memory: memStore,
    // ...
}
```

---

## What to put in memory

### Server memory (admin-managed)

Things all users should know. Set once, update occasionally.

- Site identity: "This is the Sydney Office deployment"
- Infrastructure: "BMS on 192.168.1.100, Rubix runtime on port 1660"
- Conventions: "We use metric units, temperatures in Celsius"
- Scale context: "847 nodes across 11 floors"
- Rules: "When creating nodes, always add site/building/floor tags"

### User memory (self-managed)

Things specific to one user.

- Preferences: "I prefer detailed technical responses"
- Scope: "My team manages floors 5-8"
- Context: "I'm working on a dashboard for energy monitoring"
- Past issues: "Last time I asked about offline devices, the issue was the BACnet gateway on floor 3"

### What NOT to put in memory

- Conversation history — that's what sessions are for
- Data that changes every minute — memory is for stable context, not live metrics
- Long documents — memory consumes context budget on every conversation

---

## Memory management

Memory grows over time. Two approaches to keep it clean:

### Manual curation

Review and edit directly:

```bash
nube memory me                              # review
nube memory me set "cleaned up content"     # replace with curated version
```

Or edit the markdown files in `data/memory/` with any text editor.

### AI-assisted consolidation

Ask the AI to help:

```bash
nube ask "Review my memory and consolidate it. Remove outdated entries,
merge duplicates, and summarize. Show me the proposed changes before saving."
```

The AI reads the memory (it's in its prompt), proposes a cleaned-up version, and can write it back via the memory API if it has the `memory_save` tool (future — see Phase 2 below).

---

## File map

| File | Purpose |
|---|---|
| [pkg/memory/store.go](../pkg/memory/store.go) | Core store — read/write/append, atomic file ops, thread-safe |
| [pkg/api/memory.go](../pkg/api/memory.go) | REST handlers — GET/PUT/POST for server and user memory |
| [pkg/api/router.go](../pkg/api/router.go) | Route registration (5 routes under `/api/memory/`) |
| [pkg/cli/cmd_memory.go](../pkg/cli/cmd_memory.go) | CLI commands — `nube memory [server|me] [set|add]` |
| [cmd/nube-server/main.go](../cmd/nube-server/main.go) | Initialises `memory.NewStore(dataDir)` and wires into API |
| `data/memory/server.md` | Server-wide memory file (created on first write) |
| `data/memory/users/<id>.md` | Per-user memory files (created on first write) |

---

## Future phases

### Phase 2: AI memory tool

Register `memory_save` as an MCP tool so the AI can write to memory during conversations:

```
User: "Remember that the BACnet gateway on floor 3 has issues"
AI: calls memory_save → appends to user memory
AI: "Got it, I'll remember that for next time."
```

- Scope validation: users write to their own memory, admins to server
- Confirmation prompt before saving (configurable)

### Phase 3: Memory management

- Size limit warnings when memory exceeds budget
- Automatic summarization when over budget (ask AI to consolidate)
- Memory versioning / audit log of changes
