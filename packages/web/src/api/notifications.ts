import type { NotificationItem } from '@/types'

import { cpHttp, request } from './client'

export interface NotificationListResult {
  notifications: NotificationItem[]
  unread: boolean
}

export async function listNotifications(): Promise<NotificationListResult> {
  const result = await request<{ notifications: NotificationItem[]; unread?: boolean }>(cpHttp, {
    method: 'get',
    url: '/notifications',
  })
  return { notifications: result.notifications ?? [], unread: result.unread ?? false }
}

export async function markNotificationsRead(readAt: string): Promise<void> {
  await request<{ unread: boolean }>(cpHttp, {
    method: 'post',
    url: '/notifications/read',
    data: { readAt },
  })
}
