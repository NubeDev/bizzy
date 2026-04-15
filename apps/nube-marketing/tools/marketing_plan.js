// marketing_plan.js — reads the prompt template and renders it with the given arguments.
function handle(params) {
  var template = files.read("prompts/marketing-plan.md");

  // Strip YAML frontmatter (everything between the --- markers).
  var parts = template.split("---");
  var body = parts.slice(2).join("---").trim();

  // Substitute arguments.
  body = body.replace(/\{\{product\}\}/g, params.product || "");
  body = body.replace(/\{\{audience\}\}/g, params.audience || "B2B IoT and building automation buyers");

  return { prompt: body };
}
