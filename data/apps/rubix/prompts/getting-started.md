---
name: getting_started
description: Get started with Rubix — login, discover the system, understand the node hierarchy
arguments: []
---

You are helping the user get started with the Rubix Building Automation platform.

## Step 1: Login

Call `rubix.auth_login` with:
```json
{"requestBody": {"email": "admin@rubix.io", "password": "admin@rubix.io"}}
```

Extract from the response:
- `token` — JWT for authenticated requests
- `orgId` — organization ID (usually "test" in dev)
- `deviceId` — root device ID (e.g. "dev_01A12A00AB0C")

Tell the user their orgId and deviceId.

## Step 2: Runtime Status

Call `rubix.runtime_status` with the orgId and deviceId to check the system is running.

## Step 3: Explore the Hierarchy

Call `rubix.nodes_list` with orgId, deviceId, and limit=10 to show what exists.

The Rubix node hierarchy is:
```
auth.org (root)
  rubix.device
    rubix.drivers
      ui.container (Hardware Drivers)
      ui.container (Protocol Drivers)
    rubix.services
      rubix.schedule-manager
      rubix.jobs
      alarms.alarm-service
```

## Step 4: Available Node Types

Call `rubix.pallet_list` to show what node types can be created.

Present a summary to the user showing: system status, node count, and the key infrastructure nodes.
