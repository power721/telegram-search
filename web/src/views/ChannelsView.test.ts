import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiPatch, apiPost } from '@/api/client'
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
  apiPatch: vi.fn().mockResolvedValue({
    id: 1,
    title: 'Movies',
    username: 'movies',
    type: 'channel',
    sync_state: 'metadata_only',
    listen_state: 'enabled',
    sync_profile: 'Normal',
    web_access: false,
    history_sync_enabled: false,
    listen_enabled: true,
    remote_search_allowed: true
  }),
  apiPost: vi.fn().mockResolvedValue({ job_id: 'job-1', status: 'queued' })
}))

describe('ChannelsView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('renders channel list in Chinese without remote search controls', async () => {
    const wrapper = mount(ChannelsView, {
      global: {
        stubs: {
          'n-button': { template: '<button @click="$emit(\'click\')"><slot /></button>' },
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

    expect(wrapper.text()).toContain('频道')
    expect(wrapper.text()).toContain('Movies')
    expect(wrapper.text()).toContain('@movies')
    expect(wrapper.text()).toContain('频道类')
    expect(wrapper.text()).toContain('仅元数据')
    expect(wrapper.text()).toContain('未启用')
    for (const label of ['快速', '普通', '深度', '完整']) expect(wrapper.text()).toContain(label)
    for (const label of ['标题', '用户名', '类型', '同步状态', '监听状态', '同步档位', '网页访问', '操作']) {
      expect(wrapper.text()).toContain(label)
    }
    expect(wrapper.text()).toContain('同步')
    expect(wrapper.text()).toContain('监听')
    expect(wrapper.text()).not.toContain('Channels')
    expect(wrapper.text()).not.toContain('Refresh')
    expect(wrapper.text()).not.toContain('Edit Controls')
    expect(wrapper.text()).not.toContain('Analyze')
    expect(wrapper.text()).not.toContain('Check Web Access')
    expect(wrapper.text()).not.toContain('No Web Access')
    expect(wrapper.text()).not.toContain('Remote Search')
    expect(wrapper.text()).not.toContain('channel')
    expect(wrapper.text()).not.toContain('metadata_only')
    expect(wrapper.text()).not.toContain('disabled')
    expect(wrapper.find('.remote-input').exists()).toBe(false)
  })

  it('syncs history and enables listening from row actions', async () => {
    const wrapper = mount(ChannelsView, {
      global: {
        stubs: {
          'n-button': { template: '<button @click="$emit(\'click\')"><slot /></button>' },
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

    const buttons = wrapper.findAll('button')
    await buttons.find((button) => button.text() === '同步')?.trigger('click')
    await buttons.find((button) => button.text() === '监听')?.trigger('click')

    expect(apiPost).toHaveBeenCalledWith('/api/channels/sync', { channel_ids: [1] })
    expect(apiPatch).toHaveBeenCalledWith('/api/channels/1/control', {
      history_sync_enabled: false,
      sync_profile: 'Normal',
      listen_enabled: true,
      remote_search_allowed: true
    })
  })
})
