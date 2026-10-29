import { request } from './client'
import { toList } from './normalize'
import type { NotificationInput, OpsNotification } from '@/types'

export async function listNotifications(): Promise<OpsNotification[]> {
  const result = await request<OpsNotification[] | { notifications: OpsNotification[] }>({
    method: 'get',
    url: '/notifications',
  })
  return toList(result, 'notifications')
}

export function createNotification(input: NotificationInput): Promise<OpsNotification> {
  return request<OpsNotification>({ method: 'post', url: '/notifications', data: input })
}

export function updateNotification(id: number, input: NotificationInput): Promise<OpsNotification> {
  return request<OpsNotification>({ method: 'put', url: `/notifications/${id}`, data: input })
}

export function publishNotification(id: number): Promise<OpsNotification> {
  return request<OpsNotification>({ method: 'post', url: `/notifications/${id}/publish` })
}

export function unpublishNotification(id: number): Promise<OpsNotification> {
  return request<OpsNotification>({ method: 'post', url: `/notifications/${id}/unpublish` })
}

export async function deleteNotification(id: number): Promise<void> {
  await request<{ deleted: boolean }>({ method: 'delete', url: `/notifications/${id}` })
}
