# Backend: nube-server

The main API server for NubeIO. Manages workspaces, users, system apps (on disk), app installs, MCP tool serving, and agent sessions.

---

## Quick start

```bash
make server          # starts on :8090
# or
make start           # starts nube-server + store-server + fakeserver
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
| `NUBE_DATA_DIR` | `./data` | JSON database directory |
| `NUBE_APPS_DIR` | `./apps` | System apps directory |

---

## Data storage

File-backed JSON collections in `NUBE_DATA_DIR/`:

| File | Model | Description |
|---|---|---|
| `workspaces.json` | `Workspace` | Multi-tenant workspaces |
| `users.json` | `User` | Users with bearer tokens |
| `app_installs.json` | `AppInstall` | Per-user app installations |
| `sessions.json` | `Session` | Agent session history |

All collections use `pkg/jsondb.Collection[T]` -- thread-safe, atomic writes (tmp + rename).

---

## Authentication

Bearer token middleware on all routes except `/health` and `/bootstrap`.

- Token resolved to `User` from the `users.json` collection.
- **Dev mode**: if no `Authorization` header is sent, falls back to the first user in DB.
- **Admin impersonation**: set `X-Act-As-User: <userId>` header (admin-only).

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

### System apps (on-disk)

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/apps` | user | List all apps |
| `GET` | `/apps/:id` | user | Get app details |
| `POST` | `/apps` | admin | Create app (directory + app.yaml) |
| `PUT` | `/apps/:id` | admin | Update app metadata |
| `DELETE` | `/apps/:id` | admin | Delete app (removes files) |
| `GET` | `/apps/:id/tools` | user | List tools |
| `POST` | `/apps/:id/tools` | admin | Create JS tool (.js + .json) |
| `PUT` | `/apps/:id/tools/:name` | admin | Update tool |
| `DELETE` | `/apps/:id/tools/:name` | admin | Delete tool |
| `GET` | `/apps/:id/prompts` | user | List prompts |
| `POST` | `/apps/:id/prompts` | admin | Create prompt (.md) |
| `DELETE` | `/apps/:id/prompts/:name` | admin | Delete prompt |

### App installs

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/apps/:id/install` | user | Install an app |
| `GET` | `/app-installs` | user | List user's installs |
| `PATCH` | `/app-installs/:id` | user | Update install (settings, enable/disable) |
| `DELETE` | `/app-installs/:id` | user | Uninstall |

### User tools and prompts (REST)

| Method | Path | Description |
|---|---|---|
| `GET` | `/my/tools` | List current user's namespaced tools |
| `GET` | `/my/prompts` | List current user's prompts |
| `GET` | `/my/prompts/:name` | Render prompt with query args |

### Agents

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/agents` | List agents (derived from installed apps) |
| `POST` | `/api/agents/tools/:name` | Call a tool directly |
| `GET` | `/api/agents/sessions` | List session history |
| `GET` | `/api/agents/sessions/:id` | Get session detail |

### WebSocket

| Path | Auth | Description |
|---|---|---|
| `/api/agents/run?token=<token>` | query param | Streaming agent chat |
| `/api/agents/qa?token=<token>` | query param | Interactive QA flows |

### MCP

| Path | Description |
|---|---|
| `/mcp`, `/mcp/*path` | Per-user MCP tool serving (StreamableHTTP) |

### Admin

| Method | Path | Description |
|---|---|---|
| `POST` | `/admin/reload-apps` | Reload system apps from disk |

---

## MCP tool serving

The `MCPFactory` builds a per-user MCP server scoped to their installed apps:

1. Iterates the user's `AppInstall` records.
2. For each enabled install, tries the system registry (disk) first.
3. Falls back to store apps via `StoreAppProvider` if configured.
4. Registers three types per app:
   - **OpenAPI tools** -- from `openapi.yaml`, base URL + auth token from user settings.
   - **JS tools** -- sandboxed Goja runtime with host APIs (`http.*`, `secrets.*`, `config.*`, `log.*`, `files.read`). Timeout per app (default 5s).
   - **Prompts** -- markdown templates with `{{key}}` substitution.
5. All names are namespaced: `appName.toolName`, `appName.promptName`.

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
| `appName` | string | Links to system app or store app by name |
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
make start         # all servers (nube :8090, store :8091, fake :9001)
make stop          # stop all
make reset         # wipe data + stop
make build         # build all binaries to bin/
make test          # Go unit + integration tests
make test-api      # run API test script against running servers
```
