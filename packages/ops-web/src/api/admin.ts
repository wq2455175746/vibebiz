import { request } from './client'
import { toEntity, toList, toPage } from './normalize'
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
  PageQuery,
  PageResult,
  RoleAvatarPresign,
  RoleInput,
  RoleUpdateInput,
} from '@/types'

function pageParams(query: PageQuery): PageQuery {
  return { page: query.page, pageSize: query.pageSize, keyword: query.keyword }
}

export async function getOverview(): Promise<Overview> {
  const result = await request<Overview | { overview: Overview }>({
    method: 'get',
    url: '/overview',
  })
  return toEntity(result, 'overview')
}

export async function getUsers(query: PageQuery = {}): Promise<PageResult<AdminUser>> {
  const result = await request<AdminUser[] | Record<string, unknown>>({
    method: 'get',
    url: '/users',
    params: pageParams(query),
  })
  return toPage(result, 'users')
}

export async function getProjects(query: PageQuery = {}): Promise<PageResult<AdminProject>> {
  const result = await request<AdminProject[] | Record<string, unknown>>({
    method: 'get',
    url: '/projects',
    params: pageParams(query),
  })
  return toPage(result, 'projects')
}

export async function getSandboxes(query: PageQuery = {}): Promise<PageResult<AdminSandbox>> {
  const result = await request<AdminSandbox[] | Record<string, unknown>>({
    method: 'get',
    url: '/sandboxes',
    params: pageParams(query),
  })
  return toPage(result, 'sandboxes')
}

export async function getQuotas(): Promise<AdminQuota[]> {
  const result = await request<AdminQuota[] | { quotas: AdminQuota[] }>({
    method: 'get',
    url: '/quotas',
  })
  return toList(result, 'quotas')
}

export async function getAudit(query: PageQuery = {}): Promise<PageResult<AuditEvent>> {
  const result = await request<AuditEvent[] | Record<string, unknown>>({
    method: 'get',
    url: '/audit',
    params: pageParams(query),
  })
  return toPage(result, 'audit')
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

export async function getRoles(query: PageQuery = {}): Promise<PageResult<AdminAgentRole>> {
  const result = await request<AdminAgentRole[] | Record<string, unknown>>({
    method: 'get',
    url: '/roles',
    params: pageParams(query),
  })
  return toPage(result, 'roles')
}

export async function createRole(input: RoleInput): Promise<AdminAgentRole> {
  return request<AdminAgentRole>({ method: 'post', url: '/roles', data: input })
}

export async function updateRole(id: number, input: RoleUpdateInput): Promise<AdminAgentRole> {
  return request<AdminAgentRole>({ method: 'put', url: `/roles/${id}`, data: input })
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
