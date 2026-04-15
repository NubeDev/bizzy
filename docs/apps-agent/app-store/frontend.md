# App Store: Frontend

## Overview

The store is a **React SPA** using Tailwind CSS and shadcn/ui. It is served from the Go binary as a static embed (`web/store/dist/`) and accessed at `/store`. It shares the same auth model as the existing API — bearer token from login.

The Flutter app (nube_agent) remains the primary mobile/desktop experience for **using** skills (chat, QA flows, sessions). The React store is the experience for **discovering, creating, and managing** apps. Both apps talk to the same Go server.

---

## Tech stack

| Layer | Choice | Why |
|---|---|---|
| Framework | React 19 + Vite | Fast build, simple, works with Go embed |
| Styling | Tailwind CSS 4 | Utility-first, matches shadcn/ui |
| Components | shadcn/ui | Polished, accessible, copy-paste — no heavy dependency |
| Routing | React Router 7 | Standard, simple |
| State | TanStack Query (React Query) | Server state caching, pagination, mutations |
| Icons | Lucide React | Same icon set as the Flutter app |
| Markdown | react-markdown + remark-gfm | For app long descriptions and prompt previews |
| Code editor | Monaco Editor (react) | For editing JS tool scripts inline |

### Project location

```
web/
  store/
    package.json
    vite.config.ts
    tsconfig.json
    tailwind.config.ts
    src/
      main.tsx
      app.tsx
      index.css                  # tailwind directives
      lib/
        api.ts                   # API client (fetch wrapper with auth)
        types.ts                 # TypeScript types matching Go models
        utils.ts                 # cn(), formatDate, etc.
      hooks/
        use-auth.ts              # auth context + token management
        use-store.ts             # TanStack Query hooks for store endpoints
        use-my-apps.ts           # TanStack Query hooks for /my/apps
      components/
        ui/                      # shadcn/ui components (button, card, input, dialog, etc.)
        layout/
          app-shell.tsx          # sidebar + content layout
          nav.tsx                # navigation items
        store/
          app-card.tsx           # app card for grid/list views
          app-grid.tsx           # responsive grid of app cards
          category-pills.tsx     # horizontal scrolling category filter
          search-bar.tsx         # search input with debounce
          featured-carousel.tsx  # featured apps hero section
          rating-stars.tsx       # star display + input
          install-dialog.tsx     # settings form + install confirmation
          review-card.tsx        # single review display
          review-form.tsx        # write/edit review
          visibility-badge.tsx   # private/shared/unlisted/public badge
        my-apps/
          app-editor.tsx         # full app editor (metadata + tools + prompts)
          tool-editor.tsx        # JS tool editor with Monaco
          prompt-editor.tsx      # markdown prompt editor
          settings-builder.tsx   # drag-and-drop settings schema builder
          publish-checklist.tsx  # requirements checklist before publishing
          share-dialog.tsx       # sharing controls (users, links)
          visibility-select.tsx  # visibility dropdown
      pages/
        store-home.tsx           # browse page: featured + search + categories
        app-detail.tsx           # single app detail + install + reviews
        my-apps.tsx              # list of apps I created
        app-editor.tsx           # create/edit an app
        login.tsx                # login page
```

---

## Pages

### 1. Store Home (`/store`)

The main discovery page. Think VS Code Marketplace.

```
┌─────────────────────────────────────────────────────────┐
│  ┌─ Sidebar ──────┐  ┌─ Content ────────────────────┐  │
│  │                 │  │                              │  │
│  │  🏪 Store       │  │  ┌────────────────────────┐  │  │
│  │  📦 My Apps     │  │  │  🔍 Search apps...     │  │  │
│  │                 │  │  └────────────────────────┘  │  │
│  │  ── Categories  │  │                              │  │
│  │  IoT & Devices  │  │  ✨ Featured                 │  │
│  │  Analytics      │  │  ┌────────┐┌────────┐┌────┐  │  │
│  │  DevOps         │  │  │ Rubix  ││ Energy ││UX  │  │  │
│  │  Marketing      │  │  │ Dev    ││ Report ││Pro │  │  │
│  │  Design         │  │  │ ★4.8   ││ ★4.5   ││★4.9│  │  │
│  │  Utilities      │  │  │ ⬇ 340  ││ ⬇ 210  ││⬇180│  │  │
│  │  Integrations   │  │  └────────┘└────────┘└────┘  │  │
│  │  Automation     │  │                              │  │
│  │                 │  │  🔥 Popular                   │  │
│  │  ── Account     │  │  ┌──────────────────────────┐│  │
│  │  Settings       │  │  │ MQTT Monitor   12 tools  ││  │
│  │  Logout         │  │  │ by IoT-Team    ★4.7 ⬇340 ││  │
│  │                 │  │  ├──────────────────────────┤│  │
│  │                 │  │  │ BMS Analytics  8 tools   ││  │
│  │                 │  │  │ by NubeIO     ★4.5 ⬇210  ││  │
│  │                 │  │  └──────────────────────────┘│  │
│  │                 │  │                              │  │
│  │                 │  │  🕐 Recently Added            │  │
│  │                 │  │  [grid of app cards]         │  │
│  │                 │  │                              │  │
│  └─────────────────┘  └──────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
```

