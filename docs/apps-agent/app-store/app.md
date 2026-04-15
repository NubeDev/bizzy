# App Store: App Model

## Overview

Today an app is a directory on disk managed by the server operator. The app store changes this: **any user can create an app, control who sees it, and publish it for the community**.

The on-disk `apps/` directory becomes one source of apps (system apps shipped with the server). User-created apps are stored in the database and served identically to system apps through the same install flow and MCP pipeline.

---

## App data model (extended)

The existing `App` struct in `pkg/apps/types.go` describes on-disk system apps. User-created apps use a new `StoreApp` model stored in `store_apps.json`:

```go
type StoreApp struct {
    ID          string       `json:"id"`          // "app-<random>"
    Name        string       `json:"name"`        // unique slug: "my-cool-tool"
    DisplayName string       `json:"displayName"` // "My Cool Tool"
    Description string       `json:"description"` // short — shown in cards
    LongDesc    string       `json:"longDescription"` // markdown — shown on detail page
    Version     string       `json:"version"`     // semver: "1.0.0"
    Icon        string       `json:"icon"`        // lucide icon name: "terminal", "megaphone"
    Color       string       `json:"color"`       // hex gradient start: "#71D5E3"
    Category    string       `json:"category"`    // one of the defined categories
    Tags        []string     `json:"tags"`        // free-form tags for search

    // Ownership
    AuthorID    string       `json:"authorId"`    // user ID of the creator
    AuthorName  string       `json:"authorName"`  // display name (denormalised for listing)
    WorkspaceID string       `json:"workspaceId"` // workspace the author belongs to

    // Visibility
    Visibility  Visibility   `json:"visibility"`  // private | shared | unlisted | public

    // Content — same structure as system apps
    Permissions Permissions  `json:"permissions"`
    Settings    []SettingDef `json:"settings"`
    Tools       []StoreTool  `json:"tools"`       // inline tool definitions
    Prompts     []StorePrompt `json:"prompts"`    // inline prompt definitions

    // Stats (updated on events)
    InstallCount  int        `json:"installCount"`
    ActiveInstalls int       `json:"activeInstalls"` // current enabled installs
    AvgRating     float64    `json:"avgRating"`      // 0.0–5.0
    ReviewCount   int        `json:"reviewCount"`

    // Timestamps
    CreatedAt   time.Time    `json:"createdAt"`
    UpdatedAt   time.Time    `json:"updatedAt"`
    PublishedAt *time.Time   `json:"publishedAt,omitempty"` // set when first made public
}
```

### StoreTool — inline tool definition

System apps store tools as `.js` + `.json` files on disk. Store apps store the same data inline in the database:

```go
type StoreTool struct {
    Name        string                `json:"name"`
    Description string                `json:"description"`
    ToolClass   string                `json:"toolClass"`   // read-only | read-write | destructive
    Mode        string                `json:"mode"`        // "" | "qa"
    Params      map[string]ToolParam  `json:"params"`
    Script      string                `json:"script"`      // JS source code
}

type ToolParam struct {
    Type        string `json:"type"`
    Required    bool   `json:"required"`
    Description string `json:"description"`
}
```

### StorePrompt — inline prompt definition

```go
type StorePrompt struct {
    Name        string           `json:"name"`
    Description string           `json:"description"`
    Arguments   []PromptArgument `json:"arguments,omitempty"`
    Body        string           `json:"body"` // markdown template with {{key}} substitution
}
```

---

## Visibility

```go
type Visibility string

const (
    VisibilityPrivate  Visibility = "private"   // only the author can see and use it
    VisibilityShared   Visibility = "shared"     // author + invited users
    VisibilityUnlisted Visibility = "unlisted"   // anyone with the link, not in search
    VisibilityPublic   Visibility = "public"     // listed in the store, searchable
)
```

| Visibility | In store search | Direct link works | Who can install |
|---|---|---|---|
| **private** | No | No (403) | Only the author |
| **shared** | No | Yes (with invite token) | Author + invited users |
| **unlisted** | No | Yes | Anyone with the link |
| **public** | Yes | Yes | Anyone |

### Share invites (for `shared` visibility)

```go
type AppShare struct {
    ID        string    `json:"id"`        // "share-<random>"
    AppID     string    `json:"appId"`
    InvitedBy string    `json:"invitedBy"` // user ID
    InviteeID string    `json:"inviteeId"` // specific user — or empty for link-based
    Token     string    `json:"token"`     // for link-based sharing
    CreatedAt time.Time `json:"createdAt"`
    ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}
```

Two ways to share:
1. **By user** — `POST /store/apps/{id}/share {userId: "usr-xxx"}` — adds a specific user
2. **By link** — `POST /store/apps/{id}/share-link` — generates a token URL: `/store/apps/{id}?invite=<token>`

---

## Categories

Fixed set for MVP. Keeps the store organised without needing admin curation:

```go
var Categories = []string{
    "iot-devices",    // IoT, sensors, BMS, controllers
    "analytics",      // Data analysis, reporting, dashboards
    "devops",         // CI/CD, deployment, infrastructure
    "marketing",      // Content, campaigns, SEO
    "design",         // UI/UX, design systems, assets
    "utilities",      // General-purpose helpers
    "integrations",   // Third-party API connectors
    "automation",     // Workflows, scheduling, triggers
}
```

---

## Reviews

```go
type AppReview struct {
    ID        string    `json:"id"`
    AppID     string    `json:"appId"`
    UserID    string    `json:"userId"`
    UserName  string    `json:"userName"`  // denormalised
    Rating    int       `json:"rating"`    // 1–5
    Comment   string    `json:"comment"`   // optional text
    CreatedAt time.Time `json:"createdAt"`
    UpdatedAt time.Time `json:"updatedAt"`
}
```

Rules:
- One review per user per app (upsert on second review)
- Author cannot review their own app
- `avgRating` and `reviewCount` on StoreApp updated on every review write
- Reviews are public (visible on the app detail page)

---

## Relationship to system apps

| Aspect | System apps (`apps/` on disk) | Store apps (database) |
|---|---|---|
| Created by | Server operator | Any user |
| Storage | Files on disk | `store_apps.json` |
| Visibility | Always visible to all users | Controlled by author |
| Editable at runtime | No (reload from disk) | Yes (API/UI) |
| Install flow | Same | Same |
| MCP integration | Same | Same |

The `AppRegistry` is extended to serve both sources. When building a user's MCP session, system apps and store apps are merged — tools and prompts work identically regardless of origin.

---

## App lifecycle

```
Create (private)
  → author uses it, tests it
  → optionally share with specific users or generate a link
  → publish to the store (visibility → public)
  → community installs, reviews
  → author pushes updates (bump version)
  → existing installs flagged as stale, users prompted to update
```

### Publishing requirements

Before an app can be set to `public`, it must have:
- `displayName` (non-empty)
- `description` (min 20 chars)
- `category` (from the fixed list)
- At least one tool or prompt
- No `allowedHosts` pointing to localhost/private IPs (security check)

These are enforced by the `POST /store/apps/{id}/publish` endpoint. Private and shared apps skip these checks — they're for personal use.

---

## Storage

```
data/
  store_apps.json       # [{StoreApp}]
  app_shares.json       # [{AppShare}]
  app_reviews.json      # [{AppReview}]
  app_installs.json     # [{AppInstall}] — existing, unchanged
```

All use the existing `jsondb.Collection[T]` pattern. No schema changes to AppInstall — it already has `appName` which maps to `StoreApp.Name`.
