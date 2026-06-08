import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { router } from './index'

vi.mock('@/api/client', () => ({
  apiGet: vi.fn((path: string) => {
    if (path === '/api/setup/status') {
      return Promise.resolve({
        complete: false,
        admin_configured: false,
        api_key_configured: false,
        telegram_configured: false
      })
    }
    return Promise.reject(new Error('unauthorized'))
  })
}))

describe('router guards', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('sends fresh installs to setup admin', async () => {
    await router.push('/')
    await router.isReady()
    expect(router.currentRoute.value.name).toBe('setup-admin')
  })
})
