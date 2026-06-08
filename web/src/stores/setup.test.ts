import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
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
})
