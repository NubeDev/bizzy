import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../lib/api'

export function usePlugins() {
  return useQuery({
    queryKey: ['plugins'],
    queryFn: async () => {
      const res = await api.plugins()
      return res.plugins
    },
    refetchInterval: 15_000,
  })
}

export function useTogglePlugin() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ name, enabled }: { name: string; enabled: boolean }) =>
      enabled ? api.enablePlugin(name) : api.disablePlugin(name),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['plugins'] })
    },
  })
}

export function useDeletePlugin() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (name: string) => api.deletePlugin(name),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['plugins'] })
    },
  })
}
