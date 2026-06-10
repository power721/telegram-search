import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { apiGet, apiPatch, apiPost } from '@/api/client'
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
      const many = Array.from({ length: 60 }, (_, index) => ({
        id: index + 1,
        title: `Channel ${index + 1}`,
        username: index === 0 ? 'movies' : '',
        type: 'channel',
        member_count: 100 + index,
        description:
          index === 0
            ? 'media channel with a long description that should not force the setup table wider than the viewport'
            : '',
        sync_state: index === 1 ? 'failed' : 'metadata_only',
        listen_state: 'disabled',
        history_sync_enabled: false,
        sync_profile: 'Normal',
        listen_enabled: false,
        remote_search_allowed: true,
        web_access: index === 0 ? true : undefined,
        web_access_error: index === 1 ? 'channel unavailable or banned' : ''
      }))
      return Promise.resolve({
        items: many
      })
    }
    return Promise.resolve({})
  }),
  apiPatch: vi.fn().mockResolvedValue({ items: [{ id: 1, sync_profile: 'Normal' }] }),
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

  afterEach(() => {
    vi.useRealTimers()
  })

  it('renders channel status in pages and saves selection with batch APIs', async () => {
    const wrapper = mount(SetupChannelSelectionView, {
      global: {
        stubs: {
          'n-checkbox': {
            emits: ['update:checked'],
            template: `<label><input type="checkbox" @change="$emit('update:checked', true)" /><slot /></label>`
          },
          'n-tag': { template: `<span class="n-tag"><slot /></span>` },
          'n-button': { emits: ['click'], template: `<button @click="$emit('click')"><slot /></button>` }
        }
      }
    })
    await flushPromises()

    expect(wrapper.findAll('tbody tr')).toHaveLength(50)
    expect(wrapper.text()).toContain('网页可访问')
    expect(wrapper.text()).toContain('封禁/不可用')
    expect(wrapper.text()).not.toContain('历史同步')

    await wrapper.find('input[type="checkbox"]').trigger('change')
    await wrapper.findAll('button').at(-1)?.trigger('click')
    await flushPromises()

    expect(apiPatch).toHaveBeenCalledWith('/api/channels/control', {
      channel_ids: [1],
      control: {
        history_sync_enabled: true,
        sync_profile: 'Normal',
        listen_enabled: true,
        remote_search_allowed: true
      }
    })
    expect(apiPost).not.toHaveBeenCalledWith('/api/watch-rules', expect.anything())
    expect(apiPost).toHaveBeenCalledWith('/api/channels/sync', { channel_ids: [1] })
    expect(apiPost).toHaveBeenCalledWith('/api/setup/complete')
    expect(push).toHaveBeenCalledWith('/')
  })

  it('explains an empty first-run channel list and refreshes automatically', async () => {
    vi.useFakeTimers()
    vi.mocked(apiGet)
      .mockResolvedValueOnce({ items: [] })
      .mockResolvedValueOnce({
        items: [
          {
            id: 1,
            title: 'Channel 1',
            username: 'movies',
            type: 'channel',
            member_count: 100,
            description: '',
            sync_state: 'metadata_only',
            listen_state: 'disabled',
            history_sync_enabled: false,
            sync_profile: 'Normal',
            listen_enabled: false,
            remote_search_allowed: true,
            web_access: true,
            web_access_error: ''
          }
        ]
      })

    const wrapper = mount(SetupChannelSelectionView, {
      global: {
        stubs: {
          'n-checkbox': {
            emits: ['update:checked'],
            template: `<label><input type="checkbox" @change="$emit('update:checked', true)" /><slot /></label>`
          },
          'n-button': { emits: ['click'], template: `<button @click="$emit('click')"><slot /></button>` }
        }
      }
    })
    await flushPromises()

    expect(wrapper.text()).toContain('正在更新频道列表')
    expect(wrapper.text()).toContain('稍后会自动刷新')

    await vi.advanceTimersByTimeAsync(3000)
    await flushPromises()

    expect(apiGet).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('Channel 1')

    wrapper.unmount()
  })
})
