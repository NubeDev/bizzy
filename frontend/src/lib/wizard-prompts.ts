/**
 * AI prompt templates for the visual app builder wizard.
 * These prompts ask the AI to generate json-render specs alongside tool definitions,
 * enabling live UI previews before code generation.
 */

/** Available json-render component types for the AI to use */
const COMPONENT_LIST = `Card, Stack, Grid, Table, Heading, Text, Badge, Alert, Input, Select, Checkbox, Switch, Slider, Button, Tabs, Accordion, Separator, Image, Link, Progress`

/**
 * Step 1 → 2: Generate a visual plan with json-render specs per tool.
 * The AI returns tool schemas + inputSpec + outputSpec + sampleOutput.
 */
export const VISUAL_PLAN_PROMPT = `You are an AI app builder for NubeIO. The user will describe an app they want. Generate a structured plan as a single JSON code block.

IMPORTANT: Respond with ONLY a \`\`\`json code block, no other text before or after it.

The JSON must have this exact structure:
\`\`\`json
{
  "name": "app-slug-name",
  "displayName": "Human Readable Name",
  "description": "A clear description of what this app does (at least 20 characters)",
  "category": "utilities",
  "tools": [
    {
      "name": "tool_name",
      "description": "What this tool does",
      "toolClass": "read-only",
      "params": {
        "param_name": { "type": "string", "required": true, "description": "What this param is" }
      },
      "inputSpec": {
        "root": "form",
        "elements": {
          "form": { "type": "Card", "props": { "title": "Tool Title" }, "children": ["fields"] },
          "fields": { "type": "Stack", "props": { "direction": "vertical", "gap": "md" }, "children": ["param_name"] },
          "param_name": { "type": "Input", "props": { "label": "Param Name", "placeholder": "example" } }
        }
      },
      "sampleOutput": { "key": "realistic sample value" },
      "outputSpec": {
        "root": "result",
        "elements": {
          "result": { "type": "Card", "props": { "title": "Result" }, "children": ["content"] },
          "content": { "type": "Table", "props": { "columns": [{"key":"k","header":"H"}], "rows": [{"k":"v"}] } }
        }
      }
    }
  ],
  "prompts": [
    {
      "name": "prompt_name",
      "description": "What this prompt does",
      "arguments": [
        { "name": "arg_name", "description": "What this arg is", "required": true }
      ]
    }
  ]
}
\`\`\`

For each tool, generate:
- "inputSpec": a json-render spec that renders the tool's input form. Use appropriate components for each param type.
- "outputSpec": a json-render spec that renders the expected output with realistic sample data baked into the spec props.
- "sampleOutput": example JSON output with realistic values matching the outputSpec.

Available json-render component types: ${COMPONENT_LIST}

Rules for specs:
- Every spec needs a "root" key pointing to the root element id, and an "elements" map.
- Elements with children use a "children" array of element ids.
- Use Card as the outer wrapper, Stack for layout, Input/Select/Checkbox for form fields.
- For output, prefer Table for tabular data, Card+Text for key-value, and Badge/Alert for status.
- Use realistic sample data (real city names, plausible numbers, actual currencies, etc).
- For Select components, provide an "options" array of {label, value} objects.

Categories: iot-devices, analytics, devops, marketing, design, utilities, integrations, automation.
Tool classes: read-only, read-write, destructive.
Generate 2-5 tools and 0-2 prompts. Keep names lowercase with underscores.
The "name" field should be a lowercase slug with hyphens.

User's app idea:
`

/**
 * Prompt for refining a single tool's visual spec via chat.
 * Used when the user says "make it a card instead of a table" etc.
 */
export const REFINE_TOOL_PROMPT = `You are helping refine the visual design of a tool in a NubeIO app.

The user wants to change how this tool looks. You will receive the current tool definition (with inputSpec, outputSpec, sampleOutput) and the user's requested change.

Respond with ONLY a \`\`\`json code block containing the updated tool object (same structure, with the requested changes applied to the specs).

Available json-render component types: ${COMPONENT_LIST}

Keep the tool name, description, params, and toolClass unchanged unless the user explicitly asks to change them. Only modify the specs as requested.

Current tool:
`

/**
 * Prompt for adding new tools to an existing plan.
 * Used when the user asks for additional tools (e.g. "add a tool that does X").
 */
export const ADD_TOOLS_PROMPT = `You are an AI app builder for NubeIO. The user has an existing app plan and wants to add more tools.

IMPORTANT: Respond with ONLY a \`\`\`json code block containing an array of new tool objects. Do NOT include tools that already exist.

Each tool must have the same structure:
\`\`\`json
[
  {
    "name": "tool_name",
    "description": "What this tool does",
    "toolClass": "read-only",
    "params": { "param_name": { "type": "string", "required": true, "description": "desc" } },
    "inputSpec": { "root": "form", "elements": { ... } },
    "sampleOutput": { ... },
    "outputSpec": { "root": "result", "elements": { ... } }
  }
]
\`\`\`

Available json-render component types: ${COMPONENT_LIST}

Rules for specs:
- Every spec needs "root" and "elements".
- Elements with children use a "children" array of element ids.
- Use Card as wrapper, Stack for layout, Input/Select/Checkbox for form fields.
- For output, use Table for tabular data, Card+Text for key-value, Badge/Alert for status.
- Use realistic sample data.

Existing tools in the plan:
`

/**
 * Step 3: Generate tool implementations (same as before but aware of specs).
 */
export const GENERATE_PROMPT = `You are an AI app builder for NubeIO. Generate the JavaScript implementation for each tool below.

IMPORTANT: Respond with ONLY JSON code blocks, one per tool, using \`\`\`json:tool markers. No other text.

For each tool, output:
\`\`\`json:tool
{
  "name": "tool_name",
  "description": "...",
  "toolClass": "...",
  "params": { ... },
  "script": "function handle(params) { ... }"
}
\`\`\`

The JS runtime APIs:
- http.get(url), http.post(url, body), http.put(url, body), http.delete(url) — returns {status, body, headers}
- secrets.get(key), config.get(key) — read user settings
- log.info(msg), log.warn(msg), log.error(msg)

Keep scripts concise and practical. Each must define a handle(params) function.

Here are the tools to implement:
`
