import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useAuthStore } from './auth'

vi.mock('@/api/client', () => ({
  apiGet: vi.fn().mockResolvedValue({ id: 1, username: 'admin', role: 'admin' }),
  apiPost: vi.fn().mockResolvedValue({ id: 1, username: 'admin', role: 'admin' })
}))

describe('auth store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('logs in and stores current user', async () => {
    const store = useAuthStore()
    await store.login('admin', 'secret123')
    expect(store.user?.username).toBe('admin')
    expect(store.authenticated).toBe(true)
  })
})
