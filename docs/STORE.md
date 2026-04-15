# App Store

The app store is built into nube-server. Any authenticated user can create apps, publish them to the community, and browse/install apps created by others.

---

## Quick start

```bash
make server          # starts nube-server on :8090 (includes store)
```

The store uses the same auth as the rest of nube-server. Bootstrap an admin, then use that token for all store operations.

---

## Data storage

Additional JSON collections in `NUBE_DATA_DIR/`:

| File | Model | Description |
|---|---|---|
| `store_apps.json` | `StoreApp` | User-created apps |
| `app_shares.json` | `AppShare` | Share invites (by user or link) |
| `app_reviews.json` | `AppReview` | Ratings and comments |

---

## API routes

All store routes are under `/api/store/` and `/api/my/apps/`, authenticated via the same bearer token as all other nube-server endpoints.

### Store catalog (authenticated)

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/store/apps` | Browse public apps (query, category, sort, pagination) |
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

### Install + reviews (authenticated)

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/store/apps/:id/install` | Install a store app for current user |
| `POST` | `/api/store/apps/:id/reviews` | Submit review (`{rating: 1-5, comment}`) |
| `PUT` | `/api/store/apps/:id/reviews` | Update review (upsert) |
| `DELETE` | `/api/store/apps/:id/reviews` | Delete own review |

Rules: one review per user per app, author cannot review own app, `avgRating` and `reviewCount` recalculated on every write.

### My Apps -- author CRUD (authenticated)

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/my/apps` | List my apps |
| `POST` | `/api/my/apps` | Create app (`{name, displayName, description, category, icon, color}`) |
| `GET` | `/api/my/apps/:id` | Get my app (full detail) |
| `PUT` | `/api/my/apps/:id` | Update metadata (partial update, all fields optional) |
| `DELETE` | `/api/my/apps/:id` | Delete app + its reviews |
| `POST` | `/api/my/apps/:id/publish` | Validate and set visibility to public |
| `PATCH` | `/api/my/apps/:id/visibility` | Set visibility (`{visibility}`) |

### Sharing (authenticated)

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/my/apps/:id/share` | Share with user (`{userId}`) |
| `POST` | `/api/my/apps/:id/share-link` | Generate token-based share link |
| `GET` | `/api/my/apps/:id/shares` | List shares for app |
| `DELETE` | `/api/my/apps/:id/shares/:shareId` | Revoke share |

### Tools within a store app (authenticated)

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/my/apps/:id/tools` | Add inline JS tool |
| `PUT` | `/api/my/apps/:id/tools/:name` | Update tool |
| `DELETE` | `/api/my/apps/:id/tools/:name` | Delete tool |

### Prompts within a store app (authenticated)

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/my/apps/:id/prompts` | Add inline prompt |
| `PUT` | `/api/my/apps/:id/prompts/:name` | Update prompt |
| `DELETE` | `/api/my/apps/:id/prompts/:name` | Delete prompt |

---

## App lifecycle

```
Create (private)
  -> author tests it
  -> optionally share with users or generate a link
  -> publish to store (visibility -> public)
  -> community browses, installs, reviews
  -> author pushes updates (bump version)
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

## Visibility

| Visibility | In store search | Direct link works | Who can access |
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

### StoreUser

| Field | Type | Notes |
|---|---|---|
| `id` | string | `su-` prefix |
| `name` | string | |
| `email` | string | Unique |
| `token` | string | 32-byte hex bearer token |
| `createdAt` | time | |

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
| `color` | string | Hex gradient start |
| `category` | string | One of the fixed categories |
| `tags` | []string | Free-form search tags |
| `authorId` | string | Creator's StoreUser ID |
| `authorName` | string | Denormalized for listing |
| `workspaceId` | string | |
| `visibility` | string | `private`, `shared`, `unlisted`, `public` |
| `permissions` | object | `{allowedHosts, defaultToolClass, secrets}` |
| `settings` | []SettingDef | `{key, label, type, required, default}` |
| `tools` | []StoreTool | Inline tool definitions |
| `prompts` | []StorePrompt | Inline prompt definitions |
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
| `mode` | string | `""` or `"qa"` |
| `params` | map | `{paramName: {type, required, description}}` |
| `script` | string | JavaScript source (must define `handle(params)`) |

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
| `invitedBy` | string | StoreUser ID |
| `inviteeId` | string | Specific user (empty for link-based) |
| `token` | string | For link-based sharing |
| `createdAt` | time | |
| `expiresAt` | time | Optional expiry |

---

## Make targets

```
make server        # nube-server on :8090 (includes store)
make start         # nube-server + fakeserver
make stop          # stop all
make build         # build all binaries
```
