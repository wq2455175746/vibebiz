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
  projectId: string
  node: string
  state: string
  startedAt: string
}

export interface AdminQuota {
  userId: string
  plan: string
  llmTokens: number
  sandboxMinutes: number
  storageMb: number
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
  publishedAt: string | null
  createdBy: number
  createdByName: string
  createdAt: string
  updatedAt: string
}

export interface NotificationInput {
  title: string
  content: string
  type: NotificationType
  version: string
  link: string
  expireAt: string
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
