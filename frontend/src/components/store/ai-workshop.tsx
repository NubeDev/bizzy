import { useState, useRef, useEffect, useMemo } from "react"
import { ArrowUp, Trash2, Plus, Loader2, Sparkles, Wrench, MessageSquare, Check, User, Play, RefreshCw, AlertTriangle, Eye, Code } from "lucide-react"
import { useAgentChat, type ChatMessage } from "@/hooks/use-agent-chat"
import { useAddTool, useAddPrompt, useUpdateTool } from "@/hooks/use-my-apps"
import { useTestTool, type TestToolResponse } from "@/hooks/use-test-tool"
import { SpecRenderer } from "@/lib/json-render-registry"
import { outputToSpec } from "@/lib/output-to-spec"
import { SchemaForm } from "@/components/workshop/schema-form"
import { HttpTrace } from "@/components/workshop/http-trace"
import { LivePreview } from "@/components/live-preview/renderer"
import { useBootstrapPrompts } from "@/hooks/use-bootstrap-prompts"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import type { StoreTool, StorePrompt, StoreApp } from "@/lib/types"

function buildSystemPrompt(compose: (names: string[]) => string, app: StoreApp): string {
  const toolSummaries = (app.tools || []).map(t => {
    const paramList = Object.entries(t.params || {}).map(([k, v]) => {
      let desc = `${k}: ${v.type}`
      if (v.required) desc += " (required)"
      if (v.options?.length) desc += ` [${v.options.join(", ")}]`
      if (v.description) desc += ` — ${v.description}`
      return desc
    }).join("\n    ")
    return `  - ${t.name} (${t.toolClass}${t.mode ? ", mode:" + t.mode : ""}): ${t.description}\n    Params:\n    ${paramList || "(none)"}`
  }).join("\n")

  const promptSummaries = (app.prompts || []).map(p =>
    `  - ${p.name}: ${p.description}`
  ).join("\n")

  const hosts = app.permissions?.allowedHosts?.join(", ") || "(none)"

  const reference = compose(["workshop", "ui_reference", "tool_naming"])

  return `You are editing the app "${app.displayName}" (${app.name}).

## Current App State
Description: ${app.description || "(none)"}
Category: ${app.category || "(none)"}
Allowed Hosts: ${hosts}

### Existing Tools (${app.tools?.length || 0})
${toolSummaries || "  (no tools yet)"}

### Existing Prompts (${app.prompts?.length || 0})
${promptSummaries || "  (no prompts yet)"}

Tool names use the format "appname.tool_name" (e.g. "${app.name}.check_weather").
TESTING: Tools available via MCP as "${app.name}.<tool_name>".

---

${reference}
`
}

interface Props {
  app: StoreApp
}

// --- Artifact types ---

interface ToolArtifact { type: 'tool'; data: StoreTool }
interface PromptArtifact { type: 'prompt'; data: StorePrompt }
interface UIArtifact { type: 'ui'; code: string; name: string }
type ExtractedArtifact = ToolArtifact | PromptArtifact | UIArtifact

