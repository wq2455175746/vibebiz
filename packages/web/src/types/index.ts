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
  name: string
  prompt: string
  state: string
  createdAt: string
  updatedAt: string
  defaultAgentRoleId?: number | null
  agentRoleIds?: number[] | null
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
