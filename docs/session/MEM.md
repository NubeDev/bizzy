# Memory System

Goal: give the AI persistent knowledge across conversations — things it should always know, things it learned, and things the user told it to remember. Two scopes: **server-wide** (shared by all users) and **per-user** (private to each user).

---

## Why memory matters

Without memory, every `nube ask` starts from zero. The AI re-discovers the same context every time:

- "What devices do I have?" → calls `rubix.query_nodes` every session
- "My site is called Sydney Office" → user has to repeat this
- "We use metric units" → forgotten next conversation
- "The BMS is on 192.168.1.100" → re-asked every time

Memory fixes this. The AI starts each conversation already knowing what it learned before.

---

## Two scopes

### Server memory (shared)

Things all users should know. Set by admins. Injected into every AI conversation regardless of who's asking.

**Examples:**
- "This is a NubeIO deployment at Sydney Office. The BMS runs on 192.168.1.100."
- "We use metric units. Temperatures in Celsius."
- "The Rubix runtime has 847 nodes across 11 floors."
- "When creating nodes, always add site/building/floor tags."

**Storage:** `data/memory/server.md` — a single markdown file. Simple, editable, version-controllable.

**Who writes it:**
- Admin via API: `PUT /api/memory/server`
- Admin via CLI: `nube memory server set "We use metric units"`
- AI itself: when the AI learns something that applies globally, it can write to server memory (with user confirmation)

### User memory (private)

Things specific to one user. Each user has their own memory that only they see.

**Examples:**
- "I prefer detailed technical responses, not summaries."
- "My team manages floors 5-8."
- "Last time I asked about offline devices, the issue was the BACnet gateway on floor 3."
- "I'm working on a dashboard for energy monitoring."

**Storage:** `data/memory/users/<user-id>.md` — one markdown file per user.

**Who writes it:**
- User via API: `PUT /api/memory/me`
- User via CLI: `nube memory set "I prefer technical responses"`
- AI itself: when it learns something user-specific (with confirmation)
- "Remember that..." → AI writes to user memory

---

## Data model

```
data/
  memory/
    server.md                    # server-wide memory (admin-managed)
    users/
      usr-abc123.md              # user memory for usr-abc123
      usr-def456.md              # user memory for usr-def456
```

Each file is plain markdown. No special format — just text that gets prepended to the system prompt.

### Why markdown, not structured data?

