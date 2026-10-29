import type { NotificationStatus, NotificationType, OpsNotification } from '@/types'

const statusLabels: Record<NotificationStatus, string> = {
  draft: '草稿',
  published: '已发布',
  unpublished: '已取消发布',
  expired: '已过期',
}

const typeLabels: Record<NotificationType, string> = {
  version_release: '版本发布',
  inbox: '站内信',
}

export function notificationStatusLabel(status: NotificationStatus): string {
  return statusLabels[status] ?? status
}

export function notificationTypeLabel(type: NotificationType): string {
  return typeLabels[type] ?? type
}

export function isExpired(item: Pick<OpsNotification, 'status'>, _now = new Date()): boolean {
  return item.status === 'expired'
}

export function canEdit(item: Pick<OpsNotification, 'status'>): boolean {
  return item.status === 'draft' || item.status === 'unpublished'
}