**Sections:**
1. **Search bar** — debounced, triggers `GET /store/apps?q=...`
2. **Featured carousel** — 3-5 curated apps as large cards with gradients, horizontally scrollable
3. **Category pills** — horizontal scroll of category badges, acts as filter
4. **Popular** — sorted by `installCount`, top 6
5. **Recently added** — sorted by `publishedAt` desc, grid of cards

**Responsive:**
- Desktop: sidebar (240px) + content
- Tablet: sidebar collapses to icons
- Mobile: bottom nav, full-width content

### 2. App Detail (`/store/apps/:id`)

The full app listing page — install, review, see what's included.

```
┌──────────────────────────────────────────────────────┐
│  ← Back to Store                                      │
│                                                        │
│  ┌──────┐                                              │
│  │ icon │  Rubix Developer                             │
│  │      │  by NubeIO  •  v1.2.0                        │
│  └──────┘  ★★★★★ 4.8 (23 reviews)  •  340 installs    │
│                                                        │
│  ┌──────────────┐  ┌─────────────┐                     │
│  │   Install     │  │ View Source │                     │
│  └──────────────┘  └─────────────┘                     │
│                                                        │
│  ┌─ Tabs ──────────────────────────────────────────┐   │
│  │  [Overview]  [Tools & Prompts]  [Reviews]       │   │
│  ├──────────────────────────────────────────────────┤   │
│  │                                                  │   │
│  │  Overview tab:                                   │   │
│  │  - Long description (rendered markdown)          │   │
│  │  - Category badge, tags                          │   │
│  │  - Settings required (form preview)              │   │
│  │                                                  │   │
│  │  Tools & Prompts tab:                            │   │
│  │  - List of tools with name, description, class   │   │
│  │  - List of prompts with name, description        │   │
│  │  - QA flow badges on interactive tools           │   │
│  │                                                  │   │
│  │  Reviews tab:                                    │   │
│  │  - Rating breakdown (5★: 18, 4★: 3, ...)        │   │
│  │  - Review cards (user, stars, comment, date)     │   │
│  │  - "Write a review" form (if installed)          │   │
│  │                                                  │   │
│  └──────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────┘
```

**Install dialog** (triggered by Install button):
- If the app has no settings: one-click install, done
- If settings are required: slide-up dialog with form fields generated from the app's `settings` schema
- Each field shows label, type hint, required badge
- Secret fields use password input
- On submit: `POST /store/apps/:id/install` — success toast, button changes to "Installed"

**Already installed state:**
- Button shows "Installed" with a checkmark
- Dropdown menu: "Open in App" (deep link to Flutter), "Settings", "Uninstall"

### 3. My Apps (`/store/my-apps`)

Dashboard for apps the user has created.

```
┌──────────────────────────────────────────────────────┐
│  My Apps                           [+ Create App]     │
│                                                        │
│  ┌─ Filter: [All] [Private] [Public] [Shared] ──────┐ │
│                                                        │
│  ┌──────────────────────────────────────────────────┐  │
│  │  🟢 My Cool Tool               v1.0.0            │  │
│  │  Does cool things                                 │  │
│  │  private • 0 installs • no reviews                │  │
│  │                                    [Edit] [···]   │  │
│  ├──────────────────────────────────────────────────┤  │
│  │  🟣 Content Reviewer            v2.1.0            │  │
│  │  AI-powered content review                        │  │
│  │  public • 45 installs • ★4.6 (12)                 │  │
│  │                                    [Edit] [···]   │  │
│  ├──────────────────────────────────────────────────┤  │
│  │  🔵 Team Dashboard              v1.0.0            │  │
│  │  Internal team metrics                            │  │
│  │  shared • 3 users                                 │  │
│  │                                    [Edit] [···]   │  │
│  └──────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────┘
```

**"..." menu per app:** Share, Publish, Duplicate, Delete

