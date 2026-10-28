import type { StorageSummary } from '@/types'

import { cpHttp, request } from './client'

export function getStorage(): Promise<StorageSummary> {
  return request<StorageSummary>(cpHttp, { method: 'get', url: '/storage' })
}
