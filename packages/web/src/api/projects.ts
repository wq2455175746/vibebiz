import { cpHttp, dpHttp, request } from './client'
import type { Project } from '@/types'

export async function listProjects(): Promise<Project[]> {
  const result = await request<{ projects: Project[] }>(cpHttp, { method: 'get', url: '/projects' })
  return result.projects
}

export async function createProject(
  prompt: string,
  agentRoleIds: number[] = [],
  mode?: Project['mode'],
): Promise<Project> {
  return request<Project>(cpHttp, {
    method: 'post',
    url: '/projects',
    data: { prompt, agentRoleIds, ...(mode ? { mode } : {}) },
  })
}

export async function getProject(id: string): Promise<Project> {
  return request<Project>(cpHttp, { method: 'get', url: `/projects/${encodeURIComponent(id)}` })
}

export async function updateProjectAgentRoles(
  id: string,
  agentRoleIds: number[],
): Promise<number[]> {
  const result = await request<{ agentRoleIds: number[] | null }>(cpHttp, {
    method: 'put',
    url: `/projects/${encodeURIComponent(id)}/agent-roles`,
    data: { agentRoleIds },
  })
  return result.agentRoleIds ?? []
}

export interface ProjectCover {
  coverS3Path: string
  coverUrl: string
}

export async function uploadProjectCover(projectId: string, image: Blob): Promise<ProjectCover> {
  return request<ProjectCover>(dpHttp, {
    method: 'post',
    url: `/projects/${encodeURIComponent(projectId)}/cover`,
    data: image,
    headers: { 'Content-Type': 'image/png' },
    timeout: 30_000,
  })
}

export async function deleteProject(id: string): Promise<void> {
  await request<{ id: string }>(cpHttp, {
    method: 'delete',
    url: `/projects/${encodeURIComponent(id)}`,
  })
}
