import { useState, useRef, useEffect } from "react"
import { ArrowUp, Trash2, Plus, Loader2, Sparkles, Wrench, MessageSquare, Check, User } from "lucide-react"
import { useAgentChat, type ChatMessage } from "@/hooks/use-agent-chat"
import { useAddTool, useAddPrompt } from "@/hooks/use-my-apps"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import type { StoreTool, StorePrompt } from "@/lib/types"

const SYSTEM_PROMPT_PREFIX = `You are an AI app builder for NubeIO. The user is building a store app and needs help creating tools and prompts.

IMPORTANT: When generating a tool, you MUST output a JSON code block with the marker "json:tool" like this:

\`\`\`json:tool
{
  "name": "tool_name",
  "description": "What the tool does",
  "toolClass": "read-only",
  "params": {
    "param_name": { "type": "string", "required": true, "description": "Param description" }
  },
  "script": "function handle(params) {\\n  const result = http.get('https://example.com/api');\\n  return { data: result };\\n}"
}
\`\`\`

When generating a prompt, use the marker "json:prompt":

\`\`\`json:prompt
{
  "name": "prompt_name",
  "description": "What the prompt does",
  "body": "Markdown template body with {{variable}} placeholders"
}
\`\`\`

CRITICAL: Always use \`\`\`json:tool or \`\`\`json:prompt markers, NOT plain \`\`\`json.

The JS runtime has these APIs:
- http.get(url), http.post(url, body), http.put(url, body), http.delete(url)
- secrets.get(key), config.get(key)
- log.info(msg), log.warn(msg), log.error(msg)

HTTP calls are restricted to the app's allowedHosts. The handle(params) function receives the tool's declared params. Keep scripts concise.

TESTING: After the user adds a tool to their app, they can ask you to test it. The app's tools are available via MCP with the naming pattern "appname.toolname". You have access to MCP tools — call them to test. If a tool needs allowedHosts to make HTTP calls, remind the user to add the domain in the Permissions tab.

The user's app context:
`

interface Props {
  appId: string
  appName: string
  appDescription: string
}

interface ExtractedArtifact {
  type: 'tool' | 'prompt'
  data: StoreTool | StorePrompt
}

function extractArtifacts(content: string): ExtractedArtifact[] {
  const artifacts: ExtractedArtifact[] = []

  // Match ```json:tool blocks
  const toolRegex = /```json:tool\s*\n([\s\S]*?)```/g
  let match
  while ((match = toolRegex.exec(content)) !== null) {
    try {
      const data = JSON.parse(match[1])
      if (data.name && data.script) {
        artifacts.push({ type: 'tool', data: { name: data.name, description: data.description || '', toolClass: data.toolClass || 'read-only', mode: data.mode || '', params: data.params || {}, script: data.script } })
      }
    } catch { /* skip */ }
  }

  // Match ```json:prompt blocks
  const promptRegex = /```json:prompt\s*\n([\s\S]*?)```/g
  while ((match = promptRegex.exec(content)) !== null) {
    try {
      const data = JSON.parse(match[1])
      if (data.name && data.body) {
        artifacts.push({ type: 'prompt', data: { name: data.name, description: data.description || '', body: data.body } })
      }
    } catch { /* skip */ }
  }

  // Fallback: also try plain ```json blocks that look like tools/prompts
  if (artifacts.length === 0) {
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

  return artifacts
}

export function AIWorkshop({ appId, appName, appDescription }: Props) {
  const { messages, isStreaming, send, clear } = useAgentChat()
  const addToolMutation = useAddTool()
  const addPromptMutation = useAddPrompt()
  const [input, setInput] = useState("")
  const scrollRef = useRef<HTMLDivElement>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const [addedArtifacts, setAddedArtifacts] = useState<Set<string>>(new Set())

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
  }, [messages])

  // Auto-resize textarea
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
      ? `${SYSTEM_PROMPT_PREFIX}App name: "${appName}", Description: "${appDescription}"\n\nUser request: ${userText}`
      : userText
    send(prompt, userText)
    setInput("")
  }

  const handleAddTool = async (tool: StoreTool) => {
    await addToolMutation.mutateAsync({ appId, tool })
    setAddedArtifacts(prev => new Set(prev).add(`tool:${tool.name}`))
  }

  const handleAddPrompt = async (prompt: StorePrompt) => {
    await addPromptMutation.mutateAsync({ appId, prompt })
    setAddedArtifacts(prev => new Set(prev).add(`prompt:${prompt.name}`))
  }

  return (
    <div className="flex flex-col h-[calc(100vh-200px)] min-h-[500px] relative">
      {/* Clear button - floating top right */}
      {messages.length > 0 && (
        <div className="absolute top-0 right-0 z-10">
          <Button
            variant="ghost"
            size="sm"
            onClick={clear}
            className="text-muted-foreground hover:text-foreground rounded-none text-xs h-8"
          >
            <Trash2 size={14} className="mr-1" /> Clear
          </Button>
        </div>
      )}

      {/* Messages area */}
      <div ref={scrollRef} className="flex-1 overflow-y-auto">
        {messages.length === 0 ? (
          /* ChatGPT-style empty state */
          <div className="flex flex-col items-center justify-center h-full px-4 relative">
            {/* Subtle dot pattern bg */}
            <div className="absolute inset-0 dots-pattern opacity-40 pointer-events-none" />

            <div className="relative w-16 h-16 rounded-none bg-secondary flex items-center justify-center mb-6 border border-border">
              <Sparkles size={28} className="text-foreground" />
            </div>
            <h1 className="font-mono text-2xl lg:text-3xl font-light mb-2 text-foreground">What do you want to build?</h1>
            <p className="text-muted-foreground text-sm mb-10 text-center max-w-md leading-relaxed">
              Describe a tool or prompt, and I'll generate the code you can add to your app.
            </p>
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 w-full max-w-2xl relative">
              {[
                { label: "Create a tool that checks if a URL is reachable", icon: "🔗" },
                { label: "Make a prompt for writing release notes", icon: "📝" },
                { label: "Build a tool that queries a REST API", icon: "🔌" },
              ].map((s) => (
                <button
                  key={s.label}
                  onClick={() => setInput(s.label)}
                  className="text-left p-4 rounded-none border border-border bg-card hover:bg-accent transition-colors text-sm text-muted-foreground hover:text-foreground"
                >
                  <span className="text-2xl mb-3 block">{s.icon}</span>
                  <span className="leading-relaxed">{s.label}</span>
                </button>
              ))}
            </div>
          </div>
        ) : (
          /* Conversation */
          <div className="max-w-3xl mx-auto py-6 space-y-6">
            {messages.map((msg, i) => (
              <MessageBubble
                key={i}
                message={msg}
                addedArtifacts={addedArtifacts}
                onAddTool={handleAddTool}
                onAddPrompt={handleAddPrompt}
                isLast={i === messages.length - 1}
                isStreaming={isStreaming}
              />
            ))}
          </div>
        )}
      </div>

      {/* ChatGPT-style floating input bar */}
      <div className="sticky bottom-0 pt-4 pb-2 bg-background">
        <div className="max-w-3xl mx-auto">
          <div className="relative flex items-end bg-card rounded-none border border-border px-4 py-3 focus-within:border-foreground/20 transition-colors">
            <textarea
              ref={textareaRef}
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && !e.shiftKey) {
                  e.preventDefault()
                  handleSend()
                }
              }}
              placeholder={messages.length === 0 ? "Describe the tool or prompt you want..." : "Ask a follow up..."}
              rows={1}
              className="flex-1 bg-transparent text-sm text-foreground placeholder:text-muted-foreground resize-none focus:outline-none min-h-[24px] max-h-[200px] py-0.5 leading-6"
              disabled={isStreaming}
            />
            <button
              onClick={handleSend}
              disabled={!input.trim() || isStreaming}
              className="ml-2 shrink-0 w-8 h-8 rounded-none bg-primary text-primary-foreground flex items-center justify-center disabled:opacity-30 disabled:cursor-not-allowed hover:opacity-50 transition-opacity"
            >
              {isStreaming ? <Loader2 size={16} className="animate-spin" /> : <ArrowUp size={16} />}
            </button>
          </div>
          <p className="text-center text-[11px] text-muted-foreground/60 mt-2">
            AI can make mistakes. Review generated tools before adding.
          </p>
        </div>
      </div>
    </div>
  )
}

