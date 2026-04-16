// Shared helpers for all rubix tools.
// This file is auto-loaded before every tool in this app.

function login() {
  var resp = http.post(
    config.rubix_host + "/api/v1/auth/login",
    { email: "admin@rubix.io", password: "admin@rubix.io" },
    { headers: { "Content-Type": "application/json" } }
  );
  if (resp.status !== 200) return { error: "Login failed: " + resp.body };
  var d = resp.json.data;
  return { token: d.token, orgId: d.orgId, deviceId: d.deviceId };
}

function apiGet(auth, path) {
  return http.get(
    config.rubix_host + "/api/v1/orgs/" + auth.orgId + "/devices/" + auth.deviceId + path,
    { headers: { "Authorization": "Bearer " + auth.token } }
  );
}

function apiPost(auth, path, body) {
  return http.post(
    config.rubix_host + "/api/v1/orgs/" + auth.orgId + "/devices/" + auth.deviceId + path,
    body,
    { headers: { "Authorization": "Bearer " + auth.token, "Content-Type": "application/json" } }
  );
}

function apiPut(auth, path, body) {
  return http.put(
    config.rubix_host + "/api/v1/orgs/" + auth.orgId + "/devices/" + auth.deviceId + path,
    body,
    { headers: { "Authorization": "Bearer " + auth.token, "Content-Type": "application/json" } }
  );
}

function apiPatch(auth, path, body) {
  return http.patch(
    config.rubix_host + "/api/v1/orgs/" + auth.orgId + "/devices/" + auth.deviceId + path,
    body,
    { headers: { "Authorization": "Bearer " + auth.token, "Content-Type": "application/json" } }
  );
}

function apiDelete(auth, path) {
  return http.delete(
    config.rubix_host + "/api/v1/orgs/" + auth.orgId + "/devices/" + auth.deviceId + path,
    { headers: { "Authorization": "Bearer " + auth.token } }
  );
}

function resolveNodeId(auth, params) {
  if (params.id) return params.id;
  if (!params.name) return null;

  var resp = apiPost(auth, "/query", {
    filter: "name like \"%" + params.name + "%\"",
    limit: 5
  });
  if (resp.status !== 200) return { error: "Search failed: " + resp.body };

  var nodes = resp.json.data || [];
  if (nodes.length === 0) return { error: "No node found matching '" + params.name + "'" };
  if (nodes.length === 1) return nodes[0].id;

  var matches = [];
  for (var i = 0; i < nodes.length; i++) {
    matches.push({ id: nodes[i].id, name: nodes[i].name, type: nodes[i].type });
  }
  return { error: "Multiple matches — pass id for exact lookup", matches: matches };
}

function resolveParentId(auth, params) {
  if (params.parentId) return params.parentId;
  if (!params.parentName) return auth.deviceId;

  var resp = apiPost(auth, "/query", {
    filter: "name like \"%" + params.parentName + "%\"",
    limit: 5
  });
  if (resp.status !== 200) return { error: "Parent search failed: " + resp.body };

  var nodes = resp.json.data || [];
  if (nodes.length === 0) return { error: "No parent found matching '" + params.parentName + "'" };
  if (nodes.length === 1) return nodes[0].id;

  var matches = [];
  for (var i = 0; i < nodes.length; i++) {
    matches.push({ id: nodes[i].id, name: nodes[i].name, type: nodes[i].type });
  }
  return { error: "Multiple parents match — pass parentId", matches: matches };
}

function addIdentityTags(auth, nodeId, tagsCSV) {
  if (!tagsCSV) return;
  var tags = tagsCSV.split(",");
  for (var i = 0; i < tags.length; i++) {
    var tag = tags[i].trim();
    if (!tag) continue;
    apiPost(auth, "/nodes/" + nodeId + "/tags/identity", { tag: tag });
  }
}

function parseSettings(settingsStr) {
  if (!settingsStr) return null;
  try {
    return JSON.parse(settingsStr);
  } catch (e) {
    return { error: "Invalid settings JSON: " + e.message };
  }
}
