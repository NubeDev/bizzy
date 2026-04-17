/**
 * File editor panel — syntax-aware editing for different file types.
 * YAML/JSON: monospace editor with line numbers
 * Markdown: edit/preview toggle
 * TSX: code editor + live preview toggle
 * JS: code editor with line numbers
 */
import { useState, useRef, useEffect } from "react"
import { Eye, Code, FileCode, Copy, Check } from "lucide-react"
import { LivePreview } from "@/components/live-preview/renderer"
import { Button } from "@/components/ui/button"
import type { AppFile } from "./types"

interface Props {
  file: AppFile
  onUpdate: (path: string, content: string) => void
  toolRunFn?: (toolName: string, params?: Record<string, unknown>) => Promise<unknown>
}

export function FileEditor({ file, onUpdate, toolRunFn }: Props) {
  const isTsx = file.type === "tsx"
  const isMd = file.type === "md"
  const [showPreview, setShowPreview] = useState(false)
  const [copied, setCopied] = useState(false)

  const handleCopy = () => {
    navigator.clipboard.writeText(file.content)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <div className="px-3 py-1.5 border-b border-border flex items-center justify-between shrink-0 bg-muted/30">
        <div className="flex items-center gap-2">
          <FileCode size={12} className="text-muted-foreground" />
          <code className="text-[11px] font-mono text-foreground">{file.path}</code>
          <span className="text-[10px] font-mono text-muted-foreground uppercase">{file.type}</span>
          {file.dirty && <span className="w-1.5 h-1.5 rounded-full bg-blue-400" />}
        </div>
        <div className="flex items-center gap-1">
          {(isTsx || isMd) && (
            <Button variant="ghost" size="sm" className="h-6 rounded-none text-[10px] font-mono px-2"
              onClick={() => setShowPreview(!showPreview)}>
              {showPreview ? <><Code size={10} className="mr-1" /> Code</> : <><Eye size={10} className="mr-1" /> Preview</>}
            </Button>
          )}
          <Button variant="ghost" size="sm" className="h-6 rounded-none text-[10px] px-2" onClick={handleCopy}>
            {copied ? <Check size={10} /> : <Copy size={10} />}
          </Button>
        </div>
      </div>

      {/* Content */}
      <div className="flex-1 min-h-0 overflow-hidden">
        {isTsx && showPreview ? (
          <div className="h-full overflow-auto p-4 bg-background">
            <LivePreview code={file.content} toolRunFn={toolRunFn} />
          </div>
        ) : isMd && showPreview ? (
          <div className="h-full overflow-auto p-4 bg-background">
            <MarkdownPreview content={file.content} />
          </div>
        ) : (
          <CodeEditor
            value={file.content}
            onChange={(v) => onUpdate(file.path, v)}
            language={file.type}
          />
        )}
      </div>
    </div>
  )
}

/** Code editor with line numbers */
function CodeEditor({ value, onChange, language }: { value: string; onChange: (v: string) => void; language: string }) {
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const lineNumbersRef = useRef<HTMLDivElement>(null)
  const lines = value.split("\n")

  // Sync scroll between line numbers and textarea
  const handleScroll = () => {
    if (textareaRef.current && lineNumbersRef.current) {
      lineNumbersRef.current.scrollTop = textareaRef.current.scrollTop
    }
  }

  // Color hint for language
  const langColor = language === "js" ? "text-yellow-500/40" : language === "json" ? "text-blue-500/40" : language === "yaml" ? "text-green-500/40" : language === "md" ? "text-purple-500/40" : "text-muted-foreground/40"

  return (
    <div className="h-full flex relative">
      {/* Line numbers */}
      <div
        ref={lineNumbersRef}
        className="shrink-0 overflow-hidden select-none bg-muted/20 border-r border-border"
        style={{ width: `${Math.max(lines.length.toString().length * 8 + 16, 36)}px` }}
      >
        <div className="py-2">
          {lines.map((_, i) => (
            <div key={i} className="text-[11px] font-mono text-muted-foreground/50 text-right pr-2 leading-[20px]">
              {i + 1}
            </div>
          ))}
        </div>
      </div>

      {/* Editor */}
      <textarea
        ref={textareaRef}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onScroll={handleScroll}
        spellCheck={false}
        className="flex-1 font-mono text-[12px] leading-[20px] p-2 bg-background text-foreground resize-none focus:outline-none border-0 focus-visible:ring-0"
        style={{ tabSize: 2 }}
      />

      {/* Language indicator */}
      <div className={`absolute bottom-2 right-3 text-[10px] font-mono ${langColor} pointer-events-none`}>
        {language.toUpperCase()}
      </div>
    </div>
  )
}

/** Simple markdown preview */
function MarkdownPreview({ content }: { content: string }) {
  // Strip YAML frontmatter
  const body = content.replace(/^---\n[\s\S]*?\n---\n/, '').trim()

  return (
    <div className="prose prose-sm prose-invert max-w-none text-sm leading-relaxed">
      {body.split("\n\n").map((block, i) => {
        if (block.startsWith("# ")) return <h1 key={i} className="text-lg font-bold mt-4 mb-2">{block.slice(2)}</h1>
        if (block.startsWith("## ")) return <h2 key={i} className="text-base font-semibold mt-3 mb-1">{block.slice(3)}</h2>
        if (block.startsWith("### ")) return <h3 key={i} className="text-sm font-semibold mt-2 mb-1">{block.slice(4)}</h3>
        if (block.startsWith("```")) {
          const lines = block.split("\n")
          const code = lines.slice(1, -1).join("\n")
          return <pre key={i} className="bg-muted p-3 rounded-none text-[11px] font-mono overflow-auto my-2">{code}</pre>
        }
        if (block.startsWith("- ")) {
          return (
            <ul key={i} className="list-disc list-inside space-y-0.5 my-1">
              {block.split("\n").map((line, j) => <li key={j} className="text-sm">{line.replace(/^-\s*/, '')}</li>)}
            </ul>
          )
        }
        return <p key={i} className="my-1">{block}</p>
      })}
    </div>
  )
}

/** Empty state when no file is selected */
export function EmptyEditor() {
  return (
    <div className="h-full flex items-center justify-center text-center px-8 bg-background">
      <div>
        <FileCode size={32} className="mx-auto mb-3 text-muted-foreground/20" />
        <p className="text-sm text-muted-foreground">Select a file to edit</p>
        <p className="text-xs text-muted-foreground/60 mt-1">or describe your app in the chat</p>
      </div>
    </div>
  )
}
