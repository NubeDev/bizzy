import { useState, useMemo, useCallback, useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'

export interface PickerItem {
  name: string
  appName: string
  description: string
  type: 'prompt' | 'tool'
  mode?: string
  prompt?: string
  arguments?: { name: string; description: string; required: boolean; options?: string[] }[]
}

interface PickerState {
  isOpen: boolean
  type: 'all' | null
  query: string
  selectedIndex: number
}

export function useCommandPicker() {
  const [state, setState] = useState<PickerState>({
    isOpen: false,
    type: null,
    query: '',
    selectedIndex: 0,
  })

  const { data, error, isLoading } = useQuery({
    queryKey: ['my-tools'],
    queryFn: () => api.myTools(),
    staleTime: 60_000,
  })

  // Log raw API data on load
  useEffect(() => {
    if (data) {
      console.log('[picker] /my/tools loaded:', data.length, 'tools')
      console.log('[picker] raw data:', data)
      const promptTools = data.filter(t => t.mode === 'prompt')
      if (promptTools.length) console.log('[picker] prompt-mode tools:', promptTools)
    }
    if (error) console.error('[picker] /my/tools error:', error)
  }, [data, error])

  const items = useMemo((): PickerItem[] => {
    if (!state.isOpen || !data) {
      if (state.isOpen && !data) console.log('[picker] open but no data yet, isLoading:', isLoading)
      return []
    }

    const all: PickerItem[] = data
      .filter(t => t.type === 'js')
      .map(t => ({
        name: t.name,
        appName: t.appName,
        description: t.description,
        type: (t.mode === 'prompt' ? 'prompt' : 'tool') as 'prompt' | 'tool',
        mode: t.mode,
        prompt: t.prompt,
        arguments: (t.params || []).map(p => ({
          name: p.name,
          description: p.description,
          required: p.required,
          options: p.options,
        })),
      }))

    console.log('[picker] items computed:', all.length, 'items, query:', state.query || '(none)')

    if (!state.query) return all

    const q = state.query.toLowerCase()
    const filtered = all.filter(item =>
      item.name.toLowerCase().includes(q) ||
      item.description.toLowerCase().includes(q) ||
      item.appName.toLowerCase().includes(q)
    )
    console.log('[picker] filtered to:', filtered.length, 'for query:', q)
    return filtered
  }, [state.isOpen, state.query, data, isLoading])

  const grouped = useMemo(() => {
    const groups: Record<string, PickerItem[]> = {}
    for (const item of items) {
      const key = item.appName
      if (!groups[key]) groups[key] = []
      groups[key].push(item)
    }
    return groups
  }, [items])

  const handleInputChange = useCallback((value: string) => {
    if (value === '/') {
      setState({ isOpen: true, type: 'all', query: '', selectedIndex: 0 })
      return true
    }

    if (state.isOpen) {
      if (value.startsWith('/')) {
        setState(s => ({ ...s, query: value.slice(1), selectedIndex: 0 }))
        return true
      }
      setState({ isOpen: false, type: null, query: '', selectedIndex: 0 })
    }

    return false
  }, [state.isOpen])

  const moveSelection = useCallback((direction: 'up' | 'down') => {
    if (!state.isOpen) return
    setState(s => {
      const max = items.length - 1
      const next = direction === 'down'
        ? Math.min(s.selectedIndex + 1, max)
        : Math.max(s.selectedIndex - 1, 0)
      return { ...s, selectedIndex: next }
    })
  }, [state.isOpen, items.length])

  const getSelected = useCallback((): PickerItem | null => {
    if (!state.isOpen || items.length === 0) return null
    return items[state.selectedIndex] || null
  }, [state.isOpen, items, state.selectedIndex])

  const dismiss = useCallback(() => {
    setState({ isOpen: false, type: null, query: '', selectedIndex: 0 })
  }, [])

  return {
    isOpen: state.isOpen,
    type: state.type,
    query: state.query,
    selectedIndex: state.selectedIndex,
    items,
    grouped,
    handleInputChange,
    moveSelection,
    getSelected,
    dismiss,
  }
}
