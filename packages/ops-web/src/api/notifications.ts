import { request } from './client'
import type {
  NotificationInput,
  NotificationPage,
  NotificationQuery,
  OpsNotification,
} from '@/types'

export async function listNotifications(query: NotificationQuery = {}): Promise<NotificationPage> {
  const page = query.page ?? 1
  const pageSize = query.pageSize ?? 20
  const result = await request<NotificationPage>({
    method: 'get',
    url: '/notifications',
    params: { page, pageSize, keyword: query.keyword },
  })
  return {
    notifications: result.notifications ?? [],
    total: result.total ?? 0,
    page: result.page ?? page,
    pageSize: result.pageSize ?? pageSize,
  }
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
