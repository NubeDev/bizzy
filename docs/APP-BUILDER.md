# App Builder — Architecture & Code Guide

> **Design Principle:** The App Builder is a **generic, framework-level tool**. It must support any use case — weather dashboards, GitHub integrations, Gmail clients, IoT controllers, task managers, CRUD apps, and anything else a user can describe. No hardcoded app-specific logic. Every feature must work for all apps.

---

## Overview

The App Builder is a bolt.new-style IDE for building NubeIO apps. It has three panels:

```
┌─ AI Chat ──────────┬─ File Tree + Editor ─────────┬─ Preview ──────────┐
│                     │                               │                    │
│  User describes     │  app.yaml                    │  [Live React UI]   │
│  what to build      │  tools/                      │     or              │
│                     │    _helpers.js                │  [Tool Test Runner]│
│  AI generates all   │    get_weather.json           │                    │
│  files at once      │    get_weather.js             │  [Data Inspector]  │
│                     │  prompts/                     │                    │
│  User iterates      │    travel_advisory.md         │  [Fix with AI]     │
│  via chat           │  ui/                          │                    │
│                     │    dashboard.tsx               │                    │
│                     │                               │                    │
└─────────────────────┴───────────────────────────────┴────────────────────┘
```

**Routes:**
- `/my-apps/create` — new app (empty project)
- `/my-apps/create/:id` — load existing app by name or ID into builder
- `/my-apps/:id/builder` — edit existing app in builder (same component)
- `/my-apps/:id/edit` — traditional editor (tabs: AI Workshop, Tools, Prompts, etc.)

---

## Frontend Architecture

### File Structure

```
frontend/src/
├── components/
│   ├── app-builder/              # Three-panel builder
│   │   ├── builder-layout.tsx    # Main layout, save/load, state orchestration, chat history loading
│   │   ├── chat-panel.tsx        # AI chat, file extraction, auto-fix messaging, session persistence
│   │   ├── file-tree.tsx         # Directory-grouped virtual file tree
│   │   ├── file-editor.tsx       # Code editor with line numbers, preview toggles
│   │   ├── preview-panel.tsx     # Live UI preview + tool test + data inspector
│   │   ├── types.ts              # AppProject, AppFile types
│   │   └── prompts.ts            # AI system prompt + file block parser
│   │
│   ├── live-preview/             # Live React component renderer
│   │   ├── renderer.tsx          # Sucrase transpiler, SCOPE injection, error boundary, multi-file compilation
│   │   └── hooks.tsx             # useToolRunner, usePromptRunner, ToolRunProvider, PromptSessionContext
│   │
│   ├── shared/                   # Reusable components
│   │   ├── tool-tester.tsx       # Params form + run + output (used everywhere)
│   │   └── tool-list.tsx         # Tool CRUD list with AI edit, test, history
│   │
│   └── store/                    # App editor components
│       ├── ai-workshop.tsx       # AI chat for existing apps
│       ├── ai-tool-editor.tsx    # Inline AI editor for a single tool
│       ├── tool-history.tsx      # Revision history UI
│       └── streaming-feedback.tsx # Streaming text display
│
├── hooks/
│   └── use-agent-chat.ts         # WebSocket chat hook with session resume + initial message support
│
├── lib/
│   └── tool-naming.ts            # TOOL_NAMING_RULES constant (shared across prompts)
│
└── pages/
    ├── create-app.tsx            # Routes to AppBuilder
    └── app-editor.tsx            # Traditional editor with tabs
```

### Key Data Types

```typescript
// Virtual file in the builder
interface AppFile {
  path: string      // "tools/get_weather.js", "ui/dashboard.tsx"
  content: string   // file content
  type: "yaml" | "js" | "json" | "md" | "tsx"
  dirty?: boolean   // has unsaved changes
}

// The full virtual project
interface AppProject {
  name: string
  displayName: string
  description: string
  category: string
  files: AppFile[]
}
```

---

## Session Memory & Chat History

### Overview

The builder supports persistent AI sessions across page reloads. Two separate systems:

