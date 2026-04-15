# App Store: Backend API

## Overview

The Go server gets a new `store` API group. It extends the existing `pkg/api/` and `pkg/apps/` packages — no new services, no new binaries. The store endpoints sit alongside the existing app catalog and install endpoints.

---

## New API endpoints

### Store — public catalog

```
GET    /store/apps                              # browse public apps (paginated, filterable)
GET    /store/apps/:id                          # public app detail (description, tools, reviews, stats)
GET    /store/categories                        # list available categories
GET    /store/featured                          # curated featured apps (admin-managed list)
GET    /store/apps/:id/reviews                  # list reviews for an app
```

### Store — authenticated actions

```
POST   /store/apps/:id/install                  # install a store app (validates settings)
POST   /store/apps/:id/reviews                  # submit a review (1-5 stars + comment)
PUT    /store/apps/:id/reviews                  # update own review
DELETE /store/apps/:id/reviews                  # delete own review
```

### My Apps — author CRUD

```
GET    /my/apps                                 # list apps I created (all visibilities)
POST   /my/apps                                 # create a new app (starts as private)
GET    /my/apps/:id                             # get my app with full detail
PUT    /my/apps/:id                             # update app metadata, tools, prompts
DELETE /my/apps/:id                             # delete app (must have 0 active installs from other users)

POST   /my/apps/:id/publish                     # set visibility to public (validates requirements)
PATCH  /my/apps/:id/visibility                  # change visibility (private/shared/unlisted/public)

POST   /my/apps/:id/tools                       # add a tool to my app
PUT    /my/apps/:id/tools/:name                  # update a tool
DELETE /my/apps/:id/tools/:name                  # remove a tool

POST   /my/apps/:id/prompts                      # add a prompt
PUT    /my/apps/:id/prompts/:name                # update a prompt
DELETE /my/apps/:id/prompts/:name                # remove a prompt
```

### Sharing

```
POST   /my/apps/:id/share                       # share with a specific user {userId}
DELETE /my/apps/:id/share/:shareId               # revoke a share
GET    /my/apps/:id/shares                       # list who I've shared with
POST   /my/apps/:id/share-link                   # generate a share link (returns token)
DELETE /my/apps/:id/share-link/:token             # revoke a share link
```

### Admin — store management

```
GET    /admin/store/apps                         # list all store apps (any visibility)
DELETE /admin/store/apps/:id                     # remove any app (moderation)
PUT    /admin/store/featured                      # set featured app IDs list
DELETE /admin/store/apps/:id/reviews/:reviewId   # remove a review (moderation)
```

---

## Endpoint details

### `GET /store/apps` — browse the store

Public apps, paginated, with filters and sorting.

**Query params:**

| Param | Type | Default | Description |
|---|---|---|---|
| `q` | string | — | Full-text search (name, displayName, description, tags) |
| `category` | string | — | Filter by category slug |
| `sort` | string | `popular` | `popular` (installs), `recent` (publishedAt), `rating` (avgRating), `name` (alpha) |
| `page` | int | 1 | Page number |
| `limit` | int | 20 | Items per page (max 50) |

**Response:**

```json
{
  "apps": [
    {
      "id": "app-a1b2c3",
      "name": "rubix-monitor",
      "displayName": "Rubix Monitor",
      "description": "Real-time device monitoring and alerting",
      "version": "1.2.0",
      "icon": "activity",
      "color": "#34D399",
      "category": "iot-devices",
      "tags": ["rubix", "monitoring", "alerts"],
      "authorName": "NubeIO",
      "installCount": 340,
      "avgRating": 4.8,
      "reviewCount": 23,
      "toolCount": 6,
      "promptCount": 2,
      "publishedAt": "2026-03-01T10:00:00Z"
    }
  ],
  "total": 42,
  "page": 1,
  "limit": 20
}
```

The listing response is a **summary** — it omits tools, prompts, settings schema, and longDescription to keep payloads small. The detail endpoint returns everything.

