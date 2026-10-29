import type { NotificationItem } from '@/types'

import { cpHttp, request } from './client'

export async function listNotifications(): Promise<NotificationItem[]> {
  const result = await request<{ notifications: NotificationItem[] }>(cpHttp, {
    method: 'get',
    url: '/notifications',
  })
  return result.notifications ?? []
}
