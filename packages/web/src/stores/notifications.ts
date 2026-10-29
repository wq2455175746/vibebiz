import { defineStore } from 'pinia'
import { ref } from 'vue'

import { listNotifications } from '@/api/notifications'
import type { NotificationItem } from '@/types'

export const useNotificationsStore = defineStore('notifications', () => {
  const items = ref<NotificationItem[]>([])
  const loading = ref(false)
  const error = ref('')

  async function load(silent = false): Promise<void> {
    if (!silent) {
      loading.value = true
    }
    error.value = ''
    try {
      items.value = await listNotifications()
    } catch (err) {
      error.value = err instanceof Error ? err.message : '加载失败'
    } finally {
      if (!silent) {
        loading.value = false
      }
    }
  }

  function clear(): void {
    items.value = []
    error.value = ''
  }

  return { items, loading, error, load, clear }
})
