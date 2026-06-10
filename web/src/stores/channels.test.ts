import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiDelete, apiGet, apiPatch, apiPost, apiPut } from '@/api/client'
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
  apiDelete: vi.fn(() => Promise.resolve({ deleted: true })),
  apiPost: vi.fn((path: string) => {
    if (path.endsWith('/analyze')) {
      return Promise.resolve({ channel: { id: 1 }, indexed_counts: { messages: 0, links: 0, files: 0 } })
    }
    if (path === '/api/channels/1/clear') {
      return Promise.resolve({
        channel: { id: 1, title: 'Movies', sync_profile: 'Normal', listen_enabled: false, indexed_message_count: 0 },
        deleted: { messages: 3, links: 2, files: 1 }
      })
    }
    if (path === '/api/admin/search/remote') {
      return Promise.resolve({ id: 1, status: 'queued', source: 'remote' })
    }
    return Promise.resolve({ items: [] })
  }),
  apiPut: vi.fn(() => Promise.resolve({ id: 1, enabled: true }))
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
    await store.clearChannel(1)
    await store.analyzeChannel(1)
    await store.updateWatchRule(7, {
      channel_id: 1,
      enabled: true,
      includes: ['movie'],
      excludes: [],
      message_types: ['link'],
      link_types: ['cloud_drive']
    })
    await store.deleteWatchRule(7)
    await store.loadGlobalListenRules()
    await store.updateGlobalListenRules({
      includes: ['movie'],
      excludes: [],
      message_types: ['link'],
      link_types: ['cloud_drive']
    })
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
    expect(apiPost).toHaveBeenCalledWith('/api/channels/1/clear')
    expect(apiPost).toHaveBeenCalledWith('/api/channels/1/analyze')
    expect(apiPut).toHaveBeenCalledWith('/api/watch-rules/7', {
      channel_id: 1,
      enabled: true,
      includes: ['movie'],
      excludes: [],
      message_types: ['link'],
      link_types: ['cloud_drive']
    })
    expect(apiDelete).toHaveBeenCalledWith('/api/watch-rules/7')
    expect(apiGet).toHaveBeenCalledWith('/api/listen-rules')
    expect(apiPut).toHaveBeenCalledWith('/api/listen-rules', {
      includes: ['movie'],
      excludes: [],
      message_types: ['link'],
      link_types: ['cloud_drive']
    })
    expect(apiPost).toHaveBeenCalledWith('/api/admin/search/remote', { channel_id: 1, query: 'ubuntu iso' })
  })
})
