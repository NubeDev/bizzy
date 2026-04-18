import type {
  StoreListResponse,
  StoreApp,
  StoreTool,
  StorePrompt,
  AppReview,
  CreateAppRequest,
  StoreQuery,
  PluginSummary,
  BootstrapPrompt,
  FlowDef,
  FlowRun,
  NodeTypeCatalog,
  FlowValidationResult,
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
  updateTool(appId: string, name: string, tool: StoreTool, changeSummary?: string) {
    return this.request<StoreTool>(`/api/my/apps/${appId}/tools/${name}`, {
      method: 'PUT',
      body: JSON.stringify(tool),
      headers: changeSummary ? { 'X-Change-Summary': changeSummary } : undefined,
    })
  }
  deleteTool(appId: string, name: string) {
    return this.request<void>(`/api/my/apps/${appId}/tools/${name}`, {
      method: 'DELETE',
    })
  }

  // Revision history
  listRevisions(appId: string, entityType: string, entityName: string) {
    return this.request<unknown[]>(`/api/my/apps/${appId}/revisions/${entityType}/${entityName}`)
  }
  revertRevision(appId: string, entityType: string, entityName: string, rev: number) {
    return this.request<unknown>(`/api/my/apps/${appId}/revisions/${entityType}/${entityName}/revert/${rev}`, {
      method: 'POST',
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

  // User's installed tools and prompts (prompt-mode tools are included in the same list)
  myTools() {
    return this.request<{
      name: string
      appName: string
      type: string
      mode?: string
      prompt?: string
      description: string
      params?: { name: string; type: string; required: boolean; description: string; options?: string[] }[]
    }[]>('/my/tools')
  }
  getPrompt(name: string) {
    return this.request<{ name: string; description: string; rendered: string }>(`/my/prompts/${name}`)
  }

  // Chat sessions
  listSessions() {
    return this.request<{
      id: string
      claude_session_id?: string
      prompt: string
      status: string
      duration_ms: number
      cost_usd: number
      created_at: string
    }[]>('/api/agents/sessions')
  }

  // Plugins
  plugins(service?: string) {
    const q = service ? `?service=${encodeURIComponent(service)}` : ''
    return this.request<{ api_version: string; plugins: PluginSummary[] }>(`/api/plugins${q}`)
  }
  plugin(name: string) {
    return this.request<{ api_version: string; plugin: PluginSummary; manifest: unknown }>(`/api/plugins/${name}`)
  }
  disablePlugin(name: string) {
    return this.request<{ status: string }>(`/api/plugins/${name}/disable`, { method: 'POST' })
  }
  enablePlugin(name: string) {
    return this.request<{ status: string }>(`/api/plugins/${name}/enable`, { method: 'POST' })
  }
  deletePlugin(name: string) {
    return this.request<{ status: string }>(`/api/plugins/${name}`, { method: 'DELETE' })
  }

  // AI providers
  providers() {
    return this.request<{
      provider: string
      available: boolean
      type: string
      models?: string[]
    }[]>('/api/agents/providers')
  }

  // Bootstrap prompts — built-in reference docs from the backend
  bootstrapPrompts() {
    return this.request<BootstrapPrompt[]>('/api/bootstrap/prompts')
  }
  bootstrapPrompt(name: string) {
    return this.request<BootstrapPrompt>(`/api/bootstrap/prompts/${name}`)
  }

  // Run a prompt via the agent API
  runPrompt(prompt: string, provider?: string, model?: string) {
    return this.request<{ session_id: string; provider: string; text: string; duration_ms: number; cost_usd: number }>('/api/agents/run/sync', {
      method: 'POST',
      body: JSON.stringify({ prompt, provider, model }),
    })
  }
  // --- Flow Engine ---

  // Flow definitions
  flows() {
    return this.request<FlowDef[]>('/api/flows')
  }
  flow(id: string) {
    return this.request<FlowDef>(`/api/flows/${id}`)
  }
  createFlow(data: Partial<FlowDef>) {
    return this.request<FlowDef>('/api/flows', {
      method: 'POST',
      body: JSON.stringify(data),
    })
  }
  updateFlow(id: string, data: Partial<FlowDef>) {
    return this.request<FlowDef>(`/api/flows/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    })
  }
  deleteFlow(id: string) {
    return this.request<void>(`/api/flows/${id}`, { method: 'DELETE' })
  }
  duplicateFlow(id: string) {
    return this.request<FlowDef>(`/api/flows/${id}/duplicate`, { method: 'POST' })
  }
  validateFlow(data: Partial<FlowDef>) {
    return this.request<FlowValidationResult>('/api/flows/validate', {
      method: 'POST',
      body: JSON.stringify(data),
    })
  }

  // Node type catalog
  nodeTypes() {
    return this.request<NodeTypeCatalog>('/api/flows/node-types')
  }

  // Flow execution
  runFlow(id: string, inputs?: Record<string, unknown>) {
    return this.request<FlowRun>(`/api/flows/${id}/run`, {
      method: 'POST',
      body: JSON.stringify({ inputs }),
    })
  }
  flowRuns(flowId: string) {
    return this.request<FlowRun[]>(`/api/flows/${flowId}/runs`)
  }
  flowRun(runId: string) {
    return this.request<FlowRun>(`/api/flow-runs/${runId}`)
  }
  approveFlowNode(runId: string, nodeId: string) {
    return this.request<unknown>(`/api/flow-runs/${runId}/approve/${nodeId}`, {
      method: 'POST',
    })
  }
  rejectFlowNode(runId: string, nodeId: string, feedback?: string) {
    return this.request<unknown>(`/api/flow-runs/${runId}/reject/${nodeId}`, {
      method: 'POST',
      body: JSON.stringify({ feedback }),
    })
  }
  cancelFlowRun(runId: string) {
    return this.request<unknown>(`/api/flow-runs/${runId}/cancel`, {
      method: 'POST',
    })
  }
}

export const api = new ApiClient()
export { ApiError }
