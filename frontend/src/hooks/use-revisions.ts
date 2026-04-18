import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'

export interface Revision {
  id: string
  entityType: string
  entityId: string
  revision: number
  data: unknown
  changeSummary: string
  authorId: string
  createdAt: string
}

export function useRevisions(appId: string, entityType: string, entityName: string) {
  return useQuery({
    queryKey: ['revisions', appId, entityType, entityName],
    queryFn: () => api.listRevisions(appId, entityType, entityName) as Promise<Revision[]>,
    enabled: !!appId && !!entityName,
  })
}

export function useRevertRevision(appId: string, entityType: string, entityName: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (rev: number) => api.revertRevision(appId, entityType, entityName, rev),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['revisions', appId, entityType, entityName] })
      qc.invalidateQueries({ queryKey: ['my', 'app', appId] })
    },
  })
}
