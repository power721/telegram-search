import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { router } from './index'

let setupStatus = {
  complete: false,
  admin_configured: false,
  api_key_configured: false,
  api_key_step_complete: false,
  telegram_configured: false,
  telegram_login_complete: false,
  listen_rules_configured: false,
  current_step: 'admin'
}

vi.mock('@/api/client', () => ({
  apiGet: vi.fn((path: string) => {
    if (path === '/api/setup/status') {
      return Promise.resolve(setupStatus)
    }
    if (path === '/api/auth/me') {
      return Promise.resolve({ id: 1, username: 'admin', role: 'admin' })
    }
    return Promise.reject(new Error('unauthorized'))
  })
}))

describe('router guards', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    setupStatus = {
      complete: false,
      admin_configured: false,
      api_key_configured: false,
      api_key_step_complete: false,
      telegram_configured: false,
      telegram_login_complete: false,
      listen_rules_configured: false,
      current_step: 'admin'
    }
  })

  it('sends fresh installs to setup admin', async () => {
    await router.push('/')
    await router.isReady()
    expect(router.currentRoute.value.name).toBe('setup-admin')
  })

  it('uses setup current_step to route authenticated first-run setup', async () => {
    setupStatus = {
      complete: false,
      admin_configured: true,
      api_key_configured: false,
      api_key_step_complete: true,
      telegram_configured: true,
      telegram_login_complete: true,
      listen_rules_configured: false,
      current_step: 'listen_rules'
    }

    await router.push('/')
    await router.isReady()

    expect(router.currentRoute.value.name).toBe('setup-listen-rules')
  })
})
