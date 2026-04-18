import { useState, useRef, useCallback, useEffect } from 'react'

export interface ChatAttachment {
  name: string
  mimeType: string
  data: string // base64
  previewUrl?: string
}

export interface ChatMessage {
  role: 'user' | 'assistant' | 'system'
  content: string
  toolCalls?: string[]
  status?: 'connecting' | 'thinking' | 'tool_call' | 'streaming' | 'done'
  attachments?: ChatAttachment[]
}

interface AgentEvent {
  type: string
  session_id: string
  content?: string
  name?: string
  model?: string
  error?: string
  duration_ms?: number
  cost_usd?: number
}

export function useAgentChat(opts?: { appName?: string; initialMessages?: ChatMessage[]; initialClaudeSessionId?: string; provider?: string; model?: string }) {
  const [messages, setMessages] = useState<ChatMessage[]>(opts?.initialMessages || [])
  const [isStreaming, setIsStreaming] = useState(false)
  const [sessionId, setSessionId] = useState<string | null>(null)
  // Claude CLI session ID — used for --resume on subsequent messages
  const claudeSessionRef = useRef<string | null>(opts?.initialClaudeSessionId || null)
  const appNameRef = useRef(opts?.appName)
  const providerRef = useRef(opts?.provider)
  const modelRef = useRef(opts?.model)

  // Keep refs in sync when opts change
  useEffect(() => {
    providerRef.current = opts?.provider
    modelRef.current = opts?.model
  }, [opts?.provider, opts?.model])

  // When initialMessages arrives async (after API fetch), update state.
  // useState only uses the initial value on first mount — this handles late arrivals.
  useEffect(() => {
    if (opts?.initialMessages?.length) {
      setMessages(opts.initialMessages)
    }
    if (opts?.initialClaudeSessionId) {
      claudeSessionRef.current = opts.initialClaudeSessionId
    }
  }, [opts?.initialMessages, opts?.initialClaudeSessionId])
  const wsRef = useRef<WebSocket | null>(null)
  const assistantBufferRef = useRef('')
  const toolCallsRef = useRef<string[]>([])

  const updateLastAssistant = useCallback((updates: Partial<ChatMessage>) => {
    setMessages(prev => {
      const next = [...prev]
      const last = next[next.length - 1]
      if (last && last.role === 'assistant') {
        next[next.length - 1] = { ...last, ...updates }
      }
      return next
    })
  }, [])

  const send = useCallback((prompt: string, displayMessage?: string, attachments?: ChatAttachment[]) => {
    setIsStreaming(true)
    assistantBufferRef.current = ''
    toolCallsRef.current = []
    // Capture attachments for this send
    const sendAttachments = attachments && attachments.length > 0 ? attachments : undefined

    setMessages(prev => [
      ...prev,
      { role: 'user', content: displayMessage || prompt, attachments: sendAttachments },
      { role: 'assistant', content: '', status: 'connecting' },
    ])

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${protocol}//${window.location.host}/api/agents/run`

    const ws = new WebSocket(wsUrl)
    wsRef.current = ws
    let promptToSend = prompt
    // Capture current value for this send — closures in useCallback
    const resumeId = claudeSessionRef.current

    ws.onopen = () => {}

    ws.onmessage = (event) => {
      let ev: AgentEvent
      try {
        ev = JSON.parse(event.data)
      } catch {
        return
      }

      switch (ev.type) {
        case 'session':
          setSessionId(ev.session_id)
          updateLastAssistant({ status: 'thinking' })
          // Send prompt + provider/model + attachments + Claude session ID for resume + agent name
          const req: Record<string, unknown> = { prompt: promptToSend }
          if (resumeId) {
            req.session_id = resumeId
          }
          if (appNameRef.current) {
            req.agent = appNameRef.current
          }
          if (providerRef.current) {
            req.provider = providerRef.current
          }
          if (modelRef.current) {
            req.model = modelRef.current
          }
          if (sendAttachments) {
            req.attachments = sendAttachments.map(a => ({
              name: a.name,
              mime_type: a.mimeType,
              data: a.data,
            }))
          }
          ws.send(JSON.stringify(req))
          break

        case 'connected':
          updateLastAssistant({ status: 'thinking' })
          break

        case 'session_id':
          // Backend sends back the Claude CLI session ID for future --resume
          if (ev.content) {
            claudeSessionRef.current = ev.content
            console.log('[chat] received claude session ID for resume:', ev.content)
          }
          break

        case 'text':
          if (ev.content) {
            assistantBufferRef.current += ev.content
            updateLastAssistant({
              content: assistantBufferRef.current,
              toolCalls: toolCallsRef.current.length > 0 ? [...toolCallsRef.current] : undefined,
              status: 'streaming',
            })
          }
          break

        case 'tool_call':
          if (ev.name) {
            toolCallsRef.current.push(ev.name)
            updateLastAssistant({
              toolCalls: [...toolCallsRef.current],
              status: 'tool_call',
            })
          }
          break

        case 'done':
          setIsStreaming(false)
          updateLastAssistant({
            status: 'done',
            toolCalls: toolCallsRef.current.length > 0 ? [...toolCallsRef.current] : undefined,
          })
          break

        case 'error':
          setIsStreaming(false)
          setMessages(prev => [...prev, { role: 'system', content: `Error: ${ev.error}` }])
          break
      }
    }

    ws.onerror = () => {
      setIsStreaming(false)
      setMessages(prev => [...prev, { role: 'system', content: 'WebSocket connection failed. Is the backend running?' }])
    }

    ws.onclose = () => {
      setIsStreaming(false)
      wsRef.current = null
    }
  }, [updateLastAssistant])

  // Resume an existing Claude session — sets the session ID so the next send() uses --resume
  const resumeSession = useCallback((claudeSessionId: string) => {
    claudeSessionRef.current = claudeSessionId
    setMessages([
      { role: 'system', content: `Resumed session. Type a message to continue the conversation.` },
    ])
    console.log('[chat] joined existing session:', claudeSessionId)
  }, [])

  const clear = useCallback(() => {
    if (wsRef.current) {
      wsRef.current.close()
      wsRef.current = null
    }
    setMessages([])
    setSessionId(null)
    claudeSessionRef.current = null
    setIsStreaming(false)
  }, [])

  return { messages, isStreaming, sessionId, send, clear, resumeSession }
}
