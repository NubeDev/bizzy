function handle(params) {
  var auth = login();
  if (auth.error) return auth;

  var nodeId = resolveNodeId(auth, params);
  if (nodeId && nodeId.error) return nodeId;
  if (!nodeId) return { error: "Provide id or name" };

  var settings = parseSettings(params.settings);
  if (!settings) return { error: "settings is required (JSON string)" };
  if (settings.error) return settings;

  var resp = apiPatch(auth, "/nodes/" + nodeId + "/settings", settings);
  if (resp.status !== 200) return { error: "Failed (" + resp.status + "): " + resp.body };

  return { id: nodeId, applied: settings, message: "Settings updated" };
}
