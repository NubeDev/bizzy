---
name: tagging
description: Manage tags and identity markers on Rubix nodes for classification and fast querying
arguments:
  - name: task
    description: "What to tag or how to organize (e.g. 'tag sensors on floor 2', 'add HVAC identity to AHU nodes')"
    required: true
---

You are helping the user manage tags on Rubix nodes.

The user wants: {{task}}

## Two Tag Systems

### 1. Identity Tags (Recommended for Filtering)

Identity tags are marker tags optimized for fast queries (5-10x faster than regular tags). Use these for classification like `sensor`, `temp`, `equip`, `point`, `hvac`.

**Add identity tag:**
Call `rubix.identity_create`:
```json
{
  "orgId": "<orgId>",
  "deviceId": "<deviceId>",
  "nodeId": "<nodeId>",
  "requestBody": {"tag": "sensor"}
}
```

**List identity tags on a node:**
Call `rubix.identity_list` with orgId, deviceId, nodeId.

**Remove identity tag:**
Call `rubix.identity_delete` with orgId, deviceId, nodeId, and the tag name.

**Query by identity (Haystack):**
- `i has "sensor"` — nodes with this tag
- `i has ["temp", "humid"]` — any of these
- `i hasAll ["temp", "sensor", "point"]` — all of these
- `i exists` — nodes with any identity tag
- `i isEmpty` — nodes without identity tags

### 2. Regular Tags (Key-Value Metadata)

Regular tags support key-value pairs. Use for metadata like `unit: "degC"`, `floor: "2"`.

**Add tag:**
Call `rubix.tags_create`:
```json
{
  "orgId": "<orgId>",
  "deviceId": "<deviceId>",
  "nodeId": "<nodeId>",
  "requestBody": {"tagName": "unit", "tagValue": "degC"}
}
```

## Common Tag Patterns (Haystack Convention)

**Classification markers (identity):** `site`, `building`, `floor`, `equip`, `point`, `sensor`, `actuator`, `command`

**Measurement types:** `temp`, `humidity`, `pressure`, `flow`, `energy`, `power`, `co2`

**Equipment types:** `ahu`, `vav`, `chiller`, `boiler`, `pump`, `fan`

## Steps

1. First query the target nodes using `rubix.query_filter`
2. Confirm with the user which nodes to tag
3. Apply tags (identity for classification, regular for key-value metadata)
4. Verify by querying with the new tags
