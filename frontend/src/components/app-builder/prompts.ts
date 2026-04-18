import type { AppProject } from "./types"

/** Default prompts always included in the app builder context. */
export const DEFAULT_BUILDER_PROMPTS = ["app_builder", "tool_naming", "ui_reference"]

/** Extra prompts the user can toggle on to give the AI more context. */
export const OPTIONAL_BUILDER_PROMPTS = [
  { name: "api_guide", label: "API Reference" },
  { name: "plugin_system", label: "Plugin System" },
  { name: "app_development", label: "App Development" },
  { name: "overview", label: "Platform Overview" },
]

/**
 * Build the system prompt for the App Architect AI.
 *
 * Static reference material (JS runtime, UI patterns, tool naming rules, etc.)
 * comes from the backend bootstrap prompts via the `compose` helper.
 * Only the dynamic project-state section is assembled here.
 *
 * @param compose  – function returned by useBootstrapPrompts().compose
 * @param project  – current project state (file list)
 * @param extraPrompts – additional backend prompt names the user selected
 */
export function buildArchitectPrompt(
  compose: (names: string[]) => string,
  project: AppProject,
  extraPrompts: string[] = [],
): string {
  const fileList = project.files.map(f => `  ${f.path} (${f.type})`).join("\n")

  const reference = compose([...DEFAULT_BUILDER_PROMPTS, ...extraPrompts])

  return `${reference}

---

## Current Project State
${fileList || "  (empty project)"}
`
}

/** Extract file blocks from AI response */
export function extractFileBlocks(content: string): { path: string; content: string; type: string }[] {
  const blocks: { path: string; content: string; type: string }[] = []

  // Match ```type:path blocks
  const regex = /```(\w+):([^\n]+)\n([\s\S]*?)```/g
  let match
  while ((match = regex.exec(content)) !== null) {
    const type = match[1]
    const path = match[2].trim()
    const fileContent = match[3].trim()

    // Validate it's a recognized file type
    if (["yaml", "js", "json", "md", "tsx"].includes(type)) {
      blocks.push({ path, content: fileContent, type })
    }
  }

  return blocks
}
