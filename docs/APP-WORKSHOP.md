# App Workshop

A tool workbench built into the app creation flow. Instead of generating an entire app blind and hoping it works, users build and test each piece individually, see results rendered as real UI, and assemble only after everything passes.

---

## Problem

The current Create flow goes `Describe → Plan → Generate → Done`. There is no way to test a tool before the app is assembled. Bugs (wrong URLs, bad auth, schema mismatches) only surface after the app is installed and a user tries to run it in chat. This is how the frankfurter.app redirect bug shipped — nothing caught it before the tool was called in production.

---

## Solution

Add a **Workshop** step between Plan and Done. Each tool is built, tested, and verified individually before the app is assembled.

```
Current:   Describe → Review Plan → Generate → Done

Proposed:  Describe → Review Plan → Workshop → Assemble → Done
                                       │
                                       ├── build tool schema
                                       ├── auto-render test form from schema
                                       ├── edit JS logic
                                       ├── run tool with real inputs
                                       ├── see output as rendered UI
                                       ├── see HTTP trace (catches URL/redirect/auth bugs)
                                       ├── iterate until it works
                                       └── repeat for each tool
```

---

## Core ideas

### 1. JSON Schema drives the forms

Every tool already has a `.json` schema defining its params. Instead of building custom form UI per tool, we derive the form automatically:

```
tool_name.json                          Rendered form
┌──────────────────────────────┐        ┌────────────────────────────┐
│ "params": {                  │        │                            │
│   "month": {                 │        │  Month    [6         ]     │
│     "type": "number",        │  ───►  │  Year     [2026      ]     │
│     "required": true,        │        │  Dest     [▾ KUL     ]     │
│     "description": "..."     │        │                            │
│   },                         │        │  [▶ Run Test]              │
│   "destination": {           │        │                            │
│     "type": "string",        │        └────────────────────────────┘
│     "options": ["KUL","BKK"] │
│   }                          │
│ }                            │
└──────────────────────────────┘
```

Mapping:

| Schema | Component |
|---|---|
| `"type": "string"` | Text input |
| `"type": "string"` + `"options": [...]` | Select dropdown |
| `"type": "number"` | Number input |
| `"type": "boolean"` | Switch/toggle |
| `"required": true` | Validation, required marker |
| `"description"` | Label + placeholder |

This means every tool gets a test form for free — zero manual UI work.

### 2. json-render for output display

