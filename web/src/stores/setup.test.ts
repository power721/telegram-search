import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiPost, setAPIKey } from '@/api/client'
import { useSetupStore } from './setup'

vi.mock('@/api/client', () => ({
  apiGet: vi.fn().mockResolvedValue({
    complete: false,
    admin_configured: false,
    api_key_configured: false,
    telegram_configured: false
  }),
  apiPost: vi.fn((path: string) => {
    if (path === '/api/setup/api-key') {
      return Promise.resolve({
        id: 1,
        name: 'default',
        prefix: '12345678',
        key: '12345678123456781234567812345678'
      })
    }
    return Promise.resolve({ ok: true })
  }),
  setAPIKey: vi.fn()
}))

describe('setup store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('loads setup status', async () => {
    const store = useSetupStore()
    await store.load()
    expect(store.status?.admin_configured).toBe(false)
    expect(store.loaded).toBe(true)
  })

  it('marks setup complete through the setup complete endpoint', async () => {
    const store = useSetupStore()
    await store.completeSetup()
    expect(apiPost).toHaveBeenCalledWith('/api/setup/complete')
  })

  it('auto-generates api key and supports listen rules setup steps', async () => {
    const store = useSetupStore()
    await store.createAPIKey()
    await store.saveListenRules({
      includes: ['电影'],
      excludes: ['预告'],
      message_types: ['link', 'text'],
      link_types: ['cloud_drive', 'magnet', 'ed2k', 'other'],
      ignored_link_patterns: ['t.me']
    })

    expect(apiPost).toHaveBeenCalledWith('/api/setup/api-key')
    expect(setAPIKey).toHaveBeenCalledWith('12345678123456781234567812345678')
    expect(apiPost).toHaveBeenCalledWith('/api/setup/listen-rules', {
      includes: ['电影'],
      excludes: ['预告'],
      message_types: ['link', 'text'],
      link_types: ['cloud_drive', 'magnet', 'ed2k', 'other'],
      ignored_link_patterns: ['t.me']
    })
  })
})
