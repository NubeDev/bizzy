function handle(params) {
  var auth = login();
  if (auth.error) return auth;

  if (!params.type) return { error: "type is required (e.g. 'core.trigger')" };
  if (!params.name) return { error: "name is required" };

  var parentId = resolveParentId(auth, params);
  if (parentId && parentId.error) return parentId;

  var refs = [{ refName: "parentRef", toNodeId: parentId }];
  if (params.siteRef) refs.push({ refName: "siteRef", toNodeId: params.siteRef });

  var body = {
    type: params.type,
    name: params.name,
    position: { x: params.x || 100, y: params.y || 100 },
    refs: refs
  };

  var settings = parseSettings(params.settings);
  if (settings && settings.error) return settings;
  if (settings) body.settings = settings;

  var resp = apiPost(auth, "/nodes", body);
  if (resp.status !== 200 && resp.status !== 201) {
    return { error: "Create failed (" + resp.status + "): " + resp.body };
  }

  var node = resp.json.data || resp.json;

  if (params.tags) addIdentityTags(auth, node.id, params.tags);

  return {
    id: node.id,
    type: node.type,
    name: node.name,
    parentId: parentId,
    tags: params.tags || null,
    message: "Created " + node.name + " (" + node.id + ")"
  };
}
