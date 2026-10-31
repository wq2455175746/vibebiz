import { cpHttp, request } from './client'
import type { FdeResource } from '@/types/enterpriseSpace'

export async function listFdeResources(): Promise<FdeResource[]> {
  const result = await request<{ resources: FdeResource[] }>(cpHttp, {
    method: 'get',
    url: '/fde-resources',
  })
  return result.resources ?? []
}
