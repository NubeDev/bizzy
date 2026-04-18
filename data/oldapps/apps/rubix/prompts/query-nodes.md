---
name: query_nodes
description: Query nodes using the Rubix Haystack filter language
arguments:
  - name: filter
    description: "What to search for (e.g. 'all sensors', 'temperature points', 'active alarms')"
    required: true
---

You are helping the user query nodes in Rubix using the Haystack filter language.

The user wants to find: {{filter}}

## How to Query

Call `rubix.query_filter` with orgId, deviceId, and a `filter` string using Haystack syntax.

## Haystack Filter Syntax

**By type:**
- `type is "core.trigger"` — exact type match
- `type in ["core.trigger", "core.counter"]` — multiple types

**By name:**
- `name contains "Supply"` — substring match
- `name like "valve%"` — wildcard

**By identity tags (fast):**
- `i has "sensor"` — has tag
- `i has ["temp", "humid"]` — has any of these tags
- `i hasAll ["temp", "sensor", "point"]` — has all tags

**By refs:**
- `siteRef is "node_001"` — at a specific site
- `equipRef is "ahu_01"` — belongs to equipment
- `parentRef is "dev_123"` — direct children

**By port values:**
- `out.valueNum > 20` — output value above 20
- `state.value is "active"` — string port value

**By alarm state:**
- `i has "alarm" and state.value is "active"` — active alarms
- `severity.value is "Critical"` — critical alarms

**By settings (JSONB):**
- `settings.productCode contains "widget"` — settings field search

**Combining:**
- `type is "sensor" and name contains "Temp"` — AND
- `i has "temp" or i has "humid"` — OR
- `not (status is "inactive")` — NOT
- `(i has "temp" or i has "humid") and siteRef is "site_1"` — grouping

**Modifiers (pipe):**
- `type is "device" | limit 50` — limit results
- `type is "device" | sort name asc` — sort
- `type is "device" | offset 10 | limit 10` — pagination
- `type is "trigger" | select out, in1` — port projection

## Instructions

1. Translate the user's natural language request into a Haystack filter
2. Call `rubix.query_filter` with the filter
3. Present the results in a clear table or list
4. If no results, suggest alternative filters