**Empty state:** Illustration + "Create your first app" CTA with a brief explanation of what apps can do.

### 4. App Editor (`/store/my-apps/:id/edit`)

The full authoring experience. Tabbed interface for managing all aspects of an app.

```
┌──────────────────────────────────────────────────────┐
│  ← My Apps          My Cool Tool        [Save] [···] │
│                                                        │
│  ┌─ Tabs ──────────────────────────────────────────┐   │
│  │  [Details]  [Tools]  [Prompts]  [Settings]      │   │
│  │  [Permissions]  [Publish]                        │   │
│  ├──────────────────────────────────────────────────┤   │
│  │                                                  │   │
│  │  Details tab:                                    │   │
│  │  ┌─────────────────────────────────┐             │   │
│  │  │ Display Name: [My Cool Tool   ]│             │   │
│  │  │ Slug:         my-cool-tool      │             │   │
│  │  │ Description:  [Does cool...   ]│             │   │
│  │  │ Long Desc:    [markdown editor ]│             │   │
│  │  │ Category:     [Utilities    ▼] │             │   │
│  │  │ Icon:         [sparkles     ▼] │             │   │
│  │  │ Color:        [#FBBF24  🎨   ] │             │   │
│  │  │ Tags:         [tag1] [tag2] [+]│             │   │
│  │  │ Version:      [1.0.0         ] │             │   │
│  │  └─────────────────────────────────┘             │   │
│  │                                                  │   │
│  │  Tools tab:                                      │   │
│  │  ┌────────────────────────────────┐  [+ Add Tool]│   │
│  │  │ check_status (read-only)       │              │   │
│  │  │ > Monaco editor with JS source │              │   │
│  │  │ > Params: device_id (string)   │              │   │
│  │  │                    [Test] [Del] │              │   │
│  │  └────────────────────────────────┘              │   │
│  │                                                  │   │
│  │  Prompts tab:                                    │   │
│  │  ┌────────────────────────────────┐ [+ Add Prompt]│  │
│  │  │ setup_guide                    │              │   │
│  │  │ > Markdown editor              │              │   │
│  │  │ > Arguments: product (required)│              │   │
│  │  │                         [Del]  │              │   │
│  │  └────────────────────────────────┘              │   │
│  │                                                  │   │
│  │  Settings tab:                                   │   │
│  │  What users fill in when installing your app.    │   │
│  │  ┌──────────────────────────────────┐ [+ Add]    │   │
│  │  │ rubix_host  url     required     │            │   │
│  │  │ rubix_token secret  required     │            │   │
│  │  └──────────────────────────────────┘            │   │
│  │                                                  │   │
│  │  Permissions tab:                                │   │
│  │  Allowed hosts: [*.nubedge.com] [+]              │   │
│  │  Default tool class: [read-write ▼]              │   │
│  │                                                  │   │
│  │  Publish tab:                                    │   │
│  │  ┌──────────────────────────────────┐            │   │
│  │  │ ✅ Display name set              │            │   │
│  │  │ ✅ Description (42 chars)        │            │   │
│  │  │ ✅ Category selected             │            │   │
│  │  │ ✅ Has 1 tool                    │            │   │
│  │  │ ✅ No localhost in allowedHosts  │            │   │
│  │  │                                  │            │   │
│  │  │ Current: private                 │            │   │
│  │  │ [Publish to Store]               │            │   │
│  │  └──────────────────────────────────┘            │   │
│  │                                                  │   │
│  └──────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────┘
```

**Key interactions:**

- **Tool editor**: Monaco editor for JS code with syntax highlighting. Params are configured via a form below the editor. "Test" button calls the tool with sample params and shows the result inline.
- **Prompt editor**: Markdown textarea with preview toggle. Arguments defined via a simple list.
- **Settings builder**: Add/remove/reorder setting fields. Each field has key, label, type (dropdown), required toggle, default value.
- **Publish checklist**: Real-time validation. Green checkmarks when requirements are met. "Publish to Store" button is disabled until all checks pass.
- **Auto-save**: Debounced save on every change (PUT /my/apps/:id). No explicit save button needed, but show a "Saved" indicator.

### 5. Login (`/store/login`)

Minimal login page — same auth as the Flutter app:

- Server URL input
- Bearer token input
- "Connect" button → validates via `GET /health` + `GET /users/me`
- Stores credentials in localStorage
- Redirects to `/store` on success

---

## Components

### AppCard

The primary building block for listings. Two variants:

