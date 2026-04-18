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

export interface BootstrapPrompt {
  name: string
  description: string
  arguments?: { name: string; description: string; required: boolean }[]
  body: string
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

// --- Flow Engine Types ---

export interface FlowPosition {
  x: number
  y: number
}

export interface FlowPortDef {
  handle: string
  label?: string
  type?: string
  required?: boolean
}

export interface FlowPortsDef {
  inputs?: FlowPortDef[]
  outputs?: FlowPortDef[]
}

export interface FlowNodeDef {
  id: string
  type: string
  label?: string
  position: FlowPosition
  data?: Record<string, unknown>
  ports?: FlowPortsDef
}

export interface FlowEdgeDef {
  id: string
  source: string
  sourceHandle: string
  target: string
  targetHandle: string
  condition?: string
  label?: string
}

export interface FlowInputDef {
  name: string
  type?: string
  description?: string
  default?: unknown
  required?: boolean
}

export interface FlowTriggerDef {
  type: string
  schedule?: string
  event?: string
  filter?: Record<string, unknown>
}

export interface FlowDef {
  id: string
  name: string
  description: string
  version: number
  nodes: FlowNodeDef[]
  edges: FlowEdgeDef[]
  inputs?: FlowInputDef[]
  trigger?: FlowTriggerDef
  settings?: Record<string, unknown>
  user_id: string
  created_at: string
  updated_at: string
}

export type FlowRunStatus = 'pending' | 'running' | 'waiting_approval' | 'completed' | 'failed' | 'cancelled'
export type NodeStatus = 'pending' | 'ready' | 'running' | 'completed' | 'failed' | 'skipped' | 'waiting'

export interface NodeState {
  status: NodeStatus
  input?: unknown
  output?: unknown
  error?: string
  started_at?: string
  finished_at?: string
  duration_ms?: number
  retries?: number
}

export interface FlowRun {
  id: string
  flow_id: string
  flow_version: number
  flow_name: string
  status: FlowRunStatus
  inputs?: Record<string, unknown>
  output?: unknown
  node_states: Record<string, NodeState>
  variables?: Record<string, unknown>
  error?: string
  user_id: string
  created_at: string
  finished_at?: string
}

export interface NodeTypeDef {
  type: string
  label: string
  description?: string
  category: string
  icon?: string
  source: string
  ports: FlowPortsDef
  settings?: unknown
}

export interface NodeTypeCatalog {
  types: NodeTypeDef[]
  grouped: Record<string, NodeTypeDef[]>
}

export interface FlowValidationResult {
  valid: boolean
  errors: string[]
}