function extractArtifacts(content: string): ExtractedArtifact[] {
  const artifacts: ExtractedArtifact[] = []

  // tsx:ui blocks — live React components
  const uiRegex = /```tsx:ui\s*\n([\s\S]*?)```/g
  let match
  while ((match = uiRegex.exec(content)) !== null) {
    const code = match[1].trim()
    const nameMatch = code.match(/function\s+(\w+)/)
    artifacts.push({ type: 'ui', code, name: nameMatch?.[1] || 'Component' })
  }

  // json:tool blocks
  const toolRegex = /```json:tool\s*\n([\s\S]*?)```/g
  while ((match = toolRegex.exec(content)) !== null) {
    try {
      const data = JSON.parse(match[1])
      if (data.name && data.script) {
        artifacts.push({ type: 'tool', data: { name: data.name, description: data.description || '', toolClass: data.toolClass || 'read-only', mode: data.mode || '', params: data.params || {}, script: data.script } })
      }
    } catch { /* skip */ }
  }

  // json:prompt blocks
  const promptRegex = /```json:prompt\s*\n([\s\S]*?)```/g
  while ((match = promptRegex.exec(content)) !== null) {
    try {
      const data = JSON.parse(match[1])
      if (data.name && data.body) {
        artifacts.push({ type: 'prompt', data: { name: data.name, description: data.description || '', arguments: data.arguments, body: data.body } })
      }
    } catch { /* skip */ }
  }

  // Fallback: plain json blocks
  if (artifacts.filter(a => a.type !== 'ui').length === 0) {
    const jsonRegex = /```json\s*\n([\s\S]*?)```/g
    while ((match = jsonRegex.exec(content)) !== null) {
      try {
        const data = JSON.parse(match[1])
        if (data.name && data.script) {
          artifacts.push({ type: 'tool', data: { name: data.name, description: data.description || '', toolClass: data.toolClass || 'read-only', mode: data.mode || '', params: data.params || {}, script: data.script } })
        } else if (data.name && data.body) {
          artifacts.push({ type: 'prompt', data: { name: data.name, description: data.description || '', body: data.body } })
        }
      } catch { /* skip */ }
    }
  }

  // Fallback: plain tsx blocks that look like components
  if (artifacts.filter(a => a.type === 'ui').length === 0) {
    const tsxRegex = /```tsx\s*\n([\s\S]*?)```/g
    while ((match = tsxRegex.exec(content)) !== null) {
      const code = match[1].trim()
      if (code.includes('function') && code.includes('return')) {
        const nameMatch = code.match(/function\s+(\w+)/)
        artifacts.push({ type: 'ui', code, name: nameMatch?.[1] || 'Component' })
      }
    }
  }

  return artifacts
}

// --- Main Workshop Component ---

