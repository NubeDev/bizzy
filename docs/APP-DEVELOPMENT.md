# App Development Guide

How to create a new Bizzy app. This guide is written so that an AI agent can build a complete app from a single description.

---

## Directory structure

Every app is a directory under `data/apps/<app-name>/` with this layout:

```
data/apps/<app-name>/
  app.yaml              # Required — metadata, permissions, settings
  prompts/              # Optional — reusable prompt templates (markdown)
    getting_started.md
    do_something.md
  tools/                # Optional — executable tools
    _helpers.js         # Optional — shared JS loaded before every tool
    my_tool.js          # Tool logic (one function per file)
    my_tool.json        # Tool schema (name, params, description)
    my_prompt_tool.json # Prompt-mode tool (AI handles execution)
```

File naming rules:
- Tool files come in pairs: `tool_name.js` + `tool_name.json` (underscores, not hyphens)
- Prompt-mode tools only need a `.json` file (no `.js`) — the AI executes them
- `_helpers.js` (prefixed with underscore) is auto-loaded before every tool in the app
- Prompt files use hyphens: `getting-started.md`

---

## app.yaml

The app manifest. Every field explained:

```yaml
name: my-app                    # Unique app ID (lowercase, hyphens)
version: 1.0.0                  # Semver
description: >                  # One-line description shown in the store
  What this app does — be specific, this is what users see
author: YourName                # Author name or org

permissions:
  allowedHosts:                 # Domains the app's JS tools can call
    - "api.example.com"
    - "*.example.com"           # Wildcard subdomains OK
    - "localhost:*"             # Any port on localhost
  defaultToolClass: read-only   # "read-only" or "read-write"
  secrets:                      # Secret keys this app needs (stored encrypted)
    - my_api_token

settings:                       # User-configurable values (shown in UI)
  - key: api_host               # Key used in JS as config.api_host
    label: API Host URL          # Human-readable label
    type: url                    # url | string | number | secret | boolean
    required: true
    default: http://localhost:8080
  - key: my_api_token
    label: API Token
    type: secret
    required: false

preamble: |                     # Injected into every AI conversation using this app
  You are a helpful assistant for My App.
  Use the my-app.* MCP tools to help the user.

tags:                           # For store search/filtering
  - iot
  - monitoring

timeout: 15s                    # Tool execution timeout (default 30s)
```

### Minimal app.yaml (no tools, prompt-only)

```yaml
name: my-templates
version: 1.0.0
description: Prompt templates for my team
author: Admin
permissions:
  allowedHosts: []
  defaultToolClass: read-only
settings: []
tags: []
```

---

## Tools

Tools are functions the AI can call. There are three types:

### 1. JS tools (API-calling tools)

A pair of files: `tool_name.json` (schema) + `tool_name.js` (logic).

**Schema — `tool_name.json`:**

```json
{
  "name": "tool_name",
  "description": "What this tool does — the AI reads this to decide when to call it",
  "toolClass": "read-only",
  "params": {
    "city": {
      "type": "string",
      "required": true,
      "description": "City name (e.g. Sydney, London)"
    },
    "limit": {
      "type": "number",
      "required": false,
      "description": "Max results (default 10)"
    },
    "include_details": {
      "type": "boolean",
      "required": false,
      "description": "Include extra detail in response"
    }
  }
}
```

Schema fields:
- `name` — tool name (must match filename without extension)
- `description` — shown to the AI; be specific about what it does and when to use it
- `toolClass` — `"read-only"` (safe, no side effects) or `"read-write"` (creates/modifies data)
- `params` — object of parameter definitions; each has `type`, `required`, `description`
- Valid param types: `"string"`, `"number"`, `"boolean"`

**Logic — `tool_name.js`:**

```js
function handle(params) {
  // Validate required params
  if (!params.city) return { error: "city is required" };

  // Make HTTP requests
  var resp = http.get("https://api.example.com/data?city=" + encodeURIComponent(params.city));

  // Check for errors
  if (resp.status !== 200) {
    return { error: "API failed (" + resp.status + "): " + resp.body };
  }

  // Return structured data (AI reads this)
  return {
    city: resp.json.name,
    value: resp.json.value,
    count: resp.json.items.length
  };
}
```

JS tool rules:
- Must export a `function handle(params)` — this is the entry point
- Return a plain object (automatically serialised to JSON)
- Return `{ error: "message" }` for errors
- No `require()`, no `import`, no `async/await` — this runs in a Goja (Go-embedded JS) runtime
- Language level is ES5 — no `let`/`const`, no arrow functions, no template literals, no destructuring