1. **Builder chat** (left panel) — the AI Architect conversation that generates code
2. **Preview prompts** (right panel) — `usePromptRunner()` calls inside AI-generated components

Both persist sessions so the AI remembers previous context.

### Builder Chat Persistence

**How it works:**

1. `useAgentChat({ appName, initialMessages, initialClaudeSessionId })` sends `agent: appName` in every WebSocket request
2. The backend tags each `Session` record with `Agent = appName`
3. On page load, `builder-layout.tsx` fetches `GET /api/my/apps/:id/chat` which reconstructs the conversation from session records
4. Previous messages are passed as `initialMessages` to `ChatPanel` → `useAgentChat`
5. `useAgentChat` uses `useEffect` to update messages when `initialMessages` arrives async (since `useState` only uses initial values on mount)
6. The `claude_session_id` is also restored so subsequent messages use `--resume` for context

**Clear button:** Calls `DELETE /api/my/apps/:id/chat` which deletes all session records for the app, giving the user a fresh start.

**Timing consideration:** The chat history fetch is async. `ChatPanel` renders immediately with empty messages, then updates when the fetch completes. The `useEffect` in `useAgentChat` handles this late arrival pattern.

### Preview Prompt Session Persistence

**How it works:**

1. `ToolRunProvider` accepts `appName` prop and creates a `PromptSessionContext`
2. On mount, fetches `GET /api/agents/sessions/app/:name` to get the latest resume session ID
3. Also reads from `localStorage` as an instant fallback
4. `usePromptRunner()` reads the session ID from context and sends `session_id` + `app` with each request
5. After each response, saves the new session ID to both the context ref and `localStorage`

**Provider-agnostic design:**
- **Claude:** Uses `claude_session_id` for `--resume` (Claude CLI manages state)
- **Ollama/others:** Uses `session_id` to load server-side `ChatHistory` (conversation replayed from DB)

**Stale closure handling:** `sessionCtxRef` (a ref to the context) ensures the memoized `run()` callback always reads the latest `storageKey` and `appName`, even though `useCallback` has `[]` deps.

### Backend Models

```go
// ChatHistory — stores conversation for stateless providers (Ollama, OpenAI, etc.)
type ChatHistory struct {
    SessionID string        `gorm:"primaryKey"`
    AppName   string        `gorm:"index"`
    Messages  []ChatMessage `gorm:"serializer:json"`
    Provider  string
    UserID    string        `gorm:"index"`
    UpdatedAt time.Time
}

// Session — audit record for every AI run (all providers)
type Session struct {
    ID              string
    Agent           string   // app name — used for chat history lookup
    ClaudeSessionID string   // for --resume
    Prompt          string   // user message
    Result          string   // AI response
    // ... provider, model, cost, tokens, etc.
}
```

### Session API Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/api/my/apps/:id/chat` | Load builder chat history (reconstructed from sessions) |
| `DELETE` | `/api/my/apps/:id/chat` | Delete all sessions for app (fresh start) |
| `GET` | `/api/agents/sessions/app/:name` | Get latest resume session ID for an app |
| `POST` | `/api/agents/run/sync` | Run prompt (accepts `session_id` + `app` for resume/tagging) |

---

## AI Code Generation

### How the AI Generates Files

The AI outputs tagged code blocks. The tag tells the system which file to create/update:

    ```yaml:app.yaml
    name: my-app
    ...
    ```

    ```json:tools/get_weather.json
    { "name": "get_weather", "params": {...} }
    ```

    ```js:tools/get_weather.js
    function handle(params) { ... }
    ```

    ```md:prompts/advisory.md
    ---
    name: travel_advisory
    ---
    Analyze the weather: {{data}}
    ```

    ```tsx:ui/dashboard.tsx
    function Dashboard() {
      const tool = useToolRunner()
      return <Card>...</Card>
    }
    ```

### File Block Extraction

`extractFileBlocks(content)` in `prompts.ts` uses regex to parse these blocks:

```typescript
const regex = /```(\w+):([^\n]+)\n([\s\S]*?)```/g
// Captures: type ("js"), path ("tools/get_weather.js"), content
```

