import type { NotificationStatus, NotificationType, OpsNotification } from '@/types'

const statusLabels: Record<NotificationStatus, string> = {
  draft: '草稿',
  published: '已发布',
  unpublished: '已取消发布',
  expired: '已过期',
}

const typeLabels: Record<NotificationType, string> = {
  version_release: '版本发布',
  inbox: '收集箱',
}

export function notificationStatusLabel(status: NotificationStatus): string {
  return statusLabels[status] ?? status
}

export function notificationTypeLabel(type: NotificationType): string {
  return typeLabels[type] ?? type
}

type NotificationExpiry = Pick<OpsNotification, 'status'> & { expireAt?: string | null }

const RETENTION_MS = 30 * 24 * 60 * 60 * 1000

export function isExpired(item: NotificationExpiry, now = new Date()): boolean {
  if (item.status === 'expired') {
    return true
  }
  if (!item.expireAt) {
    return false
  }
  const expireAt = new Date(item.expireAt).getTime()
  return !Number.isNaN(expireAt) && expireAt <= now.getTime()
}

export function canPublish(item: NotificationExpiry, now = new Date()): boolean {
  if (item.status === 'published' || item.status === 'expired') {
    return false
  }
  return !isExpired(item, now)
}

export function canUnpublish(item: Pick<OpsNotification, 'status'>): boolean {
  return item.status === 'published' || item.status === 'expired'
}

export function canEdit(item: Pick<OpsNotification, 'status'>): boolean {
  return item.status === 'draft' || item.status === 'unpublished'
}

export function notificationDateBounds(now = new Date()): {
  minDate: Date
  maxDate: Date
} {
  const minDate = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  const maxDate = new Date(minDate.getFullYear(), minDate.getMonth(), minDate.getDate() + 30)
  return { minDate, maxDate }
}

export function resolveExpireAt(date: Date, now = new Date()): string {
  const endOfDay = new Date(date.getFullYear(), date.getMonth(), date.getDate(), 23, 59, 59)
  const latest = now.getTime() + RETENTION_MS - 60_000
  return new Date(Math.min(endOfDay.getTime(), latest)).toISOString()
}