function MessageBubble({ message, addedArtifacts, onAddTool, onAddPrompt, isLast, isStreaming }: {
  message: ChatMessage
  addedArtifacts: Set<string>
  onAddTool: (tool: StoreTool) => void
  onAddPrompt: (prompt: StorePrompt) => void
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

  // Assistant message — ChatGPT style: left-aligned, no bubble
  const artifacts = extractArtifacts(message.content)
  const displayText = message.content
    .replace(/```(?:json:tool|json:prompt|json)\s*\n[\s\S]*?```/g, '')
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
          <div className="space-y-2 mt-3">
            {artifacts.map((artifact, i) => {
              const key = `${artifact.type}:${artifact.data.name}`
              const isAdded = addedArtifacts.has(key)
              const isTool = artifact.type === 'tool'
              const toolData = isTool ? artifact.data as StoreTool : null

              return (
                <div key={i} className="rounded-none border border-border bg-card p-4 space-y-2.5">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      {isTool
                        ? <Wrench size={14} className="text-muted-foreground" />
                        : <MessageSquare size={14} className="text-muted-foreground" />
                      }
                      <code className="text-sm font-semibold">{artifact.data.name}</code>
                      {toolData && <Badge variant="outline" className="text-[10px] rounded-none font-mono">{toolData.toolClass}</Badge>}
                      <Badge variant="secondary" className="text-[10px] rounded-none font-mono">{artifact.type}</Badge>
                    </div>
                    <Button
                      size="sm"
                      variant={isAdded ? "ghost" : "default"}
                      disabled={isAdded}
                      className="rounded-none h-8 text-xs font-mono uppercase tracking-wider"
                      onClick={() => {
                        if (isTool) onAddTool(artifact.data as StoreTool)
                        else onAddPrompt(artifact.data as StorePrompt)
                      }}
                    >
                      {isAdded ? (
                        <><Check size={12} className="mr-1" /> Added</>
                      ) : (
                        <><Plus size={12} className="mr-1" /> Add to App</>
                      )}
                    </Button>
                  </div>
                  <p className="text-xs text-muted-foreground leading-relaxed">{artifact.data.description}</p>
                  {toolData && (
                    <details className="text-xs">
                      <summary className="cursor-pointer text-muted-foreground hover:text-foreground transition-colors">
                        View script
                      </summary>
                      <pre className="mt-2 p-3 rounded-none bg-background overflow-x-auto text-[11px] leading-relaxed border border-border">
                        {toolData.script}
                      </pre>
                    </details>
                  )}
                </div>
              )
            })}
          </div>
        )}
      </div>
    </div>
  )
}
