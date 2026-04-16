# Chat UI — Current State & Known Issues

## What Was Built

A reusable `<AgentChat>` component with:
- WebSocket streaming via `useAgentChat` hook
- `/` command picker showing prompts AND tools together
- Markdown rendering (react-markdown + remark-gfm + react-syntax-highlighter)
- Status indicators (connecting, thinking, tool calls, streaming)

## Architecture

```
Frontend (React)                         Backend (Go :8090)
─────────────────                        ──────────────────
<AgentChat>                              
  useAgentChat hook                      
    WebSocket → ws://host/api/agents/run  → agents_ws.go
                                           → spawns Claude CLI
                                           → Claude uses MCP tools
                                           → streams events back
                                              
  useCommandPicker hook                  
    GET /my/prompts                      → tools.go:listMyPrompts
    GET /my/tools                        → tools.go:listMyTools (now with params!)
    GET /my/prompts/:name                → tools.go:getPrompt (rendered body)
```

## Key Files

### Frontend
- `frontend/src/components/chat/agent-chat.tsx` — main reusable chat component
- `frontend/src/components/chat/command-picker.tsx` — `/` dropdown picker  
- `frontend/src/components/chat/chat-message.tsx` — message rendering with markdown
- `frontend/src/hooks/use-agent-chat.ts` — WebSocket streaming hook
- `frontend/src/hooks/use-command-picker.ts` — picker state/keyboard nav
- `frontend/src/pages/chat.tsx` — standalone /chat page
- `frontend/src/lib/api.ts` — API client (myTools, myPrompts, getPrompt)

### Backend
- `pkg/api/tools.go` — /my/tools (returns params), /my/prompts, /my/prompts/:name
- `pkg/api/agents_ws.go` — WebSocket agent handler
- `pkg/apps/mcpfactory.go` — MCP tool/prompt registration
- `pkg/apps/jsruntime.go` — JS tool execution with _helpers.js support

## Current Issue: Picker Not Working

### Symptom
User types `/` in chat input → picker should show prompts + tools → **not showing or not responding**

### What to Check

1. **Backend running?** `curl http://localhost:8090/health`
2. **APIs returning data?** 
   - `curl http://localhost:8090/my/tools` — should return tools with `params` array
   - `curl http://localhost:8090/my/prompts` — should return prompts with `arguments` array
3. **Vite proxy working?** 
   - `curl http://localhost:5173/my/tools` — same data through proxy
   - Check `frontend/vite.config.ts` has `/my` proxy
4. **Browser console errors?** Open DevTools → Console → look for React errors
5. **Stale cache?** `rm -rf frontend/node_modules/.vite && pnpm dev`

### How the Picker Works

1. User types `/` as first character → `handleInputChange` sets `isOpen: true`
2. Hook fetches from `/my/prompts` and `/my/tools` (cached via React Query)
3. Both are merged into one list, grouped by `appName`
4. `CommandPicker` component renders the dropdown above input
5. Keyboard: up/down to navigate, Enter/Tab to select, Escape to dismiss
6. On select:
   - No args → calls `handleSend('/promptName')` immediately
   - Has args → sets input to `/promptName ` with hint text below

### How Prompt Resolution Works

When user sends `/rubix.navigation`:
1. `handleSend` detects `/` prefix
2. Calls `GET /my/prompts/rubix.navigation`
3. Gets back `{ rendered: "full prompt body..." }`
4. Sends the rendered body (not "/rubix.navigation") to WebSocket
5. The AI receives the full instructions and acts on them

### How Tool Auth Works

JS tools use `_helpers.js` (`data/apps/rubix/tools/_helpers.js`):
- `login()` checks `secrets.rubix_token` first (stored JWT, no new login)
- Falls back to `POST /auth/login` only if stored token fails
- `apiGet/apiPost/etc` inject the token automatically
- No orgId/deviceId needed from the user

## Servers

- **Bizzy backend**: `NUBE_ADDR=:8090 NUBE_DATA_DIR=data bin/nube-server`
- **Rubix**: must be running on port 9000 for rubix tools to work
- **Frontend**: `cd frontend && pnpm dev` (port 5173, proxies to 8090)

## What Still Needs Work

1. **Verify picker actually renders** — may be a React rendering issue, check browser console
2. **App detail Chat tab** — embeds `<AgentChat>` with app context as systemPrompt
3. **Prompt arg UI** — when prompt has args, show proper input fields not just a text hint
4. **Tool direct execution** — for tools with no AI needed (like runtime_status), could call REST directly instead of going through Claude