### `GET /store/apps/:id` — app detail

Returns the full app with tools, prompts, settings schema, and reviews summary. Used by the detail page.

**Response:**

```json
{
  "app": {
    "id": "app-a1b2c3",
    "name": "rubix-monitor",
    "displayName": "Rubix Monitor",
    "description": "Real-time device monitoring and alerting",
    "longDescription": "## Rubix Monitor\n\nFull markdown description...",
    "version": "1.2.0",
    "icon": "activity",
    "color": "#34D399",
    "category": "iot-devices",
    "tags": ["rubix", "monitoring"],
    "authorId": "usr-abc123",
    "authorName": "NubeIO",
    "visibility": "public",
    "installCount": 340,
    "activeInstalls": 280,
    "avgRating": 4.8,
    "reviewCount": 23,
    "tools": [
      {
        "name": "check_status",
        "description": "Check device online/offline status",
        "toolClass": "read-only",
        "mode": "",
        "params": {
          "device_id": {"type": "string", "required": true, "description": "Device ID"}
        }
      }
    ],
    "prompts": [
      {
        "name": "monitor_setup",
        "description": "Guide to setting up monitoring",
        "arguments": []
      }
    ],
    "settings": [
      {"key": "rubix_host", "label": "Rubix Host URL", "type": "url", "required": true},
      {"key": "rubix_token", "label": "API Token", "type": "secret", "required": true}
    ],
    "permissions": {
      "allowedHosts": ["*.nubedge.com"],
      "defaultToolClass": "read-only"
    },
    "createdAt": "2026-02-15T10:00:00Z",
    "updatedAt": "2026-03-20T14:30:00Z",
    "publishedAt": "2026-03-01T10:00:00Z"
  },
  "installed": false,
  "installId": "",
  "userReview": null
}
```

The `installed`, `installId`, and `userReview` fields are populated from the authenticated user's context — so the UI knows whether to show "Install" or "Installed" and whether the user has reviewed it.

### `POST /my/apps` — create an app

```json
{
  "name": "my-cool-tool",
  "displayName": "My Cool Tool",
  "description": "Does cool things",
  "category": "utilities",
  "icon": "sparkles",
  "color": "#FBBF24"
}
```

- `name` must be unique across all store apps, lowercase, alphanumeric + hyphens
- `name` cannot collide with system app names (checked against the registry)
- App starts as `visibility: private` with no tools or prompts
- Returns the created `StoreApp` with `id`

### `POST /my/apps/:id/publish` — publish to the store

Validates publishing requirements (see [app.md](app.md)):
- displayName, description (min 20 chars), category, at least one tool or prompt
- No localhost/private-IP in allowedHosts
- Sets `visibility: public` and `publishedAt` timestamp
- Returns the updated app

### `POST /store/apps/:id/install` — install a store app

Same flow as the existing `POST /apps/:id/install` but looks up the app from the store database instead of the on-disk registry. The install record uses the same `AppInstall` model — `appName` maps to `StoreApp.Name`.

```json
{
  "settings": {
    "rubix_host": "http://my-server:1616",
    "rubix_token": "my-secret-token"
  }
}
```

On install:
1. Validate required settings from the app's schema
2. Increment `installCount` and `activeInstalls` on the StoreApp
3. Create `AppInstall` record (same as system apps)

On uninstall:
1. Decrement `activeInstalls` (existing `DELETE /app-installs/:id` is extended to handle this)

---

## Integration with existing systems

### App Registry extension

The `AppRegistry` currently scans `apps/` on disk. It needs to also serve store apps:

```go
// Registry.GetAny returns an app from either source (disk or store).
func (r *Registry) GetAny(name string) (*UnifiedApp, bool) {
    // 1. Check on-disk system apps
    if app, ok := r.Get(name); ok {
        return &UnifiedApp{System: app}, true
    }
    // 2. Check store apps database
    if storeApp, ok := r.StoreApps.FindByName(name); ok {
        return &UnifiedApp{Store: storeApp}, true
    }
    return nil, false
}
```

