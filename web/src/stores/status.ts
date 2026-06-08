import { defineStore } from 'pinia'
import { apiGet } from '@/api/client'
import type { ServiceStatus, StorageUsage } from '@/api/types'

export const useStatusStore = defineStore('status', {
  state: () => ({
    service: undefined as ServiceStatus | undefined,
    storage: undefined as StorageUsage | undefined,
    loading: false
  }),
  actions: {
    async load() {
      this.loading = true
      try {
        const [service, storage] = await Promise.all([
          apiGet<ServiceStatus>('/api/status'),
          apiGet<StorageUsage>('/api/storage/usage')
        ])
        this.service = service
        this.storage = storage
      } finally {
        this.loading = false
      }
    }
  }
})
