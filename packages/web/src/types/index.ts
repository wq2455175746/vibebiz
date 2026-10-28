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
  status: 'running' | 'done'
}

export interface ChatMessage {
  id: string
  role: 'user' | 'assistant' | 'system'
  text: string
  tools: ToolState[]
  pending: boolean
}
