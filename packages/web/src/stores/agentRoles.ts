import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import { listAgentRoles } from '@/api/agentRoles'
import type { AgentRole } from '@/types'

export const useAgentRolesStore = defineStore('agentRoles', () => {
  const roles = ref<AgentRole[]>([])
  const loading = ref(false)
  const loaded = ref(false)
  const error = ref('')

  let inflight: Promise<void> | null = null

  const team = computed(() => roles.value.find((role) => role.roleCode === 'team') ?? null)

  function byId(id: number): AgentRole | undefined {
    return roles.value.find((role) => role.id === id)
  }

  function load(force = false): Promise<void> {
    if (loaded.value && !force) {
      return Promise.resolve()
    }
    if (inflight) {
      return inflight
    }
    inflight = (async () => {
      loading.value = true
      error.value = ''
      try {
        roles.value = await listAgentRoles()
        loaded.value = true
      } catch (err) {
        error.value = (err as Error).message
      } finally {
        loading.value = false
        inflight = null
      }
    })()
    return inflight
  }

  return { roles, loading, loaded, error, team, byId, load }
})