**Available globals in JS tools:**

| Global | Description |
|---|---|
| `http.get(url, opts?)` | HTTP GET, returns `{status, body, json}` |
| `http.post(url, body, opts?)` | HTTP POST |
| `http.put(url, body, opts?)` | HTTP PUT |
| `http.patch(url, body, opts?)` | HTTP PATCH |
| `http.delete(url, opts?)` | HTTP DELETE |
| `config.<key>` | App settings values (from `app.yaml` settings) |
| `secrets.<key>` | App secret values (from `app.yaml` secrets) |
| `encodeURIComponent(s)` | URL-encode a string |
| `JSON.parse(s)` / `JSON.stringify(o)` | JSON helpers |

HTTP options: `{ headers: { "Authorization": "Bearer " + token } }`

HTTP response shape: `{ status: 200, body: "raw string", json: { parsed: "object" } }`

### 2. Prompt-mode tools (AI-executed)

A tool where the AI follows a prompt instead of running JS code. Only needs a `.json` file.

```json
{
  "name": "navigation",
  "description": "View or edit the site hierarchy",
  "mode": "prompt",
  "prompt": "You are helping the user manage the hierarchy.\n\nThe user wants to: {{action}}\n\nFirst call `my-app.list_items` to see what exists.\nThen based on the action:\n- **show** — display the results\n- **create** — ask what to add, then call `my-app.create_item`\n- **delete** — ask which to remove, then call `my-app.delete_item`",
  "params": {
    "action": {
      "type": "string",
      "required": false,
      "description": "What to do: show, create, or delete",
      "options": ["show", "create", "delete"]
    }
  }
}
```

Use prompt-mode tools when the logic is better expressed as AI instructions than code — multi-step workflows, decision trees, or tools that orchestrate other tools.

### 3. QA-mode tools (interactive forms/quizzes)

A tool with multi-step user interaction. Schema has `"mode": "qa"` and the JS implements both form and chat flows.

```json
{
  "name": "my_quiz_qa",
  "description": "Interactive quiz that asks questions step by step",
  "toolClass": "read-only",
  "mode": "qa",
  "params": {
    "_submit": {
      "type": "boolean",
      "required": false,
      "description": "Set true to submit answers"
    },
    "name": {
      "type": "string",
      "required": false,
      "description": "Your name",
      "order": 1
    },
    "answer": {
      "type": "string",
      "required": false,
      "description": "Your answer",
      "order": 2
    }
  }
}
```

The JS implements three paths:

```js
function handle(params) {
  // Chat mode (conversational, one question at a time)
  if (params._answers !== undefined) return chatMode(params._answers);
  // Form mode — return field definitions
  if (!params._submit) return formDefinition();
  // Form submit — validate and return results
  return formSubmit(params);
}

function chatMode(answers) {
  if (!answers.name) {
    return {
      type: 'question', field: 'name',
      label: 'What is your name?',
      input: 'text', required: true
    };
  }
  // All answers collected — return result
  return { type: 'result', greeting: 'Hello ' + answers.name };
}

function formDefinition() {
  return {
    type: 'qa',
    title: 'My Quiz',
    description: 'A simple quiz',
    fields: [
      { name: 'name', label: 'Name', type: 'text', required: true }
    ]
  };
}

function formSubmit(params) {
  if (!params.name) return { type: 'validation_error', errors: [{ field: 'name', message: 'Required' }] };
  return { type: 'result', greeting: 'Hello ' + params.name };
}
```

---

## Shared helpers (_helpers.js)

If multiple tools in your app share logic (auth, API base URL, common parsers), put it in `_helpers.js`. This file is automatically loaded before every tool in the app.

```js
// _helpers.js — loaded before every tool in this app

function apiGet(path) {
  var host = config.api_host;  // from app.yaml settings
  var token = secrets.api_token;
  return http.get(host + path, {
    headers: { "Authorization": "Bearer " + token }
  });
}

function apiPost(path, body) {
  var host = config.api_host;
  var token = secrets.api_token;
  return http.post(host + path, body, {
    headers: {
      "Authorization": "Bearer " + token,
      "Content-Type": "application/json"
    }
  });
}
```

Then in any tool file, just call `apiGet("/items")` directly.

---

## Prompts

Prompt templates are markdown files in `prompts/`. They become reusable prompt commands users can invoke.

