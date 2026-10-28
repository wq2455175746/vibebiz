import { defineStore } from 'pinia'
import { ref } from 'vue'

import { getProject } from '@/api/projects'
import type { Project } from '@/types'

export const useProjectStore = defineStore('project', () => {
  const current = ref<Project | null>(null)
  const loading = ref(false)
  const error = ref('')

  async function load(id: string) {
    loading.value = true
    error.value = ''
    try {
      current.value = await getProject(id)
    } catch (err) {
      error.value = (err as Error).message
      current.value = null
    } finally {
      loading.value = false
    }
  }

  return { current, loading, error, load }
})
