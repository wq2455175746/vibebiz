import { request } from './client'
import type { AdminCardSummary, PageQuery, PageResult } from '@/types'

export async function listCards(query: PageQuery = {}): Promise<PageResult<AdminCardSummary>> {
  return request<PageResult<AdminCardSummary>>({
    method: 'get',
    url: '/cards',
    params: { page: query.page, pageSize: query.pageSize, keyword: query.keyword },
  })
}

export async function updateCardHidden(id: string, hidden: boolean): Promise<void> {
  await request<unknown>({
    method: 'put',
    url: `/cards/${encodeURIComponent(id)}/hidden`,
    data: { hidden },
  })
}

export async function deleteCard(id: string): Promise<void> {
  await request<unknown>({
    method: 'delete',
    url: `/cards/${encodeURIComponent(id)}`,
  })
}
