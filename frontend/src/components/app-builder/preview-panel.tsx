/**
 * Preview panel — shows live UI preview or tool test results.
 * Uses test-tool endpoint so tools work without saving the app first.
 *
 * Includes Data Inspector to show tool response shapes and "Fix with AI" wiring.
 */
import { useState, useCallback, useMemo } from "react"
import { Eye, FlaskConical, Database, ChevronDown, ChevronRight, Wrench, Copy, Check, AlertTriangle } from "lucide-react"
import { LivePreview, compileMultiFile, PreviewErrorBoundary } from "@/components/live-preview/renderer"
import { ToolRunProvider } from "@/components/live-preview/hooks"
import { ToolTester } from "@/components/shared/tool-tester"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Badge } from "@/components/ui/badge"
import type { AppFile, AppProject } from "./types"

interface Props {
  project: AppProject
  selectedFile: AppFile | null
  /** Called when "Fix with AI" or "Update UI to Match Data" is clicked — sends a message to the AI chat */
  onRequestFix?: (message: string) => void
  /** App name for session persistence — use the route param, not project.name (which loads async) */
  appName?: string
}

export function PreviewPanel({ project, selectedFile, onRequestFix, appName: appNameProp }: Props) {
  // Use explicit prop (from route param, available immediately) or fall back to project.name (async)
  const appName = appNameProp || project.name || undefined
  const uiFiles = useMemo(() => project.files.filter(f => f.type === "tsx"), [project.files])
  const toolPairs = useMemo(() => getToolPairs(project), [project.files])
  const [toolResults, setToolResults] = useState<Map<string, unknown>>(new Map())

  const defaultTab = selectedFile?.type === "tsx" ? "ui"
    : selectedFile?.type === "js" && selectedFile.path.startsWith("tools/") ? "test"
    : uiFiles.length > 0 ? "ui"
    : toolPairs.length > 0 ? "test"
    : "ui"

  // Capture tool results for the data inspector
  const handleToolResult = useCallback((toolName: string, data: unknown) => {
    setToolResults(prev => {
      const next = new Map(prev)
      next.set(toolName, data)
      return next
    })
  }, [])

  // Tool runner for live preview — runs scripts via test-tool endpoint (no saved app needed)
  const builderToolRunFn = useCallback(async (toolName: string, params?: Record<string, unknown>) => {
    // Strip "appname." prefix if present
    const shortName = toolName.includes(".") ? toolName.split(".").pop()! : toolName
    const tool = toolPairs.find(t => t.name === shortName)
    if (!tool) throw new Error(`Tool "${shortName}" not found in project. Available: ${toolPairs.map(t => t.name).join(", ")}`)

    const script = tool.helpers ? tool.helpers + "\n\n" + tool.script : tool.script
    const res = await fetch("/api/apps/test-tool", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ script, params: params || {}, allowedHosts: ["*"], timeout: "30s" }),
    })
    if (!res.ok) {
      const body = await res.json().catch(() => ({}))
      throw new Error(body.error || `Test failed: ${res.statusText}`)
    }
    const result = await res.json()
    if (result.error) throw new Error(result.error)
    return result.output
  }, [toolPairs])

  return (
    <div className="h-full flex flex-col">
      <Tabs defaultValue={defaultTab} className="flex flex-col h-full">
        <div className="border-b border-border px-2">
          <TabsList className="bg-transparent h-auto p-0 gap-0">
            <TabsTrigger value="ui" className="rounded-none border-b-2 border-transparent data-[state=active]:border-primary data-[state=active]:bg-transparent text-xs px-3 py-2">
              <Eye size={12} className="mr-1.5" /> Preview
            </TabsTrigger>
            <TabsTrigger value="test" className="rounded-none border-b-2 border-transparent data-[state=active]:border-primary data-[state=active]:bg-transparent text-xs px-3 py-2">
              <FlaskConical size={12} className="mr-1.5" /> Test Tools
            </TabsTrigger>
          </TabsList>
        </div>

        {/* UI Preview — multi-file compilation with data-aware fix messages */}
        <TabsContent value="ui" className="flex-1 min-h-0 overflow-auto m-0 p-4">
          {uiFiles.length > 0 ? (
            <div className="space-y-4">
              {uiFiles.length === 1 ? (
                /* Single file — use LivePreview directly */
                <div className="space-y-1">
                  <code className="text-[10px] font-mono text-muted-foreground">{uiFiles[0].path}</code>
                  <LivePreview
                    code={uiFiles[0].content}
                    toolRunFn={builderToolRunFn}
                    onRequestFix={onRequestFix ? (msg) => onRequestFix(enrichFixMessage(msg, toolResults)) : undefined}
                    onToolResult={handleToolResult}
                    appName={appName}
                  />
                </div>
              ) : (
                /* Multi-file — compile together so components can reference each other */
                <MultiFilePreview
                  files={uiFiles}
                  toolRunFn={builderToolRunFn}
                  onRequestFix={onRequestFix ? (msg) => onRequestFix(enrichFixMessage(msg, toolResults)) : undefined}
                  onToolResult={handleToolResult}
                  appName={appName}
                />
              )}

              {/* Data Inspector — shows tool response shapes after tools have been called */}
              {toolResults.size > 0 && (
                <DataInspector results={toolResults} onRequestFix={onRequestFix} />
              )}
            </div>
          ) : (
            <EmptyPreview message="No UI components yet. Ask the AI to generate a tsx:ui component." />
          )}
        </TabsContent>

        {/* Tool Testing */}
        <TabsContent value="test" className="flex-1 min-h-0 overflow-auto m-0 p-4">
          {toolPairs.length > 0 ? (
            <ToolTestList tools={toolPairs} />
          ) : (
            <EmptyPreview message="No tools yet. Ask the AI to generate tools for your app." />
          )}
        </TabsContent>
      </Tabs>
    </div>
  )
}

