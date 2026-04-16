import { useState, useRef, useCallback } from 'react'

export interface QaOption {
  value: string
  label: string
}

export interface QaQuestion {
  field: string
  label: string
  input: string // "text", "textarea", "select", "multi_select", "number"
  required: boolean
  placeholder?: string
  default?: string
  options?: QaOption[]
  min_length?: number
  max_length?: number
  context?: Record<string, unknown>
}

export interface QaExchange {
  question: QaQuestion
  answer: string | string[]
}

export type QaPhase = 'idle' | 'connecting' | 'questioning' | 'generating' | 'streaming' | 'done' | 'error'

export interface QaState {
  phase: QaPhase
  sessionId: string | null
  exchanges: QaExchange[]
  currentQuestion: QaQuestion | null
  generatingMessage: string
  resultText: string
  toolCalls: string[]
  error: string | null
  durationMs?: number
  costUsd?: number
}

export function useQaWizard() {
  const [state, setState] = useState<QaState>({
    phase: 'idle',
    sessionId: null,
    exchanges: [],
    currentQuestion: null,
    generatingMessage: '',
    resultText: '',
    toolCalls: [],
    error: null,
  })
  const wsRef = useRef<WebSocket | null>(null)
  const flowRef = useRef<string>('')

  const start = useCallback((flow: string) => {
    // Close any existing connection
    if (wsRef.current) {
      wsRef.current.close()
      wsRef.current = null
    }

    flowRef.current = flow
    setState({
      phase: 'connecting',
      sessionId: null,
      exchanges: [],
      currentQuestion: null,
      generatingMessage: '',
      resultText: '',
      toolCalls: [],
      error: null,
    })

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    // Dev mode: no token needed, backend uses first user
    const wsUrl = `${protocol}//${window.location.host}/api/agents/qa?token=dev`
    const ws = new WebSocket(wsUrl)
    wsRef.current = ws

    ws.onopen = () => {
      console.log('[qa] ws open')
    }

    ws.onmessage = (event) => {
      let ev: Record<string, unknown>
      try {
        ev = JSON.parse(event.data as string)
      } catch {
        return
      }

      const type = ev.type as string

      switch (type) {
        case 'session':
          setState(s => ({ ...s, sessionId: ev.session_id as string }))
          // Send the flow name to start
          ws.send(JSON.stringify({ flow: flowRef.current }))
          break

        case 'question':
          setState(s => ({
            ...s,
            phase: 'questioning',
            currentQuestion: {
              field: ev.field as string,
              label: ev.label as string,
              input: (ev.input as string) || 'text',
              required: (ev.required as boolean) || false,
              placeholder: ev.placeholder as string | undefined,
              default: ev.default as string | undefined,
              options: ev.options as QaOption[] | undefined,
              min_length: ev.min_length as number | undefined,
              max_length: ev.max_length as number | undefined,
              context: ev.context as Record<string, unknown> | undefined,
            },
          }))
          break

        case 'generating':
          setState(s => ({
            ...s,
            phase: 'generating',
            currentQuestion: null,
            generatingMessage: (ev.message as string) || 'Generating...',
          }))
          break

        case 'connected':
          setState(s => ({ ...s, phase: 'streaming' }))
          break

        case 'tool_call':
          if (ev.name) {
            setState(s => ({ ...s, toolCalls: [...s.toolCalls, ev.name as string] }))
          }
          break

        case 'text':
          if (ev.content) {
            setState(s => ({
              ...s,
              phase: 'streaming',
              resultText: s.resultText + (ev.content as string),
            }))
          }
          break

        case 'done':
          setState(s => ({
            ...s,
            phase: 'done',
            durationMs: ev.duration_ms as number | undefined,
            costUsd: ev.cost_usd as number | undefined,
          }))
          break

        case 'error':
          setState(s => ({
            ...s,
            phase: 'error',
            error: (ev.error as string) || 'Unknown error',
          }))
          break
      }
    }

    ws.onerror = () => {
      setState(s => ({ ...s, phase: 'error', error: 'WebSocket connection failed. Is the backend running?' }))
    }

    ws.onclose = () => {
      wsRef.current = null
    }
  }, [])

  const answer = useCallback((value: string | string[]) => {
    const ws = wsRef.current
    if (!ws || !state.currentQuestion) return

    // Record the exchange
    setState(s => ({
      ...s,
      exchanges: [...s.exchanges, { question: s.currentQuestion!, answer: value }],
      currentQuestion: null,
      phase: 'connecting', // brief loading state until next question arrives
    }))

    // Send to server
    ws.send(JSON.stringify({ answer: value }))
  }, [state.currentQuestion])

  const restart = useCallback(() => {
    if (wsRef.current) {
      wsRef.current.close()
      wsRef.current = null
    }
    if (flowRef.current) {
      start(flowRef.current)
    }
  }, [start])

  return { state, start, answer, restart }
}
