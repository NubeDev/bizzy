import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../lib/api'
import type { StoreApp, CreateAppRequest, StoreTool, StorePrompt } from '../lib/types'

export function useMyApps() {
  return useQuery({
    queryKey: ['my', 'apps'],
    queryFn: () => api.myApps(),
  })
}

export function useMyApp(id: string) {
  return useQuery({
    queryKey: ['my', 'app', id],
    queryFn: () => api.myApp(id),
    enabled: !!id,
  })
}

export function useCreateApp() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: CreateAppRequest) => api.createApp(data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['my', 'apps'] })
    },
  })
}

export function useUpdateApp() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: Partial<StoreApp> }) => api.updateApp(id, data),
    onSuccess: (_, { id }) => {
      qc.invalidateQueries({ queryKey: ['my', 'app', id] })
      qc.invalidateQueries({ queryKey: ['my', 'apps'] })
    },
  })
}

export function useDeleteApp() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.deleteApp(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['my', 'apps'] })
    },
  })
}

export function usePublishApp() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.publishApp(id),
    onSuccess: (_, id) => {
      qc.invalidateQueries({ queryKey: ['my', 'app', id] })
      qc.invalidateQueries({ queryKey: ['my', 'apps'] })
    },
  })
}

export function useAddTool() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ appId, tool }: { appId: string; tool: StoreTool }) => api.addTool(appId, tool),
    onSuccess: (_, { appId }) => {
      qc.invalidateQueries({ queryKey: ['my', 'app', appId] })
    },
  })
}

export function useUpdateTool() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ appId, name, tool }: { appId: string; name: string; tool: StoreTool }) =>
      api.updateTool(appId, name, tool),
    onSuccess: (_, { appId }) => {
      qc.invalidateQueries({ queryKey: ['my', 'app', appId] })
    },
  })
}

export function useDeleteTool() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ appId, name }: { appId: string; name: string }) => api.deleteTool(appId, name),
    onSuccess: (_, { appId }) => {
      qc.invalidateQueries({ queryKey: ['my', 'app', appId] })
    },
  })
}

export function useAddPrompt() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ appId, prompt }: { appId: string; prompt: StorePrompt }) =>
      api.addPrompt(appId, prompt),
    onSuccess: (_, { appId }) => {
      qc.invalidateQueries({ queryKey: ['my', 'app', appId] })
    },
  })
}

export function useUpdatePrompt() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ appId, name, prompt }: { appId: string; name: string; prompt: StorePrompt }) =>
      api.updatePrompt(appId, name, prompt),
    onSuccess: (_, { appId }) => {
      qc.invalidateQueries({ queryKey: ['my', 'app', appId] })
    },
  })
}

export function useDeletePrompt() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ appId, name }: { appId: string; name: string }) =>
      api.deletePrompt(appId, name),
    onSuccess: (_, { appId }) => {
      qc.invalidateQueries({ queryKey: ['my', 'app', appId] })
    },
  })
}