/** Renders a pre-compiled component with error boundary and tool wiring */
function MultiFileRenderer({ Component, code, toolRunFn, onRequestFix, onToolResult, appName }: {
  Component: React.FC<Record<string, unknown>>
  code: string
  toolRunFn: (toolName: string, params?: Record<string, unknown>) => Promise<unknown>
  onRequestFix?: (message: string) => void
  onToolResult?: (toolName: string, data: unknown) => void
  appName?: string
}) {
  const [renderError, setRenderError] = useState<string | null>(null)
  const [showCode, setShowCode] = useState(false)

  const buildFixMessage = useCallback((error: string) => {
    const truncatedCode = code.length > 2000 ? code.slice(0, 2000) + "\n// ... truncated" : code
    return `The UI component crashed with this error:\n\n\`\`\`\n${error}\n\`\`\`\n\nHere is the current code:\n\n\`\`\`tsx\n${truncatedCode}\n\`\`\`\n\nPlease fix the component. Common fixes:\n- Use \`var data = hook.data || {}\` then \`get(data, "field", fallback)\` — NEVER \`get(hook, "data.field")\`\n- Use Array.isArray() before calling .map()\n- Use \`str(value)\` to safely render data that might be an object`
  }, [code])

  if (renderError) {
    return (
      <div className="space-y-3">
        <div className="p-4 border border-destructive/30 bg-destructive/5 rounded-none text-xs space-y-2">
          <div className="flex items-center gap-2 text-destructive font-semibold">
            <AlertTriangle size={14} />
            Render Error
          </div>
          <pre className="font-mono text-destructive/80 whitespace-pre-wrap text-[11px]">{renderError}</pre>
          <p className="text-[11px] text-muted-foreground">
            Tip: The AI may have tried to render an object directly or used get() on a hook instance instead of .data
          </p>
          {onRequestFix && (
            <button
              onClick={() => onRequestFix(buildFixMessage(renderError))}
              className="flex items-center gap-1.5 px-2.5 py-1.5 text-[11px] font-medium bg-primary text-primary-foreground hover:opacity-80 transition-opacity"
            >
              <Wrench size={11} /> Fix with AI
            </button>
          )}
        </div>
        <button onClick={() => setShowCode(!showCode)} className="text-[11px] font-mono text-muted-foreground hover:text-foreground">
          {showCode ? "Hide code" : "Show code"}
        </button>
        {showCode && (
          <pre className="p-3 rounded-none bg-muted/50 border border-border overflow-auto max-h-60 text-[11px] font-mono">{code}</pre>
        )}
      </div>
    )
  }

  return (
    <PreviewErrorBoundary
      key={code}
      onError={setRenderError}
      onRequestFix={onRequestFix ? (err) => onRequestFix(buildFixMessage(err)) : undefined}
    >
      <ToolRunProvider runFn={toolRunFn} onResult={onToolResult} appName={appName}>
        <Component />
      </ToolRunProvider>
    </PreviewErrorBoundary>
  )
}

