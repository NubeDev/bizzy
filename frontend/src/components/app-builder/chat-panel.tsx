/**
 * Chat panel for the App Builder.
 * Drives AI generation. Uses shared chat lib for rendering.
 */
import { useState, useRef, useEffect } from "react"
import { BookOpen, ChevronDown, ChevronRight, Sparkles, Trash2 } from "lucide-react"
import { useAgentChat, type ChatMessage } from "@/hooks/use-agent-chat"
import { ChatBubble, ChatInput, ChatSuggestions, useAutoScroll } from "@/lib/chat"
import { BouncingBalls } from "@/components/ui/bouncing-balls"
import { ProviderSelector } from "@/components/chat/provider-selector"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { extractFileBlocks, buildArchitectPrompt, OPTIONAL_BUILDER_PROMPTS } from "./prompts"
import { useBootstrapPrompts } from "@/hooks/use-bootstrap-prompts"
import type { AppProject, AppFile } from "./types"

interface Props {
  project: AppProject
  onFilesGenerated: (files: AppFile[]) => void
  pendingMessage?: string | null
  onPendingMessageConsumed?: () => void
  appName?: string
  initialMessages?: ChatMessage[]
  initialClaudeSessionId?: string
  onClearHistory?: () => void
}

export function ChatPanel({ project, onFilesGenerated, pendingMessage, onPendingMessageConsumed, appName, initialMessages, initialClaudeSessionId, onClearHistory }: Props) {
  const { compose } = useBootstrapPrompts()
  const [extraPrompts, setExtraPrompts] = useState<string[]>([])
  const [showPromptPicker, setShowPromptPicker] = useState(false)
  const [provider, setProvider] = useState('')
  const [model, setModel] = useState('')
  const { messages, isStreaming, send, clear } = useAgentChat({
    appName,
    initialMessages,
    initialClaudeSessionId,
    provider: provider || undefined,
    model: model || undefined,
  })
  const [input, setInput] = useState("")
  const scrollRef = useRef<HTMLDivElement>(null)
  const [processedMsgs, setProcessedMsgs] = useState<Set<number>>(new Set())
  const loadedCountRef = useRef(0)
  const [historyExpanded, setHistoryExpanded] = useState(false)

  useEffect(() => {
    if (initialMessages?.length && loadedCountRef.current === 0) {
      loadedCountRef.current = initialMessages.length
    }
  }, [initialMessages])

  useAutoScroll(scrollRef, [messages, isStreaming])

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
      ? buildArchitectPrompt(compose, project, extraPrompts) + "\n\nUser: " + pendingMessage
      : pendingMessage
    send(prompt, pendingMessage)
    onPendingMessageConsumed?.()
  }, [pendingMessage]) // eslint-disable-line react-hooks/exhaustive-deps

  const toggleExtraPrompt = (name: string) => {
    setExtraPrompts(prev => prev.includes(name) ? prev.filter(n => n !== name) : [...prev, name])
  }

  const handleSend = () => {
    if (!input.trim() || isStreaming) return
    const userText = input.trim()
    const prompt = messages.length === 0
      ? buildArchitectPrompt(compose, project, extraPrompts) + "\n\nUser: " + userText
      : userText
    send(prompt, userText)
    setInput("")
  }

  const stripFileBlocks = (content: string) =>
    content.replace(/```\w+:[^\n]+\n[\s\S]*?```/g, '').trim()

  const renderFileBadges = (msg: ChatMessage) => {
    const fileBlocks = extractFileBlocks(msg.content)
    if (fileBlocks.length === 0) return null
    return (
      <div className="flex flex-wrap gap-1">
        {fileBlocks.map((f, i) => (
          <Badge key={i} variant="secondary" className="text-[10px] rounded-none font-mono">{f.path}</Badge>
        ))}
      </div>
    )
  }

  const builderSuggestions = [
    { label: "Build a weather monitoring app with city lookup and AI travel advice", prompt: "Build a weather monitoring app with city lookup, conditions display, and AI travel advice" },
    { label: "Create a REST API tester with auth and response history", prompt: "Create a REST API tester with auth, saved endpoints, and response history" },
    { label: "Build a building automation app with IoT dashboards", prompt: "Build a building automation app that queries IoT nodes and shows dashboards" },
    { label: "Make a content review tool with guided Q&A", prompt: "Make a content review tool with guided Q&A and AI-powered feedback" },
  ]

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <div className="px-4 h-10 border-b border-border flex items-center justify-between shrink-0">
        <span className="text-xs font-mono font-medium uppercase tracking-wider" style={{ color: "#6366f1" }}>Chat</span>
        {messages.length > 0 && (
          <Button variant="ghost" size="sm" onClick={() => { clear(); setProcessedMsgs(new Set()); onClearHistory?.() }}
            className="text-muted-foreground hover:text-foreground rounded-none text-[11px] h-6">
            <Trash2 size={10} className="mr-1" /> Clear
          </Button>
        )}
      </div>

      {/* Messages */}
      <div ref={scrollRef} className="flex-1 min-h-0 overflow-y-auto scroll-smooth">
        {messages.length === 0 ? (
          <ChatSuggestions
            suggestions={builderSuggestions}
            onSelect={setInput}
            title=""
            subtitle="Describe your app and the AI will generate the full project."
            icon={<Sparkles size={24} style={{ color: "#6366f1" }} />}
            compact
          />
        ) : (
          <div className="p-3 space-y-4">
            {loadedCountRef.current > 0 && (
              <>
                <button
                  onClick={() => setHistoryExpanded(prev => !prev)}
                  className="flex items-center gap-1.5 w-full text-[11px] text-muted-foreground hover:text-foreground transition-colors font-mono"
                >
                  {historyExpanded ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
                  Previous session ({loadedCountRef.current} messages)
                </button>
                {historyExpanded && messages.slice(0, loadedCountRef.current).map((msg, i) => (
                  <ChatBubble key={`h-${i}`} message={msg} isLast={false} isStreaming={false}
                    compact accentColor="#6366f1" contentTransform={stripFileBlocks} extraContent={renderFileBadges(msg)} />
                ))}
                {!historyExpanded && <div className="border-b border-border/50" />}
              </>
            )}
            {messages.slice(loadedCountRef.current).map((msg, i) => {
              const idx = loadedCountRef.current + i
              return (
                <ChatBubble key={idx} message={msg} isLast={idx === messages.length - 1} isStreaming={isStreaming}
                  compact accentColor="#6366f1" contentTransform={stripFileBlocks} extraContent={renderFileBadges(msg)} />
              )
            })}
          </div>
        )}
      </div>

      {/* Input */}
      <div className="p-3 border-t border-border shrink-0">
        <ChatInput
          value={input}
          onChange={setInput}
          onSend={handleSend}
          isStreaming={isStreaming}
          placeholder={messages.length === 0 ? "Describe the app you want to build..." : "Make changes — add tools, fix bugs, update config..."}
          compact
          accentColor="#6366f1"
          header={
            <div className="pb-1.5 space-y-1.5">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <BouncingBalls active={isStreaming} size={8} />
                  <button
                    onClick={() => setShowPromptPicker(p => !p)}
                    className="flex items-center gap-1 text-[10px] font-mono text-muted-foreground hover:text-foreground transition-colors"
                  >
                    <BookOpen size={10} />
                    Context{extraPrompts.length > 0 && ` (${extraPrompts.length})`}
                  </button>
                </div>
                <ProviderSelector
                  provider={provider}
                  model={model}
                  onProviderChange={setProvider}
                  onModelChange={setModel}
                />
              </div>
              {showPromptPicker && (
                <div className="flex flex-wrap gap-1">
                  {OPTIONAL_BUILDER_PROMPTS.map(p => (
                    <button
                      key={p.name}
                      onClick={() => toggleExtraPrompt(p.name)}
                      className={`text-[10px] font-mono px-1.5 py-0.5 border transition-colors ${
                        extraPrompts.includes(p.name)
                          ? "bg-primary text-primary-foreground border-primary"
                          : "bg-background text-muted-foreground border-border hover:border-foreground/20"
                      }`}
                    >
                      {p.label}
                    </button>
                  ))}
                </div>
              )}
            </div>
          }
        />
      </div>
    </div>
  )
}
