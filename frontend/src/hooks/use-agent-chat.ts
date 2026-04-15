import { useState, useRef, useCallback } from 'react'

export interface ChatMessage {
  role: 'user' | 'assistant' | 'system'
  content: string
  toolCalls?: string[]
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

export function useAgentChat() {
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [isStreaming, setIsStreaming] = useState(false)
  const [sessionId, setSessionId] = useState<string | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const assistantBufferRef = useRef('')
  const toolCallsRef = useRef<string[]>([])

  const send = useCallback((prompt: string, displayMessage?: string) => {
    setIsStreaming(true)
    assistantBufferRef.current = ''
    toolCallsRef.current = []

    // Add user message
    setMessages(prev => [...prev, { role: 'user', content: displayMessage || prompt }])
    // Add empty assistant message to stream into
    setMessages(prev => [...prev, { role: 'assistant', content: '' }])

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${protocol}//${window.location.host}/api/agents/run`
    console.log('[ai-chat] connecting to', wsUrl)

    const ws = new WebSocket(wsUrl)
    wsRef.current = ws

    // Store prompt to send after session event
    let promptToSend = prompt

    ws.onopen = () => {
      console.log('[ai-chat] ws open')
    }

    ws.onmessage = (event) => {
      let ev: AgentEvent
      try {
        ev = JSON.parse(event.data)
      } catch {
        console.warn('[ai-chat] failed to parse:', event.data)
        return
      }

      console.log('[ai-chat] event:', ev.type, ev.type === 'text' ? `(${(ev.content || '').length} chars)` : '')

      switch (ev.type) {
        case 'session':
          setSessionId(ev.session_id)
          console.log('[ai-chat] sending prompt to session', ev.session_id)
          ws.send(JSON.stringify({ prompt: promptToSend }))
          break

        case 'connected':
          break

        case 'text':
          if (ev.content) {
            assistantBufferRef.current += ev.content
            const text = assistantBufferRef.current
            const tools = [...toolCallsRef.current]
            setMessages(prev => {
              const next = [...prev]
              next[next.length - 1] = { role: 'assistant', content: text, toolCalls: tools.length > 0 ? tools : undefined }
              return next
            })
          }
          break

        case 'tool_call':
          if (ev.name) {
            toolCallsRef.current.push(ev.name)
          }
          break

        case 'done':
          console.log('[ai-chat] done, duration:', ev.duration_ms, 'ms')
          setIsStreaming(false)
          break

        case 'error':
          console.error('[ai-chat] error:', ev.error)
          setIsStreaming(false)
          setMessages(prev => [...prev, { role: 'system', content: `Error: ${ev.error}` }])
          break
      }
    }

    ws.onerror = (err) => {
      console.error('[ai-chat] ws error:', err)
      setIsStreaming(false)
      setMessages(prev => [...prev, { role: 'system', content: 'WebSocket connection failed. Is the backend running?' }])
    }

    ws.onclose = (event) => {
      console.log('[ai-chat] ws closed:', event.code, event.reason)
      setIsStreaming(false)
      wsRef.current = null
    }
  }, [])

  const clear = useCallback(() => {
    if (wsRef.current) {
      wsRef.current.close()
      wsRef.current = null
    }
    setMessages([])
    setSessionId(null)
    setIsStreaming(false)
  }, [])

  return { messages, isStreaming, sessionId, send, clear }
}
