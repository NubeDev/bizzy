/**
 * Chat panel for the App Builder.
 * Drives AI generation. Renders markdown + syntax-highlighted code blocks.
 */
import { useState, useRef, useEffect, useCallback } from "react"
import { ArrowUp, Loader2, Sparkles, Trash2, User, Copy, Check } from "lucide-react"
import ReactMarkdown from "react-markdown"
import remarkGfm from "remark-gfm"
import { Prism as SyntaxHighlighter } from "react-syntax-highlighter"
import { oneDark } from "react-syntax-highlighter/dist/esm/styles/prism"
import { useAgentChat, type ChatMessage } from "@/hooks/use-agent-chat"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { extractFileBlocks, buildArchitectPrompt } from "./prompts"
import type { AppProject, AppFile } from "./types"

// --- Copy hook ---

function useCopy(ms = 1800) {
  const [copied, setCopied] = useState(false)
  const copy = useCallback((text: string) => {
    navigator.clipboard.writeText(text).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), ms)
    })
  }, [ms])
  return { copied, copy }
}

function CopyButton({ text, className = "" }: { text: string; className?: string }) {
  const { copied, copy } = useCopy()
  return (
    <button onClick={() => copy(text)} title={copied ? "Copied!" : "Copy"}
      className={`flex items-center gap-1 text-[10px] font-mono text-muted-foreground hover:text-foreground transition-colors ${className}`}>
      {copied ? <Check size={10} className="text-emerald-400" /> : <Copy size={10} />}
    </button>
  )
}

// --- Main ---

interface Props {
  project: AppProject
  onFilesGenerated: (files: AppFile[]) => void
  /** When set, auto-sends this message as a fix/update request then clears it */
  pendingMessage?: string | null
  onPendingMessageConsumed?: () => void
}

export function ChatPanel({ project, onFilesGenerated, pendingMessage, onPendingMessageConsumed }: Props) {
  const { messages, isStreaming, send, clear } = useAgentChat()
  const [input, setInput] = useState("")
  const scrollRef = useRef<HTMLDivElement>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const [processedMsgs, setProcessedMsgs] = useState<Set<number>>(new Set())

  // Auto-scroll on new messages
  useEffect(() => {
    if (scrollRef.current) {
      const el = scrollRef.current
      // Only auto-scroll if user is near bottom (within 100px)
      const isNearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 100
      if (isNearBottom) el.scrollTop = el.scrollHeight
    }
  }, [messages, isStreaming])

  // Auto-resize textarea
  useEffect(() => {
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto'
      textareaRef.current.style.height = Math.min(textareaRef.current.scrollHeight, 150) + 'px'
    }
  }, [input])

  // Extract files from new AI messages
  useEffect(() => {
    if (isStreaming) return
    messages.forEach((msg, i) => {
      if (msg.role !== 'assistant' || processedMsgs.has(i)) return
      const blocks = extractFileBlocks(msg.content)
      if (blocks.length > 0) {
        const files: AppFile[] = blocks.map(b => ({
          path: b.path,
          content: b.content,
          type: b.type as AppFile["type"],
          dirty: true,
        }))
        onFilesGenerated(files)
        setProcessedMsgs(prev => new Set(prev).add(i))
      }
    })
  }, [messages, isStreaming, processedMsgs, onFilesGenerated])

  // Auto-send pending fix/update messages from the preview panel
  useEffect(() => {
    if (!pendingMessage || isStreaming) return
    const prompt = messages.length === 0
      ? buildArchitectPrompt(project) + "\n\nUser: " + pendingMessage
      : pendingMessage
    send(prompt, pendingMessage)
    onPendingMessageConsumed?.()
  }, [pendingMessage]) // eslint-disable-line react-hooks/exhaustive-deps

  const handleSend = () => {
    if (!input.trim() || isStreaming) return
    const userText = input.trim()
    const prompt = messages.length === 0
      ? buildArchitectPrompt(project) + "\n\nUser: " + userText
      : userText
    send(prompt, userText)
    setInput("")
  }

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <div className="px-3 py-2 border-b border-border flex items-center justify-between shrink-0">
        <span className="text-xs font-mono font-medium text-muted-foreground uppercase tracking-wider">AI Architect</span>
        {messages.length > 0 && (
          <Button variant="ghost" size="sm" onClick={() => { clear(); setProcessedMsgs(new Set()) }}
            className="text-muted-foreground hover:text-foreground rounded-none text-[11px] h-6">
            <Trash2 size={10} className="mr-1" /> Clear
          </Button>
        )}
      </div>

      {/* Messages — scrollable */}
      <div ref={scrollRef} className="flex-1 min-h-0 overflow-y-auto scroll-smooth">
        {messages.length === 0 ? (
          <EmptyChat onSuggestion={setInput} />
        ) : (
          <div className="p-3 space-y-4">
            {messages.map((msg, i) => (
              <ChatBubble key={i} message={msg} isLast={i === messages.length - 1} isStreaming={isStreaming} />
            ))}
          </div>
        )}
      </div>

      {/* Input — fixed at bottom */}
      <div className="p-3 border-t border-border shrink-0">
        <div className="flex items-end gap-2 bg-card rounded-none border border-border px-3 py-2 focus-within:border-foreground/20 transition-colors">
          <textarea
            ref={textareaRef}
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); handleSend() } }}
            placeholder={messages.length === 0 ? "Describe the app you want to build..." : "Make changes — add tools, fix bugs, update config..."}
            rows={1}
            className="flex-1 bg-transparent text-xs text-foreground placeholder:text-muted-foreground resize-none focus:outline-none min-h-[20px] max-h-[150px] py-0.5 leading-5"
            disabled={isStreaming}
          />
          <button onClick={handleSend} disabled={!input.trim() || isStreaming}
            className="shrink-0 w-7 h-7 rounded-none bg-primary text-primary-foreground flex items-center justify-center disabled:opacity-30 hover:opacity-50 transition-opacity">
            {isStreaming ? <Loader2 size={12} className="animate-spin" /> : <ArrowUp size={12} />}
          </button>
        </div>
      </div>
    </div>
  )
}

