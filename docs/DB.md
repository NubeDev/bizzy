# Database

Bizzy uses **SQLite** (via GORM) as its database. The database file lives at `data/bizzy.db`.

## Tables

| Table | What it stores |
|---|---|
| `users` | User accounts, tokens, preferences |
| `workspaces` | Multi-tenant workspaces |
| `sessions` | AI session history (prompt, result, cost, tool calls) |
| `app_installs` | Per-user app installations with settings/secrets |
| `store_apps` | App store catalog (metadata, tools, prompts, ratings) |
| `app_reviews` | User reviews and ratings |
| `app_shares` | Share invites and link tokens |
| `workflow_runs` | Workflow execution state and stage results |
| `provider_configs` | Global AI provider settings (single row) |

## Key indexes

- `sessions.user_id` + `sessions.created_at` — fast session listing per user
- `app_installs.user_id` + `app_installs.app_name` — install lookups
- `store_apps.name` (unique) — app name deduplication
- `store_apps.author_id` + `store_apps.category` — browsing/filtering
- `users.token` — auth token lookup

## How it works

- Schema is auto-migrated on startup via `database.Open()` in `pkg/database/`
- SQLite runs in WAL mode with busy timeout for concurrent access
- Complex fields (JSON arrays/maps like `Tags`, `Settings`, `ToolCallLog`) are stored using GORM's `serializer:json`
- App definitions on disk (`data/apps/`) are **not** in the database — the disk registry is the source of truth for app code. The `store_apps` table holds catalog metadata only

## Migration from JSON

On first startup after the upgrade, `database.Open()` checks for legacy JSON files (`users.json`, `sessions.json`, etc.). If a table is empty and the corresponding JSON file exists, it bulk-imports the records and renames the file to `.json.bak`. This is a one-time migration.
