/**
 * Tool naming convention rules — shared across all AI prompts.
 * Must match the backend pkg/toolname validation.
 */
export const TOOL_NAMING_RULES = `
## Tool Naming Convention (ENFORCED — the API will reject invalid names)
- Names: lowercase letters, digits, underscores. 2-60 chars. Must start with a letter.
- Regular tools: use plain names like \`check_weather\`, \`get_data\`, \`create_node\`
- QA/interactive tools: MUST end with \`_qa\` suffix and have \`"mode": "qa"\` in the JSON schema. Example: \`travel_quiz_qa\`, \`content_review_qa\`
- The suffix and mode MUST match:
  - Name ends with \`_qa\` → mode must be \`"qa"\`
  - Mode is \`"qa"\` → name must end with \`_qa\`
- Helpers: start with \`_\` (e.g. \`_helpers.js\`) — these are not tools, they're shared code
- INVALID names: \`CheckWeather\` (no uppercase), \`check-weather\` (no hyphens), \`_private\` (reserved prefix), \`a\` (too short)
`.trim()