The `ChatPanel` watches for new AI messages, extracts file blocks, and calls `onFilesGenerated(files)` to update the project state.

### System Prompt — Generic Pattern-Based Architecture

`buildArchitectPrompt(project)` in `prompts.ts` generates a comprehensive system prompt. It is **pattern-based, not app-specific** — it teaches composable patterns the AI combines for any use case.

The prompt includes:

1. **Output format** — examples of every file type with correct markers
2. **Tool naming rules** — from `TOOL_NAMING_RULES` (must match `pkg/toolname` validation)
3. **Full JS runtime API** — http.get/post/put/patch/delete with headers, secrets, config, log
4. **App config patterns** — allowedHosts, settings (including type: "secret" for API tokens)
5. **Tool patterns** — authenticated helpers, read/search/list, write/create/update/delete, multi-step chaining, pagination
6. **UI scope** — all available React hooks, shadcn components, icons, safety helpers
7. **Data access rules** — how to safely access tool data (see Safety Helpers section)
8. **UI patterns** — search→detail, CRUD with forms, auto-refresh/polling, tabs with multiple data sources, AI analysis
9. **Multi-file UI** — how to split complex UIs into composable components
10. **Current project state** — list of existing files so the AI knows what exists

**Supported use cases (non-exhaustive):**
- Weather dashboards, IoT monitoring, building automation
- GitHub/GitLab integrations (repo search, issue tracking, CI/CD)
- Email clients (Gmail, Outlook via API)
- Project/task management (CRUD boards, kanban)
- REST API testers and explorers
- Content review tools with guided Q&A
- Data analysis with AI-powered insights
- Any app that calls HTTP APIs and displays results

---

## Live Preview (v0-style)

### How It Works

1. AI generates a `tsx:ui` block — a React function component
2. `compileComponent(code, extraScope?)` in `renderer.tsx`:
   - Strips import/export statements (everything is in SCOPE)
   - Transpiles JSX to JS using **sucrase** (lightweight, ~15KB)
   - Creates a `new Function(...)` with all SCOPE variables as parameters
   - Invokes the factory to get the component function
   - Optional `extraScope` allows injecting sibling components (for multi-file)
3. Component renders inside a `PreviewErrorBoundary`
4. If it crashes, shows compile error or render error with **Fix with AI** button

### Memoization & Stability

The preview uses content-based memoization to prevent unnecessary recompilation:

- `PreviewPanel` memoizes `uiFiles` and `toolPairs` with `useMemo([project.files])` so tool result state changes don't cause re-renders
- `MultiFilePreview` uses a content-based `filesKey` (file paths + content joined) as the `useMemo` dependency instead of array reference equality
- This prevents the compiled component from being unmounted/remounted when parent state changes (e.g., Data Inspector updates), which would destroy all hook state inside the component

### SCOPE — What AI-Generated Components Can Use

All of these are available without imports:

| Category | Available |
|----------|-----------|
| React | `useState`, `useEffect`, `useMemo`, `useCallback`, `useRef` |
| Backend | `useToolRunner()`, `usePromptRunner()` |
| Safety | `str(value)`, `get(obj, "path", fallback)` |
| shadcn | `Button`, `Input`, `Card`, `Select`, `Tabs`, `Badge`, `Label`, `Textarea`, `Separator`, `Skeleton` |
| Icons | `Sun`, `Cloud`, `Thermometer`, `Wind`, `Loader2`, `Search`, `Check`, `AlertTriangle`, ~25 more |
| Built-ins | `JSON`, `String`, `Math`, `Date`, `Array`, `Object`, `console`, etc. |
| Styling | All Tailwind CSS utility classes |

### Safety Helpers

#### `str(value)` — Safe String Coercion
Converts any value to a string safe for React rendering. Prevents "Objects are not valid as React child" errors.

```tsx
{str(data.wind)}              // safe — converts object/array/null to string
{str(get(data, "name", "?"))} // safe — wraps get() for display
```

#### `get(obj, path, fallback)` — Safe Nested Property Access
Navigates a dot-separated path and returns the **raw value**. Works with all types — strings, numbers, arrays, objects.

