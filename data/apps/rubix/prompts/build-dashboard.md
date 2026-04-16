---
name: build_dashboard
description: Create dashboard pages with widgets, templates, and data queries for the Rubix scene builder
arguments:
  - name: description
    description: "What dashboard to build (e.g. 'AHU overview with temperature and status', 'site energy dashboard')"
    required: true
---

You are helping the user build a dashboard in Rubix.

The user wants: {{description}}

## Dashboard Architecture

```
ui.page
  ui.tab (optional, for multi-tab pages)
    ui.template (layout container)
      ui.widget (individual components — charts, gauges, stats)
```

## Step 1: Create a Page

Call `rubix.pages_create`:
```json
{
  "orgId": "<orgId>",
  "deviceId": "<deviceId>",
  "requestBody": {
    "type": "ui.page",
    "name": "AHU Overview",
    "settings": {
      "route": "/buildings/{buildingId}/ahu",
      "title": "AHU Overview - {buildingName}"
    }
  }
}
```

## Step 2: Create Widgets

Call `rubix.widgets_create`:
```json
{
  "orgId": "<orgId>",
  "deviceId": "<deviceId>",
  "requestBody": {
    "type": "ui.widget",
    "name": "Temperature",
    "data": {
      "widgetType": "stat",
      "query": "equipRef is {{context.equipId}} and i has \"temp\" and i has \"point\"",
      "aggregation": "last",
      "unit": "degC"
    }
  }
}
```

**Common widget types:**
- `stat` — single value display
- `chart` — time-series line/bar chart
- `gauge` — circular gauge
- `timeline` — event timeline
- `alerts-table` — alarm list
- `button` — action trigger
- `input` — value input
- `schedule` — schedule viewer
- `site-detail` — site info card

## Step 3: Attach Page to a Node

Call `rubix.nodes_pages_attach` to link the page to an equipment or site node so it appears in navigation.

## Context Variables

Widgets support context-driven queries using template variables:
- `{{context.nodeId}}` — current node from navigation
- `{{context.siteId}}` — site context
- `{{context.equipId}}` — equipment context
- `{{route.org}}` — org from URL
- `{{route.device}}` ��� device from URL
- `{{r.equipRef}}` — ref from scene builder connections

## Step 4: Resolve and Test

Call `rubix.pages_resolve` to test that the page renders with data:
```json
{
  "orgId": "<orgId>",
  "deviceId": "<deviceId>",
  "pageId": "<pageId>",
  "context": {"equipId": "ahu-1"}
}
```

## Steps

1. Ask the user what data they want to display
2. Query existing nodes to understand what's available
3. Create the page, template, and widgets
4. Attach the page to the appropriate node
5. Test with `pages_resolve`
