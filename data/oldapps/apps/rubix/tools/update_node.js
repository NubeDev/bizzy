function handle(params) {
  var auth = login();
  if (auth.error) return auth;

  var nodeId = resolveNodeId(auth, params);
  if (nodeId && nodeId.error) return nodeId;
  if (!nodeId) return { error: "Provide id or name" };

  var body = {};
  if (params.newName) body.name = params.newName;
  if (params.x !== undefined || params.y !== undefined) {
    body.position = {};
    if (params.x !== undefined) body.position.x = params.x;
    if (params.y !== undefined) body.position.y = params.y;
  }

  if (Object.keys(body).length > 0) {
    var resp = apiPut(auth, "/nodes/" + nodeId, body);
    if (resp.status !== 200) return { error: "Update failed (" + resp.status + "): " + resp.body };
  }

  if (params.settings) {
    var settings = parseSettings(params.settings);
    if (settings && settings.error) return settings;
    var resp2 = apiPatch(auth, "/nodes/" + nodeId + "/settings", settings);
    if (resp2.status !== 200) return { error: "Settings update failed (" + resp2.status + "): " + resp2.body };
  }

  return { id: nodeId, message: "Node updated" };
}
