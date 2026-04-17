import type { Spec } from "@json-render/core"

/**
 * Inspects tool output JSON and generates a json-render Spec for display.
 *
 * Mapping rules:
 * - Array of objects -> Table
 * - Object with "error" key -> Alert (destructive)
 * - Flat key-value object -> Card with key-value rows
 * - Nested objects with arrays -> Tabs with tables
 * - Primitives -> Text
 */
export function outputToSpec(output: unknown): Spec {
  // Null / undefined
  if (output === null || output === undefined) {
    return textSpec("No output")
  }

  // Primitive
  if (typeof output !== "object") {
    return textSpec(String(output))
  }

  // Array of objects -> Table
  if (Array.isArray(output)) {
    if (output.length === 0) return textSpec("Empty array")
    if (typeof output[0] === "object" && output[0] !== null) {
      return tableSpec(output as Record<string, unknown>[])
    }
    // Array of primitives -> simple list
    return listSpec(output)
  }

  const obj = output as Record<string, unknown>

  // Object with "error" key -> Alert
  if ("error" in obj && obj.error) {
    return alertSpec(obj)
  }

  // Check for nested arrays (tabbed layout)
  const nestedArrayKeys = Object.entries(obj).filter(
    ([, v]) => Array.isArray(v) && v.length > 0 && typeof v[0] === "object",
  )
  if (nestedArrayKeys.length > 1) {
    return tabbedSpec(obj, nestedArrayKeys)
  }

  // Single nested array -> card header + table
  if (nestedArrayKeys.length === 1) {
    return nestedSingleArraySpec(obj, nestedArrayKeys[0])
  }

  // Flat key-value -> Card
  return keyValueSpec(obj)
}

function textSpec(text: string): Spec {
  return {
    root: "text",
    elements: {
      text: { type: "Text", props: { text } },
    },
  }
}

function tableSpec(rows: Record<string, unknown>[]): Spec {
  const columns = Object.keys(rows[0]).map((key) => ({
    key,
    header: formatHeader(key),
  }))

  return {
    root: "table",
    elements: {
      table: {
        type: "Table",
        props: {
          columns,
          rows,
        },
      },
    },
  }
}

function listSpec(items: unknown[]): Spec {
  const elements: Record<string, { type: string; props: Record<string, unknown> }> = {}
  const children: string[] = []

  items.forEach((item, i) => {
    const key = `item_${i}`
    elements[key] = { type: "Text", props: { text: String(item) } }
    children.push(key)
  })

  elements["stack"] = {
    type: "Stack",
    props: { direction: "vertical", gap: "sm" },
  }

  return {
    root: "stack",
    elements: {
      ...elements,
      stack: { ...elements["stack"], children } as never,
    },
  }
}

function alertSpec(obj: Record<string, unknown>): Spec {
  const errorMsg = typeof obj.error === "string" ? obj.error : JSON.stringify(obj.error)
  const elements: Record<string, { type: string; props: Record<string, unknown> }> = {
    alert: {
      type: "Alert",
      props: {
        title: "Error",
        description: errorMsg,
        variant: "destructive",
      },
    },
  }

  // Show other fields below the alert
  const otherKeys = Object.keys(obj).filter((k) => k !== "error")
  if (otherKeys.length > 0) {
    const kvObj: Record<string, unknown> = {}
    for (const k of otherKeys) kvObj[k] = obj[k]
    const kvSpec = keyValueSpec(kvObj)
    return {
      root: "stack",
      elements: {
        stack: { type: "Stack", props: { direction: "vertical", gap: "md" }, children: ["alert", "details"] } as never,
        alert: elements.alert,
        details: kvSpec.elements[kvSpec.root],
      },
    }
  }

  return { root: "alert", elements }
}

