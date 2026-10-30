export interface OpsUser {
  id: number
  username: string
  email: string
  roleCode: string
}

export interface Overview {
  projects: number
  users: number
  runningSandboxes: number
  todayRequests: number
}

export interface AdminUser {
  id: string
  name: string
  email: string
  plan: string
  status: string
  createdAt: string
}

export interface AdminProject {
  id: string
  name: string
  state: string
  owner: string
  createdAt: string
}

export interface AdminSandbox {
  id: string
  name: string
  host: string
  port: number
  status: string
  healthStatus: string
  activeRequests: number
  maxConcurrency: number
  lastHeartbeatAt: string | null
  lastHealthAt: string | null
  healthError: string
}

export interface AdminQuota {
  userId: string
  plan: string
  llmTokensQuota: number
  llmTokensUsed: number
  sandboxMinutesQuota: number
  sandboxMinutesUsed: number
  storageQuotaMb: number
  storageUsedMb: number
}

export interface QuotaUpdateInput {
  llmTokensQuota: number
  sandboxMinutesQuota: number
  storageQuotaMb: number
}

export interface AuditEvent {
  id: string
  actor: string
  action: string
  target: string
  createdAt: string
}

export type NotificationType = 'version_release' | 'inbox'
export type NotificationStatus = 'draft' | 'published' | 'unpublished' | 'expired'

export interface OpsNotification {
  id: number
  title: string
  content: string
  type: NotificationType
  version: string
  link: string
  status: NotificationStatus
  expireAt: string
  publishedAt: string | null | undefined
  createdBy: number
  createdByName: string
  createdAt: string
  updatedAt: string
}

export interface NotificationPage {
  notifications: OpsNotification[]
  total: number
  page: number
  pageSize: number
}

export interface NotificationQuery {
  page?: number
  pageSize?: number
  keyword?: string
}

export interface NotificationInput {
  title: string
  content: string
  type: NotificationType
  version: string
  link: string
  expireAt: string
}

export interface PageQuery {
  page?: number
  pageSize?: number
  keyword?: string
}

export interface PageResult<T> {
  items: T[]
  total: number
  page: number
  pageSize: number
}

export type TableRow = object

export interface TableColumn {
  key: string
  label: string
  width?: string
  tag?: boolean
  highlight?: string[]
  format?: (value: unknown, row: TableRow) => string
}

export interface AdminLLMModel {
  id: number
  modelName: string
  modelCode: string
  apiUrl: string
  providerCode: string
  modelType: string
  maxTokens: number
  isDefault: boolean
  status: number
  hasApiKey: boolean
  extraParams?: Record<string, unknown> | null
  createdAt: string
  updatedAt: string
}

export interface ModelInput {
  modelName: string
  modelCode: string
  apiUrl: string
  apiKey: string
  providerCode: string
  modelType: string
  maxTokens: number
  isDefault: boolean
  status: number
  extraParams?: Record<string, unknown>
}

export interface AdminAgentRole {
  id: number
  roleCode: string
  roleNameEn: string
  roleNameZh: string
  description: string
  systemPrompt: string
  avatarUrl: string
  isBuiltin: boolean
  status: number
  sortOrder: number
  version: number
  createdBy: string
  createdAt: string
  updatedAt: string
}

export interface RoleInput {
  roleCode: string
  roleNameEn: string
  roleNameZh: string
  description: string
  systemPrompt: string
  avatarKey?: string
  sortOrder: number
  status: number
}

export interface RoleUpdateInput {
  description: string
  systemPrompt: string
  status: number
}

export interface RoleAvatarPresign {
  uploadUrl: string
  key: string
  headers: Record<string, string>
}
