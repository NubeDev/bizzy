# App Store

The app store is built into nube-server. All apps -- whether shipped with the code or created by users -- live in one unified system. Users can create apps, test them privately, publish to the store, and browse/install apps created by others.

---

## Quick start

```bash
make server          # starts nube-server on :8090 (includes store)
```

The store uses the same auth as the rest of nube-server. Bootstrap an admin, then use that token for all store operations.

---

## Architecture

**Database is the single source of truth.** Store apps are DB-only — no disk files are written or read for user-created apps.

```
Database (SQLite)
  store_apps table         # StoreApp records with tools, prompts, uiComponents as JSON columns
  app_installs table       # Per-user installations
  app_reviews table        # Ratings and comments
  app_shares table         # Share invites

data/apps/                 # System apps only (shipped with code, read-only)
  nube-marketing/
    app.yaml
    tools/*.js + *.json
```

- **StoreApp** records hold everything: metadata, tools (with JS scripts inline), prompts, UI components, permissions, settings.
- On startup, system apps on disk without a DB record get one auto-created.
- When a user creates/edits an app via the API, only the DB record is updated. The registry is reloaded and MCP factory rebuilt.

---

## App lifecycle

```
Create (private, auto-installed for author)
  -> add tools + prompts (DB updated, registry reloaded)
  -> author tests immediately (tools are live via MCP and REST)
  -> optionally share with specific users or generate a link
  -> publish to store (visibility -> public, validation checks run)
  -> community browses, installs, reviews
  -> author pushes updates (bump version, add tools)
```

### Publishing requirements

Before `POST /api/my/apps/:id/publish` succeeds, the app must have:

- `displayName` (non-empty)
- `description` (min 20 characters)
- `category` (from the fixed list)
- At least one tool or prompt
- No `allowedHosts` pointing to localhost or private IPs

Private and shared apps skip these checks.

---

## Data storage

All store data lives in SQLite (via GORM):

| Table | Model | Description |
|---|---|---|
| `store_apps` | `StoreApp` | App records with inline tools, prompts, UI components |
| `app_installs` | `AppInstall` | Per-user installations with settings |
| `app_shares` | `AppShare` | Share invites (by user or link) |
| `app_reviews` | `AppReview` | Ratings and comments |

---

## API routes

All store routes are authenticated via bearer token (dev mode: no token falls back to first user).

### Store catalog

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/store/apps` | Browse all apps (query, category, sort, pagination) |
| `GET` | `/api/store/apps/:id` | Get app detail (includes `installed` flag for current user) |
| `GET` | `/api/store/categories` | List fixed categories |
| `GET` | `/api/store/apps/:id/reviews` | List app reviews |

### Browse query parameters

| Param | Default | Values |
|---|---|---|
| `q` | | Free-text search (name, displayName, description, tags) |
| `category` | | Filter by category slug |
| `sort` | `popular` | `popular`, `recent`, `rating`, `name` |
| `page` | `1` | Page number |
| `limit` | `20` | Results per page (max 50) |

### Install + reviews

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/store/apps/:id/install` | Install a store app for current user |
| `POST` | `/api/store/apps/:id/reviews` | Submit review (`{rating: 1-5, comment}`) |
| `PUT` | `/api/store/apps/:id/reviews` | Update review (upsert) |
| `DELETE` | `/api/store/apps/:id/reviews` | Delete own review |

Rules: one review per user per app, author cannot review own app, `avgRating` and `reviewCount` recalculated on every write.

### My Apps -- author CRUD

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/my/apps` | List apps I authored |
| `POST` | `/api/my/apps` | Create app (DB record, auto-installs for author) |
| `GET` | `/api/my/apps/:id` | Get my app (full detail) |
| `PUT` | `/api/my/apps/:id` | Update metadata (partial update) |
| `DELETE` | `/api/my/apps/:id` | Delete app (DB record + reviews) |
| `POST` | `/api/my/apps/:id/publish` | Validate and set visibility to public |
| `PATCH` | `/api/my/apps/:id/visibility` | Set visibility (`{visibility}`) |

### Sharing

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/my/apps/:id/share` | Share with user (`{userId}`) |
| `POST` | `/api/my/apps/:id/share-link` | Generate token-based share link |
| `GET` | `/api/my/apps/:id/shares` | List shares for app |
| `DELETE` | `/api/my/apps/:id/shares/:shareId` | Revoke share |

### Tools within an app

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/my/apps/:id/tools` | Add tool (DB update, reloads registry) |
| `PUT` | `/api/my/apps/:id/tools/:name` | Update tool (DB update, reloads) |
| `DELETE` | `/api/my/apps/:id/tools/:name` | Delete tool (DB update, reloads) |

### Prompts within an app

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/my/apps/:id/prompts` | Add prompt (DB update, reloads registry) |
| `PUT` | `/api/my/apps/:id/prompts/:name` | Update prompt |
| `DELETE` | `/api/my/apps/:id/prompts/:name` | Delete prompt |

---

## Tool execution

When a tool is added via the API, the server:

1. Updates the `StoreApp` record in the database (tools stored as JSON column with inline script).
2. Reloads the app registry so the tool is immediately available.
3. Rebuilds the MCP factory cache.

