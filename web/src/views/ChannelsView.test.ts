import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ChannelsView from './ChannelsView.vue'

vi.mock('@/api/client', () => ({
  apiGet: vi.fn().mockResolvedValue({
    items: [
      {
        id: 1,
        title: 'Movies',
        username: 'movies',
        type: 'channel',
        sync_state: 'metadata_only',
        listen_state: 'disabled',
        sync_profile: 'Normal',
        web_access: false,
        history_sync_enabled: false,
        listen_enabled: false,
        remote_search_allowed: true
      }
    ]
  }),
  apiPatch: vi.fn(),
  apiPost: vi.fn()
}))

describe('ChannelsView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('renders channel control table and sync profile labels', async () => {
    const wrapper = mount(ChannelsView, {
      global: {
        stubs: {
          'n-button': { template: '<button><slot /></button>' },
          'n-tag': { template: '<span><slot /></span>' },
          'n-select': true,
          'n-drawer': true,
          'n-drawer-content': true,
          'n-switch': true,
          'n-input': true
        }
      }
    })
    await flushPromises()

    expect(wrapper.text()).toContain('Channels')
    expect(wrapper.text()).toContain('Movies')
    expect(wrapper.text()).toContain('@movies')
    expect(wrapper.text()).toContain('channel')
    expect(wrapper.text()).toContain('metadata_only')
    expect(wrapper.text()).toContain('disabled')
    for (const label of ['Quick', 'Normal', 'Deep', 'Full']) {
      expect(wrapper.text()).toContain(label)
    }
  })
})