Tool output is JSON. Instead of showing raw JSON, we use [json-render](https://github.com/vercel-labs/json-render) (`@json-render/react` + `@json-render/shadcn`) to render results as real interactive components.

The same tool output renders differently depending on its shape:

| Output shape | Rendered as |
|---|---|
| Array of objects | Table |
| Key-value pairs | Card with labeled rows |
| Object with `error` field | Alert (destructive) |
| Nested objects | Accordion / collapsible sections |
| URLs | Clickable links |
| Numbers with labels | Stat cards / badges |

A simple mapper function inspects the JSON shape and generates a json-render spec. The AI can also generate custom render specs for richer layouts.

Example — the flight search tool output:

```
Raw JSON                                    Rendered UI
┌────────────────────────────┐              ┌────────────────────────────────┐
│ {                          │              │  Weekend Flights: June 2026    │
│   "origin": "DAD",        │              │  DAD → KUL                     │
│   "routes": [{            │              │                                │
│     "destination": "KUL", │    ───►      │  ┌────────┬─────────┬────────┐ │
│     "weekends": [{        │              │  │Weekend │ Est VND │ Book   │ │
│       "weekend": "Fri 5..",│              │  ├────────┼─────────┼────────┤ │
│       "google_flights":   │              │  │Fri 5-8 │2.3M-7.7M│ Link → │ │
│         "https://..."     │              │  │Fri 12..│2.3M-7.7M│ Link → │ │
│     }]                    │              │  └────────┴─────────┴────────┘ │
│   }]                      │              │                                │
│ }                          │              │  Airlines: AirAsia, VietJet   │
└────────────────────────────┘              └────────────────────────────────┘
```

### 3. HTTP trace catches bugs early

The backend test-tool endpoint returns not just the tool output, but a log of every HTTP call the tool made. This is displayed below the results:

```
HTTP Log
─────────────────────────────────────────────────────
GET  api.frankfurter.dev/v1/latest?from=USD  → 200  (142ms)
GET  api.frankfurter.app/latest?from=USD     → 301  → api.frankfurter.dev  (redirect!)
POST api.example.com/query                   → 403  (auth failed)
```

This immediately surfaces:
- Wrong URLs
- Redirects that fail host checks
- Auth failures
- Timeouts
- Unexpected status codes

### 4. Apps get a real UI

Once json-render is integrated, the tool schemas don't just generate test forms in the workshop — they generate **permanent UI for the installed app**. Users can run tools directly from a form interface without going through AI chat.

```
Today:  User → AI Chat → AI calls tool → raw JSON → AI formats response
Future: User → App UI (auto-generated form) → tool runs → rendered result
```

Both paths remain available. The chat path is better for complex queries. The form path is faster for routine operations.

---

## Architecture

### New backend endpoint

```
POST /api/apps/test-tool
```

Accepts a tool definition and test input, executes it in a sandboxed Goja runtime, returns the output plus HTTP trace.

**Request:**

```json
{
  "script": "function handle(params) { ... }",
  "helpers": "function apiGet(path) { ... }",
  "params": { "month": 6, "year": 2026 },
  "allowedHosts": ["api.frankfurter.dev"],
  "settings": { "api_host": "http://localhost:9000" },
  "secrets": { "api_token": "test-token" },
  "timeout": "30s"
}
```

**Response:**

```json
{
  "output": { "count": 4, "routes": [...] },
  "error": null,
  "duration_ms": 340,
  "http_log": [
    {
      "method": "GET",
      "url": "https://api.frankfurter.dev/v1/latest?from=USD&to=VND,AUD",
      "status": 200,
      "duration_ms": 142,
      "redirected_from": null
    }
  ]
}
```

This endpoint:
- Uses the same Goja runtime as production (identical sandbox)
- Enforces `allowedHosts` (same security model)
- Captures all outbound HTTP calls in a trace log
- Returns structured errors (syntax error, runtime error, timeout, host blocked)
- Does **not** persist anything — pure stateless execution

### New frontend packages

| Package | Purpose |
|---|---|
| `@json-render/core` | Schema definitions, catalog, spec types |
| `@json-render/react` | React renderer |
| `@json-render/shadcn` | 36 pre-built shadcn/ui components (matches existing frontend stack) |

The frontend already uses Radix UI + Tailwind (shadcn-style), so `@json-render/shadcn` components will match the existing design system.

### Frontend components

| Component | Location | Purpose |
|---|---|---|
| `WorkshopPage` | `pages/workshop.tsx` | Full workshop view, replaces step 3 of wizard |
| `ToolWorkbench` | `components/workshop/tool-workbench.tsx` | Single tool: schema editor + JS editor + test runner + output |
| `SchemaForm` | `components/workshop/schema-form.tsx` | Auto-generates form inputs from tool JSON schema |
| `ToolOutput` | `components/workshop/tool-output.tsx` | Renders tool output via json-render |
| `HttpTrace` | `components/workshop/http-trace.tsx` | HTTP call log table |
| `PromptPreview` | `components/workshop/prompt-preview.tsx` | Renders prompt markdown with sample args |

---

## Workshop UI

### Layout

```
┌─────────────────────────────────────────────────────────────┐
│  Workshop: flight-search                                    │
│                                                             │
│  Tools                          Prompts                     │
│  ┌─────────────────────┐        ┌────────────────────────┐  │
│  │ ● exchange_rate     │        │ ○ weekend_flights      │  │
│  │ ○ search_flights    │        └────────────────────────┘  │
│  └─────────────────────┘                                    │
│  ● = tested & passing   ○ = untested                        │
│                                                             │
│  ┌─ exchange_rate ──────────────────────────────────────┐   │
│  │                                                      │   │
│  │  ┌─ Schema ──────────┐  ┌─ Script ────────────────┐  │   │
│  │  │ {                 │  │ function handle(params) { │  │   │
│  │  │  "name": "...",   │  │   var rates = ...        │  │   │
│  │  │  "params": {...}  │  │   return {...}           │  │   │
│  │  │ }                 │  │ }                        │  │   │
│  │  └───────────────────┘  └──────────────────────────┘  │   │
│  │                                                      │   │
│  │  ┌─ Test (auto-generated from schema) ────────────┐  │   │
│  │  │                                                │  │   │
│  │  │  Amount (USD)  [ 100           ]               │  │   │
│  │  │                                                │  │   │
│  │  │                          [ ▶ Run Test ]        │  │   │
│  │  └────────────────────────────────────────────────┘  │   │
│  │                                                      │   │
│  │  ┌─ Result ───────────────────────────────────────┐  │   │
│  │  │                                                │  │   │
│  │  │  ┌──────────┬───────────┬───────────────┐      │  │   │
│  │  │  │ Currency │ Rate      │ Converted     │      │  │   │
│  │  │  ├──────────┼───────────┼───────────────┤      │  │   │
│  │  │  │ VND      │ 25,831    │ 2,583,100 VND │      │  │   │
│  │  │  │ AUD      │ 1.58      │ A$158.00      │      │  │   │
│  │  │  └──────────┴───────────┴───────────────┘      │  │   │
│  │  │                                                │  │   │
│  │  │  [Raw JSON ▾]                                  │  │   │
│  │  └────────────────────────────────────────────────┘  │   │
│  │                                                      │   │
│  │  ┌─ HTTP Log ─────────────────────────────────────┐  │   │
│  │  │ GET api.frankfurter.dev/v1/latest  → 200 142ms │  │   │
│  │  └────────────────────────────────────────────────┘  │   │
│  └──────────────────────────────────────────────────────┘   │
│                                                             │
│  [← Back to Plan]                     [Assemble App →]      │
│                                  (all tools must pass)      │
└─────────────────────────────────────────────────────────────┘
```

### Interactions

| Action | What happens |
|---|---|
| Click tool tab | Switches to that tool's workbench |
| Edit schema JSON | Form below auto-updates (live) |
| Edit JS script | Nothing until Run — no auto-execute |
| Click **Run Test** | `POST /api/apps/test-tool` with current schema + script + form values |
| Test passes | Tool gets green dot, output rendered via json-render |
| Test fails | Error shown in Alert component, HTTP log highlights the failed request |
| Click **Raw JSON** | Toggle between rendered output and raw JSON |
| Click **Assemble App** | Only enabled when all tools have at least one passing test |
| Click tool status dot | Quick view: last test result (pass/fail, timestamp) |

### Prompt preview

Prompts don't need a test runner — they're markdown templates. The preview shows:

1. Rendered markdown with sample arguments filled in
2. List of tools the prompt references (validated against actual tool names)
3. Warning if a referenced tool doesn't exist

---

## Schema-to-form mapping (detail)

The `SchemaForm` component reads a tool's `params` object and renders a form.

### Basic types

```typescript
// Input: tool schema params
{
  "city": { "type": "string", "required": true, "description": "City name" },
  "limit": { "type": "number", "required": false, "description": "Max results" },
  "verbose": { "type": "boolean", "required": false, "description": "Include details" }
}

// Output: json-render spec (auto-generated)
{
  "root": "form",
  "elements": {
    "form": { "type": "Stack", "props": { "direction": "vertical", "gap": 4 }, "children": ["city", "limit", "verbose", "submit"] },
    "city": { "type": "Input", "props": { "label": "City name", "required": true, "placeholder": "City name", "$bindState": "/params/city" }},
    "limit": { "type": "Input", "props": { "label": "Max results", "type": "number", "$bindState": "/params/limit" }},
    "verbose": { "type": "Switch", "props": { "label": "Include details", "$bindState": "/params/verbose" }},
    "submit": { "type": "Button", "props": { "label": "Run Test", "variant": "default" }, "handlers": { "press": { "action": "run_test" }}}
  }
}
```

### Extended types

| Schema pattern | Component | Notes |
|---|---|---|
| `"type": "string"` | `Input` (text) | Default |
| `"type": "string", "options": [...]` | `Select` | Dropdown |
| `"type": "string", "multiline": true` | `Textarea` | Multi-line |
| `"type": "number"` | `Input` (number) | With step buttons |
| `"type": "number", "min"/"max"` | `Slider` | Range with bounds |
| `"type": "boolean"` | `Switch` | Toggle |
| `"required": true` | Red asterisk + validation | Blocks Run if empty |

### App-level form (installed app UI)

The same mapping is used when an installed app renders its tool forms. The user sees a form for each tool in the app's detail page, can fill it in, and run it directly — no AI chat needed.

---

## json-render integration

### Catalog definition

Define a Bizzy-specific component catalog that wraps the shadcn preset:

```typescript
import { shadcnCatalog } from "@json-render/shadcn"

const bizzyCatalog = defineCatalog(schema, {
  ...shadcnCatalog,
  components: {
    ...shadcnCatalog.components,
    // Bizzy-specific additions
    HttpTrace: {
      props: z.object({
        requests: z.array(z.object({
          method: z.string(),
          url: z.string(),
          status: z.number(),
          duration_ms: z.number(),
          redirected_from: z.string().nullable(),
        }))
      }),
      description: "HTTP request trace log from tool execution",
    },
    ToolStatus: {
      props: z.object({
        name: z.string(),
        passed: z.boolean(),
        lastRun: z.string().optional(),
      }),
      description: "Tool test status indicator",
    },
  },
})
```

### Output-to-spec mapper

A function that inspects tool output JSON and generates a json-render spec:

```typescript
function outputToSpec(output: unknown): JsonRenderSpec {
  // Array of objects → Table
  if (Array.isArray(output) && output.length > 0 && typeof output[0] === 'object') {
    return tableSpec(output)
  }

  // Object with "error" key → Alert
  if (typeof output === 'object' && output !== null && 'error' in output) {
    return alertSpec(output)
  }

  // Object with nested arrays → Tabs with tables
  if (hasNestedArrays(output)) {
    return tabbedSpec(output)
  }

  // Flat key-value → Card with rows
  return keyValueSpec(output)
}
```

For complex or domain-specific tools, the AI can generate a custom json-render spec alongside the tool code. This spec gets stored with the tool and used instead of the auto-mapper.

---

## Implementation plan

### Phase 1: Backend test-tool endpoint — DONE

- [x] `POST /api/apps/test-tool` endpoint in `pkg/api/test_tool.go`
- [x] `LoggingRoundTripper` in `pkg/apps/jsruntime.go` wraps HTTP transport, captures every outbound request (method, url, status, duration_ms, redirected_from)
- [x] `NewTestJSRuntime()` + `ExecuteScript()` for stateless execution without an App on disk
- [x] Returns `{output, error, duration_ms, http_log}`
- [x] Route on authed group (bearer token, no admin required)

### Phase 2: Workshop UI — DONE

- [x] `hooks/use-test-tool.ts` — React Query mutation for the test-tool endpoint
- [x] `components/workshop/schema-form.tsx` — auto-generates form inputs from tool param schema (string, number, boolean, select dropdown, multiline)
- [x] `components/workshop/http-trace.tsx` — HTTP log table with color-coded status + redirect indicators
- [x] `components/workshop/tool-workbench.tsx` — JS editor (collapsible helpers), schema form, run button, rendered output, HTTP trace
- [x] `pages/workshop.tsx` — standalone page with app picker sidebar → tool list → workbench
- [x] Top-level nav tab "Workshop" at `/workshop`
- [x] Also accessible per-app at `/my-apps/:id/workshop`
- [x] "Tool Workshop" tab in the app editor (`pages/app-editor.tsx`)
- [x] "Test in Workshop" button on wizard Done step

### Phase 3: json-render integration — DONE

- [x] `@json-render/core`, `@json-render/react`, `@json-render/shadcn` installed via pnpm
- [x] `lib/json-render-registry.ts` — component registry using shadcn components
- [x] `lib/output-to-spec.ts` — auto-maps tool output to json-render specs:
  - Array of objects → Table
  - Object with `error` key → Alert (destructive)
  - Nested objects with arrays → Tabs with tables
  - Flat key-value → Card with rows
  - Primitives → Text
- [x] `tool-workbench.tsx` uses `<Renderer>` from json-render for formatted output (toggle to raw JSON)

### Phase 4: App UI (installed apps) — TODO

- [ ] Add tool forms to installed app detail page (app-detail.tsx)
- [ ] Users can run tools directly from the auto-generated form (no chat required)
- [ ] Results rendered inline via json-render
- [ ] AI chat remains available for complex multi-tool queries

---

## Files created / modified

### Backend (Go)

| File | Status | Change |
|---|---|---|
| `pkg/api/router.go` | Modified | Added `POST /api/apps/test-tool` route |
| `pkg/api/test_tool.go` | New | Handler — parses request, wires logging transport, executes script, returns result |
| `pkg/apps/jsruntime.go` | Modified | `HTTPLogEntry`, `LoggingRoundTripper`, `NewTestJSRuntime()`, `ExecuteScript()`, transport injection |

### Frontend (React/TypeScript)

| File | Status | Change |
|---|---|---|
| `package.json` | Modified | Added `@json-render/core`, `@json-render/react`, `@json-render/shadcn` |
| `src/App.tsx` | Modified | Added `/workshop` and `/my-apps/:id/workshop` routes |
| `src/components/layout/app-shell.tsx` | Modified | Added "Workshop" nav tab |
| `src/components/store/app-wizard.tsx` | Modified | Added "Test in Workshop" button on Done step |
| `src/pages/app-editor.tsx` | Modified | Added "Tool Workshop" tab with inline workbench |
| `src/pages/workshop.tsx` | New | Standalone workshop page — app picker → tool list → workbench |
| `src/hooks/use-test-tool.ts` | New | React Query mutation + request/response types |
| `src/components/workshop/tool-workbench.tsx` | New | JS editor + schema form + run button + json-render output + HTTP trace |
| `src/components/workshop/schema-form.tsx` | New | JSON schema → form inputs (text, number, boolean, select, multiline) |
| `src/components/workshop/http-trace.tsx` | New | HTTP log table |
| `src/lib/json-render-registry.ts` | New | shadcn component registry for json-render |
| `src/lib/output-to-spec.ts` | New | Tool output → json-render display spec |

---

## What this does NOT cover

- **Visual drag-and-drop builder** — json-render is code/AI-generated, not drag-and-drop. Users describe what they want, AI generates the spec. Manual editing is via JSON/code, not a visual canvas.
- **Custom CSS / theming per app** — apps use the global Bizzy design system via shadcn components.
- **Workflow testing** — multi-step workflows (MULTI-APP-WORKFLOW.md) are a separate concern. This covers individual tool testing only.
- **Load testing / rate limit simulation** — the test endpoint runs one call at a time.