export function AIWorkshop({ app }: Props) {
  const { compose } = useBootstrapPrompts()
  const { messages, isStreaming, send, clear } = useAgentChat()
  const addToolMutation = useAddTool()
  const updateToolMutation = useUpdateTool()
  const addPromptMutation = useAddPrompt()
  const [input, setInput] = useState("")
  const scrollRef = useRef<HTMLDivElement>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const [appliedArtifacts, setAppliedArtifacts] = useState<Set<string>>(new Set())

  const existingToolNames = useMemo(() => new Set((app.tools || []).map(t => t.name)), [app.tools])
  const existingPromptNames = useMemo(() => new Set((app.prompts || []).map(p => p.name)), [app.prompts])

  useEffect(() => {
    if (scrollRef.current) scrollRef.current.scrollTop = scrollRef.current.scrollHeight
  }, [messages])

  useEffect(() => {
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto'
      textareaRef.current.style.height = Math.min(textareaRef.current.scrollHeight, 200) + 'px'
    }
  }, [input])

  const handleSend = () => {
    if (!input.trim() || isStreaming) return
    const userText = input.trim()
    const prompt = messages.length === 0
      ? buildSystemPrompt(compose, app) + "\n\nUser request: " + userText
      : userText
    send(prompt, userText)
    setInput("")
  }

  const handleApplyTool = async (tool: StoreTool) => {
    if (existingToolNames.has(tool.name)) {
      await updateToolMutation.mutateAsync({ appId: app.id, name: tool.name, tool, changeSummary: "AI Workshop" })
    } else {
      await addToolMutation.mutateAsync({ appId: app.id, tool })
    }
    setAppliedArtifacts(prev => new Set(prev).add(`tool:${tool.name}`))
  }

  const handleApplyPrompt = async (prompt: StorePrompt) => {
    await addPromptMutation.mutateAsync({ appId: app.id, prompt })
    setAppliedArtifacts(prev => new Set(prev).add(`prompt:${prompt.name}`))
  }

  const suggestions = useMemo(() => {
    const s: { label: string; icon: string }[] = []
    if (!app.tools?.length) {
      s.push({ label: "Create a weather dashboard UI with city selector and live stats", icon: "🌤" })
      s.push({ label: "Build a URL health checker with status cards", icon: "🔗" })
      s.push({ label: "Make a currency converter with dropdown selectors", icon: "💱" })
    } else {
      s.push({ label: `Build a UI that displays results from my ${app.tools[0]?.name} tool`, icon: "🎨" })
      s.push({ label: `Review my ${app.tools.length} tools and suggest improvements`, icon: "🔍" })
      if (!app.tools.some(t => t.mode === "qa")) {
        s.push({ label: "Turn my tools into an interactive QA experience", icon: "❓" })
      }
      if (!app.prompts?.length) {
        s.push({ label: "Generate useful prompts for my existing tools", icon: "📝" })
      }
    }
    s.push({ label: "Show me a beautiful dashboard UI with cards and charts", icon: "📊" })
    return s.slice(0, 4)
  }, [app.tools, app.prompts])

  return (
    <div className="flex flex-col h-[calc(100vh-200px)] min-h-[500px] relative">
      {messages.length > 0 && (
        <div className="absolute top-0 right-0 z-10">
          <Button variant="ghost" size="sm" onClick={clear} className="text-muted-foreground hover:text-foreground rounded-none text-xs h-8">
            <Trash2 size={14} className="mr-1" /> Clear
          </Button>
        </div>
      )}

      <div ref={scrollRef} className="flex-1 overflow-y-auto">
        {messages.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-full px-4 relative">
            <div className="absolute inset-0 dots-pattern opacity-40 pointer-events-none" />
            <div className="relative w-16 h-16 rounded-none bg-secondary flex items-center justify-center mb-6 border border-border">
              <Sparkles size={28} className="text-foreground" />
            </div>
            <h1 className="font-mono text-2xl lg:text-3xl font-light mb-2 text-foreground">{app.displayName}</h1>
            <p className="text-muted-foreground text-sm mb-2 text-center max-w-md leading-relaxed">
              Build tools, prompts, and live UI components. Everything renders instantly.
            </p>
            <p className="text-muted-foreground text-xs mb-8">
              {app.tools?.length || 0} tools, {app.prompts?.length || 0} prompts
              {app.permissions?.allowedHosts?.length ? ` — hosts: ${app.permissions.allowedHosts.join(", ")}` : ""}
            </p>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 w-full max-w-2xl relative">
              {suggestions.map((s) => (
                <button key={s.label} onClick={() => setInput(s.label)}
                  className="text-left p-4 rounded-none border border-border bg-card hover:bg-accent transition-colors text-sm text-muted-foreground hover:text-foreground">
                  <span className="text-xl mb-2 block">{s.icon}</span>
                  <span className="leading-relaxed">{s.label}</span>
                </button>
              ))}
            </div>
          </div>
        ) : (
          <div className="max-w-3xl mx-auto py-6 space-y-6">
            {messages.map((msg, i) => (
              <MessageBubble
                key={i}
                message={msg}
                appliedArtifacts={appliedArtifacts}
                existingToolNames={existingToolNames}
                existingPromptNames={existingPromptNames}
                onApplyTool={handleApplyTool}
                onApplyPrompt={handleApplyPrompt}
                isLast={i === messages.length - 1}
                isStreaming={isStreaming}
              />
            ))}
          </div>
        )}
      </div>

      <div className="sticky bottom-0 pt-4 pb-2 bg-background">
        <div className="max-w-3xl mx-auto">
          <div className="relative flex items-end bg-card rounded-none border border-border px-4 py-3 focus-within:border-foreground/20 transition-colors">
            <textarea
              ref={textareaRef}
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => { if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); handleSend() } }}
              placeholder={messages.length === 0 ? "Describe what you want to build..." : "Ask a follow up — tweak the UI, add features, fix bugs..."}
              rows={1}
              className="flex-1 bg-transparent text-sm text-foreground placeholder:text-muted-foreground resize-none focus:outline-none min-h-[24px] max-h-[200px] py-0.5 leading-6"
              disabled={isStreaming}
            />
            <button onClick={handleSend} disabled={!input.trim() || isStreaming}
              className="ml-2 shrink-0 w-8 h-8 rounded-none bg-primary text-primary-foreground flex items-center justify-center disabled:opacity-30 hover:opacity-50 transition-opacity">
              {isStreaming ? <Loader2 size={16} className="animate-spin" /> : <ArrowUp size={16} />}
            </button>
          </div>
          <p className="text-center text-[11px] text-muted-foreground/60 mt-2">
            AI generates live React components + backend tools. Review before applying.
          </p>
        </div>
      </div>
    </div>
  )
}