No disk files are written. The JS script lives in `StoreTool.Script` in the database.

Tools execute via the Goja JS sandbox with these host APIs:
- `http.get/post/put/patch/delete(url, body?, opts?)` -- outbound HTTP (restricted by `allowedHosts`)
- `secrets.{key}` -- access secret settings from the user's install
- `config.{key}` -- access non-secret settings
- `log.info/error(msg)` -- server-side logging
- `files.read(relPath)` -- read files within the app directory

### QA tools

Tools with `mode: "qa"` support an interactive conversational flow:

- **Chat mode**: called with `{_answers: {}}`, returns `{type: "question", field, label, input, options, ...}`. Client collects answers one at a time and re-calls with accumulated answers until the tool returns `{type: "prompt"}` (triggers Claude) or `{type: "result"}` (direct result).
- **Form mode**: called with `{_submit: false}` to get field definitions, then `{_submit: true, ...fields}` to validate and execute.

The WebSocket endpoint `/api/agents/qa` drives the chat mode flow automatically.

---

## Visibility

| Visibility | In store browse | Direct link works | Who can access |
|---|---|---|---|
| `private` | No | Author only | Only the author |
| `shared` | No | Yes (with invite token) | Author + invited users |
| `unlisted` | No | Yes | Anyone with the link |
| `public` | Yes | Yes | Anyone |

---

## Categories

Fixed set:

```
iot-devices, analytics, devops, marketing, design, utilities, integrations, automation
```

---

## Models

### StoreApp

| Field | Type | Notes |
|---|---|---|
| `id` | string | `app-` prefix |
| `name` | string | Unique slug (`^[a-z][a-z0-9-]{1,48}[a-z0-9]$`) |
| `displayName` | string | |
| `description` | string | Short, shown in cards |
| `longDescription` | string | Markdown, shown on detail page |
| `version` | string | Semver |
| `icon` | string | Lucide icon name |
| `color` | string | Hex color |
| `category` | string | One of the fixed categories |
| `tags` | []string | Free-form search tags |
| `authorId` | string | Creator's User ID (or `"system"` for auto-synced apps) |
| `authorName` | string | Denormalized for listing |
| `workspaceId` | string | |
| `visibility` | string | `private`, `shared`, `unlisted`, `public` |
| `permissions` | object | `{allowedHosts, defaultToolClass, secrets}` |
| `settings` | []SettingDef | `{key, label, type, required, default}` |
| `tools` | []StoreTool | Tools with inline JS scripts (single source of truth) |
| `prompts` | []StorePrompt | Prompt templates (single source of truth) |
| `uiComponents` | []UIComponent | React UI components for live preview |
| `installCount` | int | Total installs |
| `activeInstalls` | int | Currently enabled |
| `avgRating` | float | 0.0--5.0 |
| `reviewCount` | int | |
| `createdAt` | time | |
| `updatedAt` | time | |
| `publishedAt` | time | Set on first publish (nullable) |

### StoreTool

| Field | Type | Notes |
|---|---|---|
| `name` | string | |
| `description` | string | |
| `toolClass` | string | `read-only`, `read-write`, `destructive` |
| `mode` | string | `""` (standard) or `"qa"` (interactive wizard) |
| `params` | map | `{paramName: {type, required, description}}` |
| `script` | string | JavaScript source (must define `handle(params)`) |

### StorePrompt

| Field | Type | Notes |
|---|---|---|
| `name` | string | |
| `description` | string | |
| `arguments` | []object | `{name, description, required}` |
| `body` | string | Markdown with `{{key}}` substitution |

### UIComponent

| Field | Type | Notes |
|---|---|---|
| `name` | string | Component name (e.g. `dashboard`, `search-bar`) |
| `code` | string | Raw TSX source (no imports, uses SCOPE) |

### AppReview

| Field | Type | Notes |
|---|---|---|
| `id` | string | `rev-` prefix |
| `appId` | string | |
| `userId` | string | |
| `userName` | string | Denormalized |
| `rating` | int | 1--5 |
| `comment` | string | Optional |
| `createdAt` | time | |
| `updatedAt` | time | |

### AppShare

| Field | Type | Notes |
|---|---|---|
| `id` | string | `share-` prefix |
| `appId` | string | |
| `invitedBy` | string | User ID |
| `inviteeId` | string | Specific user (empty for link-based) |
| `token` | string | For link-based sharing |
| `createdAt` | time | |
| `expiresAt` | time | Optional expiry |

---

## System app disk structure

System apps (shipped with the code) live on disk at `data/apps/`. User-created apps do NOT use disk — they are DB-only.

```
data/apps/{app-name}/
  app.yaml                 # name, version, description, author, permissions, settings, tags, timeout
  tools/
    {tool-name}.json       # manifest: name, description, toolClass, mode, params
    {tool-name}.js         # script: must define handle(params) function
  prompts/
    {prompt-name}.md       # YAML frontmatter (name, description, arguments) + body
  openapi.yaml             # (optional) OpenAPI spec for REST-based tools
```

---

## Make targets

```
make server        # nube-server on :8090
make start         # same (foreground with banner)
make stop          # stop servers
make reset         # wipe data + stop
make build         # build all binaries to bin/
make test          # Go unit + integration tests
```