```tsx
get(data, "name", "?")         // → "Sydney" (string)
get(data, "cities", [])        // → [{name: "Sydney"}, ...] (real array — can .map())
get(data, "meta.count", 0)     // → 42 (number)
get(data, "missing.field", []) // → [] (fallback)
```

**Important:** `get()` returns raw values. Arrays stay arrays, objects stay objects. For rendering in JSX, wrap with `str()`. For iterating, use directly (with `Array.isArray()` guard before `.map()`).

**Data access pattern:**
```tsx
var tool = useToolRunner()
var d = tool.data || {}                          // extract plain JSON
var items = get(d, "results", [])                // raw array
var safe = Array.isArray(items) ? items : []     // guard
safe.map(function(item) { return <p>{str(get(item, "name", "?"))}</p> })
```

### useToolRunner — Connect UI to Backend Tools

```tsx
function Dashboard() {
  const weather = useToolRunner()

  return (
    <>
      <Button onClick={() => weather.run("myapp.get_weather", { city: "London" })}>
        {weather.loading ? <Loader2 className="animate-spin" /> : "Check"}
      </Button>
      {weather.error && <div className="text-red-500">{weather.error}</div>}
      {weather.data && <div>{str(get(weather.data, "temperature", "?"))}°C</div>}
    </>
  )
}
```

**In the builder** (unsaved app): `useToolRunner` uses `ToolRunProvider` to call `POST /api/apps/test-tool` with the script directly — no saved app needed.

**In the AI Workshop** (saved app): calls `POST /api/agents/tools/:name` via the MCP agent endpoint.

### usePromptRunner — Connect UI to AI

```tsx
const ai = usePromptRunner()
ai.run("What should I wear?", { conditions: JSON.stringify(weather.data) })
// ai.text = "Bring a jacket, it's 13°C with wind..."
```

Calls `POST /api/agents/run/sync`. Has a 60-second timeout. Supports multi-turn session resume — sends `session_id` and `app` automatically when inside a `ToolRunProvider` with `appName`.

**Provider support:**
- **Claude:** `session_id` maps to `--resume` for native multi-turn
- **Ollama/others:** `session_id` maps to server-side `ChatHistory` (full message replay)

### ToolRunProvider — Context-Based Tool Execution & Session Management

`ToolRunProvider` wraps the AI component and provides two contexts:

1. **ToolRunContext** — injects a custom tool runner function so `useToolRunner()` works in the builder
2. **PromptSessionContext** — manages session persistence for `usePromptRunner()` across component remounts and page reloads

```tsx
<ToolRunProvider runFn={builderToolRunFn} onResult={handleToolResult} appName="weather-monitor">
  <Component />
</ToolRunProvider>
```

Props:
- `runFn` — the function to call when `tool.run()` is invoked
- `onResult` — optional callback fired (as a microtask) after each tool run completes, used to populate the Data Inspector
- `appName` — tags sessions and controls localStorage key for prompt session persistence

**Session loading on mount:**
1. Reads from `localStorage` (instant, synchronous)
2. Fetches from `GET /api/agents/sessions/app/:name` (authoritative, async)
3. Whichever arrives, sets `promptSessionRef.current` for `usePromptRunner` to read

---

## Multi-File UI Components

For complex apps, the UI can be split into multiple `ui/*.tsx` files. Each file exports one component. Components from sibling files are **automatically available in scope** using PascalCase of the filename.

| File | Available as |
|------|-------------|
| `ui/search-bar.tsx` | `SearchBar` |
| `ui/weather-card.tsx` | `WeatherCard` |
| `ui/data-table.tsx` | `DataTable` |
| `ui/dashboard.tsx` | Rendered as **main** entry point |

### How Multi-File Compilation Works

`compileMultiFile(files)` in `renderer.tsx`:

1. **Sorts files** — sub-components first (alphabetical), main file last
2. **Determines main** — files named `dashboard`, `app`, `page`, `main`, or `index` are treated as the entry point. If none match, the last file alphabetically is main.
3. **Compiles in order** — each compiled component is added to `extraScope` for subsequent files
4. **Returns** — the compiled main component + any sub-component errors

