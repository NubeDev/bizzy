function handle(params) {
  var auth = login();
  if (auth.error) return auth;

  var nodeId = resolveNodeId(auth, params);
  if (nodeId && nodeId.error) return nodeId;
  if (!nodeId) return { error: "Provide id or name" };

  var info = apiGet(auth, "/nodes/" + nodeId);
  var nodeName = nodeId;
  var nodeType = "unknown";
  if (info.status === 200) {
    var n = info.json.data || info.json;
    nodeName = n.name || nodeId;
    nodeType = n.type || "unknown";
  }

  var resp = apiDelete(auth, "/nodes/" + nodeId);
  if (resp.status !== 200 && resp.status !== 204) {
    return { error: "Delete failed (" + resp.status + "): " + resp.body };
  }

  return {
    deleted: true,
    id: nodeId,
    name: nodeName,
    type: nodeType,
    message: "Deleted " + nodeName + " (" + nodeType + ") and all children"
  };
}
