import { request } from './client'
import { toEntity, toList } from './normalize'
import type {
  AdminProject,
  AdminQuota,
  AdminSandbox,
  AdminUser,
  AuditEvent,
  Overview,
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