```typescript
const result = compileMultiFile([
  { name: "search-bar", code: "function SearchBar({onSelect}) { ... }" },
  { name: "weather-card", code: "function WeatherCard({city}) { ... }" },
  { name: "dashboard", code: "function Dashboard() { return <div><SearchBar .../><WeatherCard .../></div> }" },
])
// result.main.Component = Dashboard (with SearchBar and WeatherCard in scope)
```

### Preview Panel Rendering

`PreviewPanel` handles single and multi-file apps:

- **1 TSX file** — renders via `LivePreview` directly
- **2+ TSX files** — renders via `MultiFilePreview` which uses `compileMultiFile` and `MultiFileRenderer`

Multi-file preview shows badges for each file and per-file error indicators. Sub-component compile errors get their own **Fix with AI** button targeting that specific file.

---

## Debugging Features

### Fix with AI

When the preview shows a compile error or render error, a **Fix with AI** button appears. Clicking it:

1. Collects the error message + failing source code
2. Appends tool data shapes from the Data Inspector (if available) via `enrichFixMessage()`
3. Auto-sends it to the AI Chat panel as a `pendingMessage`
4. The AI responds with a corrected file block that auto-applies

**Data flow:**
```
Error in Preview
    │ onRequestFix(message)
    ▼
PreviewPanel
    │ enrichFixMessage(message, toolResults)
    ▼
AppBuilder state (pendingMessage)
    │
    ▼
ChatPanel → auto-sends → AI responds → files extracted → preview updates
```

The fix message includes:
- The error text
- The current source code (truncated to 2000 chars)
- Actual tool data shapes from the Data Inspector (so the AI knows real field names and types)
- Reminder about data access rules (`get()`, `str()`, `Array.isArray()`)

### Data Inspector

After any tool runs inside the live preview, a **Data Inspector** panel appears below the preview. It shows:

- **Tool name** and result type (`array[5]`, `object`, etc.)
- **Expandable JSON view** of the raw response data
- **Copy** button to copy the JSON
- **Update UI** button — sends the data shape to the AI, asking it to fix the component to match the actual data structure

Tool results are captured via `ToolRunProvider`'s `onResult` callback, which fires as a microtask after each tool run completes. Results are stored in a `Map<string, unknown>` in `PreviewPanel` state.

`describeShape(data)` generates a concise type description for the AI:
```
{
  cities: Array<{
    name: string (e.g. "Sydney"),
    country: string (e.g. "Australia"),
    latitude: number (e.g. -33.86785),
    population: number (e.g. 5231147)
  }> (5 items)
}
```

### PreviewErrorBoundary

Class component that catches render-time React errors. Shows:
- Error message
- **Fix with AI** button (when `onRequestFix` is provided)
- Expandable stack trace

Exported from `renderer.tsx` so both `LivePreview` and `MultiFileRenderer` can use it.

---

## Tool Naming Convention

Enforced by `pkg/toolname` on the backend (hard rejection at API) and `lib/tool-naming.ts` on the frontend (AI prompt instructions).

| Pattern | Name Example | Mode | Files |
|---------|-------------|------|-------|
| Regular tool | `check_weather` | `""` | `check_weather.js` + `.json` |
| QA tool | `travel_quiz_qa` | `"qa"` | `travel_quiz_qa.js` + `.json` |
| Helper | `_helpers` | n/a | `_helpers.js` (shared code, not a tool) |

**Rules:**
- Lowercase letters, digits, underscores only. 2-60 chars. Must start with a letter.
- `_qa` suffix required when `mode: "qa"`, and vice versa.
- Names starting with `_` are reserved for helpers.

---

## Saving & Loading

### Save Flow (Builder → Database)

When the user clicks "Save App":

1. **Parse `app.yaml`** — extract name, description, category
2. **Create app** via `POST /api/my/apps` (if new)
3. **Extract tools** — match `.json` + `.js` pairs from `tools/`:
   - Parse `.json` for schema (name, params, toolClass, mode)
   - Use `.js` content as script
   - Prepend `_helpers.js` if it exists
   - Call `POST /api/my/apps/:id/tools` for each
