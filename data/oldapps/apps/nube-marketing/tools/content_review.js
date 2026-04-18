// content_review.js — reads the prompt template and renders it with the given content.
function handle(params) {
  var template = files.read("prompts/content-review.md");

  // Strip YAML frontmatter (everything between the --- markers).
  var parts = template.split("---");
  var body = parts.slice(2).join("---").trim();

  // Substitute arguments.
  body = body.replace(/\{\{content\}\}/g, params.content || "");

  return { prompt: body };
}
