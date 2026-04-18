import { useState, useCallback } from 'react'
import { User, Sparkles, AlertCircle, Wrench, Loader2, Zap, Copy, Check } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import type { ChatMessage } from '@/hooks/use-agent-chat'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter'
import { oneDark } from 'react-syntax-highlighter/dist/esm/styles/prism'

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
    <button
      onClick={() => copy(text)}
      title={copied ? "Copied!" : "Copy"}
      className={`flex items-center gap-1 text-[11px] font-mono text-muted-foreground hover:text-foreground transition-colors px-1.5 py-1 ${className}`}
    >
      {copied ? <Check size={12} className="text-emerald-400" /> : <Copy size={12} />}
      {copied ? "copied" : "copy"}
    </button>
  )
}

interface Props {
  message: ChatMessage
  isLast: boolean
  isStreaming: boolean
}

export function ChatMessageBubble({ message, isLast, isStreaming }: Props) {
  if (message.role === 'system') {
    return (
      <div className="flex items-start gap-2 px-3 py-2 text-xs text-destructive bg-destructive/5 border border-destructive/10">
        <AlertCircle size={13} className="mt-0.5 shrink-0" />
        <span>{message.content}</span>
      </div>
    )
  }

  if (message.role === 'user') {
    return (
      <div className="flex items-start gap-3">
        <div className="w-6 h-6 rounded-none bg-primary flex items-center justify-center shrink-0 mt-0.5">
          <User size={12} className="text-primary-foreground" />
        </div>
        <div className="flex-1 min-w-0 pt-0.5">
          <p className="text-sm leading-relaxed">{message.content}</p>
        </div>
      </div>
    )
  }

  const showCursor = isLast && isStreaming && message.status === 'streaming'

  return (
    <div className="flex items-start gap-3 group">
      <div className="w-6 h-6 rounded-none bg-foreground flex items-center justify-center shrink-0 mt-0.5">
        <Sparkles size={12} className="text-background" />
      </div>
      <div className="flex-1 min-w-0 space-y-1 pt-0.5">
        {/* Status indicator */}
        {isLast && isStreaming && !message.content && (
          <StatusIndicator status={message.status} />
        )}

        {/* Tool call badges */}
        {message.toolCalls && message.toolCalls.length > 0 && (
          <div className="flex flex-wrap gap-1 mb-1">
            {message.toolCalls.map((name, i) => (
              <Badge key={i} variant="outline" className="text-[10px] rounded-none font-mono gap-1 text-muted-foreground">
                <Wrench size={9} /> {name}
              </Badge>
            ))}
          </div>
        )}

        {/* Message content */}
        {message.content ? (
          <div className="markdown-body text-sm leading-relaxed max-w-none relative">
            <ReactMarkdown
              remarkPlugins={[remarkGfm]}
              components={{
                h1({ children }) {
                  return <h1 className="text-lg font-semibold mt-4 mb-2 text-foreground">{children}</h1>
                },
                h2({ children }) {
                  return <h2 className="text-base font-semibold mt-3 mb-1.5 text-foreground">{children}</h2>
                },
                h3({ children }) {
                  return <h3 className="text-sm font-semibold mt-2 mb-1 text-foreground">{children}</h3>
                },
                p({ children }) {
                  return <p className="my-2 text-foreground/90 leading-relaxed">{children}</p>
                },
                strong({ children }) {
                  return <strong className="font-semibold text-foreground">{children}</strong>
                },
                ul({ children }) {
                  return <ul className="my-2 ml-4 space-y-0.5 list-disc text-foreground/90">{children}</ul>
                },
                ol({ children }) {
                  return <ol className="my-2 ml-4 space-y-0.5 list-decimal text-foreground/90">{children}</ol>
                },
                li({ children }) {
                  return <li className="leading-relaxed">{children}</li>
                },
                blockquote({ children }) {
                  return <blockquote className="border-l-2 border-primary/30 pl-3 my-2 text-muted-foreground italic">{children}</blockquote>
                },
                hr() {
                  return <hr className="my-3 border-border" />
                },
                code({ className, children, ...props }) {
                  const match = /language-(\w+)/.exec(className || '')
                  const text = String(children).replace(/\n$/, '')

                  // Multi-line code block (has language or contains newlines)
                  if (match || text.includes('\n')) {
                    return (
                      <div className="relative group/code">
                        <div className="absolute top-2 right-2 z-10 opacity-0 group-hover/code:opacity-100 transition-opacity">
                          <CopyButton text={text} className="bg-black/40 backdrop-blur-sm border border-white/10 rounded" />
                        </div>
                        <SyntaxHighlighter
                          style={oneDark}
                          language={match?.[1] || 'text'}
                          PreTag="div"
                          customStyle={{
                            margin: 0,
                            borderRadius: 0,
                            fontSize: '12px',
                            lineHeight: '1.6',
                            padding: '12px 16px',
                          }}
                        >
                          {text}
                        </SyntaxHighlighter>
                      </div>
                    )
                  }

                  // Inline code
                  return (
                    <code className="px-1.5 py-0.5 bg-muted text-[12px] font-mono text-foreground/80" {...props}>
                      {children}
                    </code>
                  )
                },
                pre({ children }) {
                  return <div className="my-2 border border-border overflow-hidden">{children}</div>
                },
                table({ children }) {
                  return (
                    <div className="overflow-x-auto my-3 border border-border">
                      <table className="w-full text-xs">{children}</table>
                    </div>
                  )
                },
                thead({ children }) {
                  return <thead className="bg-muted/50">{children}</thead>
                },
                th({ children }) {
                  return <th className="px-3 py-2 text-left font-medium text-foreground border-b border-border text-xs">{children}</th>
                },
                td({ children }) {
                  return <td className="px-3 py-2 border-b border-border text-foreground/80">{children}</td>
                },
                a({ href, children }) {
                  return <a href={href} target="_blank" rel="noopener" className="text-primary underline underline-offset-2 hover:opacity-70">{children}</a>
                },
              }}
            >
              {message.content}
            </ReactMarkdown>
            {showCursor && (
              <span className="inline-block w-[2px] h-[14px] bg-foreground/50 animate-pulse ml-0.5 align-text-bottom" />
            )}
            {!isStreaming && (
              <div className="flex justify-end mt-2 opacity-0 group-hover:opacity-100 transition-opacity">
                <CopyButton text={message.content} />
              </div>
            )}
          </div>
        ) : null}
      </div>
    </div>
  )
}

function StatusIndicator({ status }: { status?: string }) {
  switch (status) {
    case 'connecting':
      return (
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          <Loader2 size={12} className="animate-spin" />
          <span>Connecting...</span>
        </div>
      )
    case 'thinking':
      return (
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          <Loader2 size={12} className="animate-spin" />
          <span>Thinking...</span>
        </div>
      )
    case 'tool_call':
      return (
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          <Zap size={12} className="text-yellow-500" />
          <span>Calling tools...</span>
        </div>
      )
    default:
      return (
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          <Loader2 size={12} className="animate-spin" />
          <span>Working...</span>
        </div>
      )
  }
}