4. **Extract prompts** — parse `.md` YAML frontmatter:
   - Extract name, description, arguments from frontmatter
   - Body is everything after the `---` markers
   - Call `POST /api/my/apps/:id/prompts` for each
5. **Extract UI components** — collect `ui/*.tsx` files:
   - Name from filename (`dashboard.tsx` → `dashboard`)
   - Code is raw TSX
   - Saved via `PUT /api/my/apps/:id` with `uiComponents` field
6. **Update app metadata** via `PUT /api/my/apps/:id`

### Load Flow (Database → Builder)

`appToProject(app: StoreApp)` converts a saved app back to builder format:

- `app.yaml` — reconstructed from app metadata + permissions
- `tools/<name>.json` — from `StoreTool.params` (one per tool)
- `tools/<name>.js` — from `StoreTool.script` (one per tool)
- `prompts/<name>.md` — from `StorePrompt` with YAML frontmatter
- `ui/<name>.tsx` — from `UIComponent.code`

### App Selector Navigation

When selecting an app from the top bar dropdown, the builder navigates to `/my-apps/create/:name`. This:
- Sets `existingAppId` from the route param (available immediately, no async wait)
- Triggers `useMyApp(existingAppId)` to fetch the app data
- Loads chat history via `GET /api/my/apps/:id/chat`
- The `appName` prop flows through to `ToolRunProvider` for session persistence

The backend `getMyStoreApp` handler accepts both UUIDs and app names (`id = ? OR name = ?`).

---

## Revision History

Generic undo/revert system for any entity type.

### Model (`pkg/models/revision.go`)

```go
type Revision struct {
    ID            string          // UUID
    EntityType    string          // "tool", "prompt"
    EntityID      string          // composite key "appId:toolName"
    Revision      int             // auto-incremented per entity
    Data          json.RawMessage // full JSON snapshot before change
    ChangeSummary string          // "AI edit", "manual edit", "deleted"
    AuthorID      string
    CreatedAt     time.Time
}
```

### Store API (`pkg/revision/revision.go`)

```go
Save(entityType, entityID, authorID, summary, data)  // snapshot before update
List(entityType, entityID) → []Revision               // newest first
Get(entityType, entityID, revisionNum) → *Revision
GetData(entityType, entityID, revisionNum, dest)       // fetch + unmarshal
EntityKey("appId", "toolName") → "appId:toolName"     // composite key builder
```

Auto-prunes to last 10 revisions per entity (`MaxRevisions = 10`).

### API Endpoints (`pkg/api/revision_handler.go`)

- `GET /api/my/apps/:id/revisions/:type/:entityName` — list revisions
- `POST /api/my/apps/:id/revisions/:type/:entityName/revert/:rev` — revert to version

Revert flow: get old data → snapshot current state → restore old entity → save → reload.

### Where Revisions Are Created

In `store.go`, before every tool/prompt update or delete:
- `updateStoreTool` — snapshots old tool, reads `X-Change-Summary` header
- `deleteStoreTool` — snapshots with summary "deleted"
- `updateStorePrompt` — same pattern
- `deleteStorePrompt` — same pattern

---

## QA Tools (Interactive Question/Answer)

QA tools implement a dual-mode pattern for guided flows:

```javascript
function handle(params) {
  if (params._answers !== undefined) return chatMode(params._answers);
  if (!params._submit) return formDefinition();
  return formSubmit(params);
}
```

**Chat mode** — returns one question at a time:
```javascript
function chatMode(answers) {
  if (!answers.city)
    return { type: 'question', field: 'city', label: 'City?', input: 'text' };
  if (!answers.activity)
    return { type: 'question', field: 'activity', label: 'Activity?', input: 'select',
      options: [{value: 'hiking', label: 'Hiking'}] };
  return { type: 'result', data: '...' };
}
```

**Naming:** QA tools must end with `_qa` suffix and declare `mode: "qa"`.

---

## Backend Models

