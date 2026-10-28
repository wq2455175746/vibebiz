import { defineStore } from 'pinia'

export const useOpsStore = defineStore('ops', {
  state: () => ({
    environment: 'LOCAL',
    apiBase: import.meta.env.VITE_OPS_API_BASE || '/cp/api/v1/admin',
  }),
})
