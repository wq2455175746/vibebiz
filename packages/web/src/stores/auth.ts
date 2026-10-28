import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import * as authApi from '@/api/auth'
import { ApiError } from '@/api/client'
import type { AuthUser } from '@/types'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<AuthUser | null>(null)
  const ready = ref(false)
  const loading = ref(false)
  const isAuthenticated = computed(() => user.value !== null)

  async function login(identifier: string, password: string): Promise<AuthUser> {
    loading.value = true
    try {
      const current = await authApi.login(identifier, password)
      user.value = current
      ready.value = true
      return current
    } finally {
      loading.value = false
    }
  }

  async function ensure(): Promise<void> {
    if (ready.value) {
      return
    }
    loading.value = true
    try {
      user.value = await authApi.me()
      ready.value = true
    } catch (error) {
      if (error instanceof ApiError && error.status === 401) {
        user.value = null
        ready.value = true
        return
      }
      throw error
    } finally {
      loading.value = false
    }
  }

  function clear(): void {
    user.value = null
    ready.value = true
  }

  async function logout(): Promise<void> {
    try {
      await authApi.logout()
    } catch {
      // The local session is cleared even when the server logout fails.
    } finally {
      clear()
    }
  }

  return { user, ready, loading, isAuthenticated, login, logout, ensure, clear }
})
