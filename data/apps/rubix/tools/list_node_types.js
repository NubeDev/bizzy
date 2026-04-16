function handle(params) {
  var auth = login();
  if (auth.error) return auth;

  var resp = apiGet(auth, "/runtime/pallet");
  if (resp.status !== 200) return { error: "Failed (" + resp.status + "): " + resp.body };

  var types = resp.json.data.nodeTypes || resp.json.data || [];
  var filtered = [];

  for (var i = 0; i < types.length; i++) {
    var t = types[i];
    if (params.category) {
      var cat = (t.category || t.type.split(".")[0] || "").toLowerCase();
      if (cat !== params.category.toLowerCase()) continue;
    }
    filtered.push({
      type: t.type,
      displayName: t.displayName || "",
      category: t.category || t.type.split(".")[0] || ""
    });
  }

  return { count: filtered.length, totalAvailable: types.length, nodeTypes: filtered };
}
