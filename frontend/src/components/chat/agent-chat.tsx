import { useState, useRef, useEffect } from 'react'
import { ArrowUp, Loader2, Trash2 } from 'lucide-react'
import { BouncingBalls } from '@/components/ui/bouncing-balls'
import { useAgentChat } from '@/hooks/use-agent-chat'
import { useCommandPicker, type PickerItem } from '@/hooks/use-command-picker'
import { CommandPicker } from './command-picker'
import { ChatMessageBubble } from './chat-message'

interface Props {
  /** System prompt prepended to the first message. Gives the AI context about what app/page it's in. */
  systemPrompt?: string
  /** Placeholder text for the input */
  placeholder?: string
  /** Suggested prompts shown in empty state */
  suggestions?: { label: string; prompt: string }[]
  /** CSS class for the outer container */
  className?: string
  /** Resume an existing Claude session by ID */
  resumeSessionId?: string | null
}

export function AgentChat({ systemPrompt, placeholder, suggestions, className, resumeSessionId }: Props) {
  const { messages, isStreaming, send, clear, resumeSession } = useAgentChat()

  // If a resumeSessionId is provided, join that session on mount
  useEffect(() => {
    if (resumeSessionId) {
      resumeSession(resumeSessionId)
    }
  }, [resumeSessionId, resumeSession])
  const picker = useCommandPicker()
  const [input, setInput] = useState('')
  const scrollRef = useRef<HTMLDivElement>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
  }, [messages])

  useEffect(() => {
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto'
      textareaRef.current.style.height = Math.min(textareaRef.current.scrollHeight, 160) + 'px'
    }
  }, [input])

  const [pendingPromptItem, setPendingPromptItem] = useState<PickerItem | null>(null)
  const [promptOptions, setPromptOptions] = useState<string[]>([])

  const handleSend = (text?: string, promptBody?: string) => {
    const value = (text || input).trim()
    if (!value || isStreaming) return

    setInput('')
    setPendingPromptItem(null)
    setPromptOptions([])
    picker.dismiss()

    let prompt = promptBody || value
    const displayText = value

    // If user typed "/name args" and we have a pending prompt-mode item, fill in the template
    if (!promptBody && pendingPromptItem?.prompt && value.startsWith('/')) {
      const args = value.slice(1 + pendingPromptItem.name.length).trim()
      prompt = pendingPromptItem.prompt.replace(/\{\{[^}]+\}\}/g, args || '(not specified — use your best judgment)')
      console.log('[chat] resolved prompt template with args:', args)
    }

    // Prepend system prompt on first message
    if (messages.length === 0 && systemPrompt) {
      prompt = `${systemPrompt}\n\n${prompt}`
    }

    console.log('[chat] sending:', { displayText, promptLength: prompt.length, promptPreview: prompt.slice(0, 120) })
    send(prompt, displayText)
  }

  const handleOptionClick = (option: string) => {
    if (!pendingPromptItem) return
    handleSend(`/${pendingPromptItem.name} ${option}`)
  }

  const handlePickerSelect = (item: PickerItem) => {
    console.log('[picker] selected:', {
      name: item.name,
      type: item.type,
      mode: item.mode,
      hasPrompt: !!item.prompt,
      prompt: item.prompt?.slice(0, 80),
      arguments: item.arguments,
    })
    picker.dismiss()

    const hasArgs = item.arguments && item.arguments.length > 0

    if (hasArgs) {
      // Collect all options from params
      const allOptions = item.arguments!.flatMap(a => a.options || [])

      setInput(`/${item.name} `)
      setPromptOptions(allOptions)
      if (item.prompt) setPendingPromptItem(item)
    } else {
      handleSend(`/${item.name}`, item.prompt || undefined)
      return
    }
    textareaRef.current?.focus()
  }

  const handleInputChange = (value: string) => {
    const handled = picker.handleInputChange(value)
    setInput(value)
    if (promptOptions.length && !value.startsWith('/')) {
      setPromptOptions([])
      setPendingPromptItem(null)
    }
    if (!handled) return
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (picker.isOpen) {
      if (e.key === 'ArrowDown') {
        e.preventDefault()
        picker.moveSelection('down')
        return
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault()
        picker.moveSelection('up')
        return
      }
      if (e.key === 'Enter') {
        e.preventDefault()
        const selected = picker.getSelected()
        if (selected) handlePickerSelect(selected)
        return
      }
      if (e.key === 'Escape') {
        e.preventDefault()
        picker.dismiss()
        setInput('')
        return
      }
      if (e.key === 'Tab') {
        e.preventDefault()
        const selected = picker.getSelected()
        if (selected) handlePickerSelect(selected)
        return
      }
    }

    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  return (
    <div className={`flex flex-col h-full ${className || ''}`}>
      {/* Messages */}
      <div ref={scrollRef} className="flex-1 overflow-y-auto">
        {messages.length === 0 ? (
          <EmptyState
            suggestions={suggestions}
            onInputChange={(text) => { setInput(text); textareaRef.current?.focus() }}
          />
        ) : (
          <div className="max-w-3xl mx-auto py-4 px-4 space-y-5">
            {messages.map((msg, i) => (
              <ChatMessageBubble
                key={i}
                message={msg}
                isLast={i === messages.length - 1}
                isStreaming={isStreaming}
              />
            ))}
            {isStreaming && (
              <div className="pl-1 py-2">
                <BouncingBalls active={true} size={10} />
              </div>
            )}
          </div>
        )}
      </div>

      {/* Input bar */}
      <div className="sticky bottom-0 pt-3 pb-2 px-4 bg-background">
        <div className="max-w-3xl mx-auto relative">
          {picker.isOpen && (
            <CommandPicker
              items={picker.items}
              grouped={picker.grouped}
              selectedIndex={picker.selectedIndex}
              onSelect={handlePickerSelect}
            />
          )}

          <div className="flex items-end bg-card border border-border px-3 py-2 focus-within:border-foreground/20 transition-colors">
            <textarea
              ref={textareaRef}
              value={input}
              onChange={(e) => handleInputChange(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder={placeholder || 'Ask anything... (/ for prompts, # for tools)'}
              rows={1}
              className="flex-1 bg-transparent text-sm text-foreground placeholder:text-muted-foreground resize-none focus:outline-none min-h-[24px] max-h-[160px] py-0.5 leading-6"
              disabled={isStreaming}
            />
            <div className="flex items-center gap-1 ml-2">
              {messages.length > 0 && (
                <button
                  onClick={clear}
                  className="w-7 h-7 flex items-center justify-center text-muted-foreground hover:text-foreground transition-colors"
                  title="Clear"
                >
                  <Trash2 size={14} />
                </button>
              )}
              <button
                onClick={() => handleSend()}
                disabled={!input.trim() || isStreaming}
                className="w-7 h-7 bg-primary text-primary-foreground flex items-center justify-center disabled:opacity-30 hover:opacity-70 transition-opacity"
              >
                {isStreaming ? <Loader2 size={14} className="animate-spin" /> : <ArrowUp size={14} />}
              </button>
            </div>
          </div>

          {promptOptions.length > 0 ? (
            <div className="flex items-center gap-1.5 mt-2 flex-wrap">
              {promptOptions.map(opt => (
                <button
                  key={opt}
                  onClick={() => handleOptionClick(opt)}
                  className="px-3 py-1 text-xs font-mono border border-border bg-card hover:bg-accent hover:text-accent-foreground transition-colors"
                >
                  {opt}
                </button>
              ))}
              <button
                onClick={() => { setPromptOptions([]); textareaRef.current?.focus() }}
                className="px-3 py-1 text-xs font-mono border border-border text-muted-foreground hover:bg-accent hover:text-accent-foreground transition-colors"
              >
                other...
              </button>
            </div>
          ) : (
            <p className="text-center text-[10px] text-muted-foreground/50 mt-1.5">
              <kbd className="border border-border px-1 py-0.5 text-[9px]">/</kbd> commands
              {' '}<kbd className="border border-border px-1 py-0.5 text-[9px]">shift+enter</kbd> newline
            </p>
          )}
        </div>
      </div>
    </div>
  )
}

function EmptyState({ suggestions, onInputChange }: {
  suggestions?: { label: string; prompt: string }[]
  onInputChange: (text: string) => void
}) {
  const defaults = suggestions || [
    { label: 'Show system status', prompt: 'Show me the runtime status' },
    { label: 'List all nodes', prompt: 'List all nodes in the system' },
    { label: 'What can you do?', prompt: 'What tools and prompts do you have access to?' },
  ]

  return (
    <div className="flex flex-col items-center justify-center h-full px-4">
      <h2 className="font-mono text-lg font-light mb-1 text-foreground">What can I help with?</h2>
      <p className="text-xs text-muted-foreground mb-8">
        Type a message, or use <kbd className="border border-border px-1 py-0.5 text-[9px] mx-0.5">/</kbd> for prompts
      </p>
      <div className="flex flex-wrap gap-2 justify-center max-w-lg">
        {defaults.map((s) => (
          <button
            key={s.label}
            onClick={() => onInputChange(s.prompt)}
            className="text-left px-3 py-2 border border-border bg-card hover:bg-accent transition-colors text-xs text-muted-foreground hover:text-foreground"
          >
            {s.label}
          </button>
        ))}
      </div>
    </div>
  )
}
