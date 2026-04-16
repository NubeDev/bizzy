import type {
  StoreListResponse,
  StoreApp,
  StoreTool,
  StorePrompt,
  AppReview,
  CreateAppRequest,
  StoreQuery,
} from './types'

class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

function qs(params: Record<string, string | number | undefined | null>): string {
  const sp = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== null && v !== '') {
      sp.set(k, String(v))
    }
  }
  return sp.toString()
}

class ApiClient {
  private async request<T>(path: string, options?: RequestInit): Promise<T> {
    const res = await fetch(path, {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        ...options?.headers,
      },
    })
    if (!res.ok) {
      const body = await res.json().catch(() => ({}))
      throw new ApiError(res.status, body.error || res.statusText)
    }
    return res.json()
  }

  // Auth (dev mode — no token needed, backend uses first user)
  getMe() {
    return this.request<{ id: string; name: string; email: string }>('/users/me')
  }

  // Store catalog
  storeApps(params: StoreQuery) {
    return this.request<StoreListResponse>(`/api/store/apps?${qs(params as Record<string, string | number | undefined>)}`)
  }
  storeApp(id: string) {
    return this.request<{ app: StoreApp; installed: boolean; installId: string }>(`/api/store/apps/${id}`)
  }
  storeCategories() {
    return this.request<string[]>('/api/store/categories')
  }
  installStoreApp(id: string, settings: Record<string, string>) {
    return this.request<unknown>(`/api/store/apps/${id}/install`, {
      method: 'POST',
      body: JSON.stringify({ settings }),
    })
  }

  // Reviews
  appReviews(appId: string) {
    return this.request<AppReview[]>(`/api/store/apps/${appId}/reviews`)
  }
  submitReview(appId: string, rating: number, comment: string) {
    return this.request<AppReview>(`/api/store/apps/${appId}/reviews`, {
      method: 'POST',
      body: JSON.stringify({ rating, comment }),
    })
  }
  deleteReview(appId: string) {
    return this.request<void>(`/api/store/apps/${appId}/reviews`, {
      method: 'DELETE',
    })
  }

  // My Apps
  myApps() {
    return this.request<StoreApp[]>('/api/my/apps')
  }
  myApp(id: string) {
    return this.request<StoreApp>(`/api/my/apps/${id}`)
  }
  createApp(data: CreateAppRequest) {
    return this.request<StoreApp>('/api/my/apps', {
      method: 'POST',
      body: JSON.stringify(data),
    })
  }
  updateApp(id: string, data: Partial<StoreApp>) {
    return this.request<StoreApp>(`/api/my/apps/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    })
  }
  deleteApp(id: string) {
    return this.request<void>(`/api/my/apps/${id}`, { method: 'DELETE' })
  }
  publishApp(id: string) {
    return this.request<StoreApp>(`/api/my/apps/${id}/publish`, {
      method: 'POST',
    })
  }

  // Tools within my app
  addTool(appId: string, tool: StoreTool) {
    return this.request<StoreTool>(`/api/my/apps/${appId}/tools`, {
      method: 'POST',
      body: JSON.stringify(tool),
    })
  }
  updateTool(appId: string, name: string, tool: StoreTool) {
    return this.request<StoreTool>(`/api/my/apps/${appId}/tools/${name}`, {
      method: 'PUT',
      body: JSON.stringify(tool),
    })
  }
  deleteTool(appId: string, name: string) {
    return this.request<void>(`/api/my/apps/${appId}/tools/${name}`, {
      method: 'DELETE',
    })
  }

  // Call a tool via the agent API
  callTool(toolName: string, params: Record<string, unknown>) {
    return this.request<unknown>(`/api/agents/tools/${toolName}`, {
      method: 'POST',
      body: JSON.stringify(params),
    })
  }

  // Prompts within my app
  addPrompt(appId: string, prompt: StorePrompt) {
    return this.request<StorePrompt>(`/api/my/apps/${appId}/prompts`, {
      method: 'POST',
      body: JSON.stringify(prompt),
    })
  }
  updatePrompt(appId: string, name: string, prompt: StorePrompt) {
    return this.request<StorePrompt>(`/api/my/apps/${appId}/prompts/${name}`, {
      method: 'PUT',
      body: JSON.stringify(prompt),
    })
  }
  deletePrompt(appId: string, name: string) {
    return this.request<void>(`/api/my/apps/${appId}/prompts/${name}`, {
      method: 'DELETE',
    })
  }

  // Run a prompt via the agent API
  runPrompt(prompt: string, provider?: string, model?: string) {
    return this.request<{ session_id: string; provider: string; text: string; duration_ms: number; cost_usd: number }>('/api/agents/run/sync', {
      method: 'POST',
      body: JSON.stringify({ prompt, provider, model }),
    })
  }
}

export const api = new ApiClient()
export { ApiError }
