import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiPut } from '@/api/client'
import { useAuthStore } from './auth'

vi.mock('@/api/client', () => ({
  apiGet: vi.fn().mockResolvedValue({ id: 1, username: 'admin', role: 'admin' }),
  apiPost: vi.fn().mockResolvedValue({ id: 1, username: 'admin', role: 'admin' }),
  apiPut: vi.fn().mockResolvedValue({ id: 1, username: 'root', role: 'admin' })
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

  it('updates admin credentials and stores returned user', async () => {
    const store = useAuthStore()
    await store.updateCredentials('root', 'secret123', 'newsecret123')
    expect(apiPut).toHaveBeenCalledWith('/api/settings/admin', {
      username: 'root',
      current_password: 'secret123',
      new_password: 'newsecret123'
    })
    expect(store.user?.username).toBe('root')
  })
})
