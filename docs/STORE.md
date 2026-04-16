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

There is **one app system**, not two:

```
data/
  apps/                    # Disk files for ALL apps
    nube-marketing/        #   System app (shipped with code)
    weather-checker/       #   User-created app (via store API or AI wizard)
  store_apps.json          # Metadata for ALL apps (StoreApp records)
```

- **Disk files** (`data/apps/{name}/`) hold the executable content: `app.yaml`, `tools/*.js + *.json`, `prompts/*.md`.
- **store_apps.json** holds community metadata: visibility, ratings, reviews, install counts, author info.
- On startup, any disk app without a store record gets one auto-created.
- When a user creates/edits an app via the API, both the JSON record and disk files are updated together.

---

## App lifecycle

```
Create (private, auto-installed for author)
  -> add tools + prompts (disk files written, registry reloaded)
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

JSON collections in `NUBE_DATA_DIR/`:

| File | Model | Description |
|---|---|---|
| `store_apps.json` | `StoreApp` | Metadata for all apps (system + user-created) |
| `app_shares.json` | `AppShare` | Share invites (by user or link) |
| `app_reviews.json` | `AppReview` | Ratings and comments |

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
| `POST` | `/api/my/apps` | Create app (writes disk files + JSON record, auto-installs for author) |
| `GET` | `/api/my/apps/:id` | Get my app (full detail) |
| `PUT` | `/api/my/apps/:id` | Update metadata (partial update, re-writes app.yaml) |
| `DELETE` | `/api/my/apps/:id` | Delete app (removes disk dir + JSON record + reviews) |
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
| `POST` | `/api/my/apps/:id/tools` | Add tool (writes `.js` + `.json` to disk, reloads registry) |
| `PUT` | `/api/my/apps/:id/tools/:name` | Update tool (re-writes files, reloads) |
| `DELETE` | `/api/my/apps/:id/tools/:name` | Delete tool (removes files, reloads) |

### Prompts within an app

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/my/apps/:id/prompts` | Add prompt (writes `.md` to disk, reloads registry) |
| `PUT` | `/api/my/apps/:id/prompts/:name` | Update prompt |
| `DELETE` | `/api/my/apps/:id/prompts/:name` | Delete prompt |

---

## Tool execution

When a tool is added via the API, the server:

1. Updates the `StoreApp` record in `store_apps.json` (keeps inline copy for metadata display).
2. Writes `tools/{name}.json` (manifest) + `tools/{name}.js` (script) to `data/apps/{appName}/`.
3. Reloads the app registry so the tool is immediately available.
4. Rebuilds the MCP factory cache.

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
| `tools` | []StoreTool | Tool metadata (inline copy for display; execution uses disk files) |
| `prompts` | []StorePrompt | Prompt metadata (inline copy; execution uses disk files) |
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
| `script` | string | JavaScript source (must define `handle(params)`). Also written to disk. |

### StorePrompt

| Field | Type | Notes |
|---|---|---|
| `name` | string | |
| `description` | string | |
| `arguments` | []object | `{name, description, required}` |
| `body` | string | Markdown with `{{key}}` substitution |

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

## Disk app structure

Every app (system or user-created) has the same structure:

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