// --- Message Bubble ---

function MessageBubble({ message, appliedArtifacts, existingToolNames, existingPromptNames, onApplyTool, onApplyPrompt, isLast, isStreaming }: {
  message: ChatMessage
  appliedArtifacts: Set<string>
  existingToolNames: Set<string>
  existingPromptNames: Set<string>
  onApplyTool: (tool: StoreTool) => void
  onApplyPrompt: (prompt: StorePrompt) => void
  isLast: boolean
  isStreaming: boolean
}) {
  if (message.role === 'system') {
    return (
      <div className="text-xs text-destructive bg-destructive/10 rounded-none p-3 mx-auto max-w-md text-center">
        {message.content}
      </div>
    )
  }

  if (message.role === 'user') {
    return (
      <div className="flex items-start gap-4">
        <div className="w-7 h-7 rounded-none bg-primary flex items-center justify-center shrink-0 mt-0.5">
          <User size={14} className="text-white" />
        </div>
        <div className="flex-1 min-w-0">
          <p className="text-sm font-semibold mb-1">You</p>
          <p className="text-sm leading-relaxed text-foreground">{message.content}</p>
        </div>
      </div>
    )
  }

  const artifacts = extractArtifacts(message.content)
  const displayText = message.content
    .replace(/```(?:tsx:ui|tsx|json:tool|json:prompt|json)\s*\n[\s\S]*?```/g, '')
    .trim()

  return (
    <div className="flex items-start gap-4">
      <div className="w-7 h-7 rounded-none bg-foreground flex items-center justify-center shrink-0 mt-0.5">
        <Sparkles size={14} className="text-background" />
      </div>
      <div className="flex-1 min-w-0 space-y-3">
        <p className="text-sm font-semibold mb-1">App Builder</p>

        {displayText && (
          <div className="text-sm leading-relaxed text-foreground/90 whitespace-pre-wrap">
            {displayText}
            {isLast && isStreaming && (
              <span className="inline-block w-[3px] h-[18px] bg-foreground/60 animate-pulse ml-0.5 align-text-bottom" />
            )}
          </div>
        )}

        {!displayText && isLast && isStreaming && (
          <div className="text-sm">
            <span className="inline-block w-[3px] h-[18px] bg-foreground/60 animate-pulse" />
          </div>
        )}

        {artifacts.length > 0 && (
          <div className="space-y-3 mt-3">
            {artifacts.map((artifact, i) => {
              if (artifact.type === 'ui') {
                return <UIArtifactCard key={i} artifact={artifact} />
              }
              return (
                <ToolPromptArtifactCard
                  key={i}
                  artifact={artifact}
                  isApplied={appliedArtifacts.has(`${artifact.type}:${artifact.data.name}`)}
                  isUpdate={artifact.type === 'tool' ? existingToolNames.has(artifact.data.name) : existingPromptNames.has(artifact.data.name)}
                  onApplyTool={onApplyTool}
                  onApplyPrompt={onApplyPrompt}
                />
              )
            })}
          </div>
        )}
      </div>
    </div>
  )
}

// --- UI Artifact Card (live React preview) ---

