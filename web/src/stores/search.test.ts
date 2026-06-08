import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiGet, apiPost } from '@/api/client'
import { useSearchStore } from './search'

vi.mock('@/api/client', () => ({
  apiGet: vi.fn(),
  apiPost: vi.fn()
}))

describe('useSearchStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(apiGet).mockReset()
    vi.mocked(apiPost).mockReset()
  })

  it('loads grouped global search results', async () => {
    vi.mocked(apiGet).mockResolvedValue({
      messages: { items: [{ id: 1, text: 'ubuntu local', source: 'local' }], total: 1 },
      links: { items: [], total: 0 },
      files: { items: [], total: 0 },
      channels: { items: [], total: 0 }
    })
    const store = useSearchStore()

    await store.searchGlobal('ubuntu')

    expect(apiGet).toHaveBeenCalledWith('/api/search/global?q=ubuntu&limit=50')
    expect(store.global?.messages.total).toBe(1)
  })

  it('creates and loads remote search results', async () => {
    vi.mocked(apiPost).mockResolvedValue({ id: 7, query: 'ubuntu', source: 'remote' })
    vi.mocked(apiGet).mockResolvedValue({ task: { id: 7 }, items: [{ source: 'remote' }] })
    const store = useSearchStore()

    await store.createRemoteSearch(3, 'ubuntu')
    await store.loadRemoteResults(7)

    expect(apiPost).toHaveBeenCalledWith('/api/search/remote', { channel_id: 3, query: 'ubuntu' })
    expect(apiGet).toHaveBeenCalledWith('/api/search/remote/7')
    expect(store.remoteResults?.items[0].source).toBe('remote')
  })
})