/** Append data shape context to fix messages so the AI knows the real data structure */
function enrichFixMessage(message: string, toolResults: Map<string, unknown>): string {
  if (toolResults.size === 0) return message
  const shapes = Array.from(toolResults.entries()).map(([name, data]) => {
    const shape = describeShape(data)
    return `Tool "${name}" returned:\n${shape}`
  }).join("\n\n")
  return message + `\n\n## Actual Tool Data Shapes (from Data Inspector)\n\n\`\`\`\n${shapes}\n\`\`\`\n\nIMPORTANT: Access hook data correctly — use \`var data = hook.data || {}\` first, then \`get(data, "field", fallback)\`. NEVER do \`get(hook, "data.field")\`. Always use Array.isArray() before .map().`
}

/** Multi-file preview — compiles all TSX files with cross-referencing and renders the main component */
function MultiFilePreview({ files, toolRunFn, onRequestFix, onToolResult, appName }: {
  files: AppFile[]
  toolRunFn: (toolName: string, params?: Record<string, unknown>) => Promise<unknown>
  onRequestFix?: (message: string) => void
  onToolResult?: (toolName: string, data: unknown) => void
  appName?: string
}) {
  // Use content-based key so recompilation only happens when file content actually changes,
  // not when the files array gets a new reference (e.g. from parent re-rendering on tool results)
  const filesKey = files.map(f => f.path + "\0" + f.content).join("\0\0")
  const compiled = useMemo(() => {
    const namedFiles = files.map(f => ({
      name: f.path.replace("ui/", "").replace(".tsx", ""),
      code: f.content,
    }))
    return compileMultiFile(namedFiles)
  }, [filesKey])

  const { main, mainName, subErrors } = compiled
  const MainComponent = main.Component

  return (
    <div className="space-y-2">
      {/* Show sub-component files as badges */}
      {files.length > 1 && (
        <div className="flex flex-wrap items-center gap-1">
          {files.map(f => {
            const name = f.path.replace("ui/", "").replace(".tsx", "")
            const isMain = name === mainName
            const hasError = subErrors.some(e => e.name === name) || (isMain && main.error)
            return (
              <Badge key={f.path} variant={hasError ? "destructive" : isMain ? "default" : "secondary"} className="text-[10px] rounded-none font-mono">
                {f.path}{isMain ? " (main)" : ""}
              </Badge>
            )
          })}
        </div>
      )}

      {/* Show sub-component compile errors */}
      {subErrors.length > 0 && (
        <div className="space-y-2">
          {subErrors.map(({ name, error }) => (
            <div key={name} className="p-3 border border-destructive/30 bg-destructive/5 text-xs space-y-1">
              <div className="flex items-center gap-2 text-destructive font-semibold">
                <AlertTriangle size={12} />
                Compile error in ui/{name}.tsx
              </div>
              <pre className="font-mono text-destructive/80 whitespace-pre-wrap text-[11px]">{error}</pre>
              {onRequestFix && (
                <button
                  onClick={() => {
                    const file = files.find(f => f.path === `ui/${name}.tsx`)
                    onRequestFix(`Compile error in ui/${name}.tsx:\n\n\`\`\`\n${error}\n\`\`\`\n\nCurrent code:\n\n\`\`\`tsx\n${file?.content || ""}\n\`\`\`\n\nPlease fix this component. Output the full corrected file as \`\`\`tsx:ui/${name}.tsx`)
                  }}
                  className="flex items-center gap-1.5 px-2.5 py-1.5 text-[11px] font-medium bg-primary text-primary-foreground hover:opacity-80 transition-opacity"
                >
                  <Wrench size={11} /> Fix with AI
                </button>
              )}
            </div>
          ))}
        </div>
      )}

      {/* Main component render */}
      {main.error ? (
        <div className="p-3 border border-destructive/30 bg-destructive/5 text-xs space-y-1">
          <div className="flex items-center gap-2 text-destructive font-semibold">
            <AlertTriangle size={12} />
            Compile error in ui/{mainName}.tsx
          </div>
          <pre className="font-mono text-destructive/80 whitespace-pre-wrap text-[11px]">{main.error}</pre>
          {onRequestFix && (
            <button
              onClick={() => {
                const file = files.find(f => f.path === `ui/${mainName}.tsx`)
                onRequestFix(`Compile error in ui/${mainName}.tsx:\n\n\`\`\`\n${main.error}\n\`\`\`\n\nCurrent code:\n\n\`\`\`tsx\n${file?.content || ""}\n\`\`\`\n\nPlease fix this component.`)
              }}
              className="flex items-center gap-1.5 px-2.5 py-1.5 text-[11px] font-medium bg-primary text-primary-foreground hover:opacity-80 transition-opacity"
            >
              <Wrench size={11} /> Fix with AI
            </button>
          )}
        </div>
      ) : MainComponent ? (
        <MultiFileRenderer
          Component={MainComponent}
          code={files.find(f => f.path === `ui/${mainName}.tsx`)?.content || ""}
          toolRunFn={toolRunFn}
          onRequestFix={onRequestFix}
          onToolResult={onToolResult}
          appName={appName}
        />
      ) : (
        <div className="text-xs text-muted-foreground p-4">No component to render.</div>
      )}
    </div>
  )
}

