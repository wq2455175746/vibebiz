export interface WorkspaceMembership {
  id: number
  name: string
  isOwner: boolean
  role: string
}

export interface AuthUser {
  id: number
  username: string
  email: string
  avatarUrl: string | null
  currentWorkspaceId: number
  workspaces: WorkspaceMembership[]
}

export interface Workspace {
  id: number
  name: string
  description: string
  avatarUrl: string | null
  isOwner: boolean
  role: string
  memberCount: number
}

export interface Member {
  userId: number
  username: string
  email: string
  avatarUrl: string | null
  role: string
  status: number
  joinedAt: string | null
  isOwner: boolean
}

export interface WorkspaceFolder {
  id: number
  parentId: number
  name: string
  createdAt: string
}

export interface Presign {
  uploadUrl: string
  key: string
  headers: Record<string, string>
}

export interface StorageProject {
  id: string
  name: string
  sizeBytes: number
  ratio: number
  fileCount: number
  dirCount: number
  lastModifiedAt: string
  available: boolean
}

export interface StorageSummary {
  usedBytes: number
  quotaBytes: number
  projectCount: number
  projects: StorageProject[]
}

export interface Project {
  id: string
  mode?: 'standard' | 'enterprise'
  name: string
  prompt: string
  previewPath: string
  state: string
  createdAt: string
  updatedAt: string
  defaultAgentRoleId?: number | null
  agentRoleIds?: number[] | null
  coverS3Path?: string | null
  coverUrl?: string | null
}

export type ProjectStageStatus = 'locked' | 'active' | 'done' | 'placeholder'
export type ProjectStageWorkspace = 'document' | 'development' | 'placeholder'

export interface ProjectStage {
  stageKey: string
  seq: number
  title: string
  roleCode: string
  roleName: string
  artifactPath: string
  workspace: ProjectStageWorkspace
  status: ProjectStageStatus
  startedAt?: string | null
  completedAt?: string | null
}

export interface ProjectStages {
  mode: 'standard' | 'enterprise'
  currentStage: string
  stages: ProjectStage[]
}

export interface StageMutation {
  stage: ProjectStage
  nextStage?: ProjectStage
}

export interface Template {
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

export interface TopicNode {
  id: number
  code: string
  name: string
  children: TopicNode[]
}

export interface AgentRole {
  id: number
  roleCode: string
  roleNameEn: string
  roleNameZh: string
  description: string
  avatarUrl: string
}

export interface FileEntry {
  path: string
  type: 'file' | 'dir'
  size: number
  mtime: number
}

export interface FileContent {
  path: string
  content: string
  version: string
  size: number
  mtime: number
  isBinary: boolean
}

export interface WriteResult {
  path: string
  version: string
  size: number
  mtime: number
}

export interface ToolState {
  id: string
  name: string
  status: 'running' | 'done' | 'error'
}

export interface ChatReference {
  kind: 'resource' | 'project_file'
  id?: string
  name: string
  path: string
  contentType?: string
  resourceKind?: string
}

export interface ChatMessage {
  id: string
  role: 'user' | 'assistant' | 'system'
  text: string
  createdAt?: string
  tools: ToolState[]
  references?: ChatReference[]
  pending: boolean
}

export type NotificationChannelType = 'webhook' | 'feishu' | 'wecom' | 'dingtalk' | 'email'

export interface NotificationChannelConfig {
  url?: string
  secret?: string
  to?: string
}

export interface NotificationChannel {
  id: number
  name: string
  type: NotificationChannelType
  config: NotificationChannelConfig
  enabled: boolean
  lastTestAt?: string | null
  lastTestStatus: '' | 'success' | 'failed'
  lastTestError: string
}

export interface NotificationChannelInput {
  name: string
  type: NotificationChannelType
  config: NotificationChannelConfig
  enabled: boolean
}

export interface ProjectResource {
  id: string
  name: string
  displayName: string
  storageName: string
  path: string
  contentType: string
  size: number
  kind: 'document' | 'text' | 'image'
  createdAt: string
}

export type LogLevel = 'info' | 'warn' | 'error'

export interface EditorLog {
  id: string
  seq: number
  ts: string
  level: LogLevel
  source: string
  message: string
}

export type NotificationType = 'version_release' | 'inbox'

export interface NotificationItem {
  id: number
  title: string
  content: string
  type: NotificationType
  version: string
  link: string
  publishedAt: string
  expireAt: string
}

export interface ScheduledTaskDraft {
  name: string
  description: string
  scheduleType: 'daily' | 'weekly' | 'cron'
  time: string
  weekdays: number[]
  cronExpression: string
  timezone: string
}

export interface ScheduledTaskInput extends ScheduledTaskDraft {
  source?: 'ai' | 'manual'
}

export interface ScheduledTask {
  id: number
  name: string
  description: string
  scheduleType: 'daily' | 'weekly' | 'cron'
  time: string
  weekdays: number[]
  cronExpression: string
  timezone: string
  source: 'ai' | 'manual'
  enabled: boolean
  projectId: string
  nextRunAt?: string
  lastRunAt?: string
  lastStatus: string
  lastError: string
  runCount: number
}