**Prompt file — `prompts/getting-started.md`:**

```markdown
---
name: getting_started
description: Get started with My App — connect and explore
arguments: []
---

You are helping the user get started with My App.

## Step 1: Check Connection

Call `my-app.health_check` to verify the system is running.

## Step 2: List Items

Call `my-app.list_items` with limit=5 to show what exists.

Present a summary to the user.
```

**Prompt with arguments — `prompts/search.md`:**

```markdown
---
name: search
description: Search for items by keyword
arguments:
  - name: query
    description: "What to search for"
    required: true
  - name: max_results
    description: "Max results to return"
    required: false
---

Search for items matching: {{query}}

Call `my-app.search_items` with the query.
Present results in a table. If no results, suggest alternative searches.
Max results: {{max_results}}
```

Prompt frontmatter fields:
- `name` — prompt ID (underscores, must match what tools reference)
- `description` — shown to users and AI when listing available prompts
- `arguments` — list of `{name, description, required}` objects
- Use `{{argument_name}}` in the body to interpolate argument values

---

## Complete example: weather app

A simple app with one API tool and one prompt.

**`data/apps/weather-checker/app.yaml`:**

```yaml
name: weather-checker
version: 1.0.0
description: Check current weather for any city using Open-Meteo free API
author: Admin
permissions:
  allowedHosts:
    - api.open-meteo.com
    - geocoding-api.open-meteo.com
  defaultToolClass: read-only
timeout: 30s
settings: []
tags: []
```

**`data/apps/weather-checker/tools/get_weather.json`:**

```json
{
  "name": "get_weather",
  "description": "Get current weather for a city. Returns temperature, humidity, wind speed, and conditions.",
  "toolClass": "read-only",
  "params": {
    "city": {
      "type": "string",
      "required": true,
      "description": "City name (e.g. Sydney, London, New York)"
    }
  }
}
```

**`data/apps/weather-checker/tools/get_weather.js`:**

```js
function handle(params) {
  var geo = http.get(
    "https://geocoding-api.open-meteo.com/v1/search?name="
    + encodeURIComponent(params.city) + "&count=1"
  );
  if (!geo.json || !geo.json.results || geo.json.results.length === 0) {
    return { error: "City not found: " + params.city };
  }

  var loc = geo.json.results[0];
  var url = "https://api.open-meteo.com/v1/forecast"
    + "?latitude=" + loc.latitude
    + "&longitude=" + loc.longitude
    + "&current=temperature_2m,relative_humidity_2m,wind_speed_10m,weather_code"
    + "&timezone=auto";

  var wx = http.get(url);
  if (!wx.json || !wx.json.current) {
    return { error: "Failed to fetch weather data" };
  }

  var c = wx.json.current;
  var codes = {
    0: "Clear sky", 1: "Mainly clear", 2: "Partly cloudy", 3: "Overcast",
    61: "Slight rain", 63: "Moderate rain", 65: "Heavy rain",
    95: "Thunderstorm"
  };

  return {
    city: loc.name,
    country: loc.country,
    temperature_c: c.temperature_2m,
    humidity_pct: c.relative_humidity_2m,
    wind_speed_kmh: c.wind_speed_10m,
    conditions: codes[c.weather_code] || "Unknown (" + c.weather_code + ")"
  };
}
```

**`data/apps/weather-checker/prompts/weather_report.md`:**

```markdown
---
name: weather_report
description: Generate a weather report for a city
arguments:
  - name: city
    description: City to report on
    required: true
---

Use the get_weather tool to fetch the current weather for {{city}}, then write a brief weather report including:
- Current conditions and temperature
- Whether it is good weather for outdoor activities
- What to wear based on the conditions
- Any weather warnings if applicable
```

---

## Complete example: API integration with auth

An app that connects to an external API with authentication.

**`data/apps/my-api/app.yaml`:**

```yaml
name: my-api
version: 1.0.0
description: Manage items in My Service — list, create, update, delete
author: NubeIO

permissions:
  allowedHosts:
    - "*.myservice.com"
    - "localhost:*"
  defaultToolClass: read-write
  secrets:
    - api_token

settings:
  - key: api_host
    label: API Host
    type: url
    required: true
    default: http://localhost:9000
  - key: api_token
    label: API Token
    type: secret
    required: false

preamble: |
  You are an assistant for My Service. Use the my-api.* tools to help
  the user manage their items. Auth is automatic.

tags:
  - api
  - management

timeout: 15s
```

