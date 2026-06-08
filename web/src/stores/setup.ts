import { defineStore } from 'pinia'
import { apiGet, apiPost } from '@/api/client'
import type { SetupStatus } from '@/api/types'

export const useSetupStore = defineStore('setup', {
  state: () => ({
    status: undefined as SetupStatus | undefined,
    loaded: false,
    loading: false
  }),
  actions: {
    async load() {
      this.loading = true
      try {
        this.status = await apiGet<SetupStatus>('/api/setup/status')
        this.loaded = true
      } finally {
        this.loading = false
      }
    },
    async createAdmin(username: string, password: string) {
      await apiPost('/api/setup/admin', { username, password })
      await this.load()
    },
    async completeSetup() {
      this.status = await apiPost<SetupStatus>('/api/setup/complete')
      this.loaded = true
    }
  }
})