// --- Empty state ---

function EmptyChat({ onSuggestion }: { onSuggestion: (s: string) => void }) {
  const suggestions = [
    "Build a weather monitoring app with city lookup, conditions display, and AI travel advice",
    "Create a REST API tester with auth, saved endpoints, and response history",
    "Build a building automation app that queries IoT nodes and shows dashboards",
    "Make a content review tool with guided Q&A and AI-powered feedback",
  ]

  return (
    <div className="flex flex-col items-center justify-center h-full px-4 py-8">
      <Sparkles size={24} className="text-muted-foreground/40 mb-3" />
      <p className="text-xs text-muted-foreground text-center mb-4">
        Describe your app and the AI will generate the full project.
      </p>
      <div className="space-y-2 w-full">
        {suggestions.map(s => (
          <button key={s} onClick={() => onSuggestion(s)}
            className="w-full text-left p-2.5 rounded-none border border-border hover:bg-accent text-[11px] text-muted-foreground hover:text-foreground transition-colors leading-relaxed">
            {s}
          </button>
        ))}
      </div>
    </div>
  )
}

// --- Chat bubble with proper markdown rendering ---

function ChatBubble({ message, isLast, isStreaming }: { message: ChatMessage; isLast: boolean; isStreaming: boolean }) {
  if (message.role === 'user') {
    return (
      <div className="flex items-start gap-2">
        <div className="w-5 h-5 rounded-none bg-primary flex items-center justify-center shrink-0 mt-0.5">
          <User size={10} className="text-white" />
        </div>
        <div className="flex-1 min-w-0">
          <p className="text-xs leading-relaxed text-foreground">{message.content}</p>
        </div>
      </div>
    )
  }

  // Strip file blocks from display — they show as badges instead
  const displayText = message.content
    .replace(/```\w+:[^\n]+\n[\s\S]*?```/g, '')
    .trim()

  const fileBlocks = extractFileBlocks(message.content)
  const showCursor = isLast && isStreaming

  return (
    <div className="flex items-start gap-2 group">
      <div className="w-5 h-5 rounded-none bg-foreground flex items-center justify-center shrink-0 mt-0.5">
        <Sparkles size={10} className="text-background" />
      </div>
      <div className="flex-1 min-w-0 space-y-2">
        {/* Copy entire message button */}
        {displayText && !isStreaming && (
          <div className="flex justify-end opacity-0 group-hover:opacity-100 transition-opacity -mb-1">
            <CopyButton text={displayText} />
          </div>
        )}

        {/* Rendered markdown */}
        {displayText && (
          <div className="text-xs leading-relaxed">
            <ReactMarkdown
              remarkPlugins={[remarkGfm]}
              components={{
                h1({ children }) { return <h1 className="text-sm font-bold mt-3 mb-1.5 text-foreground">{children}</h1> },
                h2({ children }) { return <h2 className="text-xs font-bold mt-2.5 mb-1 text-foreground">{children}</h2> },
                h3({ children }) { return <h3 className="text-xs font-semibold mt-2 mb-1 text-foreground">{children}</h3> },
                p({ children }) { return <p className="my-1.5 text-foreground/90 leading-relaxed">{children}</p> },
                strong({ children }) { return <strong className="font-semibold text-foreground">{children}</strong> },
                ul({ children }) { return <ul className="my-1.5 ml-3 space-y-0.5 list-disc text-foreground/90">{children}</ul> },
                ol({ children }) { return <ol className="my-1.5 ml-3 space-y-0.5 list-decimal text-foreground/90">{children}</ol> },
                li({ children }) { return <li className="leading-relaxed">{children}</li> },
                blockquote({ children }) { return <blockquote className="border-l-2 border-primary/30 pl-2 my-1.5 text-muted-foreground italic">{children}</blockquote> },
                hr() { return <hr className="my-2 border-border" /> },
                a({ href, children }) { return <a href={href} target="_blank" rel="noopener" className="text-primary underline underline-offset-2 hover:opacity-70">{children}</a> },
                table({ children }) {
                  return <div className="overflow-x-auto my-2 border border-border"><table className="w-full text-[11px]">{children}</table></div>
                },
                thead({ children }) { return <thead className="bg-muted/50">{children}</thead> },
                th({ children }) { return <th className="px-2 py-1 text-left font-medium text-foreground border-b border-border">{children}</th> },
                td({ children }) { return <td className="px-2 py-1 border-b border-border text-foreground/80">{children}</td> },
                code({ className, children, ...props }) {
                  const match = /language-(\w+)/.exec(className || '')
                  const text = String(children).replace(/\n$/, '')

                  if (match || text.includes('\n')) {
                    return (
                      <div className="relative group/code my-2">
                        <div className="absolute top-1.5 right-1.5 z-10 opacity-0 group-hover/code:opacity-100 transition-opacity">
                          <CopyButton text={text} className="bg-black/40 backdrop-blur-sm border border-white/10 rounded px-1.5 py-0.5" />
                        </div>
                        <SyntaxHighlighter
                          style={oneDark}
                          language={match?.[1] || 'text'}
                          PreTag="div"
                          customStyle={{ margin: 0, borderRadius: 0, fontSize: '11px', lineHeight: '1.5', padding: '10px 12px' }}
                        >
                          {text}
                        </SyntaxHighlighter>
                      </div>
                    )
                  }

                  return (
                    <code className="px-1 py-0.5 bg-muted text-[11px] font-mono text-foreground/80" {...props}>
                      {children}
                    </code>
                  )
                },
                pre({ children }) { return <div className="border border-border overflow-hidden">{children}</div> },
              }}
            >
              {displayText}
            </ReactMarkdown>
            {showCursor && <span className="inline-block w-[2px] h-[14px] bg-foreground/60 animate-pulse ml-0.5" />}
          </div>
        )}

        {/* Streaming with no text yet */}
        {!displayText && showCursor && (
          <div className="text-xs"><span className="inline-block w-[2px] h-[14px] bg-foreground/60 animate-pulse" /></div>
        )}

        {/* Generated file badges */}
        {fileBlocks.length > 0 && (
          <div className="flex flex-wrap gap-1">
            {fileBlocks.map((f, i) => (
              <Badge key={i} variant="secondary" className="text-[10px] rounded-none font-mono">{f.path}</Badge>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
