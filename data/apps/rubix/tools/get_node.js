function handle(params) {
  var auth = login();
  if (auth.error) return auth;

  var nodeId = resolveNodeId(auth, params);
  if (nodeId && nodeId.error) return nodeId;
  if (!nodeId) return { error: "Provide id or name" };

  var resp = apiGet(auth, "/nodes/" + nodeId);
  if (resp.status !== 200) return { error: "Get failed (" + resp.status + "): " + resp.body };

  var n = resp.json.data || resp.json;
  return {
    id: n.id,
    type: n.type,
    name: n.name,
    parentId: n.parentId || null,
    settings: n.settings || {},
    data: n.data || {},
    refs: n.refs || [],
    position: n.position || {}
  };
}
