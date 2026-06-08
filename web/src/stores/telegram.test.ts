import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiGet, apiPost } from '@/api/client'
import { useTelegramStore } from './telegram'

vi.mock('@/api/client', () => ({
  apiGet: vi.fn((path: string) => {
    if (path === '/api/accounts') {
      return Promise.resolve({
        items: [
          {
            id: 1,
            phone: '+10000000000',
            telegram_user_id: 42,
            first_name: 'Ada',
            last_name: 'Lovelace',
            username: 'ada',
            status: 'ONLINE',
            last_error: ''
          }
        ]
      })
    }
    if (path === '/api/settings/telegram-api') {
      return Promise.resolve({ configured: true, app_id: 123456, app_hash_set: true })
    }
    return Promise.reject(new Error(`unexpected get ${path}`))
  }),
  apiPost: vi.fn((path: string) => {
    if (path === '/api/telegram/login/sign-in') {
      return Promise.resolve({ status: 'LOGIN_REQUIRED', password_required: true })
    }
    if (path === '/api/telegram/login/password') {
      return Promise.resolve({
        status: 'ONLINE',
        account: { id: 1, phone: '+10000000000', status: 'ONLINE', last_error: '' },
        metadata_sync: { status: 'succeeded', channel_count: 3 }
      })
    }
    return Promise.resolve({ status: 'LOGIN_REQUIRED' })
  })
}))

describe('telegram store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('uses the telegram setup and login API paths', async () => {
    const store = useTelegramStore()

    await store.loadSettings()
    await store.saveTelegramAPI(123456, 'hash-secret')
    await store.sendCode('+10000000000')
    await store.signIn('12345')
    await store.submitPassword('2fa-secret')
    await store.loadAccounts()

    expect(apiGet).toHaveBeenCalledWith('/api/settings/telegram-api')
    expect(apiPost).toHaveBeenCalledWith('/api/setup/telegram-api', {
      app_id: 123456,
      app_hash: 'hash-secret'
    })
    expect(apiPost).toHaveBeenCalledWith('/api/telegram/login/send-code', {
      phone: '+10000000000'
    })
    expect(apiPost).toHaveBeenCalledWith('/api/telegram/login/sign-in', {
      phone: '+10000000000',
      code: '12345'
    })
    expect(apiPost).toHaveBeenCalledWith('/api/telegram/login/password', {
      phone: '+10000000000',
      password: '2fa-secret'
    })
    expect(apiGet).toHaveBeenCalledWith('/api/accounts')
  })

  it('keeps password required state after a 2FA challenge response', async () => {
    const store = useTelegramStore()
    await store.sendCode('+10000000000')
    const response = await store.signIn('12345')

    expect(response.password_required).toBe(true)
    expect(store.passwordRequired).toBe(true)
    expect(store.loginResult?.status).toBe('LOGIN_REQUIRED')
  })
})
