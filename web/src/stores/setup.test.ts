import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiPost } from '@/api/client'
import { useSetupStore } from './setup'

vi.mock('@/api/client', () => ({
  apiGet: vi.fn().mockResolvedValue({
    complete: false,
    admin_configured: false,
    api_key_configured: false,
    telegram_configured: false
  }),
  apiPost: vi.fn().mockResolvedValue({ ok: true })
}))

describe('setup store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
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

  it('supports optional api key and listen rules setup steps', async () => {
    const store = useSetupStore()
    await store.createAPIKey('cli')
    await store.skipAPIKey()
    await store.saveListenRules({
      includes: ['电影'],
      excludes: ['预告'],
      message_types: ['link', 'text'],
      link_types: ['cloud_drive', 'magnet', 'ed2k', 'other']
    })

    expect(apiPost).toHaveBeenCalledWith('/api/setup/api-key', { name: 'cli' })
    expect(apiPost).toHaveBeenCalledWith('/api/setup/api-key/skip')
    expect(apiPost).toHaveBeenCalledWith('/api/setup/listen-rules', {
      includes: ['电影'],
      excludes: ['预告'],
      message_types: ['link', 'text'],
      link_types: ['cloud_drive', 'magnet', 'ed2k', 'other']
    })
  })
})
