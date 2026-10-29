import { request } from './client'
import { toEntity, toList } from './normalize'
import type {
  AdminAgentRole,
  AdminLLMModel,
  AdminProject,
  AdminQuota,
  AdminSandbox,
  AdminUser,
  AuditEvent,
  ModelInput,
  Overview,
  RoleAvatarPresign,
  RoleInput,
} from '@/types'

export async function getOverview(): Promise<Overview> {
  const result = await request<Overview | { overview: Overview }>({
    method: 'get',
    url: '/overview',
  })
  return toEntity(result, 'overview')
}

export async function getUsers(): Promise<AdminUser[]> {
  const result = await request<AdminUser[] | { users: AdminUser[] }>({
    method: 'get',
    url: '/users',
  })
  return toList(result, 'users')
}

export async function getProjects(): Promise<AdminProject[]> {
  const result = await request<AdminProject[] | { projects: AdminProject[] }>({
    method: 'get',
    url: '/projects',
  })
  return toList(result, 'projects')
}

export async function getSandboxes(): Promise<AdminSandbox[]> {
  const result = await request<AdminSandbox[] | { sandboxes: AdminSandbox[] }>({
    method: 'get',
    url: '/sandboxes',
  })
  return toList(result, 'sandboxes')
}

export async function getQuotas(): Promise<AdminQuota[]> {
  const result = await request<AdminQuota[] | { quotas: AdminQuota[] }>({
    method: 'get',
    url: '/quotas',
  })
  return toList(result, 'quotas')
}

export async function getAudit(): Promise<AuditEvent[]> {
  const result = await request<AuditEvent[] | { audit: AuditEvent[] }>({
    method: 'get',
    url: '/audit',
  })
  return toList(result, 'audit')
}

export async function getModels(): Promise<AdminLLMModel[]> {
  const result = await request<AdminLLMModel[] | { models: AdminLLMModel[] }>({
    method: 'get',
    url: '/models',
  })
  return toList(result, 'models')
}

export async function createModel(input: ModelInput): Promise<AdminLLMModel> {
  return request<AdminLLMModel>({ method: 'post', url: '/models', data: input })
}

export async function getRoles(): Promise<AdminAgentRole[]> {
  const result = await request<AdminAgentRole[] | { roles: AdminAgentRole[] }>({
    method: 'get',
    url: '/roles',
  })
  return toList(result, 'roles')
}

export async function createRole(input: RoleInput): Promise<AdminAgentRole> {
  return request<AdminAgentRole>({ method: 'post', url: '/roles', data: input })
}

export async function presignRoleAvatar(input: {
  filename: string
  contentType: string
  size: number
}): Promise<RoleAvatarPresign> {
  return request<RoleAvatarPresign>({
    method: 'post',
    url: '/roles/avatar/presign',
    data: input,
  })
}

export async function uploadRoleAvatar(file: File): Promise<string> {
  const result = await request<{ key: string }>({
    method: 'post',
    url: '/roles/avatar/upload',
    data: file,
    headers: { 'Content-Type': file.type },
  })
  return result.key
}
