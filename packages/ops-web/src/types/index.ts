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

export type SkillScope = 'public' | 'user'

export interface AdminSkill {
  name: string
  description: string
  source: SkillScope
  updatedAt?: string
}

export interface AdminUserDetail {
  id: string
  username: string
  name: string
  nickname: string
  email: string
  phoneNumber: string
  avatarUrl: string
  plan: string
  source: string
  status: string
  emailVerifiedAt?: string
  lastLoginTime?: string
  lastLoginIp: string
  disabledReason: string
  disabledAt?: string
  disabledBy: string
  createdAt: string
  updatedAt: string
}

export interface CreateUserInput {
  username: string
  password: string
  email?: string
  nickname?: string
  plan: string
  status: number
}

export interface AdminProject {
  id: string
  name: string
  state: string
  hidden: boolean
  owner: string
  ownerUserId: number
  workspaceId: number
  coverUrl: string
  reviewStatus: string
  reviewNote: string
  createdAt: string
}

export interface AdminLeaderboardRow {
  rank: number
  projectId: string
  name: string
  author: string
  visits: number
  exposure: number
  clicks: number
  hidden: boolean
  publishedAt: string
}

export interface ProjectFile {
  path: string
  type: string
  size: number
  mtime: number
}

export interface TopicNode {
  id: number
  code: string
  name: string
  description: string
  promptTemplate: string
  parentId?: number | null
  sortOrder: number
  status: number
  children: TopicNode[]
}

export interface AdminTemplate {
  id: number
  templateCode: string
  templateName: string
  description: string
  topicId: number
  topicName: string
  parentTopicId: number
  parentTopicName: string
  coverUrl: string
  recommendScore: number
  status: number
}

export interface TopicInput {
  parentId: number | null
  code: string
  name: string
  description: string
  promptTemplate: string
  sortOrder: number
  status: number
}

export interface AdminTemplateInput {
  templateName: string
  description: string
  topicId: number
  recommendScore: number
  status: number
}

export interface TemplateInput {
  templateName: string
  description: string
  topicId: number
  recommendScore: number
}

export interface AdminFdeResource {
  id: number
  name: string
  contact: string
  skills: string[]
  region: string
  note: string
  enabled: boolean
  createdAt: string
  updatedAt: string
}

export interface FdeResourceInput {
  name: string
  contact: string
  skills: string[]
  region: string
  note: string
  enabled: boolean
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
  projectDailyQuota: number
  llmTokenDailyQuota: number
  uploadDailyCountQuota: number
  uploadDailyBytesQuota: number
}

export interface QuotaUpdateInput {
  llmTokensQuota: number
  sandboxMinutesQuota: number
  storageQuotaMb: number
  projectDailyQuota: number
  llmTokenDailyQuota: number
  uploadDailyCountQuota: number
  uploadDailyBytesQuota: number
}

export interface UsageDailyPoint {
  date: string
  projectsCreated: number
  inputTokens: number
  outputTokens: number
  modelRequests: number
  uploadCount: number
  uploadBytes: number
}

export interface UsageDailyTotals {
  projectsCreated: number
  inputTokens: number
  outputTokens: number
  modelRequests: number
  uploadCount: number
  uploadBytes: number
}

export interface UsageDailyResult {
  from: string
  to: string
  userId: number
  series: UsageDailyPoint[]
  totals: UsageDailyTotals
}

export interface AuditEvent {
  id: string
  actor: string
  action: string
  target: string
  createdAt: string
}

export interface IPStat {
  ip: string
  requests: number
}

export interface IPStats {
  items: IPStat[]
  totalRequests: number
  uniqueIps: number
}

export interface AccessLog {
  id: string
  eventType: 'login' | 'request'
  userId: number
  username: string
  workspaceId: number
  projectId: string
  ip: string
  region: string
  deviceId: string
  deviceType: string
  os: string
  browser: string
  userAgent: string
  method: string
  path: string
  query: string
  route: string
  statusCode: number
  durationMs: number
  loginResult: string
  errorMessage: string
  eventAt: string
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

export interface AccessLogQuery extends PageQuery {
  eventType?: string
  from?: string
  to?: string
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

export type ChannelGuideStatus = 'all' | 'enabled' | 'disabled'

export interface AdminChannelGuide {
  channel: string
  title: string
  summary: string
  enabled: boolean
  builtin: boolean
  sortOrder: number
  markdownBytes: number
  updatedAt: string
}

export interface AdminChannelGuideDetail extends AdminChannelGuide {
  markdown: string
  steps: string[]
  requirements: string[]
}

export interface ChannelGuideInput {
  title: string
  summary: string
  markdown: string
  steps: string[]
  requirements: string[]
  enabled: boolean
  sortOrder: number
}

export interface ChannelGuideCreateInput extends ChannelGuideInput {
  channel: string
}

export interface AdminCardSummary {
  id: string
  ownerUserId: number
  ownerName: string
  headline: string
  tags: string[]
  projectCount: number
  featuredType: string
  published: boolean
  hidden: boolean
  status: string
  updatedAt: string
}

export interface AdminScheduledTask {
  id: number
  userId: number
  username: string
  name: string
  projectId: string
  cronExpression: string
  timezone: string
  enabled: boolean
  lastStatus: string
  lastError: string
  lastRunAt?: string
  nextRunAt?: string
  runCount: number
}