/** Describe the shape of data for the AI — concise type description */
function describeShape(data: unknown, depth = 0): string {
  if (data === null) return "null"
  if (data === undefined) return "undefined"
  if (typeof data === "string") return `string (e.g. "${data.length > 40 ? data.slice(0, 40) + "..." : data}")`
  if (typeof data === "number") return `number (e.g. ${data})`
  if (typeof data === "boolean") return `boolean (${data})`
  if (Array.isArray(data)) {
    if (data.length === 0) return "[] (empty array)"
    return `Array<${describeShape(data[0], depth + 1)}> (${data.length} items)`
  }
  if (typeof data === "object" && depth < 2) {
    const entries = Object.entries(data as Record<string, unknown>)
    if (entries.length === 0) return "{} (empty object)"
    const fields = entries.slice(0, 10).map(([k, v]) => `  ${k}: ${describeShape(v, depth + 1)}`)
    const suffix = entries.length > 10 ? `\n  ... +${entries.length - 10} more fields` : ""
    return `{\n${fields.join(",\n")}${suffix}\n}`
  }
  return typeof data
}

/** Data Inspector — shows actual tool response data and shape */
function DataInspector({ results, onRequestFix }: {
  results: Map<string, unknown>
  onRequestFix?: (message: string) => void
}) {
  const [expanded, setExpanded] = useState(true)
  const [expandedTools, setExpandedTools] = useState<Set<string>>(new Set())
  const [copied, setCopied] = useState<string | null>(null)

  const toggleTool = (name: string) => {
    setExpandedTools(prev => {
      const next = new Set(prev)
      if (next.has(name)) next.delete(name)
      else next.add(name)
      return next
    })
  }

  const handleCopy = (name: string, data: unknown) => {
    navigator.clipboard.writeText(JSON.stringify(data, null, 2))
    setCopied(name)
    setTimeout(() => setCopied(null), 1500)
  }

  const handleUpdateUI = (toolName: string, data: unknown) => {
    if (!onRequestFix) return
    const shape = describeShape(data)
    const sample = JSON.stringify(data, null, 2)
    const truncatedSample = sample.length > 1500 ? sample.slice(0, 1500) + "\n// ... truncated" : sample
    onRequestFix(
      `The tool "${toolName}" returned data with this shape:\n\n\`\`\`\n${shape}\n\`\`\`\n\nSample response:\n\`\`\`json\n${truncatedSample}\n\`\`\`\n\nPlease update the UI component to correctly render this data structure. Use \`get()\` for safe nested access and \`str()\` for safe display.`
    )
  }

  return (
    <div className="border border-border bg-muted/30">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center gap-2 px-3 py-2 text-[11px] font-mono text-muted-foreground hover:text-foreground transition-colors"
      >
        {expanded ? <ChevronDown size={11} /> : <ChevronRight size={11} />}
        <Database size={11} />
        Data Inspector
        <span className="text-[10px] ml-auto">{results.size} tool{results.size !== 1 ? "s" : ""}</span>
      </button>

      {expanded && (
        <div className="border-t border-border">
          {Array.from(results.entries()).map(([toolName, data]) => (
            <div key={toolName} className="border-b border-border last:border-b-0">
              <div className="flex items-center gap-2 px-3 py-1.5">
                <button onClick={() => toggleTool(toolName)} className="flex items-center gap-1.5 text-[11px] font-mono text-foreground hover:text-primary transition-colors">
                  {expandedTools.has(toolName) ? <ChevronDown size={10} /> : <ChevronRight size={10} />}
                  {toolName}
                </button>
                <span className="text-[10px] text-muted-foreground">
                  {Array.isArray(data) ? `array[${data.length}]` : typeof data}
                </span>
                <div className="ml-auto flex items-center gap-1">
                  <button onClick={() => handleCopy(toolName, data)} className="p-1 text-muted-foreground hover:text-foreground transition-colors" title="Copy JSON">
                    {copied === toolName ? <Check size={10} className="text-emerald-500" /> : <Copy size={10} />}
                  </button>
                  {onRequestFix && (
                    <button
                      onClick={() => handleUpdateUI(toolName, data)}
                      className="flex items-center gap-1 px-1.5 py-0.5 text-[10px] text-primary hover:bg-primary/10 transition-colors"
                      title="Ask AI to update the UI component to match this data shape"
                    >
                      <Wrench size={9} /> Update UI
                    </button>
                  )}
                </div>
              </div>
              {expandedTools.has(toolName) && (
                <pre className="px-3 pb-2 text-[10px] font-mono text-muted-foreground overflow-auto max-h-48 whitespace-pre-wrap">
                  {JSON.stringify(data, null, 2)}
                </pre>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

/** Parse tool .json + .js pairs from the project files */
function getToolPairs(project: AppProject): { name: string; params: Record<string, unknown>; script: string; helpers?: string }[] {
  const tools: { name: string; params: Record<string, unknown>; script: string; helpers?: string }[] = []
  const helpers = project.files.find(f => f.path === "tools/_helpers.js")

  for (const f of project.files) {
    if (f.type !== "json" || !f.path.startsWith("tools/")) continue
    const name = f.path.replace("tools/", "").replace(".json", "")
    const jsFile = project.files.find(j => j.path === `tools/${name}.js`)
    if (!jsFile) continue

    try {
      const schema = JSON.parse(f.content)
      tools.push({
        name: schema.name || name,
        params: schema.params || {},
        script: jsFile.content,
        helpers: helpers?.content,
      })
    } catch { /* skip invalid json */ }
  }

  return tools
}

function ToolTestList({ tools }: { tools: { name: string; params: Record<string, unknown>; script: string; helpers?: string }[] }) {
  const [activeTool, setActiveTool] = useState(tools[0]?.name || "")
  const tool = tools.find(t => t.name === activeTool)

  return (
    <div className="space-y-3">
      {tools.length > 1 && (
        <div className="flex flex-wrap gap-1">
          {tools.map(t => (
            <button key={t.name} onClick={() => setActiveTool(t.name)}
              className={`px-2 py-1 text-xs font-mono rounded-none transition-colors ${
                activeTool === t.name ? "bg-accent text-foreground border border-border" : "text-muted-foreground hover:bg-accent/50"
              }`}>
              {t.name}
            </button>
          ))}
        </div>
      )}
      {tool && (
        <ToolTester
          key={tool.name}
          tool={{
            name: tool.name, description: "", toolClass: "read-only",
            params: tool.params as Record<string, { type: string; required: boolean; description: string }>,
            script: tool.helpers ? tool.helpers + "\n\n" + tool.script : tool.script,
          }}
          showScript compact
        />
      )}
    </div>
  )
}

function EmptyPreview({ message }: { message: string }) {
  return (
    <div className="h-full flex items-center justify-center text-center px-8">
      <div>
        <Eye size={32} className="mx-auto mb-3 text-muted-foreground/30" />
        <p className="text-xs text-muted-foreground">{message}</p>
      </div>
    </div>
  )
}
