import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type { StoreQuery } from '@/lib/types'

export function useStoreApps(params: StoreQuery) {
  return useQuery({
    queryKey: ['store', 'apps', params],
    queryFn: () => api.storeApps(params),
    placeholderData: (prev) => prev,
  })
}

export function useStoreApp(id: string) {
  return useQuery({
    queryKey: ['store', 'app', id],
    queryFn: () => api.storeApp(id),
    enabled: !!id,
  })
}

export function useCategories() {
  return useQuery({
    queryKey: ['store', 'categories'],
    queryFn: () => api.storeCategories(),
    staleTime: Infinity,
  })
}

export function useAppReviews(appId: string) {
  return useQuery({
    queryKey: ['store', 'reviews', appId],
    queryFn: () => api.appReviews(appId),
    enabled: !!appId,
  })
}

export function useInstallApp() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, settings }: { id: string; settings: Record<string, string> }) =>
      api.installStoreApp(id, settings),
    onSuccess: (_, { id }) => {
      qc.invalidateQueries({ queryKey: ['store', 'app', id] })
    },
  })
}

export function useSubmitReview() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ appId, rating, comment }: { appId: string; rating: number; comment: string }) =>
      api.submitReview(appId, rating, comment),
    onSuccess: (_, { appId }) => {
      qc.invalidateQueries({ queryKey: ['store', 'reviews', appId] })
      qc.invalidateQueries({ queryKey: ['store', 'app', appId] })
    },
  })
}
