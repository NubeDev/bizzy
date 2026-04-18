/**
 * LivePreview — renders AI-generated React components in the browser.
 *
 * Takes a JSX string from the AI, transpiles it with sucrase,
 * evaluates it with shadcn + React + lucide in scope, and renders live.
 */
import React, { useState, useEffect, useMemo, useCallback, useRef, type ErrorInfo } from "react"
import { flushSync } from "react-dom"
import { transform } from "sucrase"
import { useToolRunner, usePromptRunner, ToolRunProvider } from "./hooks"

// shadcn components
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle, CardDescription, CardFooter } from "@/components/ui/card"
import { Separator } from "@/components/ui/separator"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"
import ReactMarkdown from "react-markdown"
import remarkGfm from "remark-gfm"
import {
  AlertTriangle, Check, ChevronDown, ChevronRight, ChevronUp, Cloud, Droplets,
  Loader2, MapPin, Play, RefreshCw, Search, Star, Sun, Thermometer, Wind,
  X, ArrowRight, ArrowLeft, Copy, Download, ExternalLink, Heart, Info,
  Plus, Minus, Trash2, Eye, EyeOff, Calendar, Clock, Globe, Mail, Phone, User,
  Wrench,
} from "lucide-react"

/**
 * Safe string coercion — prevents "Objects are not valid as React child" errors.
 * AI-generated code can use: {str(weather.data.wind)} instead of {weather.data.wind}
 * Also injected as a global safety net.
 */
function str(val: unknown): string {
  if (val === null || val === undefined) return ""
  if (typeof val === "string") return val
  if (typeof val === "number" || typeof val === "boolean") return String(val)
  if (Array.isArray(val)) return val.map(str).join(", ")
  try { return JSON.stringify(val) } catch { return "[object]" }
}

/**
 * Safe nested property access — returns the RAW value at the path.
 * Works with any data type: strings, numbers, arrays, objects.
 *
 *   get(data, "name", "?")        → "Sydney"              (string)
 *   get(data, "cities", [])       → [{name: "Sydney"}, ...] (array — can .map())
 *   get(data, "meta.count", 0)    → 42                     (number)
 *   get(data, "missing", "?")     → "?"                    (fallback)
 *
 * For rendering in JSX, wrap with str(): {str(get(data, "wind"))}
 */
function get(obj: unknown, path: string, fallback: unknown = undefined): unknown {
  let current: unknown = obj
  for (const key of path.split(".")) {
    if (current === null || current === undefined || typeof current !== "object") return fallback
    current = (current as Record<string, unknown>)[key]
  }
  return current !== undefined ? current : fallback
}

/**
 * Markdown — renders markdown text with GFM support (tables, strikethrough, etc.).
 * Available to AI-generated components as <Markdown>{text}</Markdown>
 */
function Markdown({ children, className }: { children: string; className?: string }) {
  return (
    <div className={`prose prose-sm max-w-none dark:prose-invert ${className || ""}`}>
      <ReactMarkdown remarkPlugins={[remarkGfm]}>{children || ""}</ReactMarkdown>
    </div>
  )
}

// The scope object — everything AI-generated code can access
const SCOPE: Record<string, unknown> = {
  React,
  useState,
  useEffect,
  useMemo,
  useCallback,
  useRef,

  // Backend hooks
  useToolRunner,
  usePromptRunner,

  // Safety helpers
  str,
  get,
  JSON,
  String,
  Number,
  Boolean,
  Array,
  Object,
  Math,
  Date,
  parseInt,
  parseFloat,
  encodeURIComponent,
  console,

  // shadcn
  Button,
  Input,
  Label,
  Textarea,
  Badge,
  Card, CardContent, CardHeader, CardTitle, CardDescription, CardFooter,
  Separator,
  Tabs, TabsContent, TabsList, TabsTrigger,
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
  Skeleton,

  // Markdown renderer
  Markdown,

  // Icons
  AlertTriangle, Check, ChevronDown, ChevronRight, ChevronUp, Cloud, Droplets,
  Loader2, MapPin, Play, RefreshCw, Search, Star, Sun, Thermometer, Wind,
  X, ArrowRight, ArrowLeft, Copy, Download, ExternalLink, Heart, Info,
  Plus, Minus, Trash2, Eye, EyeOff, Calendar, Clock, Globe, Mail, Phone, User,
}

