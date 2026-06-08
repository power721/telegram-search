import { defineStore } from 'pinia'
import { apiGet, apiPost, setAPIKey } from '@/api/client'
import type { APIKeySetupResponse, ListenRulesPayload, SetupStatus } from '@/api/types'

export const useSetupStore = defineStore('setup', {
  state: () => ({
    status: undefined as SetupStatus | undefined,
    listenRules: {
      includes: [],
      excludes: [],
      message_types: ['link', 'text'],
      link_types: ['cloud_drive', 'magnet', 'ed2k', 'other']
    } as ListenRulesPayload,
    createdAPIKey: null as APIKeySetupResponse | null,
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
    async createAPIKey() {
      this.createdAPIKey = await apiPost<APIKeySetupResponse>('/api/setup/api-key')
      setAPIKey(this.createdAPIKey.key)
      await this.load()
      return this.createdAPIKey
    },
    async saveListenRules(payload: ListenRulesPayload) {
      this.status = await apiPost<SetupStatus>('/api/setup/listen-rules', payload)
      this.listenRules = payload
      this.loaded = true
    },
    async completeSetup() {
      this.status = await apiPost<SetupStatus>('/api/setup/complete')
      this.loaded = true
    }
  }
})
