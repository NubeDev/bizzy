/** Virtual file in the app project */
export interface AppFile {
  /** Relative path, e.g. "tools/query_nodes.js" or "app.yaml" */
  path: string
  /** File content */
  content: string
  /** File type for syntax/icon purposes */
  type: "yaml" | "js" | "json" | "md" | "tsx"
  /** Whether the file has unsaved AI changes */
  dirty?: boolean
}

/** The full virtual app project that the builder operates on */
export interface AppProject {
  name: string
  displayName: string
  description: string
  category: string
  files: AppFile[]
}

/** Creates an empty project scaffold */
export function emptyProject(): AppProject {
  return {
    name: "",
    displayName: "",
    description: "",
    category: "utilities",
    files: [
      {
        path: "app.yaml",
        type: "yaml",
        content: `name: my-app
version: 1.0.0
description: ""
author: ""
permissions:
  allowedHosts: []
  defaultToolClass: read-only
settings: []
tags: []
`,
      },
    ],
  }
}

/** Group files by directory for tree rendering */
export function groupFiles(files: AppFile[]): Record<string, AppFile[]> {
  const groups: Record<string, AppFile[]> = { "/": [] }
  for (const f of files) {
    const parts = f.path.split("/")
    if (parts.length === 1) {
      groups["/"].push(f)
    } else {
      const dir = parts.slice(0, -1).join("/")
      if (!groups[dir]) groups[dir] = []
      groups[dir].push(f)
    }
  }
  return groups
}

/** Get file extension icon */
export function fileIcon(type: AppFile["type"]): string {
  switch (type) {
    case "yaml": return "gear"
    case "js": return "code"
    case "json": return "braces"
    case "md": return "text"
    case "tsx": return "component"
    default: return "file"
  }
}
