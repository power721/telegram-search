import { defineStore } from 'pinia'
import { apiGet, apiPost, apiPut } from '@/api/client'
import type { User } from '@/api/types'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    user: undefined as User | undefined,
    loaded: false,
    loading: false
  }),
  getters: {
    authenticated: (state) => state.user !== undefined
  },
  actions: {
    async loadMe() {
      this.loading = true
      try {
        this.user = await apiGet<User>('/api/auth/me')
      } catch {
        this.user = undefined
      } finally {
        this.loaded = true
        this.loading = false
      }
    },
    async login(username: string, password: string) {
      this.user = await apiPost<User>('/api/auth/login', { username, password })
      this.loaded = true
    },
    async updateCredentials(username: string, currentPassword: string, newPassword: string) {
      this.user = await apiPut<User>('/api/settings/admin', {
        username,
        current_password: currentPassword,
        new_password: newPassword
      })
      this.loaded = true
      return this.user
    },
    async logout() {
      await apiPost('/api/auth/logout')
      this.user = undefined
      this.loaded = true
    }
  }
})
