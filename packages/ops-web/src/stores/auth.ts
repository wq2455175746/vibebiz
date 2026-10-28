import { defineStore } from 'pinia'

import * as authApi from '@/api/auth'
import { ApiError } from '@/api/client'

import type { OpsUser } from '@/types'

export const useOpsAuthStore = defineStore('ops-auth', {
  state: () => ({
    user: null as OpsUser | null,
    ready: false,
    loading: false,
  }),
  getters: {
    isAuthenticated: (state) => state.user !== null,
  },
  actions: {
    async login(identifier: string, password: string) {
      this.loading = true
      try {
        this.user = await authApi.login(identifier, password)
        this.ready = true
      } finally {
        this.loading = false
      }
    },
    async ensure() {
      if (this.ready) return
      this.loading = true
      try {
        this.user = await authApi.me()
        this.ready = true
      } catch (error) {
        if (error instanceof ApiError && error.status === 401) {
          this.user = null
          this.ready = true
          return
        }
        throw error
      } finally {
        this.loading = false
      }
    },
    async logout() {
      await authApi.logout().catch(() => undefined)
      this.clear()
    },
    clear() {
      this.user = null
      this.ready = true
    },
  },
})