function UIArtifactCard({ artifact }: { artifact: UIArtifact }) {
  const [showCode, setShowCode] = useState(false)
  const [copied, setCopied] = useState(false)

  const handleCopy = () => {
    navigator.clipboard.writeText(artifact.code)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className="rounded-none border border-border bg-card overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-2.5 border-b border-border bg-muted/30">
        <div className="flex items-center gap-2">
          <Eye size={14} className="text-muted-foreground" />
          <code className="text-sm font-semibold">{artifact.name}</code>
          <Badge variant="secondary" className="text-[10px] rounded-none font-mono">live UI</Badge>
        </div>
        <div className="flex items-center gap-1">
          <Button size="sm" variant="ghost" className="rounded-none h-7 text-xs font-mono" onClick={() => setShowCode(!showCode)}>
            <Code size={12} className="mr-1" /> {showCode ? "Preview" : "Code"}
          </Button>
          <Button size="sm" variant="ghost" className="rounded-none h-7 text-xs font-mono" onClick={handleCopy}>
            {copied ? <Check size={12} /> : <Sparkles size={12} />}
          </Button>
        </div>
      </div>

      {/* Live Preview or Code */}
      {showCode ? (
        <pre className="p-4 text-[11px] font-mono leading-relaxed overflow-auto max-h-96 bg-background">
          {artifact.code}
        </pre>
      ) : (
        <div className="p-4">
          <LivePreview code={artifact.code} />
        </div>
      )}
    </div>
  )
}

// --- Tool/Prompt Artifact Card (with inline test) ---

function ToolPromptArtifactCard({ artifact, isApplied, isUpdate, onApplyTool, onApplyPrompt }: {
  artifact: ToolArtifact | PromptArtifact
  isApplied: boolean
  isUpdate: boolean
  onApplyTool: (tool: StoreTool) => void
  onApplyPrompt: (prompt: StorePrompt) => void
}) {
  const isTool = artifact.type === 'tool'
  const toolData = isTool ? artifact.data as StoreTool : null
  const [showTest, setShowTest] = useState(false)
  const [showScript, setShowScript] = useState(false)

  return (
    <div className="rounded-none border border-border bg-card overflow-hidden">
      <div className="p-4 space-y-2.5">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            {isTool ? <Wrench size={14} className="text-muted-foreground" /> : <MessageSquare size={14} className="text-muted-foreground" />}
            <code className="text-sm font-semibold">{artifact.data.name}</code>
            {toolData && <Badge variant="outline" className="text-[10px] rounded-none font-mono">{toolData.toolClass}</Badge>}
            {toolData?.mode === "qa" && <Badge variant="outline" className="text-[10px] rounded-none font-mono">QA</Badge>}
            {isUpdate ? <Badge variant="secondary" className="text-[10px] rounded-none font-mono">update</Badge>
              : <Badge variant="secondary" className="text-[10px] rounded-none font-mono">new</Badge>}
          </div>
          <div className="flex items-center gap-1">
            {isTool && (
              <Button size="sm" variant="ghost" className="rounded-none h-7 text-xs font-mono" onClick={() => setShowTest(!showTest)}>
                <Play size={12} className="mr-1" /> {showTest ? "Hide" : "Test"}
              </Button>
            )}
            <Button size="sm" variant={isApplied ? "ghost" : "default"} disabled={isApplied}
              className="rounded-none h-7 text-xs font-mono uppercase tracking-wider"
              onClick={() => { if (isTool) onApplyTool(artifact.data as StoreTool); else onApplyPrompt(artifact.data as StorePrompt) }}>
              {isApplied ? <><Check size={12} className="mr-1" /> Applied</> : isUpdate ? <><RefreshCw size={12} className="mr-1" /> Update</> : <><Plus size={12} className="mr-1" /> Add</>}
            </Button>
          </div>
        </div>
        <p className="text-xs text-muted-foreground leading-relaxed">{artifact.data.description}</p>

        {toolData && Object.keys(toolData.params).length > 0 && (
          <div className="flex flex-wrap gap-1.5">
            {Object.entries(toolData.params).map(([name, def]) => (
              <span key={name} className="text-[10px] font-mono bg-muted px-1.5 py-0.5 rounded-none">
                {name}: {def.type}{def.required ? '*' : ''}{def.options?.length ? ` [${def.options.length}]` : ''}
              </span>
            ))}
          </div>
        )}

        {toolData && (
          <button onClick={() => setShowScript(!showScript)} className="text-[11px] font-mono text-muted-foreground hover:text-foreground transition-colors">
            {showScript ? "Hide script" : `View script (${toolData.script.split('\n').length} lines)`}
          </button>
        )}
        {showScript && toolData && (
          <pre className="p-3 rounded-none bg-background overflow-x-auto text-[11px] leading-relaxed border border-border max-h-60 overflow-y-auto">{toolData.script}</pre>
        )}

        {!isTool && (
          <details className="text-xs">
            <summary className="cursor-pointer text-muted-foreground hover:text-foreground">View prompt body</summary>
            <pre className="mt-2 p-3 rounded-none bg-background overflow-auto text-[11px] border border-border max-h-40">{(artifact.data as StorePrompt).body}</pre>
          </details>
        )}
      </div>

      {showTest && toolData && (
        <div className="border-t border-border p-4">
          <InlineToolTest tool={toolData} />
        </div>
      )}
    </div>
  )
}

