import type { NotificationItem } from '@/types'

export function groupNotifications(items: NotificationItem[]) {
  return {
    version_release: items.filter((item) => item.type === 'version_release'),
    inbox: items.filter((item) => item.type === 'inbox'),
  }
}
