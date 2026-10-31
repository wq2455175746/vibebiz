import { cpHttp, request } from './client'
import type { PublishedProject } from '@/types'

export interface PublishInput {
  title: string
  description: string
  tags: string[]
}

export interface PublishResult {
  url: string
  slug: string
  publishedAt: string
}

export async function listPublishedProjects(): Promise<PublishedProject[]> {
  const result = await request<{ projects: PublishedProject[] }>(cpHttp, {
    method: 'get',
    url: '/me/published-projects',
  })
  return result.projects
}

export async function deletePublishedProject(projectId: string): Promise<void> {
  await request<{ id: string }>(cpHttp, {
    method: 'delete',
    url: `/projects/${encodeURIComponent(projectId)}/publish`,
  })
}

export function publishProject(projectId: string, input: PublishInput): Promise<PublishResult> {
  return request<PublishResult>(cpHttp, {
    method: 'post',
    url: `/projects/${encodeURIComponent(projectId)}/publish`,
    data: input,
  })
}