// --- Inline tool test runner ---

function InlineToolTest({ tool }: { tool: StoreTool }) {
  const [paramValues, setParamValues] = useState<Record<string, unknown>>({})
  const [result, setResult] = useState<TestToolResponse | null>(null)
  const [showRaw, setShowRaw] = useState(false)
  const testTool = useTestTool()

  const handleRun = () => {
    testTool.mutate(
      { script: tool.script, params: paramValues, allowedHosts: ["*"], timeout: "30s" },
      { onSuccess: (data) => setResult(data) },
    )
  }

  const hasRequiredEmpty = Object.entries(tool.params).some(
    ([name, def]) => def.required && (paramValues[name] === undefined || paramValues[name] === ""),
  )

  return (
    <div className="space-y-3">
      <SchemaForm params={tool.params} values={paramValues} onChange={setParamValues} />
      <Button onClick={handleRun} disabled={testTool.isPending || hasRequiredEmpty} size="sm" className="rounded-none font-mono text-xs uppercase tracking-wider">
        {testTool.isPending ? <><Loader2 size={12} className="mr-1 animate-spin" /> Running</> : <><Play size={12} className="mr-1" /> Run</>}
      </Button>

      {testTool.isError && (
        <div className="rounded-none border border-destructive/30 bg-destructive/10 p-2 text-xs text-destructive font-mono">{(testTool.error as Error).message}</div>
      )}

      {result && (
        <div className="space-y-2">
          {result.error && (
            <div className="flex items-start gap-2 rounded-none border border-destructive/30 bg-destructive/10 p-2">
              <AlertTriangle size={12} className="text-destructive mt-0.5 shrink-0" />
              <pre className="text-xs font-mono text-destructive whitespace-pre-wrap">{result.error}</pre>
            </div>
          )}
          {result.output && (
            <div className="space-y-1">
              <div className="flex items-center justify-between">
                <span className="text-[11px] font-mono text-muted-foreground">{Math.round(result.duration_ms)}ms</span>
                <button onClick={() => setShowRaw(!showRaw)} className="text-[11px] font-mono text-muted-foreground hover:text-foreground">{showRaw ? "Formatted" : "Raw"}</button>
              </div>
              {showRaw ? (
                <pre className="text-[11px] font-mono bg-muted/50 border border-border p-2 overflow-auto max-h-60 whitespace-pre-wrap">{JSON.stringify(result.output, null, 2)}</pre>
              ) : (
                <div className="rounded-none border border-border p-2 overflow-auto max-h-60"><SpecRenderer spec={outputToSpec(result.output)} /></div>
              )}
            </div>
          )}
          {result.http_log?.length > 0 && <HttpTrace entries={result.http_log} />}
        </div>
      )}
    </div>
  )
}
