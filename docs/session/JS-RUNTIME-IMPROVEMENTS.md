# JS Runtime Improvements — Session Scope

Date: 2026-04-18

## Context

The Goja JS runtime is the execution engine for all app tools. Every tool call creates a Goja VM, injects host APIs, runs the script's `handle(params)`, and returns JSON. This session adds new host APIs to make tools more capable and reduce boilerplate.

## What Existed Before

| API | Description |
|---|---|
| `http.get/post/put/patch/delete` | HTTP requests with allowedHosts enforcement |
| `secrets.<key>` | App install secrets |
| `config.<key>` | App install settings |
| `log.info/error` | Server-side logging |
| `files.read(path)` | Read files from app directory |
| `plugins.exists/info/list/call` | Plugin discovery and tool calling |

## What This Session Adds

### 1. `tools.call(name, params)` — Tool Composition

Call another tool in the same app from JS. Enables tools to build on each other instead of the AI orchestrating everything.

```js
function handle(params) {
  // Call another tool in this app
  var items = tools.call("list_items", {status: "active"});
  if (items.error) return items;

  // Process the results
  var summary = {total: 0, names: []};
  for (var i = 0; i < items.result.items.length; i++) {
    summary.names.push(items.result.items[i].name);
    summary.total++;
  }
  return summary;
}
```

Returns `{result: <tool output>}` on success, `{error: "..."}` on failure. Only calls tools within the same app (security boundary).

### 2. `base64.encode(str)` / `base64.decode(str)` — Base64 Encoding

Common need for auth headers, JWT payloads, binary data.

```js
function handle(params) {
  // Basic auth header
  var creds = base64.encode(config.username + ":" + secrets.password);
  var resp = http.get(config.api_host + "/data", {
    headers: { "Authorization": "Basic " + creds }
  });
  return resp.json;
}
```

### 3. `url.buildQuery(params)` / `url.parse(urlStr)` — URL Helpers

Eliminates manual query string building with encodeURIComponent.

```js
function handle(params) {
  var qs = url.buildQuery({
    city: params.city,
    units: "metric",
    limit: 5
  });
  var resp = http.get(config.api_host + "/search?" + qs);
  return resp.json;
}
```

`url.parse(str)` returns `{protocol, host, path, query, hash}`.

### 4. `crypto.sha256(data)` / `crypto.hmac(algo, key, data)` — Crypto

Webhook signature verification and data hashing.

```js
function handle(params) {
  var hash = crypto.sha256(params.data);
  var sig = crypto.hmac("sha256", secrets.webhook_secret, params.body);
  return { hash: hash, signature: sig };
}
```

Supports: sha256, sha1, md5.

### 5. `env.get(key)` — Environment Variables

Read server environment variables. Useful for system-level config that shouldn't require user input.

```js
function handle(params) {
  var host = env.get("OLLAMA_HOST") || "http://localhost:11434";
  var resp = http.get(host + "/api/tags");
  return resp.json;
}
```

Only exposes a curated allowlist (not arbitrary env vars).

## Files Changed

| File | Change |
|---|---|
| `pkg/apps/jsruntime_inject.go` | New inject methods for all 5 APIs |
| `pkg/apps/jsruntime.go` | Add toolCaller field, SetToolCaller method |
| `pkg/apps/mcpfactory.go` | ToolCallerSource interface definition |
| `pkg/apps/mcp_jstools.go` | Wire tool caller into runtime |
| `pkg/services/tools.go` | Wire tool caller into ToolService path |
| `pkg/apps/plugins_api_test.go` | Tests for all new host APIs |
| `cmd/nube-server/bootstrap_app.go` | Updated prompts with new API docs |
| `docs/APP-DEVELOPMENT.md` | Updated globals table |
| `docs/MCP.md` | Updated globals table |

## Design Decisions

- **tools.call is same-app only** — prevents security boundary violations. An app's tool can't call tools from other apps or plugins (use `plugins.call` for that).
- **env.get uses an allowlist** — not arbitrary env access. Only vars prefixed with `BIZZY_`, `NUBE_`, `OLLAMA_`, or `GITHUB_` are readable. Prevents leaking PATH, credentials, etc.
- **No sleep()** — considered but deferred. Rate limiting should be handled by the caller or the http client, not by blocking the VM goroutine.
- **No store.get/set** — deferred. Persistent state is a bigger feature requiring DB schema changes.
- **No VM reuse** — deferred. Fresh VM per call is simpler and avoids state leaks. Profile before optimizing.
