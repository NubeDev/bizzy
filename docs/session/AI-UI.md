# AI-Driven Visual App Builder

Replace the text-only app creation wizard with a visual, AI-driven flow where users see live UI previews of their tools before any code is written.

---

## Problem

The current Create flow is blind:

```
Describe → Plan (text list) → Generate code → Done
```

The user never sees what their app looks like until after it's built. The "plan" is a bullet list of tool names. There's no visual feedback, no way to say "no, make the output a card not a table" before code is generated. The Workshop we built lets you test tools, but only after the JS is written — too late for design changes.

---

## Solution

The AI generates **live UI previews** alongside the tool definitions. The user sees rendered forms and sample output for every tool, iterates on the design via chat, then approves before code generation.

```
Current:   Describe → Text Plan → Generate → Done
                      (abstract)   (blind)

Proposed:  Describe → Visual Plan → Workshop (test) → Done
                      (you see it)  (you prove it)
```

---

## How it works

### Step 1: Describe (unchanged)

User types: "I want a currency converter that gets live exchange rates"

### Step 2: Visual Plan (new)

The AI generates three things per tool:

1. **Tool schema** — params with types, same as today
2. **Input form spec** — a json-render spec that renders the tool's input form
3. **Sample output spec** — a json-render spec showing what the result will look like with mock data

Both specs render live on screen using `@json-render/react` + `@json-render/shadcn`:

```
┌─ exchange_rate ──────────────────────────────────────────┐
│                                                          │
│  INPUT                                                   │
│  ┌────────────────────────────────────────────────────┐  │
│  │  Amount         [ 100              ]               │  │
│  │  From Currency  [▾ USD             ]               │  │
│  │  To Currency    [▾ VND             ]               │  │
│  └────────────────────────────────────────────────────┘  │
│                                                          │
│  EXPECTED OUTPUT (sample data)                           │
│  ┌────────────────────────────────────────────────────┐  │
│  │  ┌──────────┬──────────┬──────────────┐            │  │
│  │  │ Currency │ Rate     │ Converted    │            │  │
│  │  ├──────────┼──────────┼──────────────┤            │  │
│  │  │ VND      │ 25,831   │ 2,583,100    │            │  │
│  │  │ AUD      │ 1.58     │ 158.00       │            │  │
│  │  └──────────┴──────────┴──────────────┘            │  │
│  └────────────────────────────────────────────────────┘  │
│                                                          │
│  ┌─────────────────┐  ┌───────────────────┐              │
│  │  ✓ Looks good   │  │  ✎ Change this    │              │
│  └─────────────────┘  └───────────────────┘              │
│                                                          │
└──────────────────────────────────────────────────────────┘
```

The user can:
- **Approve** — tool design is locked, move to next tool
- **Edit via chat** — "make it a card layout instead of a table", "add a date picker for the date param", "show the rate as a big number not a table row"
- The AI regenerates the json-render spec, preview updates live

### Step 3: Workshop (test with real data)

Once all tools are visually approved, the AI generates the JS implementations. The user tests each tool with real inputs:

- Fill in the form (same form they just approved)
- Hit Run Test
- See real output rendered with the approved json-render spec
- See HTTP trace (catches bad URLs, redirects, auth failures)
- Iterate if the real output doesn't match expectations

### Step 4: Done

App is assembled with tested tools + approved UI specs.

---

## What the AI generates

For each tool, the AI prompt asks for a json-render spec alongside the tool definition. Example AI output:

```json
{
  "name": "exchange_rate",
  "description": "Get live exchange rates",
  "params": {
    "amount": { "type": "number", "required": true, "description": "Amount to convert" },
    "from": { "type": "string", "required": true, "description": "Source currency", "options": ["USD", "EUR", "GBP"] },
    "to": { "type": "string", "required": true, "description": "Target currency", "options": ["VND", "AUD", "THB"] }
  },
  "inputSpec": {
    "root": "form",
    "elements": {
      "form": { "type": "Card", "props": { "title": "Exchange Rate" }, "children": ["fields", "submit"] },
      "fields": { "type": "Stack", "props": { "direction": "vertical", "gap": "md" }, "children": ["amount", "from", "to"] },
      "amount": { "type": "Input", "props": { "label": "Amount", "type": "number", "placeholder": "100" } },
      "from": { "type": "Select", "props": { "label": "From", "options": [{"label":"USD","value":"USD"},{"label":"EUR","value":"EUR"}] } },
      "to": { "type": "Select", "props": { "label": "To", "options": [{"label":"VND","value":"VND"},{"label":"AUD","value":"AUD"}] } },
      "submit": { "type": "Button", "props": { "label": "Convert" } }
    }
  },
  "sampleOutput": {
    "amount": 100,
    "from": "USD",
    "rates": { "VND": 2583100, "AUD": 158.00 }
  },
  "outputSpec": {
    "root": "result",
    "elements": {
      "result": { "type": "Card", "props": { "title": "Conversion Result" }, "children": ["table"] },
      "table": { "type": "Table", "props": {
        "columns": [
          { "key": "currency", "header": "Currency" },
          { "key": "rate", "header": "Rate" },
          { "key": "converted", "header": "Converted" }
        ],
        "rows": [
          { "currency": "VND", "rate": "25,831", "converted": "2,583,100 VND" },
          { "currency": "AUD", "rate": "1.58", "converted": "A$158.00" }
        ]
      }}
    }
  }
}
```

