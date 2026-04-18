// Types matching Go backend models.

export type Visibility = 'private' | 'shared' | 'unlisted' | 'public'

export interface ToolParam {
  type: string
  required: boolean
  description: string
  options?: string[] | { value: string; label: string }[]
}

export interface StoreTool {
  name: string
  description: string
  toolClass: string
  mode?: string
  params: Record<string, ToolParam>
  script: string
}

export interface PromptArgument {
  name: string
  description: string
  required: boolean
}

export interface UIComponent {
  name: string
  code: string
}

export interface StorePrompt {
  name: string
  description: string
  arguments?: PromptArgument[]
  body: string
}

export interface SettingDef {
  key: string
  label: string
  type: string
  required: boolean
  default: string
}

export interface Permissions {
  allowedHosts: string[]
  defaultToolClass: string
  secrets: string[]
}

export interface StoreApp {
  id: string
  name: string
  displayName: string
  description: string
  longDescription: string
  version: string
  icon: string
  color: string
  category: string
  tags: string[]
  authorId: string
  authorName: string
  workspaceId: string
  visibility: Visibility
  permissions: Permissions
  settings: SettingDef[]
  tools: StoreTool[]
  prompts: StorePrompt[]
  uiComponents?: UIComponent[]
  installCount: number
  activeInstalls: number
  avgRating: number
  reviewCount: number
  createdAt: string
  updatedAt: string
  publishedAt?: string
}

export interface StoreAppSummary {
  id: string
  name: string
  displayName: string
  description: string
  version: string
  icon: string
  color: string
  category: string
  tags: string[]
  authorName: string
  installCount: number
  avgRating: number
  reviewCount: number
  toolCount: number
  promptCount: number
  publishedAt?: string
}

export interface StoreListResponse {
  apps: StoreAppSummary[]
  total: number
  page: number
  limit: number
}

export interface AppReview {
  id: string
  appId: string
  userId: string
  userName: string
  rating: number
  comment: string
  createdAt: string
  updatedAt: string
}

export interface StoreQuery {
  q?: string
  category?: string
  sort?: string
  page?: number
  limit?: number
}

export interface CreateAppRequest {
  name: string
  displayName?: string
  description?: string
  category?: string
  icon?: string
  color?: string
}

export interface PluginSummary {
  name: string
  version: string
  description?: string
  services: string[]
  status: string
  registered_at: string
  last_heartbeat?: string
  health_failures: number
  tool_count: number
  prompt_count: number
}

