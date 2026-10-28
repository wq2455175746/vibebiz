import { cpHttp, request } from './client'
import type { Project } from '@/types'

export async function listProjects(): Promise<Project[]> {
  const result = await request<{ projects: Project[] }>(cpHttp, { method: 'get', url: '/projects' })
  return result.projects
}

export async function createProject(prompt: string): Promise<Project> {
  return request<Project>(cpHttp, { method: 'post', url: '/projects', data: { prompt } })
}

export async function getProject(id: string): Promise<Project> {
  return request<Project>(cpHttp, { method: 'get', url: `/projects/${encodeURIComponent(id)}` })
}