- AI reads and writes it naturally
- Humans can edit it in any text editor
- No schema to maintain
- Works with any provider (it's just text in the prompt)
- Easy to version control (git diff shows exactly what changed)

### Size limits

Memory gets prepended to the system prompt, so it consumes context window. Practical limits:

| Scope | Recommended max | Why |
|---|---|---|
| Server memory | ~2000 words | Injected into every conversation for every user |
| User memory | ~1000 words | Injected into every conversation for that user |

Total memory budget: ~3000 words (~4000 tokens). This leaves the vast majority of the context window for the actual conversation + tool results.

If memory grows too large, the AI should be able to summarize/consolidate it (see "Memory management" below).

---

## How memory is injected

When a conversation starts, memory is prepended to the system prompt:

```
[Server Memory]
This is a NubeIO deployment at Sydney Office. The BMS runs on 192.168.1.100.
We use metric units. Temperatures in Celsius.
The Rubix runtime has 847 nodes across 11 floors.

[User Memory]
I prefer detailed technical responses, not summaries.
My team manages floors 5-8.
I'm working on a dashboard for energy monitoring.

[System Prompt]
You are a helpful AI assistant with access to the user's installed apps,
tools, and prompts via MCP. Help them accomplish tasks using the available tools.

[Conversation begins here]
```

### Injection points

| Entry point | How memory is injected |
|---|---|
| WS `/api/agents/run` | Server prepends memory to prompt before passing to runner |
| REST `/api/agents/run/sync` | Same |
| Jobs `/api/agents/jobs` | Same |
| CLI `nube ask` | Server-side (CLI sends prompt to server, server adds memory) |
| CLI `nube ask --direct` | No memory (bypasses server). Could read local memory file in future. |
| Claude via MCP | Claude has its own memory. Server memory exposed as an MCP resource (future). |

The key design: memory injection happens **server-side**, not client-side. The client sends a bare prompt; the server prepends memory before passing to the runner. This means memory works with all providers and all clients.

---

## API

### Server memory (admin)

```
GET  /api/memory/server          → {"content": "...markdown..."}
PUT  /api/memory/server          ← {"content": "...markdown..."}
```

### User memory

```
GET  /api/memory/me              → {"content": "...markdown..."}
PUT  /api/memory/me              ← {"content": "...markdown..."}
```

### AI-initiated memory writes

When the AI wants to remember something, it calls a memory tool:

```
Tool: memory_save
Params: {
  "scope": "user",           // or "server" (requires admin)
  "content": "User prefers Celsius for all temperature readings."
}
```

This tool is registered as an MCP tool (like any other app tool). The AI can call it during a conversation. For server-scope writes, the tool checks that the user is an admin.

### CLI

```bash
nube memory                           # show both server + user memory
nube memory server                    # show server memory
nube memory server set "new content"  # replace server memory (admin)
nube memory me                        # show my memory
nube memory me set "new content"      # replace my memory
nube memory me add "I prefer Celsius" # append to my memory
```

---

## Memory management

### Problem: memory grows forever

Every "remember this" adds a line. After months, memory becomes bloated with outdated or contradictory entries. Two strategies:

### 1. Manual curation

User or admin reads and edits the file directly:

```bash
nube memory me                    # review
nube memory me set "cleaned up content"   # replace
```

Or edit `data/memory/users/usr-xxx.md` in a text editor.

### 2. AI-assisted consolidation

Periodically, the AI can review and consolidate memory:

```bash
nube ask "Review my memory and consolidate it. Remove outdated entries, \
merge duplicates, and summarize. Show me the proposed changes before saving."
```

The AI reads the memory file, proposes a cleaned-up version, and (with user confirmation) writes it back. This is just a prompt pattern — no special infrastructure needed.

### 3. Automatic summarization (future)

If memory exceeds the size limit, automatically ask the AI to summarize before the next conversation:

```
Memory is 3500 words (over 2000 limit).
→ Call LLM: "Summarize this memory to under 1500 words, preserving the most important facts."
→ Replace memory with summary.
→ Log what was removed for audit.
```

This is the picoclaw approach (they trigger at 75% of context budget). Defer to later — manual curation is sufficient for now.

---

## Interaction with session resume

Memory and sessions are complementary:

- **Sessions** = conversation history (what was said). Ephemeral. Used for multi-turn within a conversation.
- **Memory** = persistent knowledge (what the AI should always know). Durable. Carried across conversations.

When resuming a session (`nube ask --session ses-xxx`):
- Memory is still prepended (it may have changed since the last message)
- Session history is loaded (Claude does this natively via `--resume`)
- The AI sees both: what it always knows (memory) + what happened in this conversation (session)

---

## Implementation plan

### Phase 1: Basic memory (files + API)

- `data/memory/server.md` and `data/memory/users/<id>.md`
- `GET/PUT /api/memory/server` (admin) and `GET/PUT /api/memory/me` (user)
- Memory prepended to prompt in WS/REST/job handlers
- `nube memory` CLI commands
- No AI-initiated writes yet — humans only

### Phase 2: AI memory tool

- Register `memory_save` as an MCP tool
- AI can call it during conversations ("remember that...")
- Scope validation (user can write to own memory, admin to server)
- Confirmation prompt before saving (configurable)

### Phase 3: Memory management

- Size limit warnings
- AI-assisted consolidation prompt
- Automatic summarization when over budget

---

## What NOT to do

- **Don't use a vector database** — memory is small (< 4K tokens). Full-text injection is fine. Vector search adds complexity for no benefit at this scale.
- **Don't store memory per-session** — that's what sessions are for. Memory is cross-session by design.
- **Don't make memory structured/typed** — free-form markdown is more flexible and the AI handles it naturally. Structured memory (key-value, categories) adds schema maintenance for no UX gain.
- **Don't auto-save without confirmation** — the AI should propose what to remember and the user confirms. Silent auto-save creates trust issues.
- **Don't inject memory into `--direct` mode** — direct mode bypasses the server, so there's no server-side memory to inject. This is a known limitation, not a bug.

---

## Comparison with picoclaw

| Aspect | picoclaw | bizzy (this design) |
|---|---|---|
| Storage format | JSONL (append-only, per-session) | Markdown files (per-scope) |
| Scopes | per-agent + per-channel + per-user | server-wide + per-user |
| Session history | JSONL messages + metadata | `sessions.json` + Claude native resume |
| Long-term memory | `MEMORY.md` + daily notes per agent | `server.md` + `users/<id>.md` |
| Context compression | Automatic summarization at 75% budget | Manual first, auto-summarize later |
| Injection | Static cache + dynamic context per-request | Prepend to system prompt |
| Concurrency | Sharded locking (64 mutexes) | jsondb mutex (single process) |
| Multi-tenant | Session key hashing (agent:channel:user) | User ID isolation + admin server scope |

### Key differences from picoclaw

1. **Simpler scoping** — picoclaw has agent:channel:user:dimensions. We have two scopes: server and user. Enough for a single-server deployment.
2. **No JSONL message store** — picoclaw stores full message history in JSONL for replay. We rely on Claude's native `--resume` for now, and will add message history in Phase 3 (multi-provider).
3. **No automatic compression** — picoclaw triggers summarization at 75% context budget. We start with manual curation and add auto-summarize later.
4. **Markdown, not structured** — picoclaw uses JSON messages. We use plain markdown because it's human-editable and AI-readable.

### What we take from picoclaw

- The idea of **dual-tier memory** (conversation history vs long-term knowledge)
- **Server-side injection** (memory is added to prompts on the server, not the client)
- **Size budgets** (memory shouldn't consume the whole context window)
- The principle that **memory is append-mostly** with periodic consolidation
