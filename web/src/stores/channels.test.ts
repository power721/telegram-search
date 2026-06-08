import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiGet, apiPatch, apiPost } from '@/api/client'
import { useChannelsStore } from './channels'

vi.mock('@/api/client', () => ({
  apiGet: vi.fn().mockResolvedValue({
    items: [
      {
        id: 1,
        account_id: 1,
        telegram_channel_id: 100,
        title: 'Movies',
        username: 'movies',
        type: 'channel',
        sync_profile: 'Normal',
        web_access: false
      }
    ]
  }),
  apiPatch: vi.fn((path: string) => {
    if (path === '/api/channels/control') {
      return Promise.resolve({ items: [{ id: 1, sync_profile: 'Normal' }] })
    }
    return Promise.resolve({ id: 1, sync_profile: 'Deep' })
  }),
  apiPost: vi.fn((path: string) => {
    if (path.endsWith('/analyze')) {
      return Promise.resolve({ channel: { id: 1 }, indexed_counts: { messages: 0, links: 0, files: 0 } })
    }
    if (path === '/api/search/remote') {
      return Promise.resolve({ id: 1, status: 'queued', source: 'remote' })
    }
    return Promise.resolve({ items: [] })
  })
}))

describe('channels store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('calls channel control API endpoints', async () => {
    const store = useChannelsStore()
    await store.loadChannels(1)
    await store.updateControl(1, {
      history_sync_enabled: true,
      sync_profile: 'Deep',
      listen_enabled: false,
      remote_search_allowed: true
    })
    await store.updateControls([1], {
      history_sync_enabled: true,
      sync_profile: 'Normal',
      listen_enabled: true,
      remote_search_allowed: true
    })
    await store.checkWebAccess([1])
    await store.syncChannels([1], 250)
    await store.analyzeChannel(1)
    await store.createRemoteSearch(1, 'ubuntu iso')

    expect(apiGet).toHaveBeenCalledWith('/api/channels?account_id=1')
    expect(apiPatch).toHaveBeenCalledWith('/api/channels/1/control', {
      history_sync_enabled: true,
      sync_profile: 'Deep',
      listen_enabled: false,
      remote_search_allowed: true
    })
    expect(apiPatch).toHaveBeenCalledWith('/api/channels/control', {
      channel_ids: [1],
      control: {
        history_sync_enabled: true,
        sync_profile: 'Normal',
        listen_enabled: true,
        remote_search_allowed: true
      }
    })
    expect(apiPost).toHaveBeenCalledWith('/api/channels/web-access/check', { channel_ids: [1] })
    expect(apiPost).toHaveBeenCalledWith('/api/channels/sync', { channel_ids: [1], max_messages: 250 })
    expect(apiPost).toHaveBeenCalledWith('/api/channels/1/analyze')
    expect(apiPost).toHaveBeenCalledWith('/api/search/remote', { channel_id: 1, query: 'ubuntu iso' })
  })
})
