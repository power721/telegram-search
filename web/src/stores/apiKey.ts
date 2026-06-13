import { defineStore } from 'pinia'
import { apiGet, apiPost } from '@/api/client'
import type { APIKeyResponse } from '@/api/types'

export const useAPIKeyStore = defineStore('apiKey', {
  state: () => ({
    current: undefined as APIKeyResponse | undefined,
    loading: false,
    error: ''
  }),
  actions: {
    async load() {
      this.loading = true
      this.error = ''
      try {
        this.current = await apiGet<APIKeyResponse>('/api/settings/api-key')
        return this.current
      } catch (error) {
        this.error = error instanceof Error ? error.message : '无法加载 API 密钥'
        throw error
      } finally {
        this.loading = false
      }
    },
    async regenerate() {
      this.loading = true
      this.error = ''
      try {
        this.current = await apiPost<APIKeyResponse>('/api/settings/api-key/regenerate')
        return this.current
      } catch (error) {
        this.error = error instanceof Error ? error.message : '无法重新生成 API 密钥'
        throw error
      } finally {
        this.loading = false
      }
    }
  }
})
