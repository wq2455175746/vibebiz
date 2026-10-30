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
  AdminTemplate,
  AdminTemplateInput,
  ProjectFile,
  QuotaUpdateInput,
  TemplateInput,
  TopicInput,
  TopicNode,
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

export async function getProjectFiles(id: string): Promise<ProjectFile[]> {
  const result = await request<{ files: ProjectFile[] }>({
    method: 'get',
    url: `/projects/${id}/files`,
  })
  return result.files ?? []
}

export function projectPreviewURL(id: string): string {
  const base = import.meta.env.VITE_OPS_API_BASE || '/cp/api/v1/admin'
  return `${base}/projects/${encodeURIComponent(id)}/live/`
}

export async function reviewProject(
  id: string,
  status: 'approved' | 'rejected',
  note: string,
): Promise<void> {
  await request<{ id: string }>({
    method: 'put',
    url: `/projects/${id}/review`,
    data: { status, note },
  })
}

export async function getTemplateTopics(): Promise<TopicNode[]> {
  const result = await request<{ topics: TopicNode[] }>({ method: 'get', url: '/topics' })
  return result.topics ?? []
}

export async function getAdminTopics(): Promise<TopicNode[]> {
  const result = await request<{ topics: TopicNode[] }>({
    method: 'get',
    url: '/topics',
    params: { all: 1 },
  })
  return result.topics ?? []
}

export async function createTopic(input: TopicInput): Promise<TopicNode> {
  return request<TopicNode>({ method: 'post', url: '/topics', data: input })
}

export async function updateTopic(id: number, input: TopicInput): Promise<TopicNode> {
  return request<TopicNode>({ method: 'put', url: `/topics/${id}`, data: input })
}

export async function getAdminTemplates(query: PageQuery = {}): Promise<PageResult<AdminTemplate>> {
  const result = await request<Record<string, unknown>>({
    method: 'get',
    url: '/templates',
    params: pageParams(query),
  })
  return toPage<AdminTemplate>(result, 'templates')
}

export async function updateTemplate(
  id: number,
  input: AdminTemplateInput,
): Promise<AdminTemplate> {
  return request<AdminTemplate>({ method: 'put', url: `/templates/${id}`, data: input })
}

export async function deleteTemplate(id: number): Promise<void> {
  await request<{ deleted: boolean }>({ method: 'delete', url: `/templates/${id}` })
}

export async function getTemplateFiles(id: number): Promise<ProjectFile[]> {
  const result = await request<{ files: ProjectFile[] }>({
    method: 'get',
    url: `/templates/${id}/files`,
  })
  return result.files ?? []
}

export async function createTemplate(
  projectId: string,
  input: TemplateInput,
): Promise<{ id: number }> {
  return request<{ id: number }>({
    method: 'post',
    url: `/projects/${projectId}/templates`,
    data: input,
  })
}

export async function updateUserStatus(id: string, status: number): Promise<void> {
  await request<{ id: string; status: number }>({
    method: 'put',
    url: `/users/${id}/status`,
    data: { status },
  })
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

export async function updateQuota(userId: string, input: QuotaUpdateInput): Promise<void> {
  await request<{ userId: string }>({
    method: 'put',
    url: `/quotas/${userId}`,
    data: input,
  })
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