The `inputSpec` and `outputSpec` are standard json-render specs. The `<Renderer>` component renders them directly — no custom code per tool.

---

## Architecture

```
┌─ Frontend ──────────────────────────────────────────┐
│                                                      │
│  AI Chat (existing useAgentChat)                     │
│    ↓ generates json-render specs                     │
│  <Renderer spec={inputSpec} registry={shadcn} />     │
│  <Renderer spec={outputSpec} registry={shadcn} />    │
│    ↓ user approves                                   │
│  AI generates JS script                              │
│    ↓ user hits Run Test                              │
│  POST /api/apps/test-tool  ←── already built         │
│    ↓ real output                                     │
│  <Renderer spec={outputSpec} registry={shadcn} />    │
│                                                      │
└──────────────────────────────┬───────────────────────┘
                               │
┌─ Go Backend (done) ──────────┴───────────────────────┐
│  test_tool.go → jsruntime.go → Goja VM              │
│  LoggingRoundTripper captures HTTP trace             │
│  Returns {output, error, duration_ms, http_log}      │
└──────────────────────────────────────────────────────┘
```

No new backend code needed. The Go test-tool endpoint is already built and handles execution + tracing. This is purely a frontend change to the wizard flow.

---

## Implementation

### What to build

| File | Change |
|---|---|
| `components/store/app-wizard.tsx` | Rewrite Plan step to render json-render specs instead of text list. Add approve/edit flow per tool. |
| `lib/wizard-prompts.ts` | New — AI prompt templates that ask for json-render specs + sample output alongside tool definitions |
| `components/workshop/visual-plan.tsx` | New — renders input + output previews for a single tool using `<Renderer>` |
| `components/workshop/tool-workbench.tsx` | Update — form is the hero (full width), script collapsed at bottom, output via json-render |
| `pages/workshop.tsx` | Update — single column layout, no sidebar columns |

### What already exists (no changes)

| File | Purpose |
|---|---|
| `lib/json-render-registry.ts` | shadcn component registry |
| `lib/output-to-spec.ts` | Auto-mapper for when AI doesn't provide a custom spec |
| `hooks/use-test-tool.ts` | React Query mutation for test-tool endpoint |
| `components/workshop/schema-form.tsx` | Fallback form when no json-render inputSpec exists |
| `components/workshop/http-trace.tsx` | HTTP log table |
| `pkg/api/test_tool.go` | Go handler |
| `pkg/apps/jsruntime.go` | Goja VM + LoggingRoundTripper |

### AI prompt strategy

The Plan step prompt needs to ask the AI for json-render specs. The key addition to the existing `PLAN_PROMPT`:

```
For each tool, also generate:
- "inputSpec": a json-render spec using shadcn components (Card, Stack, Input, Select, Button, etc.) that renders the tool's input form
- "outputSpec": a json-render spec that renders the expected output shape with realistic sample data
- "sampleOutput": example JSON output with realistic values

Available json-render component types: Card, Stack, Grid, Table, Heading, Text, Badge, Alert, Input, Select, Checkbox, Switch, Slider, Button, Tabs, Accordion, Separator, Image, Link, Progress
```

The AI already knows json-render (it's a public package with docs). Providing the component list is enough for it to generate valid specs.

---

## What this does NOT cover

- **Drag-and-drop** — the AI is the builder, not a visual canvas. Users edit by talking ("make it a card"), not by dragging.
- **Custom component authoring** — uses the standard shadcn catalog only.
- **Prompt preview** — prompts are markdown templates, no visual spec needed. Unchanged from current flow.
- **Runtime spec storage** — for now, specs are generated at plan time and used in the wizard. Persisting specs per tool for the installed app UI is a Phase 4 concern.
