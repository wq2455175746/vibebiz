import { defineStore } from 'pinia'
import { ref } from 'vue'

import { listNotifications, markNotificationsRead } from '@/api/notifications'
import type { NotificationItem } from '@/types'

export const useNotificationsStore = defineStore('notifications', () => {
  const items = ref<NotificationItem[]>([])
  const unread = ref(false)
  const loading = ref(false)
  const error = ref('')

  async function load(silent = false): Promise<void> {
    if (!silent) {
      loading.value = true
      error.value = ''
    }
    try {
      const result = await listNotifications()
      items.value = result.notifications
      unread.value = result.unread
      error.value = ''
    } catch (err) {
      if (!silent) {
        error.value = err instanceof Error ? err.message : '加载失败'
      }
    } finally {
      if (!silent) {
        loading.value = false
      }
    }
  }

  async function markRead(): Promise<void> {
    const newest = items.value.reduce(
      (latest, item) => (item.publishedAt > latest ? item.publishedAt : latest),
      '',
    )
    if (!newest) {
      unread.value = false
      return
    }
    try {
      await markNotificationsRead(newest)
      unread.value = false
    } catch {
      // Keep the unread dot when the read marker cannot be stored.
    }
  }

  function clear(): void {
    items.value = []
    unread.value = false
    error.value = ''
  }

  return { items, unread, loading, error, load, markRead, clear }
})
