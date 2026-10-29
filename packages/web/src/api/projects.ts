import { cpHttp, request } from './client'
import type { Project } from '@/types'

export async function listProjects(): Promise<Project[]> {
  const result = await request<{ projects: Project[] }>(cpHttp, { method: 'get', url: '/projects' })
  return result.projects
}

export async function createProject(prompt: string, agentRoleIds: number[] = []): Promise<Project> {
  return request<Project>(cpHttp, {
    method: 'post',
    url: '/projects',
    data: { prompt, agentRoleIds },
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