**Grid card** (store home):
```
┌───────────────────────┐
│  ┌────┐               │
│  │icon│  App Name      │
│  └────┘  by Author     │
│                        │
│  Short description     │
│  goes here...          │
│                        │
│  ★4.8  ⬇ 340  🔧 10   │
│  [IoT & Devices]       │
└───────────────────────┘
```

**List card** (search results, my apps):
```
┌──────────────────────────────────────────┐
│  ┌────┐  App Name  v1.0.0        ★4.8   │
│  │icon│  Short description       ⬇ 340  │
│  └────┘  [category] [tag] [tag]  🔧 10   │
└──────────────────────────────────────────┘
```

### InstallDialog

shadcn/ui `Dialog` that generates a form from the app's settings schema:

```tsx
<Dialog>
  <DialogContent>
    <DialogHeader>
      <DialogTitle>Install {app.displayName}</DialogTitle>
      <DialogDescription>Configure the settings for this app.</DialogDescription>
    </DialogHeader>

    {app.settings.map(setting => (
      <div key={setting.key}>
        <Label>{setting.label} {setting.required && <Badge>Required</Badge>}</Label>
        <Input
          type={setting.type === 'secret' ? 'password' : 'text'}
          placeholder={setting.default}
        />
      </div>
    ))}

    <DialogFooter>
      <Button variant="outline">Cancel</Button>
      <Button onClick={install}>Install</Button>
    </DialogFooter>
  </DialogContent>
</Dialog>
```

### RatingStars

Interactive star component for reviews:
- Display mode: filled/half/empty stars based on rating
- Input mode: hover highlights, click sets rating
- Shows numeric rating + count: "★4.8 (23)"

### VisibilityBadge

Color-coded badge for app visibility status:
- private: gray, lock icon
- shared: blue, users icon
- unlisted: amber, link icon
- public: green, globe icon

---

## API client

Thin fetch wrapper with auth injection:

```typescript
// lib/api.ts
const API_BASE = ''; // same-origin, proxied by Go server

class ApiClient {
  private token: string;

  constructor(token: string) {
    this.token = token;
  }

  private async request<T>(path: string, options?: RequestInit): Promise<T> {
    const res = await fetch(`${API_BASE}${path}`, {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${this.token}`,
        ...options?.headers,
      },
    });
    if (!res.ok) {
      const body = await res.json().catch(() => ({}));
      throw new ApiError(res.status, body.error || res.statusText);
    }
    return res.json();
  }

  // Store
  storeApps(params: StoreQuery) { return this.request<StoreListResponse>(`/store/apps?${qs(params)}`); }
  storeApp(id: string) { return this.request<StoreDetailResponse>(`/store/apps/${id}`); }
  storeCategories() { return this.request<string[]>('/store/categories'); }
  storeFeatured() { return this.request<StoreApp[]>('/store/featured'); }
  installStoreApp(id: string, settings: Record<string, string>) {
    return this.request<AppInstall>(`/store/apps/${id}/install`, {
      method: 'POST', body: JSON.stringify({ settings }),
    });
  }

  // Reviews
  appReviews(appId: string) { return this.request<AppReview[]>(`/store/apps/${appId}/reviews`); }
  submitReview(appId: string, rating: number, comment: string) {
    return this.request<AppReview>(`/store/apps/${appId}/reviews`, {
      method: 'POST', body: JSON.stringify({ rating, comment }),
    });
  }

  // My Apps
  myApps() { return this.request<StoreApp[]>('/my/apps'); }
  createApp(data: CreateAppRequest) {
    return this.request<StoreApp>('/my/apps', { method: 'POST', body: JSON.stringify(data) });
  }
  updateApp(id: string, data: Partial<StoreApp>) {
    return this.request<StoreApp>(`/my/apps/${id}`, { method: 'PUT', body: JSON.stringify(data) });
  }
  deleteApp(id: string) {
    return this.request<void>(`/my/apps/${id}`, { method: 'DELETE' });
  }
  publishApp(id: string) {
    return this.request<StoreApp>(`/my/apps/${id}/publish`, { method: 'POST' });
  }

  // Tools & Prompts within my app
  addTool(appId: string, tool: StoreTool) {
    return this.request<StoreTool>(`/my/apps/${appId}/tools`, { method: 'POST', body: JSON.stringify(tool) });
  }
  updateTool(appId: string, name: string, tool: StoreTool) {
    return this.request<StoreTool>(`/my/apps/${appId}/tools/${name}`, { method: 'PUT', body: JSON.stringify(tool) });
  }
  deleteTool(appId: string, name: string) {
    return this.request<void>(`/my/apps/${appId}/tools/${name}`, { method: 'DELETE' });
  }

  // Sharing
  shareApp(appId: string, userId: string) {
    return this.request<AppShare>(`/my/apps/${appId}/share`, { method: 'POST', body: JSON.stringify({ userId }) });
  }
  createShareLink(appId: string) {
    return this.request<AppShare>(`/my/apps/${appId}/share-link`, { method: 'POST' });
  }
}
```

---

## TanStack Query hooks

```typescript
// hooks/use-store.ts
export function useStoreApps(params: StoreQuery) {
  return useQuery({
    queryKey: ['store', 'apps', params],
    queryFn: () => api.storeApps(params),
    keepPreviousData: true, // smooth pagination
  });
}

