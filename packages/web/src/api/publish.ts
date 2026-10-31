import { cpHttp, request } from './client'

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

export function publishProject(projectId: string, input: PublishInput): Promise<PublishResult> {
  return request<PublishResult>(cpHttp, {
    method: 'post',
    url: `/projects/${encodeURIComponent(projectId)}/publish`,
    data: input,
  })
}
