import { defineStore } from 'pinia'
import { ref } from 'vue'

import { listFdeResources } from '@/api/fdeResources'
import type { FdeResource } from '@/types/enterpriseSpace'

export const useFdeResourcesStore = defineStore('fdeResources', () => {
  const items = ref<FdeResource[]>([])
  const loading = ref(false)
  const error = ref('')

  async function load(): Promise<void> {
    loading.value = true
    error.value = ''
    try {
      items.value = await listFdeResources()
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'FDE 资源加载失败'
    } finally {
      loading.value = false
    }
  }

  return { items, loading, error, load }
})
