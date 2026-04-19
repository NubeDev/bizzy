import { useEffect, useRef, useCallback, useState } from 'react'

export type WSStatus = 'connecting' | 'connected' | 'disconnected'

interface EventWSOptions {
  /** Topics to subscribe to, e.g. ["flow.>", "job.>"] */
  topics: string[]
  /** Optional field-level filter, e.g. { flow_id: "abc" } */
  filter?: Record<string, unknown>
  /** Called for each event received */
  onEvent: (topic: string, data: unknown) => void
  /** Whether the connection is enabled (default true) */
  enabled?: boolean
}

/**
 * useEventWS opens a single WebSocket to /api/events/ws and subscribes
 * to the requested NATS topic patterns. Events are pushed to the onEvent
 * callback in real-time.
 *
 * Auto-reconnects with exponential backoff on disconnect.
 * Unsubscribes and closes on unmount or when topics change.
 */
export function useEventWS({ topics, filter, onEvent, enabled = true }: EventWSOptions) {
  const [status, setStatus] = useState<WSStatus>('disconnected')
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectTimer = useRef<ReturnType<typeof setTimeout>>(undefined)
  const backoff = useRef(1000)
  const onEventRef = useRef(onEvent)
  onEventRef.current = onEvent

  const topicsKey = topics.join(',')
  const filterKey = filter ? JSON.stringify(filter) : ''

  const connect = useCallback(() => {
    if (!enabled || topics.length === 0) return

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const ws = new WebSocket(`${protocol}//${window.location.host}/api/events/ws?token=dev`)
    wsRef.current = ws
    setStatus('connecting')

    ws.onopen = () => {
      setStatus('connected')
      backoff.current = 1000

      // Subscribe to each topic.
      for (const topic of topics) {
        const msg: Record<string, unknown> = { subscribe: topic }
        if (filter && Object.keys(filter).length > 0) {
          msg.filter = filter
        }
        ws.send(JSON.stringify(msg))
      }
    }

    ws.onmessage = (ev) => {
      try {
        const msg = JSON.parse(ev.data)
        if (msg.topic && msg.data) {
          onEventRef.current(msg.topic, msg.data)
        }
      } catch {
        // ignore malformed messages
      }
    }

    ws.onclose = () => {
      setStatus('disconnected')
      wsRef.current = null
      // Reconnect with exponential backoff (max 30s).
      if (enabled) {
        reconnectTimer.current = setTimeout(() => {
          backoff.current = Math.min(backoff.current * 2, 30000)
          connect()
        }, backoff.current)
      }
    }

    ws.onerror = () => {
      // onclose will fire after this — reconnect happens there.
      ws.close()
    }
  }, [enabled, topicsKey, filterKey])

  useEffect(() => {
    connect()
    return () => {
      clearTimeout(reconnectTimer.current)
      if (wsRef.current) {
        wsRef.current.onclose = null // prevent reconnect on intentional close
        wsRef.current.close()
        wsRef.current = null
      }
      setStatus('disconnected')
    }
  }, [connect])

  return { status }
}
