# Backend: nube-server

The main API server for NubeIO. Manages workspaces, users, the app store, app installs, MCP tool serving, and agent sessions -- all in a single process.

---

## Quick start

```bash
make server          # starts on :8090
```

Bootstrap the first admin:

```bash
curl -X POST http://localhost:8090/bootstrap \
  -H 'Content-Type: application/json' \
  -d '{"workspaceName":"My Org","adminName":"Admin","adminEmail":"admin@example.com"}'
```

Returns `{workspace, user, token}`. Use the token as `Authorization: Bearer <token>`.

---

## Environment variables

| Variable | Default | Description |
|---|---|---|
| `NUBE_ADDR` | `:8090` | Listen address |
| `NUBE_DATA_DIR` | `./data` | Data directory (resolved to absolute path on startup) |

Apps live at `$NUBE_DATA_DIR/apps/`. There is no separate apps directory env var.

---

## Data storage

Everything lives under `NUBE_DATA_DIR/`:

```
data/
  apps/                    # All app disk files (system + user-created)
    nube-admin/
      app.yaml
      tools/*.js + *.json
    nube-marketing/
      app.yaml
      tools/*.js + *.json
      prompts/*.md
    weather-checker/       # Created via store API
      app.yaml
      tools/*.js + *.json
      prompts/*.md
  store_apps.json          # Metadata for ALL apps (ratings, visibility, author, etc.)
  app_installs.json        # Per-user app installations
  app_shares.json          # Share invites (by user or link)
  app_reviews.json         # Ratings and comments
  workspaces.json          # Multi-tenant workspaces
  users.json               # Users with bearer tokens
  sessions.json            # Agent session history
```

All JSON collections use `pkg/jsondb.Collection[T]` -- thread-safe, atomic writes (tmp + rename).

### Startup sync

On startup the server runs two sync operations:

1. **Store-to-disk migration**: any `StoreApp` record with inline tools/prompts but no disk directory gets its files written to `data/apps/`.
2. **Disk-to-store sync**: any app on disk without a `store_apps.json` record gets one auto-created (visibility=public, author from app.yaml).

This means system apps shipped with the code (e.g. nube-marketing) automatically appear in the store alongside user-created apps.

---

## Authentication

Bearer token middleware on all routes except `/health` and `/bootstrap`.

- Token resolved to `User` from the `users.json` collection.
- **Dev mode**: if no `Authorization` header is sent, falls back to the first user in DB.
- **Admin impersonation**: set `X-Act-As-User: <userId>` header (admin-only).
- **WebSocket auth**: `?token=<bearer-token>` query param. Dev mode (`?token=dev` or no token) falls back to first user.

---

## API routes

### Public (no auth)

| Method | Path | Description |
|---|---|---|
| `GET` | `/health` | Health check (`{status, users, apps}`) |
| `POST` | `/bootstrap` | Create first workspace + admin (409 if users exist) |

### User management

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/users/me` | user | Current user info |
| `GET` | `/users/:id` | admin | Get user by ID |
| `DELETE` | `/users/:id` | admin | Delete user |
| `POST` | `/users/:id/token` | self/admin | Rotate token |
| `DELETE` | `/users/:id/token` | self/admin | Revoke token |

### Workspaces

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/workspaces` | user | List (scoped by role) |
| `GET` | `/workspaces/:id` | user | Get workspace |
| `POST` | `/workspaces` | admin | Create workspace |
| `DELETE` | `/workspaces/:id` | admin | Delete workspace |
| `POST` | `/workspaces/:id/users` | admin | Create user in workspace |
| `GET` | `/workspaces/:id/users` | user | List workspace users |

### App store -- see [STORE.md](STORE.md) for full details

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/store/apps` | Browse all apps (query, category, sort, pagination) |
| `GET` | `/api/store/apps/:id` | App detail + installed flag |
| `POST` | `/api/store/apps/:id/install` | Install app for current user |
| `GET/POST/DELETE` | `/api/store/apps/:id/reviews` | Reviews CRUD |
| `GET/POST/PUT/DELETE` | `/api/my/apps/...` | Author CRUD (create, edit, publish, tools, prompts, sharing) |

### App installs

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/apps/:id/install` | user | Install an app |
| `GET` | `/app-installs` | user | List user's installs |
| `PATCH` | `/app-installs/:id` | user | Update install (settings, enable/disable) |
| `DELETE` | `/app-installs/:id` | user | Uninstall |

### Agents

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/agents` | List agents (derived from installed apps) |
| `POST` | `/api/agents/tools/:name` | Call a tool directly (e.g. `weather-checker.get_weather`) |
| `GET` | `/api/agents/sessions` | List session history |
| `GET` | `/api/agents/sessions/:id` | Get session detail |
| `GET` | `/api/agents/providers` | List AI providers |
| `POST` | `/api/agents/run/sync` | Synchronous agent run |

### WebSocket

| Path | Auth | Description |
|---|---|---|
| `/api/agents/run` | `?token=` | Streaming agent chat |
| `/api/agents/qa` | `?token=` | Interactive QA wizard flows |

### MCP

| Path | Description |
|---|---|
| `/mcp`, `/mcp/*path` | Per-user MCP tool serving (StreamableHTTP) |

### Admin

| Method | Path | Description |
|---|---|---|
| `POST` | `/admin/reload-apps` | Reload apps from disk + rebuild MCP cache |

---

## MCP tool serving

The `MCPFactory` builds a per-user MCP server scoped to their installed apps:

1. Iterates the user's `AppInstall` records.
2. For each enabled install, looks up the app in the registry (all apps live in one directory).
3. Registers three types per app:
   - **OpenAPI tools** -- from `openapi.yaml`, base URL + auth token from user settings.
   - **JS tools** -- sandboxed Goja runtime with host APIs (`http.*`, `secrets.*`, `config.*`, `log.*`, `files.read`). Timeout per app (default 5s, configurable in app.yaml).
   - **Prompts** -- markdown templates with `{{key}}` substitution.
4. All names are namespaced: `appName.toolName`, `appName.promptName`.

---

## Models

### Workspace

| Field | Type | Notes |
|---|---|---|
| `id` | string | `ws-` prefix |
| `name` | string | |
| `createdAt` | time | |

### User

| Field | Type | Notes |
|---|---|---|
| `id` | string | `usr-` prefix |
| `workspaceId` | string | |
| `name` | string | |
| `email` | string | |
| `role` | string | `admin` or `user` |
| `token` | string | 32-byte hex bearer token |
| `createdAt` | time | |

### AppInstall

| Field | Type | Notes |
|---|---|---|
| `id` | string | `inst-` prefix |
| `appName` | string | Links to app by name |
| `appVersion` | string | |
| `workspaceId` | string | |
| `userId` | string | |
| `enabled` | bool | |
| `settings` | map[string]string | Non-secret config |
| `secrets` | map[string]string | Secret config (checked first by `GetSetting`) |
| `stale` | bool | True if app version changed since install |
| `createdAt` | time | |
| `updatedAt` | time | |

---

## Make targets

```
make server        # nube-server on :8090
make start         # same as server (foreground)
make stop          # stop servers
make reset         # wipe data + stop
make build         # build all binaries to bin/
make test          # Go unit + integration tests
make test-api      # run API test script against running servers
```
