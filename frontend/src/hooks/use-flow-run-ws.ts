import { useState, useCallback, useRef, useEffect } from 'react'
import { useEventWS, type WSStatus } from './use-event-ws'
import { api } from '@/lib/api'
import type { FlowRun, FlowRunStatus } from '@/lib/types'

interface FlowRunWSResult {
  /** The live run object (updated in real-time via WS) */
  run: FlowRun | undefined
  /** The latest run ID */
  runId: string | null
  /** Total number of runs for this flow */
  totalRuns: number
  /** WebSocket connection status */
  wsStatus: WSStatus
  /** Whether the initial REST load is in progress */
  isLoading: boolean
}

/**
 * useFlowRunWS replaces the polling-based useLatestFlowRun with a WebSocket
 * subscription. It:
 *
 * 1. Fetches the latest run via REST on mount (initial state)
 * 2. Opens a WS subscription to flow.> events filtered by flow_id
 * 3. Applies incremental node state updates from WS events in real-time
 * 4. Falls back to a REST refetch if the WS delivers a new run_id
 *    (meaning someone started a new run)
 */
export function useFlowRunWS(flowId: string): FlowRunWSResult {
  const [run, setRun] = useState<FlowRun | undefined>()
  const [totalRuns, setTotalRuns] = useState(0)
  const [isLoading, setIsLoading] = useState(true)
  const runRef = useRef<FlowRun | undefined>(undefined)
  runRef.current = run

  // Fetch initial state via REST.
  useEffect(() => {
    if (!flowId) {
      setIsLoading(false)
      return
    }
    let cancelled = false
    setIsLoading(true)

    api.flowRuns(flowId).then((runs) => {
      if (cancelled) return
      setTotalRuns(runs?.length || 0)
      const latestId = runs?.[0]?.id
      if (!latestId) {
        setIsLoading(false)
        return
      }
      return api.flowRun(latestId).then((fullRun) => {
        if (cancelled) return
        setRun(fullRun)
        setIsLoading(false)
      })
    }).catch(() => {
      if (!cancelled) setIsLoading(false)
    })

    return () => { cancelled = true }
  }, [flowId])

  // Handle WS events — apply incremental updates to the run state.
  const onEvent = useCallback((topic: string, data: unknown) => {
    const ev = data as Record<string, unknown>
    const eventRunId = ev.run_id as string | undefined
    const current = runRef.current

    // A new run started — fetch its full state via REST.
    if (topic === 'flow.started') {
      if (!current || eventRunId !== current.id) {
        // New run — fetch full state.
        if (eventRunId) {
          api.flowRun(eventRunId).then((fullRun) => {
            setRun(fullRun)
            setTotalRuns((prev) => prev + 1)
          })
        }
      }
      return
    }

    // If the event is for a different run, ignore it.
    if (!current || eventRunId !== current.id) return

    // Apply incremental updates based on event type.
    switch (topic) {
      case 'flow.node.started': {
        const nodeId = ev.node_id as string
        if (!nodeId) return
        setRun((prev) => {
          if (!prev) return prev
          return {
            ...prev,
            status: 'running' as FlowRunStatus,
            node_states: {
              ...prev.node_states,
              [nodeId]: {
                ...prev.node_states[nodeId],
                status: 'running',
                started_at: (ev.timestamp as string) || new Date().toISOString(),
              },
            },
          }
        })
        break
      }

      case 'flow.node.completed': {
        const nodeId = ev.node_id as string
        if (!nodeId) return
        setRun((prev) => {
          if (!prev) return prev
          return {
            ...prev,
            node_states: {
              ...prev.node_states,
              [nodeId]: {
                ...prev.node_states[nodeId],
                status: 'completed',
                duration_ms: (ev.duration_ms as number) || 0,
              },
            },
          }
        })
        break
      }

      case 'flow.node.failed': {
        const nodeId = ev.node_id as string
        if (!nodeId) return
        setRun((prev) => {
          if (!prev) return prev
          return {
            ...prev,
            node_states: {
              ...prev.node_states,
              [nodeId]: {
                ...prev.node_states[nodeId],
                status: 'failed',
                error: ev.error as string,
              },
            },
          }
        })
        break
      }

      case 'flow.node.skipped': {
        const nodeId = ev.node_id as string
        if (!nodeId) return
        setRun((prev) => {
          if (!prev) return prev
          return {
            ...prev,
            node_states: {
              ...prev.node_states,
              [nodeId]: {
                ...prev.node_states[nodeId],
                status: 'skipped',
                error: ev.error as string,
              },
            },
          }
        })
        break
      }

      case 'flow.waiting_approval': {
        const nodeId = ev.node_id as string
        if (!nodeId) return
        setRun((prev) => {
          if (!prev) return prev
          return {
            ...prev,
            status: 'waiting_approval' as FlowRunStatus,
            node_states: {
              ...prev.node_states,
              [nodeId]: {
                ...prev.node_states[nodeId],
                status: 'waiting',
              },
            },
          }
        })
        break
      }

      case 'flow.approved':
      case 'flow.rejected': {
        // Approval resolved — refetch full state to get the output.
        if (eventRunId) {
          api.flowRun(eventRunId).then(setRun)
        }
        break
      }

      case 'flow.completed': {
        // Fetch full run to get output/variables.
        if (eventRunId) {
          api.flowRun(eventRunId).then(setRun)
        }
        break
      }

      case 'flow.failed': {
        setRun((prev) => {
          if (!prev) return prev
          return {
            ...prev,
            status: 'failed' as FlowRunStatus,
            error: ev.error as string,
          }
        })
        break
      }

      case 'flow.cancelled': {
        setRun((prev) => {
          if (!prev) return prev
          return {
            ...prev,
            status: 'cancelled' as FlowRunStatus,
          }
        })
        break
      }

      case 'flow.debug': {
        const output = ev.output as Record<string, unknown> | undefined
        if (!output) return
        setRun((prev) => {
          if (!prev) return prev
          const entry = {
            ts: (output.ts as string) || new Date().toISOString(),
            node_id: (output.node_id as string) || '',
            label: (output.label as string) || '',
            msg_id: (output.msg_id as string) || '',
            value: output.value,
          }
          return {
            ...prev,
            debug_log: [...(prev.debug_log || []), entry],
          }
        })
        break
      }
    }
  }, [flowId])

  const { status: wsStatus } = useEventWS({
    topics: ['flow.>'],
    filter: { flow_id: flowId },
    onEvent,
    enabled: !!flowId,
  })

  return {
    run,
    runId: run?.id || null,
    totalRuns,
    wsStatus,
    isLoading,
  }
}