export function useStoreApp(id: string) {
  return useQuery({
    queryKey: ['store', 'app', id],
    queryFn: () => api.storeApp(id),
  });
}

export function useFeatured() {
  return useQuery({
    queryKey: ['store', 'featured'],
    queryFn: () => api.storeFeatured(),
    staleTime: 5 * 60 * 1000, // featured changes rarely
  });
}

export function useInstallApp() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, settings }: { id: string; settings: Record<string, string> }) =>
      api.installStoreApp(id, settings),
    onSuccess: (_, { id }) => {
      qc.invalidateQueries(['store', 'app', id]); // refresh installed status
      toast.success('App installed');
    },
  });
}
```

---

## Serving from Go

The React app is built to `web/store/dist/` and embedded in the Go binary:

```go
//go:embed web/store/dist
var storeFS embed.FS

func (a *API) setupStoreRoutes(r *gin.Engine) {
    // Serve static files
    store, _ := fs.Sub(storeFS, "web/store/dist")
    r.StaticFS("/store/assets", http.FS(store))

    // SPA fallback — serve index.html for all /store/* routes
    r.NoRoute(func(c *gin.Context) {
        if strings.HasPrefix(c.Request.URL.Path, "/store") {
            c.FileFromFS("index.html", http.FS(store))
            return
        }
        c.JSON(404, gin.H{"error": "not found"})
    })
}
```

Vite config sets `base: '/store/'` so all asset paths are relative to the store mount point.

---

## Dark theme

Tailwind CSS config uses CSS variables matching the Flutter app's `RubixTokens` design system:

```css
/* index.css */
@layer base {
  :root {
    --background: 220 20% 10%;
    --foreground: 210 20% 92%;
    --card: 220 18% 13%;
    --card-foreground: 210 20% 92%;
    --primary: 174 60% 55%;       /* matches RubixTokens.accentCool */
    --primary-foreground: 0 0% 0%;
    --secondary: 220 15% 18%;
    --muted: 220 15% 22%;
    --muted-foreground: 215 15% 55%;
    --border: 220 15% 20%;
    --ring: 174 60% 55%;
    --destructive: 0 72% 51%;
  }
}
```

Dark-only for MVP — matches the Flutter app. Light mode is a future enhancement.

---

## Mobile responsiveness

| Breakpoint | Layout |
|---|---|
| `< 640px` (sm) | Single column, bottom sheet dialogs, stacked cards |
| `640–1024px` (md) | 2-column card grid, collapsible sidebar |
| `> 1024px` (lg) | 3-column card grid, persistent sidebar |

The store home uses a responsive grid:
```tsx
<div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
  {apps.map(app => <AppCard key={app.id} app={app} />)}
</div>
```

---

## Build & deploy

```bash
# Development
cd web/store
npm install
npm run dev          # Vite dev server on :5173, proxies /api to Go server

# Production
npm run build        # outputs to dist/
# Go binary embeds dist/ and serves at /store
```

`vite.config.ts` proxy for development:
```typescript
export default defineConfig({
  base: '/store/',
  server: {
    proxy: {
      '/store/apps': 'http://localhost:8090',
      '/my': 'http://localhost:8090',
      '/admin': 'http://localhost:8090',
      '/users': 'http://localhost:8090',
      '/health': 'http://localhost:8090',
    },
  },
});
```

---

## Implementation order

1. **Scaffold** — Vite + React + Tailwind + shadcn/ui + Router + TanStack Query
2. **Login page** — auth flow, token storage
3. **Store home** — search + category filter + app cards (read-only browsing)
4. **App detail page** — full listing + install dialog
5. **My Apps list** — view created apps with status
6. **App editor** — create + edit (details tab first, then tools, then prompts)
7. **Tool editor** — Monaco integration for JS editing
8. **Publish flow** — checklist + publish button
9. **Reviews** — display on detail page + write form
10. **Sharing** — share dialog, invite links
11. **Featured carousel** — admin-curated section on store home
