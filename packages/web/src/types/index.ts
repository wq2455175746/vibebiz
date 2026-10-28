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
