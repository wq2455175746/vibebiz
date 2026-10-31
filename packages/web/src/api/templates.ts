import { cpHttp, request } from './client'
import type { Project, Template, TopicNode } from '@/types'

export interface TemplatePage {
  templates: Template[]
  total: number
  page: number
  pageSize: number
}

export async function listTemplates(
  params: {
    page?: number
    pageSize?: number
    keyword?: string
    parentTopicId?: number
    topicId?: number
    sort?: 'latest'
  } = {},
): Promise<TemplatePage> {
  return request<TemplatePage>(cpHttp, { method: 'get', url: '/templates', params })
}

export async function listTemplateTopics(): Promise<TopicNode[]> {
  const result = await request<{ topics: TopicNode[] }>(cpHttp, {
    method: 'get',
    url: '/templates/topics',
  })
  return result.topics ?? []
}

export async function useTemplate(id: number, name = ''): Promise<Project> {
  return request<Project>(cpHttp, {
    method: 'post',
    url: `/templates/${id}/use`,
    data: { name },
  })
}
