function handle(params) {
  var auth = login();
  if (auth.error) return auth;

  if (!params.filter) return { error: "filter is required" };

  var filter = params.filter;
  if (params.select) filter = filter + " | select " + params.select;

  var resp = apiPost(auth, "/query", {
    filter: filter,
    limit: params.limit || 50,
    runtime: params.runtime !== false
  });

  if (resp.status !== 200) return { error: "Query failed (" + resp.status + "): " + resp.body };

  var data = resp.json.data || [];
  var nodes = [];
  for (var i = 0; i < data.length; i++) {
    var n = data[i];
    var node = { id: n.id, type: n.type, name: n.name };
    if (n.parentId) node.parentId = n.parentId;
    nodes.push(node);
  }

  return {
    count: nodes.length,
    total: (resp.json.meta || {}).total || nodes.length,
    nodes: nodes
  };
}
