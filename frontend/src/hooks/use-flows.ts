import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type { FlowDef } from '@/lib/types'

export function useFlows() {
  return useQuery({
    queryKey: ['flows'],
    queryFn: () => api.flows(),
  })
}

export function useFlow(id: string) {
  return useQuery({
    queryKey: ['flows', id],
    queryFn: () => api.flow(id),
    enabled: !!id,
  })
}

export function useNodeTypes() {
  return useQuery({
    queryKey: ['flows', 'node-types'],
    queryFn: () => api.nodeTypes(),
    staleTime: 60_000,
  })
}

export function useCreateFlow() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: Partial<FlowDef>) => api.createFlow(data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['flows'] })
    },
  })
}

export function useUpdateFlow() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: Partial<FlowDef> }) =>
      api.updateFlow(id, data),
    onSuccess: (_, { id }) => {
      qc.invalidateQueries({ queryKey: ['flows'] })
      qc.invalidateQueries({ queryKey: ['flows', id] })
    },
  })
}

export function useDeleteFlow() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.deleteFlow(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['flows'] })
    },
  })
}

export function useRunFlow() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, inputs }: { id: string; inputs?: Record<string, unknown> }) =>
      api.runFlow(id, inputs),
    onSuccess: (_, { id }) => {
      qc.invalidateQueries({ queryKey: ['flow-runs', id] })
    },
  })
}

export function useFlowRuns(flowId: string) {
  return useQuery({
    queryKey: ['flow-runs', flowId],
    queryFn: () => api.flowRuns(flowId),
    enabled: !!flowId,
    refetchInterval: 5000,
  })
}

export function useFlowRun(runId: string) {
  return useQuery({
    queryKey: ['flow-run', runId],
    queryFn: () => api.flowRun(runId),
    enabled: !!runId,
    refetchInterval: (query) => {
      const status = query.state.data?.status
      if (status === 'running' || status === 'waiting_approval') return 2000
      return false
    },
  })
}

export function useApproveNode() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ runId, nodeId }: { runId: string; nodeId: string }) =>
      api.approveFlowNode(runId, nodeId),
    onSuccess: (_, { runId }) => {
      qc.invalidateQueries({ queryKey: ['flow-run', runId] })
    },
  })
}

export function useRejectNode() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ runId, nodeId, feedback }: { runId: string; nodeId: string; feedback?: string }) =>
      api.rejectFlowNode(runId, nodeId, feedback),
    onSuccess: (_, { runId }) => {
      qc.invalidateQueries({ queryKey: ['flow-run', runId] })
    },
  })
}

export function useLatestFlowRun(flowId: string, pollInterval: number | false) {
  // Fetch runs list, pick the latest, then fetch its full details.
  const runsQuery = useQuery({
    queryKey: ['flow-runs', flowId],
    queryFn: () => api.flowRuns(flowId),
    enabled: !!flowId && pollInterval !== false,
    refetchInterval: pollInterval,
  })

  const latestRunId = runsQuery.data?.[0]?.id || ''

  const runQuery = useQuery({
    queryKey: ['flow-run', latestRunId],
    queryFn: () => api.flowRun(latestRunId),
    enabled: !!latestRunId && pollInterval !== false,
    refetchInterval: pollInterval,
  })

  return {
    run: runQuery.data,
    runId: latestRunId || null,
    totalRuns: runsQuery.data?.length || 0,
    isLoading: runsQuery.isLoading,
  }
}

export function useValidateFlow() {
  return useMutation({
    mutationFn: (data: Partial<FlowDef>) => api.validateFlow(data),
  })
}
