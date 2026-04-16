---
name: create_flow
description: Create nodes and connect them with edges to build automation flows
arguments:
  - name: description
    description: "Describe the flow you want to build"
    required: true
---

You are helping the user create an automation flow in Rubix.

The user wants: {{description}}

## How to Create Nodes

Call `rubix.nodes_create` with:
```json
{
  "orgId": "<orgId>",
  "deviceId": "<deviceId>",
  "requestBody": {
    "type": "core.counter",
    "name": "My Counter",
    "position": {"x": 100, "y": 200},
    "settings": {},
    "refs": [
      {"refName": "parentRef", "toNodeId": "<deviceId>"}
    ]
  }
}
```

**IMPORTANT:** Every node needs a `parentRef`. Use the deviceId if placing at the top level.

## How to Connect Nodes (Edges)

Call `rubix.edges_create` with:
```json
{
  "orgId": "<orgId>",
  "deviceId": "<deviceId>",
  "requestBody": {
    "sourceNode": "<nodeId>",
    "sourcePort": "out",
    "targetNode": "<otherNodeId>",
    "targetPort": "in"
  }
}
```

## Bulk Create (Preferred for Multiple Nodes)

Call `rubix.nodes_bulk-create` to create multiple nodes and edges in one request:
```json
{
  "requestBody": {
    "nodes": [
      {"type": "core.trigger", "name": "Start", "position": {"x": 100, "y": 100}},
      {"type": "core.counter", "name": "Counter", "position": {"x": 300, "y": 100}}
    ],
    "edges": [
      {"sourceNode": "$0", "sourcePort": "out", "targetNode": "$1", "targetPort": "in"}
    ]
  }
}
```

## Steps

1. First call `rubix.pallet_list` to check available node types
2. Ask the user to confirm which node types to use if ambiguous
3. Create the nodes (bulk if multiple)
4. Connect them with edges
5. Verify by calling `rubix.runtime_status` or querying the created nodes

## Common Node Types

- `core.trigger` — manual trigger button
- `core.counter` — counts input triggers
- `core.math` — math operations
- `core.compare` — value comparison
- `core.switch` — conditional routing
- `core.delay` — time delay
- `flow.function` — custom logic
- `ui.container` — organizational folder
