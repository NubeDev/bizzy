---
name: navigation
description: Build site navigation hierarchy — sites, buildings, floors with containers and nav links
arguments:
  - name: structure
    description: "Describe the navigation structure (e.g. 'Sydney office with 3 floors', 'multi-site with 2 buildings each')"
    required: true
---

You are helping the user build a navigation hierarchy in Rubix.

The user wants: {{structure}}

## Navigation Architecture

Rubix navigation is built from `ui.container` nodes in a hierarchy:

```
ui.container (Site: "Sydney Office")    [tags: site]
  ui.container (Building: "Tower A")   [tags: building]
    ui.container (Floor: "Level 1")    [tags: floor]
    ui.container (Floor: "Level 2")    [tags: floor]
```

## How To Build It

Use `rubix.create_node` for everything. It handles auth, parentRef, and tags automatically.

### Create site (at device root):
```json
{"type": "ui.container", "name": "Sydney Office", "tags": "site", "settings": "{\"containerType\": \"site\", \"geoCity\": \"Sydney\"}"}
```

### Create building (under site — use parentName to search):
```json
{"type": "ui.container", "name": "Tower A", "parentName": "Sydney Office", "tags": "building", "siteRef": "<site_id>"}
```

### Create floor (under building):
```json
{"type": "ui.container", "name": "Level 1", "parentName": "Tower A", "tags": "floor", "siteRef": "<site_id>"}
```

## Verify

Call `rubix.query_nodes` with filter `i has "site"` or `i has "building"` to see what was created.

## Steps

1. Parse the user's desired structure
2. Create site container with `rubix.create_node` (tags: "site")
3. Note the site ID from the response
4. Create building containers under the site (parentName, tags: "building", siteRef)
5. Create floor containers under buildings (parentName, tags: "floor", siteRef)
6. Verify with `rubix.query_nodes`
