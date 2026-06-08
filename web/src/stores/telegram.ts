import { defineStore } from 'pinia'
import { apiGet, apiPost } from '@/api/client'
import type {
  TelegramAccountsResponse,
  TelegramAccount,
  TelegramAPISettingsResponse,
  TelegramLoginResponse
} from '@/api/types'

export const useTelegramStore = defineStore('telegram', {
  state: () => ({
    settings: null as TelegramAPISettingsResponse | null,
    accounts: [] as TelegramAccount[],
    phone: '',
    passwordRequired: false,
    loading: false,
    error: '',
    loginResult: null as TelegramLoginResponse | null
  }),
  actions: {
    async loadSettings() {
      return this.withLoading(async () => {
        this.settings = await apiGet<TelegramAPISettingsResponse>('/api/settings/telegram-api')
        return this.settings
      })
    },
    async saveTelegramAPI(appID: number, appHash: string) {
      return this.withLoading(async () => {
        this.settings = await apiPost<TelegramAPISettingsResponse>('/api/setup/telegram-api', {
          app_id: appID,
          app_hash: appHash
        })
        return this.settings
      })
    },
    async sendCode(phone: string) {
      return this.withLoading(async () => {
        this.phone = phone
        this.passwordRequired = false
        this.loginResult = await apiPost<TelegramLoginResponse>('/api/telegram/login/send-code', {
          phone
        })
        return this.loginResult
      })
    },
    async signIn(code: string) {
      return this.withLoading(async () => {
        this.loginResult = await apiPost<TelegramLoginResponse>('/api/telegram/login/sign-in', {
          phone: this.phone,
          code
        })
        this.passwordRequired = this.loginResult.password_required === true
        if (this.loginResult.account) {
          await this.loadAccounts()
        }
        return this.loginResult
      })
    },
    async submitPassword(password: string) {
      return this.withLoading(async () => {
        this.loginResult = await apiPost<TelegramLoginResponse>('/api/telegram/login/password', {
          phone: this.phone,
          password
        })
        this.passwordRequired = false
        if (this.loginResult.account) {
          await this.loadAccounts()
        }
        return this.loginResult
      })
    },
    async loadAccounts() {
      return this.withLoading(async () => {
        const response = await apiGet<TelegramAccountsResponse>('/api/accounts')
        this.accounts = response.items
        return this.accounts
      })
    },
    async withLoading<T>(fn: () => Promise<T>): Promise<T> {
      this.loading = true
      this.error = ''
      try {
        return await fn()
      } catch (error) {
        this.error = error instanceof Error ? error.message : 'Request failed'
        throw error
      } finally {
        this.loading = false
      }
    }
  }
})