### StoreApp (`pkg/models/store.go`)

```go
type StoreApp struct {
    // Identity
    ID, Name, DisplayName, Description, LongDesc string
    Version, Icon, Color, Category string
    Tags []string

    // Ownership
    AuthorID, AuthorName, WorkspaceID string
    Visibility Visibility  // private | shared | unlisted | public

    // Content (all serialized as JSON columns)
    Permissions  Permissions       // allowedHosts, defaultToolClass, secrets
    Settings     []SettingDef      // user-configurable settings
    Tools        []StoreTool       // backend JS tools
    Prompts      []StorePrompt     // prompt templates
    UIComponents []UIComponent     // client-side React components

    // Stats + timestamps
    InstallCount, ActiveInstalls int
    AvgRating float64
    CreatedAt, UpdatedAt time.Time
    PublishedAt *time.Time
}
```

### StoreTool
```go
type StoreTool struct {
    Name        string                // validated by pkg/toolname
    Description string
    ToolClass   string                // "read-only", "read-write", "destructive"
    Mode        string                // "" (regular) or "qa"
    Params      map[string]ToolParam  // name → {type, required, description, options}
    Script      string                // JS function body
}
```

### UIComponent
```go
type UIComponent struct {
    Name string  // component name (e.g., "dashboard")
    Code string  // raw TSX (no imports, uses SCOPE)
}
```

---

## JS Tool Runtime — Full API

### HTTP (global `http`)
```js
// All return: { status: number, body: string, headers: {}, json?: any }
http.get(url)
http.get(url, { headers: { "Authorization": "Bearer " + secrets.api_token } })
http.post(url, bodyObject)
http.post(url, bodyObject, { headers: { "X-Custom": "value" } })
http.put(url, bodyObject)
http.patch(url, bodyObject)
http.delete(url)
```
- Body auto-serialized to JSON, Content-Type set automatically
- Response body auto-parsed as JSON in `.json` field when possible
- All requests validated against `allowedHosts` — blocked if host not listed

### Secrets & Config (global objects)
```js
secrets.api_key      // from settings where type: "secret"
config.base_url      // from settings where type is NOT "secret"
```

### Logging (global `log`)
```js
log.info("fetched 5 items")
log.error("API returned " + res.status)
```

### Entry Point
Every tool must define: `function handle(params) { ... return result }`

---

## API Endpoints (Builder-relevant)

| Method | Path | Purpose |
|--------|------|---------|
| `POST` | `/api/my/apps` | Create new app |
| `PUT` | `/api/my/apps/:id` | Update app (metadata, permissions, uiComponents) |
| `GET` | `/api/my/apps/:id` | Load app by ID or name (includes tools, prompts, uiComponents) |
| `DELETE` | `/api/my/apps/:id` | Delete app |
| `GET` | `/api/my/apps/:id/chat` | Load builder chat history (from session records) |
| `DELETE` | `/api/my/apps/:id/chat` | Clear builder chat history (fresh start) |
| `POST` | `/api/my/apps/:id/tools` | Add tool |
| `PUT` | `/api/my/apps/:id/tools/:name` | Update tool (accepts X-Change-Summary header) |
| `DELETE` | `/api/my/apps/:id/tools/:name` | Delete tool |
| `POST` | `/api/my/apps/:id/prompts` | Add prompt |
| `PUT` | `/api/my/apps/:id/prompts/:name` | Update prompt |
| `DELETE` | `/api/my/apps/:id/prompts/:name` | Delete prompt |
| `POST` | `/api/apps/test-tool` | Run JS script in sandbox (no saved app needed) |
| `POST` | `/api/agents/tools/:name` | Call a saved tool via MCP agent |
| `POST` | `/api/agents/run/sync` | Send prompt to AI (accepts `session_id` + `app` for resume) |
| `GET` | `/api/agents/sessions/app/:name` | Get latest resume session ID for an app |
| `GET` | `/api/my/apps/:id/revisions/:type/:name` | List revision history |
| `POST` | `/api/my/apps/:id/revisions/:type/:name/revert/:rev` | Revert to revision |