**`data/apps/my-api/tools/_helpers.js`:**

```js
function apiGet(path) {
  return http.get(config.api_host + "/api/v1" + path, {
    headers: { "Authorization": "Bearer " + secrets.api_token }
  });
}

function apiPost(path, body) {
  return http.post(config.api_host + "/api/v1" + path, body, {
    headers: {
      "Authorization": "Bearer " + secrets.api_token,
      "Content-Type": "application/json"
    }
  });
}

function apiDelete(path) {
  return http.delete(config.api_host + "/api/v1" + path, {
    headers: { "Authorization": "Bearer " + secrets.api_token }
  });
}
```

**`data/apps/my-api/tools/list_items.json`:**

```json
{
  "name": "list_items",
  "description": "List all items. Supports filtering by status and pagination.",
  "toolClass": "read-only",
  "params": {
    "status": {
      "type": "string",
      "required": false,
      "description": "Filter by status: active, inactive, all (default: all)"
    },
    "limit": {
      "type": "number",
      "required": false,
      "description": "Max results (default 50)"
    }
  }
}
```

**`data/apps/my-api/tools/list_items.js`:**

```js
function handle(params) {
  var query = "?limit=" + (params.limit || 50);
  if (params.status && params.status !== "all") {
    query += "&status=" + encodeURIComponent(params.status);
  }

  var resp = apiGet("/items" + query);
  if (resp.status !== 200) {
    return { error: "List failed (" + resp.status + "): " + resp.body };
  }

  var items = resp.json.data || [];
  var result = [];
  for (var i = 0; i < items.length; i++) {
    result.push({
      id: items[i].id,
      name: items[i].name,
      status: items[i].status
    });
  }

  return { count: result.length, items: result };
}
```

**`data/apps/my-api/tools/create_item.json`:**

```json
{
  "name": "create_item",
  "description": "Create a new item.",
  "toolClass": "read-write",
  "params": {
    "name": {
      "type": "string",
      "required": true,
      "description": "Item name"
    },
    "description": {
      "type": "string",
      "required": false,
      "description": "Item description"
    }
  }
}
```

**`data/apps/my-api/tools/create_item.js`:**

```js
function handle(params) {
  if (!params.name) return { error: "name is required" };

  var body = { name: params.name };
  if (params.description) body.description = params.description;

  var resp = apiPost("/items", body);
  if (resp.status !== 200 && resp.status !== 201) {
    return { error: "Create failed (" + resp.status + "): " + resp.body };
  }

  var item = resp.json.data || resp.json;
  return {
    id: item.id,
    name: item.name,
    message: "Created " + item.name
  };
}
```

---

## Checklist for a new app

1. Create `data/apps/<app-name>/app.yaml` with name, description, permissions, settings
2. Create `data/apps/<app-name>/tools/` directory
3. For each tool: write `tool_name.json` (schema) and `tool_name.js` (logic)
4. If tools share auth/helpers: write `_helpers.js`
5. Create `data/apps/<app-name>/prompts/` directory (if needed)
6. For each prompt: write a markdown file with frontmatter (name, description, arguments)
7. Reload apps: `POST /admin/reload-apps` or restart the server
8. Install the app for a user: `POST /api/apps/install`

## Common patterns

| Pattern | How to implement |
|---|---|
| Read-only data fetch | Single JS tool, `toolClass: "read-only"`, use `http.get()` |
| CRUD operations | Multiple JS tools (list, get, create, update, delete) + `_helpers.js` for shared auth |
| Multi-step workflow | Prompt-mode tool (`.json` with `"mode": "prompt"`) that orchestrates other tools |
| Interactive form/quiz | QA-mode tool (`.json` with `"mode": "qa"`) + `.js` with `chatMode`/`formDefinition`/`formSubmit` |
| Prompt-only (no API) | No tools directory; just `prompts/` with markdown templates |
| External API with auth | `_helpers.js` for auth, `secrets` in app.yaml, `config.*` for host URL |

## Tips

- Keep tool descriptions clear and specific — the AI uses them to decide which tool to call
- Return structured JSON from tools, not prose — the AI formats the response for the user
- Use `_helpers.js` to keep individual tool files short and focused
- Set `toolClass: "read-only"` for safe tools, `"read-write"` for anything that modifies data
- List all external domains in `allowedHosts` — tools cannot call unlisted hosts
- Use `preamble` to give the AI context about how to use your app's tools together
