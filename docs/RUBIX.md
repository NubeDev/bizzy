# Rubix in Claude Code

Control your Rubix Building Automation system directly from Claude Code.

## Setup

1. Make sure the Rubix server is running (`make dev` in the rubix repo, default port 9000)
2. Make sure the bizzy server is running (`make server`, port 8090)
3. Install the `rubix` app if not already installed

## Prompts (Slash Commands)

Type `/nube:rubix.` in Claude Code to see available prompts:

| Command | What it does |
|---|---|
| `/nube:rubix.getting_started` | Login, check system status, explore what's running |
| `/nube:rubix.query_nodes` | Find nodes — sensors, devices, equipment, alarms |
| `/nube:rubix.create_flow` | Build automation flows with nodes and edges |
| `/nube:rubix.history_analysis` | Analyze time-series data — temperature, energy, trends |
| `/nube:rubix.tagging` | Tag nodes for fast filtering and classification |
| `/nube:rubix.build_dashboard` | Create dashboard pages with widgets and charts |
| `/nube:rubix.navigation` | Build site/building/floor navigation hierarchy |

## Examples

**Get started:**
```
/nube:rubix.getting_started
```
Claude logs in, shows you the system status, node count, and what's running.

**Find nodes:**
```
/nube:rubix.query_nodes "all temperature sensors"
```
Claude queries using Haystack syntax and shows matching nodes.

**Check history:**
```
/nube:rubix.history_analysis "temperature trends last 24 hours"
```
Claude fetches time-series data and summarizes it.

**Build a dashboard:**
```
/nube:rubix.build_dashboard "AHU overview with temperature and status"
```
Claude creates a page with widgets for your equipment.

## Or Just Ask

You don't need a slash command. Just ask Claude directly:

- "Show me the runtime status"
- "List all nodes of type core.trigger"
- "Create a counter node connected to a trigger"
- "What alarms are active?"
- "Query nodes where name contains Supply"

Claude has access to 163 Rubix API tools and will figure out which ones to call.

## Available Tools (163)

The full Rubix API is available, organized by area:

- **Nodes** — list, create, update, delete, bulk operations, port values
- **Edges** — connect nodes, bulk wiring
- **Query** — Haystack filter language, history, batch queries
- **Runtime** — status, start/stop, live values, pallet (node types)
- **Histories** — time-series data, port config, diagnostics
- **Pages/Widgets/Templates** — dashboard building
- **Tags/Identity** — classification and fast filtering
- **Navigation** — sidebar tree, site hierarchy
- **Auth** — login, user management
