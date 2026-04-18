/**
 * Inline AI editor for an existing tool.
 * User describes a change ("add humidity to the output"), AI rewrites the script + params.
 * Shows diff-style before/after and lets user apply or discard.
 */
import { useState, useRef, useEffect } from "react"
import { ArrowUp, Loader2, Sparkles, Check, X, RotateCcw } from "lucide-react"
import { useAgentChat } from "@/hooks/use-agent-chat"
import { useBootstrapPrompts } from "@/hooks/use-bootstrap-prompts"
import { Button } from "@/components/ui/button"
import type { StoreTool, ToolParam } from "@/lib/types"

interface Props {
  tool: StoreTool
  onApply: (updated: StoreTool) => void
  onClose: () => void
}

export function AIToolEditor({ tool, onApply, onClose }: Props) {
  const { compose } = useBootstrapPrompts()
  const { messages, isStreaming, send, clear } = useAgentChat()
  const [input, setInput] = useState("")
  const [updatedTool, setUpdatedTool] = useState<StoreTool | null>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  // Auto-resize textarea
  useEffect(() => {
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto'
      textareaRef.current.style.height = Math.min(textareaRef.current.scrollHeight, 120) + 'px'
    }
  }, [input])

  // Watch for AI response and extract updated tool
  const lastAssistantMsg = messages.filter(m => m.role === 'assistant').pop()
  const lastContent = lastAssistantMsg?.content || ''

  useEffect(() => {
    if (isStreaming || !lastContent || updatedTool) return
    const toolRegex = /```json:tool\s*\n([\s\S]*?)```/
    const match = lastContent.match(toolRegex) || lastContent.match(/```json\s*\n([\s\S]*?)```/)
    if (match) {
      try {
        const parsed = JSON.parse(match[1])
        if (parsed.name && parsed.script) {
          setUpdatedTool({
            name: parsed.name,
            description: parsed.description || tool.description,
            toolClass: parsed.toolClass || tool.toolClass,
            mode: parsed.mode || tool.mode || '',
            params: parsed.params || tool.params,
            script: parsed.script,
          })
        }
      } catch { /* skip */ }
    }
  }, [isStreaming, lastContent, updatedTool, tool])

  const handleSend = () => {
    if (!input.trim() || isStreaming) return
    setUpdatedTool(null)
    const toolJson = JSON.stringify({
      name: tool.name,
      description: tool.description,
      toolClass: tool.toolClass,
      params: tool.params,
      script: tool.script,
    }, null, 2)
    const reference = compose(["tool_editor", "tool_naming"])
    const prompt = reference + "\n\nCurrent tool:\n" + toolJson + "\n\nUser's requested change:\n" + input.trim()
    send(prompt, input.trim())
    setInput("")
  }

  const handleApply = () => {
    if (updatedTool) {
      onApply(updatedTool)
      clear()
      onClose()
    }
  }

  const handleRetry = () => {
    setUpdatedTool(null)
    clear()
  }

  // Count changes
  const changes: string[] = []
  if (updatedTool) {
    if (updatedTool.script !== tool.script) changes.push("script")
    if (updatedTool.description !== tool.description) changes.push("description")
    if (JSON.stringify(updatedTool.params) !== JSON.stringify(tool.params)) changes.push("params")
    if (updatedTool.toolClass !== tool.toolClass) changes.push("toolClass")
  }

  return (
    <div className="border border-border rounded-none p-4 space-y-3 bg-accent/20">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Sparkles size={14} className="text-muted-foreground" />
          <span className="text-xs font-mono font-medium text-muted-foreground uppercase tracking-wider">Edit with AI</span>
        </div>
        <Button variant="ghost" size="icon" className="h-6 w-6 rounded-none" onClick={onClose}>
          <X size={14} />
        </Button>
      </div>

      {/* Input */}
      {!updatedTool && (
        <div className="flex items-end gap-2">
          <textarea
            ref={textareaRef}
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); handleSend() } }}
            placeholder='e.g. "also return humidity and wind speed", "add a date parameter", "handle errors better"'
            rows={1}
            className="flex-1 bg-background border border-border rounded-none px-3 py-2 text-xs font-mono resize-none focus:outline-none focus:border-foreground/20 min-h-[32px] max-h-[120px]"
            disabled={isStreaming}
            autoFocus
          />
          <button
            onClick={handleSend}
            disabled={!input.trim() || isStreaming}
            className="shrink-0 w-8 h-8 rounded-none bg-primary text-primary-foreground flex items-center justify-center disabled:opacity-30"
          >
            {isStreaming ? <Loader2 size={14} className="animate-spin" /> : <ArrowUp size={14} />}
          </button>
        </div>
      )}

      {/* Streaming indicator */}
      {isStreaming && !updatedTool && (
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          <Loader2 size={12} className="animate-spin" />
          Rewriting {tool.name}...
        </div>
      )}

      {/* Updated tool preview */}
      {updatedTool && (
        <div className="space-y-3">
          <div className="text-xs text-muted-foreground">
            Changed: {changes.length > 0 ? changes.join(", ") : "no changes detected"}
          </div>

          {/* Description change */}
          {updatedTool.description !== tool.description && (
            <div className="space-y-1">
              <span className="text-[11px] font-mono text-muted-foreground">Description:</span>
              <div className="text-xs bg-red-500/10 border-l-2 border-red-500 px-2 py-1 line-through text-muted-foreground">{tool.description}</div>
              <div className="text-xs bg-emerald-500/10 border-l-2 border-emerald-500 px-2 py-1">{updatedTool.description}</div>
            </div>
          )}

          {/* Params change */}
          {JSON.stringify(updatedTool.params) !== JSON.stringify(tool.params) && (
            <div className="space-y-1">
              <span className="text-[11px] font-mono text-muted-foreground">Params:</span>
              <div className="grid grid-cols-2 gap-2 text-[11px] font-mono">
                <div className="bg-red-500/10 border border-red-500/20 p-2 rounded-none">
                  {Object.entries(tool.params).map(([k, v]) => (
                    <div key={k}>{k}: {(v as ToolParam).type}{(v as ToolParam).required ? '*' : ''}</div>
                  ))}
                  {Object.keys(tool.params).length === 0 && <span className="text-muted-foreground">none</span>}
                </div>
                <div className="bg-emerald-500/10 border border-emerald-500/20 p-2 rounded-none">
                  {Object.entries(updatedTool.params).map(([k, v]) => (
                    <div key={k} className={!(k in tool.params) ? "text-emerald-400" : ""}>
                      {k}: {(v as ToolParam).type}{(v as ToolParam).required ? '*' : ''}
                      {!(k in tool.params) && " (new)"}
                    </div>
                  ))}
                </div>
              </div>
            </div>
          )}

          {/* Script change */}
          {updatedTool.script !== tool.script && (
            <details className="text-xs">
              <summary className="cursor-pointer text-muted-foreground hover:text-foreground transition-colors font-mono">
                View updated script ({updatedTool.script.split('\n').length} lines)
              </summary>
              <pre className="mt-2 p-3 rounded-none bg-background overflow-x-auto text-[11px] leading-relaxed border border-border max-h-60 overflow-y-auto">
                {updatedTool.script}
              </pre>
            </details>
          )}

          {/* Action buttons */}
          <div className="flex items-center gap-2">
            <Button size="sm" className="rounded-none text-xs" onClick={handleApply}>
              <Check size={12} className="mr-1" /> Apply Changes
            </Button>
            <Button size="sm" variant="outline" className="rounded-none text-xs" onClick={handleRetry}>
              <RotateCcw size={12} className="mr-1" /> Try Again
            </Button>
            <Button size="sm" variant="ghost" className="rounded-none text-xs" onClick={onClose}>
              Discard
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
