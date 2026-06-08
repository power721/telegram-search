import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiPatch, apiPost } from '@/api/client'
import SetupChannelSelectionView from './SetupChannelSelectionView.vue'

const push = vi.fn()

vi.mock('vue-router', () => ({
  useRouter: () => ({ push })
}))

vi.mock('naive-ui', async () => {
  const actual = await vi.importActual<typeof import('naive-ui')>('naive-ui')
  return {
    ...actual,
    useMessage: () => ({ error: vi.fn(), success: vi.fn() })
  }
})

vi.mock('@/api/client', () => ({
  apiGet: vi.fn((path: string) => {
    if (path === '/api/channels') {
      return Promise.resolve({
        items: [
          {
            id: 1,
            title: 'Movies',
            username: 'movies',
            type: 'channel',
            member_count: 100,
            description: 'media channel',
            sync_state: 'metadata_only',
            listen_state: 'disabled',
            history_sync_enabled: false,
            sync_profile: 'Normal',
            listen_enabled: false,
            remote_search_allowed: true,
            web_access_error: ''
          }
        ]
      })
    }
    return Promise.resolve({})
  }),
  apiPatch: vi.fn().mockResolvedValue({ id: 1, sync_profile: 'Normal' }),
  apiPost: vi.fn((path: string) => {
    if (path === '/api/channels/sync') return Promise.resolve({ job_id: 1, status: 'queued' })
    if (path === '/api/setup/complete') return Promise.resolve({ complete: true, current_step: 'complete' })
    return Promise.resolve({})
  })
}))

describe('SetupChannelSelectionView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('saves selected channel controls, starts history sync, then completes setup', async () => {
    const wrapper = mount(SetupChannelSelectionView, {
      global: {
        stubs: {
          'n-checkbox': {
            emits: ['update:checked'],
            template: `<label><input type="checkbox" @change="$emit('update:checked', true)" /><slot /></label>`
          },
          'n-select': true,
          'n-button': { emits: ['click'], template: `<button @click="$emit('click')"><slot /></button>` }
        }
      }
    })
    await flushPromises()

    await wrapper.find('input[type="checkbox"]').trigger('change')
    await wrapper.findAll('button').at(-1)?.trigger('click')
    await flushPromises()

    expect(apiPatch).toHaveBeenCalledWith('/api/channels/1/control', {
      history_sync_enabled: true,
      sync_profile: 'Normal',
      listen_enabled: true,
      remote_search_allowed: true
    })
    expect(apiPost).toHaveBeenCalledWith('/api/watch-rules', {
      channel_id: 1,
      enabled: true,
      includes: [],
      excludes: [],
      message_types: ['link', 'text'],
      link_types: ['cloud_drive', 'magnet', 'ed2k', 'other']
    })
    expect(apiPost).toHaveBeenCalledWith('/api/channels/sync', { channel_ids: [1] })
    expect(apiPost).toHaveBeenCalledWith('/api/setup/complete')
    expect(push).toHaveBeenCalledWith('/')
  })
})
