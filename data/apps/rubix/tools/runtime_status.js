function handle(params) {
  var auth = login();
  if (auth.error) return auth;

  var resp = apiGet(auth, "/runtime/status");
  if (resp.status !== 200) return { error: "Failed (" + resp.status + "): " + resp.body };

  var d = resp.json.data;
  return {
    orgId: d.orgId,
    deviceId: auth.deviceId,
    status: d.status,
    nodeCount: d.nodeCount,
    edgeCount: d.edgeCount,
    uptime: d.uptime
  };
}
