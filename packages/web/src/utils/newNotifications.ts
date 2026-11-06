import type { NotificationItem } from '@/types'

export function newNotifications(
  previous: NotificationItem[],
  current: NotificationItem[],
): NotificationItem[] {
  const seen = new Set(previous.map((item) => item.id))
  return current.filter((item) => !seen.has(item.id))
}
