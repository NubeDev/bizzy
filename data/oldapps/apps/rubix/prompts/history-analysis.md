---
name: history_analysis
description: Query and analyze historical time-series data from Rubix nodes
arguments:
  - name: question
    description: "What history data to analyze (e.g. 'temperature last 24 hours', 'energy consumption this week')"
    required: true
---

You are helping the user analyze historical data from Rubix.

The user wants: {{question}}

## How to Query History

Call `rubix.query_history` with:
```json
{
  "orgId": "<orgId>",
  "deviceId": "<deviceId>",
  "requestBody": {
    "filter": "i has \"temp\" and i has \"sensor\"",
    "portHandle": "output",
    "range": "last24h",
    "limit": 50
  }
}
```

### Required Parameters

- `orgId` (path) — Organization ID
- `deviceId` (path) — Device ID
- `requestBody.filter` (body, **required**) — Haystack filter query (see Query Syntax below)

### Optional Body Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `portHandle` | string | `"out"` | Port handle to query history for |
| `limit` | integer | `60` | Max samples per node |
| `range` | string | — | Relative time range (e.g., `last1h`, `last24h`, `last7d`, `last30d`) |
| `from` | string | — | Absolute start (RFC3339 or `YYYY-MM-DD HH:MM:SS`). Requires `to` |
| `to` | string | — | Absolute end (RFC3339 or `YYYY-MM-DD HH:MM:SS`). Requires `from` |
| `timezone` | string | — | IANA timezone (e.g., `Asia/Bangkok`, `America/New_York`) |
| `dateFormat` | string | `rfc3339` | One of: `rfc3339`, `iso8601`, `unix`, `custom` |
| `dateFormatCustom` | string | — | Custom Go time format (when `dateFormat` is `custom`) |
| `withOverride` | boolean | `false` | Include override info (isOverridden, overrideValue, lastValue) |
| `withMeta` | boolean | `false` | Include unit metadata and display values |
| `withAlarms` | boolean | `false` | Include alarm events near sample timestamps |
| `alarmWindow` | integer | `30` | Seconds (±) to search for alarm events |

### Example: Absolute Date Range

```json
{
  "orgId": "test",
  "deviceId": "dev_01A12A00AB0C",
  "requestBody": {
    "filter": "type is \"core.trigger\"",
    "portHandle": "output",
    "from": "2026-04-15T00:00:00Z",
    "to": "2026-04-16T00:00:00Z",
    "limit": 100
  }
}
```

### Example: Relative Range

```json
{
  "orgId": "test",
  "deviceId": "dev_01A12A00AB0C",
  "requestBody": {
    "filter": "i has \"sensor\"",
    "range": "last7d",
    "limit": 200
  }
}
```

## Date Range Shortcuts

Use the `range` field for relative queries:
- `last1h` — Last hour
- `last24h` — Last 24 hours
- `last7d` — Last 7 days
- `last30d` — Last 30 days

Or use `from`/`to` for absolute ranges with ISO 8601 timestamps.

## Multi-Port Queries

Use pipe select in the filter to get multiple ports:
```
i has "ahu" and i has "point" | select temperature, humidity, pressure
```

## Auto-Downsampling

The system auto-downsamples to ~360 points:
- 1 hour range → 10s resolution
- 24 hours → 5m resolution
- 7 days → 1h resolution
- 30 days → 6h resolution
- >30 days → 1d resolution

## Query Syntax Quick Reference

### Filter by node fields
- `type is "core.trigger"` — by node type
- `id is "node_001"` — by node ID
- `name contains "Supply"` — by name substring

### Filter by identity tags
- `i has "sensor"` — has tag
- `i has ["temp", "humid"]` — has any (OR)
- `i hasAll ["temp", "sensor"]` — has all (AND)

### Filter by refs
- `siteRef is "node_001"` — at site
- `equipRef is "node_002"` — in equipment
- `parentRef is "node_003"` — direct children

### Combine with logical operators
- `type is "sensor" and i has "temp"`
- `i has "temp" or i has "humid"`
- `(i has "temp" or i has "humid") and siteRef is "site_1"`

### Pipe modifiers
- `i has "sensor" | select out, in1` — select specific ports
- `type is "device" | limit 50` — limit results
- `type is "device" | sort name asc` — sort results

## Response Format

```json
{
  "data": {
    "nodes": [
      {
        "id": "node_id",
        "name": "Node Name",
        "portHandle": "output",
        "samples": [
          {
            "timestamp": "2026-04-15T00:00:00Z",
            "value": 22.5,
            "type": "number"
          }
        ]
      }
    ],
    "totalNodes": 1,
    "totalSamples": 100
  }
}
```

## Steps

1. First check what history-enabled nodes exist using `rubix.histories_list-enabled-nodes`
2. Optionally inspect a node's ports with `rubix.histories_get-node-ports` to find the right `portHandle`
3. Build the appropriate filter and date range
4. Call `rubix.query_history` with `requestBody` containing `filter`, `portHandle`, and time range
5. Present results as a summary with key statistics (min, max, avg, trends)
6. Highlight any anomalies or notable patterns

## Checking History Config

To see if a node has history enabled:
- `rubix.histories_get-node-ports` — shows all ports and their history settings
- `rubix.histories_update-port-config` — enable/disable history on a port
- `rubix.histories_get-stats` — history manager stats (buffer size, flush count)
- `rubix.histories_diagnostics` — comprehensive diagnostics for a specific node/port