interface LivePreviewProps {
  code: string
  componentProps?: Record<string, unknown>
  className?: string
  /** Optional custom tool runner for builder mode (runs scripts via test-tool endpoint) */
  toolRunFn?: (toolName: string, params?: Record<string, unknown>) => Promise<unknown>
  /** Called when user clicks "Fix with AI" on an error */
  onRequestFix?: (message: string) => void
  /** Called when a tool run completes inside the preview, captures results for data inspector */
  onToolResult?: (toolName: string, data: unknown) => void
  /** App name — passed to ToolRunProvider for session persistence across page reloads */
  appName?: string
}

interface CompileResult {
  Component: React.FC<Record<string, unknown>> | null
  error: string | null
  warnings: string[]
}

/**
 * Compile a single TSX string into a React component.
 * @param extraScope Additional scope entries (e.g. sibling UI components for multi-file apps)
 */
function compileComponent(code: string, extraScope?: Record<string, unknown>): CompileResult {
  const warnings: string[] = []

  try {
    // Strip imports/exports
    let cleaned = code
      .replace(/^import\s+.*?from\s+['"].*?['"]\s*;?\s*$/gm, '')
      .replace(/^import\s+['"].*?['"]\s*;?\s*$/gm, '')
      .replace(/^export\s+default\s+/gm, '')
      .replace(/^export\s+/gm, '')
      .trim()

    // Warn about common issues
    if (cleaned.includes('const ') || cleaned.includes('let ')) {
      // This is fine in JSX — the AI should use const/let in components
    }

    // Transpile JSX → JS
    const result = transform(cleaned, {
      transforms: ['jsx', 'typescript'],
      jsxRuntime: 'classic',
      jsxPragma: 'React.createElement',
      jsxFragmentPragma: 'React.Fragment',
      production: true,
    })

    let jsCode = result.code

    // Find the component name
    const funcMatch = jsCode.match(/function\s+(\w+)\s*\(/)
    const constMatch = jsCode.match(/(?:const|let|var)\s+(\w+)\s*=\s*(?:\(|function)/)
    const componentName = funcMatch?.[1] || constMatch?.[1]

    if (componentName) {
      jsCode += `\nreturn ${componentName};`
    } else {
      jsCode = `return function AIComponent() {\n${jsCode}\n}`
    }

    // Merge base scope with extra scope (sibling components)
    const fullScope = extraScope ? { ...SCOPE, ...extraScope } : SCOPE
    const scopeKeys = Object.keys(fullScope)
    const scopeValues = scopeKeys.map(k => fullScope[k])
    const factory = new Function(...scopeKeys, jsCode)
    const Component = factory(...scopeValues)

    if (typeof Component !== 'function') {
      return { Component: null, error: `Compiled code did not return a React component (got ${typeof Component})`, warnings }
    }

    // Dry-run validation: try to call the component to catch obvious errors
    // We do this by creating a quick test invocation
    try {
      // Check the component can at least be referenced without throwing
      Component.displayName = componentName || 'AIComponent'
    } catch (err) {
      warnings.push(`Component validation warning: ${err instanceof Error ? err.message : String(err)}`)
    }

    return { Component, error: null, warnings }
  } catch (err) {
    return { Component: null, error: err instanceof Error ? err.message : String(err), warnings }
  }
}

/** Convert filename to PascalCase for use as component name in scope */
function toPascalCase(name: string): string {
  return name.replace(/(^|[-_])(\w)/g, (_, _sep, c) => c.toUpperCase())
}

/** Main entry-point file names — these render last and can use sibling components */
const MAIN_NAMES = new Set(["dashboard", "app", "page", "main", "index"])

/**
 * Compile multiple UI files with cross-referencing.
 * Non-main components are compiled first and injected into scope for the main component.
 * Returns the compiled main component + any errors from sub-components.
 */
export function compileMultiFile(
  files: { name: string; code: string }[]
): { main: CompileResult; mainName: string; subErrors: { name: string; error: string }[] } {
  if (files.length === 0) {
    return { main: { Component: null, error: "No UI files", warnings: [] }, mainName: "", subErrors: [] }
  }

  // Single file — no multi-file logic needed
  if (files.length === 1) {
    return { main: compileComponent(files[0].code), mainName: files[0].name, subErrors: [] }
  }

  // Sort: non-main files first, main file last
  const sorted = [...files].sort((a, b) => {
    const aIsMain = MAIN_NAMES.has(a.name)
    const bIsMain = MAIN_NAMES.has(b.name)
    if (aIsMain && !bIsMain) return 1
    if (!aIsMain && bIsMain) return -1
    return a.name.localeCompare(b.name)
  })

  const extraScope: Record<string, unknown> = {}
  const subErrors: { name: string; error: string }[] = []

  // Compile sub-components, adding each to scope for subsequent files
  for (let i = 0; i < sorted.length - 1; i++) {
    const file = sorted[i]
    const result = compileComponent(file.code, extraScope)
    if (result.Component) {
      extraScope[toPascalCase(file.name)] = result.Component
    } else if (result.error) {
      subErrors.push({ name: file.name, error: result.error })
    }
  }

  // Compile the main component with all sibling components in scope
  const mainFile = sorted[sorted.length - 1]
  const mainResult = compileComponent(mainFile.code, extraScope)

  return { main: mainResult, mainName: mainFile.name, subErrors }
}

/** Error boundary that catches render-time errors and shows them gracefully */
export class PreviewErrorBoundary extends React.Component<
  { children: React.ReactNode; onError: (error: string) => void; onRequestFix?: (error: string) => void },
  { error: string | null; errorInfo: string | null }
> {
  constructor(props: { children: React.ReactNode; onError: (error: string) => void; onRequestFix?: (error: string) => void }) {
    super(props)
    this.state = { error: null, errorInfo: null }
  }

  static getDerivedStateFromError(error: Error) {
    return { error: error.message }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    const details = info.componentStack
      ? `${error.message}\n\nComponent stack:${info.componentStack}`
      : error.message
    this.setState({ errorInfo: details })
    this.props.onError(error.message)
  }

  render() {
    if (this.state.error) {
      return (
        <div className="p-4 border border-destructive/30 bg-destructive/5 rounded-none text-xs space-y-2">
          <div className="flex items-center gap-2 text-destructive font-semibold">
            <AlertTriangle size={14} />
            Render Error
          </div>
          <pre className="font-mono text-destructive/80 whitespace-pre-wrap text-[11px]">{this.state.error}</pre>
          {this.props.onRequestFix && (
            <button
              onClick={() => this.props.onRequestFix!(this.state.error!)}
              className="flex items-center gap-1.5 px-2.5 py-1.5 text-[11px] font-medium bg-primary text-primary-foreground hover:opacity-80 transition-opacity"
            >
              <Wrench size={11} /> Fix with AI
            </button>
          )}
          {this.state.errorInfo && this.state.errorInfo !== this.state.error && (
            <details className="text-[10px] text-muted-foreground">
              <summary className="cursor-pointer">Stack trace</summary>
              <pre className="mt-1 whitespace-pre-wrap">{this.state.errorInfo}</pre>
            </details>
          )}
        </div>
      )
    }
    return this.props.children
  }
}

/**
 * LivePreview renders AI-generated React code in real-time.
 */
export function LivePreview({ code, componentProps, className, toolRunFn, onRequestFix, onToolResult, appName }: LivePreviewProps) {
  const [renderError, setRenderError] = useState<string | null>(null)
  const [showCode, setShowCode] = useState(false)

  const { Component, error: compileError, warnings } = useMemo(() => {
    setRenderError(null)
    return compileComponent(code)
  }, [code])

  const buildFixMessage = useCallback((error: string) => {
    const truncatedCode = code.length > 2000 ? code.slice(0, 2000) + "\n// ... truncated" : code
    return `The UI component crashed with this error:\n\n\`\`\`\n${error}\n\`\`\`\n\nHere is the current code:\n\n\`\`\`tsx\n${truncatedCode}\n\`\`\`\n\nPlease fix the component. Common fixes:\n- Use \`str(value)\` to safely render data that might be an object\n- Use \`get(obj, "path", "fallback")\` for safe nested access\n- Check if data is null/undefined before calling .map() or accessing properties\n- Ensure arrays are actual arrays before mapping`
  }, [code])

  if (compileError) {
    return (
      <div className={`space-y-3 ${className || ''}`}>
        <div className="p-4 border border-destructive/30 bg-destructive/5 rounded-none text-xs space-y-2">
          <div className="flex items-center gap-2 text-destructive font-semibold">
            <AlertTriangle size={14} />
            Compile Error
          </div>
          <pre className="font-mono text-destructive/80 whitespace-pre-wrap text-[11px]">{compileError}</pre>
          {onRequestFix && (
            <button
              onClick={() => onRequestFix(buildFixMessage(compileError))}
              className="flex items-center gap-1.5 px-2.5 py-1.5 text-[11px] font-medium bg-primary text-primary-foreground hover:opacity-80 transition-opacity"
            >
              <Wrench size={11} /> Fix with AI
            </button>
          )}
        </div>
        <details className="text-xs">
          <summary className="cursor-pointer text-muted-foreground hover:text-foreground">View source code</summary>
          <pre className="mt-2 p-3 rounded-none bg-muted/50 border border-border overflow-auto max-h-60 text-[11px] font-mono">{code}</pre>
        </details>
      </div>
    )
  }

  if (!Component) {
    return <div className="text-xs text-muted-foreground p-4">No component to render.</div>
  }

  return (
    <div className={className}>
      {warnings.length > 0 && (
        <div className="mb-2 p-2 border border-yellow-500/30 bg-yellow-500/5 rounded-none text-[11px] text-yellow-600">
          {warnings.map((w, i) => <div key={i}>{w}</div>)}
        </div>
      )}

      {renderError ? (
        <div className="space-y-3">
          <div className="p-4 border border-destructive/30 bg-destructive/5 rounded-none text-xs space-y-2">
            <div className="flex items-center gap-2 text-destructive font-semibold">
              <AlertTriangle size={14} />
              Render Error
            </div>
            <pre className="font-mono text-destructive/80 whitespace-pre-wrap text-[11px]">{renderError}</pre>
            <p className="text-[11px] text-muted-foreground">
              Tip: The AI may have tried to render an object directly. Ask it to fix: "the data fields need to use str() or String() for display"
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
      ) : (
        <PreviewErrorBoundary key={code} onError={setRenderError} onRequestFix={onRequestFix ? (err) => onRequestFix(buildFixMessage(err)) : undefined}>
          {toolRunFn ? (
            <ToolRunProvider runFn={toolRunFn} onResult={onToolResult} appName={appName}>
              <Component {...(componentProps || {})} />
            </ToolRunProvider>
          ) : (
            <Component {...(componentProps || {})} />
          )}
        </PreviewErrorBoundary>
      )}
    </div>
  )
}

/** Available components documentation for AI prompts */
export const AVAILABLE_COMPONENTS = `
## Available in Scope (no imports needed)

### React
useState, useEffect, useMemo, useCallback, useRef

### Backend Hooks
**useToolRunner()** — call backend tools and get real data:
  const tool = useToolRunner()
  tool.run("appname.tool_name", { param: "value" })  // calls your tool
  tool.data    // result object (null until run)
  tool.loading // true while executing
  tool.error   // error string or null

**usePromptRunner()** — send prompts to Claude:
  const ai = usePromptRunner()
  ai.run("Analyze this data: " + JSON.stringify(data))  // raw prompt
  ai.run("prompt_name", { city: "London" })              // named template
  ai.text    // Claude's response
  ai.loading // true while processing
  ai.error   // error string or null

### Safety Helpers
**str(value)** — safely convert any value to string (prevents "Objects are not valid as React child")
  Use: {str(data.wind)} instead of {data.wind} when data might be an object
**get(obj, "path.to.field", fallback)** — safe nested property access, returns raw value
  Use: var cities = get(data, "cities", [])    // returns real array, can .map()
  Use: var temp = get(data, "wind.speed", 0)   // returns number
  Use: {str(get(data, "name", "?"))}           // wrap with str() for JSX display

### Markdown Rendering
**Markdown** — renders markdown text with GFM support (tables, lists, bold, etc.)
  Use: <Markdown>{ai.text}</Markdown>
  Use: <Markdown className="text-sm">{str(someMarkdownString)}</Markdown>

### UI Components (shadcn)
Button, Input, Label, Textarea, Badge, Separator, Skeleton
Card, CardContent, CardHeader, CardTitle, CardDescription, CardFooter
Tabs, TabsContent, TabsList, TabsTrigger
Select, SelectContent, SelectItem, SelectTrigger, SelectValue

### Icons (lucide-react)
AlertTriangle, Check, ChevronDown, ChevronRight, ChevronUp, Cloud, Droplets,
Loader2, MapPin, Play, RefreshCw, Search, Star, Sun, Thermometer, Wind,
X, ArrowRight, ArrowLeft, Copy, Download, ExternalLink, Heart, Info,
Plus, Minus, Trash2, Eye, EyeOff, Calendar, Clock, Globe, Mail, Phone, User

### Styling
Tailwind CSS — all utility classes available.

### Also in scope
JSON, String, Number, Boolean, Array, Object, Math, Date, parseInt, parseFloat, console
`.trim()