function tabbedSpec(
  obj: Record<string, unknown>,
  nestedArrayKeys: [string, unknown][],
): Spec {
  const tabs = nestedArrayKeys.map(([key, value]) => {
    const rows = value as Record<string, unknown>[]
    const columns = Object.keys(rows[0]).map((k) => ({
      key: k,
      header: formatHeader(k),
    }))
    return {
      label: formatHeader(key),
      value: key,
      rows,
      columns,
    }
  })

  const elements: Record<string, unknown> = {}
  const tabItems: { label: string; value: string; children: string[] }[] = []

  for (const tab of tabs) {
    const tableKey = `table_${tab.value}`
    elements[tableKey] = {
      type: "Table",
      props: { columns: tab.columns, rows: tab.rows },
    }
    tabItems.push({ label: tab.label, value: tab.value, children: [tableKey] })
  }

  elements["tabs"] = {
    type: "Tabs",
    props: { tabs: tabItems, defaultValue: tabs[0]?.value },
  }

  // Show scalar fields as a heading card
  const scalarEntries = Object.entries(obj).filter(
    ([, v]) => !Array.isArray(v) || v.length === 0 || typeof v[0] !== "object",
  )
  if (scalarEntries.length > 0) {
    const headerElements: Record<string, unknown> = {}
    const headerChildren: string[] = []
    for (const [k, v] of scalarEntries) {
      const id = `hdr_${k}`
      headerElements[id] = {
        type: "Text",
        props: { text: `${formatHeader(k)}: ${typeof v === "object" ? JSON.stringify(v) : String(v)}` },
      }
      headerChildren.push(id)
    }
    elements["header"] = {
      type: "Stack",
      props: { direction: "vertical", gap: "sm" },
      children: headerChildren,
    }
    Object.assign(elements, headerElements)
    elements["root"] = {
      type: "Stack",
      props: { direction: "vertical", gap: "md" },
      children: ["header", "tabs"],
    }
    return { root: "root", elements } as unknown as Spec
  }

  return { root: "tabs", elements } as unknown as Spec
}

function nestedSingleArraySpec(
  obj: Record<string, unknown>,
  [arrayKey, arrayValue]: [string, unknown],
): Spec {
  const rows = arrayValue as Record<string, unknown>[]
  const columns = Object.keys(rows[0]).map((k) => ({
    key: k,
    header: formatHeader(k),
  }))

  const elements: Record<string, unknown> = {
    table: { type: "Table", props: { columns, rows } },
  }

  // Scalar fields as heading
  const scalarEntries = Object.entries(obj).filter(([k]) => k !== arrayKey)
  if (scalarEntries.length > 0) {
    const headingParts: string[] = []
    for (const [k, v] of scalarEntries) {
      const id = `hdr_${k}`
      elements[id] = {
        type: "Text",
        props: { text: `${formatHeader(k)}: ${typeof v === "object" ? JSON.stringify(v) : String(v)}` },
      }
      headingParts.push(id)
    }
    elements["header"] = {
      type: "Card",
      props: { title: formatHeader(arrayKey) },
      children: headingParts,
    }
    elements["root"] = {
      type: "Stack",
      props: { direction: "vertical", gap: "md" },
      children: ["header", "table"],
    }
    return { root: "root", elements } as unknown as Spec
  }

  return { root: "table", elements } as unknown as Spec
}

function keyValueSpec(obj: Record<string, unknown>): Spec {
  const rows = Object.entries(obj).map(([key, value]) => ({
    key: formatHeader(key),
    value: typeof value === "object" && value !== null ? JSON.stringify(value) : String(value ?? "-"),
  }))

  return {
    root: "table",
    elements: {
      table: {
        type: "Table",
        props: {
          columns: [
            { key: "key", header: "Field" },
            { key: "value", header: "Value" },
          ],
          rows,
        },
      },
    },
  }
}

/** Convert snake_case / camelCase to Title Case */
function formatHeader(key: string): string {
  return key
    .replace(/_/g, " ")
    .replace(/([a-z])([A-Z])/g, "$1 $2")
    .replace(/\b\w/g, (c) => c.toUpperCase())
}