### MCP Factory extension

The `MCPFactory` builds per-user MCP servers. When a user has installed a store app, the factory:
1. Looks up the `StoreApp` by name
2. Registers its tools (JS source from `StoreTool.Script`) and prompts (markdown from `StorePrompt.Body`)
3. Settings/secrets injection works identically — from the user's `AppInstall` record

No changes to the MCP protocol or the tool execution pipeline. Store app tools run in the same Goja sandbox with the same `allowedHosts` enforcement.

### JS Runtime

Store app tools use the same `jsruntime.go` Goja VM. The only difference: instead of reading `.js` from disk, the script source comes from `StoreTool.Script` in the database.

```go
// Existing: script from file
source, _ := os.ReadFile(filepath.Join(app.Dir, "tools", tool.Name+".js"))

// Store app: script from database
source := storeTool.Script
```

Same sandbox, same host API, same timeout, same `allowedHosts` check.

---

## Search implementation

MVP search uses simple substring matching — no external search engine:

```go
func (r *Registry) SearchStoreApps(query, category, sort string, page, limit int) ([]StoreApp, int) {
    q := strings.ToLower(query)
    var results []StoreApp

    for _, app := range r.StoreApps.All() {
        if app.Visibility != VisibilityPublic {
            continue
        }
        if q != "" && !matchesQuery(app, q) {
            continue
        }
        if category != "" && app.Category != category {
            continue
        }
        results = append(results, app)
    }

    sortApps(results, sort)
    total := len(results)
    // Paginate
    start := (page - 1) * limit
    if start >= total { return nil, total }
    end := start + limit
    if end > total { end = total }
    return results[start:end], total
}

func matchesQuery(app StoreApp, q string) bool {
    if strings.Contains(strings.ToLower(app.Name), q) { return true }
    if strings.Contains(strings.ToLower(app.DisplayName), q) { return true }
    if strings.Contains(strings.ToLower(app.Description), q) { return true }
    for _, tag := range app.Tags {
        if strings.Contains(strings.ToLower(tag), q) { return true }
    }
    return false
}
```

This is fine for <1000 apps. If the store grows large, add SQLite full-text search behind the same interface.

---

## Featured apps

A simple admin-managed list of app IDs stored in `data/store_config.json`:

```json
{
  "featured": ["app-abc123", "app-def456", "app-ghi789"]
}
```

`GET /store/featured` returns the full StoreApp objects for these IDs. Admins update the list via `PUT /admin/store/featured {ids: [...]}`.

---

## Access control summary

| Endpoint | Who can call it |
|---|---|
| `GET /store/apps` | Any authenticated user |
| `GET /store/apps/:id` | Any authenticated user (respects visibility) |
| `POST /store/apps/:id/install` | Any authenticated user (if visibility allows) |
| `POST /store/apps/:id/reviews` | Any authenticated user (not the author) |
| `GET/POST/PUT/DELETE /my/apps/*` | Only the app author |
| `GET/PUT/DELETE /admin/store/*` | Admin only |

### Visibility enforcement on `GET /store/apps/:id`

```
if app.Visibility == "public":    allow
if app.Visibility == "unlisted":  allow (they have the ID/link)
if app.Visibility == "shared":    allow if user is author OR has a share record
if app.Visibility == "private":   allow only if user is author
else: 404 (not 403 — don't leak existence)
```

---

## What doesn't change

- **System apps** — `apps/` directory, `app.yaml`, on-disk tools/prompts. These keep working exactly as before. The store is additive.
- **AppInstall model** — unchanged. Both system and store apps create the same install record.
- **MCP protocol** — unchanged. Clients (Claude Code, Claude Desktop, etc.) don't know or care whether a tool came from a system app or store app.
- **Auth model** — bearer tokens, same middleware. No new auth flows.
- **JSON DB** — same `jsondb.Collection[T]` for new collections. No infrastructure changes.
