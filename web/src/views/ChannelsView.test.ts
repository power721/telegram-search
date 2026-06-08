import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { apiGet, apiPatch, apiPost } from '@/api/client'
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
        indexed_message_count: 42,
        member_count: 1200,
        description: 'Public movie releases',
        web_access: true,
        history_sync_enabled: false,
        listen_enabled: false,
        remote_search_allowed: true
      },
      {
        id: 2,
        title: 'Anime',
        username: 'animehub',
        type: 'group',
        sync_state: 'synced',
        listen_state: 'enabled',
        sync_profile: 'Normal',
        indexed_message_count: 8,
        member_count: 200,
        description: 'Private anime discussion',
        web_access: false,
        history_sync_enabled: true,
        listen_enabled: true,
        remote_search_allowed: true
      },
      {
        id: 3,
        title: 'Docs',
        username: 'manuals',
        type: 'supergroup',
        sync_state: 'failed',
        listen_state: 'error',
        sync_profile: 'Normal',
        indexed_message_count: 100,
        member_count: 50,
        description: 'Manuals and docs',
        history_sync_enabled: true,
        listen_enabled: true,
        remote_search_allowed: true
      },
      {
        id: 4,
        title: 'Saved',
        username: '',
        type: 'saved_messages',
        sync_state: 'synced',
        listen_state: 'disabled',
        sync_profile: 'Normal',
        indexed_message_count: 7,
        member_count: 0,
        description: '',
        history_sync_enabled: true,
        listen_enabled: false,
        remote_search_allowed: false
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
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  function channelTitles(wrapper: ReturnType<typeof mount>) {
    return wrapper.findAll('tbody tr').map((row) => row.findAll('td').at(0)?.text() ?? '')
  }

  function channelRow(wrapper: ReturnType<typeof mount>, title: string) {
    const row = wrapper.findAll('tbody tr').find((row) => row.findAll('td').at(0)?.text() === title)
    if (!row) throw new Error(`missing row ${title}`)
    return row
  }

  function mountChannelsView() {
    return mount(ChannelsView, {
      global: {
        stubs: {
          'n-button': {
            emits: ['click'],
            props: ['disabled', 'loading'],
            template:
              '<button :disabled="disabled" :data-loading="loading ? \'true\' : \'false\'" @click="$emit(\'click\', $event)"><slot /></button>'
          },
          'n-tag': { template: '<span><slot /></span>' },
          'n-modal': { props: ['show'], template: '<div v-if="show"><slot /></div>' },
          'n-card': { template: '<section class="sync-modal"><slot /></section>' },
          'n-input-number': {
            props: ['value'],
            template: '<input :value="value" @input="$emit(\'update:value\', Number($event.target.value))" />'
          },
          'n-input': {
            props: ['value'],
            template: '<input v-bind="$attrs" :value="value" @input="$emit(\'update:value\', $event.target.value)" />'
          },
          'n-select': {
            props: ['value', 'options'],
            template:
              '<select v-bind="$attrs" :value="value" @change="$emit(\'update:value\', $event.target.value)"><option v-for="option in options" :key="option.value" :value="option.value">{{ option.label }}</option></select>'
          },
          'n-drawer': true,
          'n-drawer-content': true,
          'n-switch': true
        }
      }
    })
  }

  it('renders channel list in Chinese without remote search controls', async () => {
    const wrapper = mountChannelsView()
    await flushPromises()

    expect(wrapper.text()).toContain('频道')
    expect(wrapper.text()).toContain('Movies')
    expect(wrapper.text()).toContain('@movies')
    expect(wrapper.text()).toContain('频道')
    expect(wrapper.text()).toContain('保存的消息')
    expect(wrapper.text()).toContain('无')
    expect(wrapper.text()).toContain('未监听')
    expect(wrapper.text()).toContain('监听中')
    for (const label of ['标题', '用户名', '类型', '成员数', '描述', '同步状态', '监听状态', '已索引消息', '网页访问', '操作']) {
      expect(wrapper.text()).toContain(label)
    }
    expect(wrapper.find('.profile-legend').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('同步档位')
    expect(wrapper.text()).not.toContain('普通')
    expect(wrapper.find('.web-access-text').exists()).toBe(false)
    expect(wrapper.text()).toContain('42')
    expect(wrapper.text()).toContain('1200')
    expect(wrapper.text()).toContain('Public movie releases')
    const usernameCell = channelRow(wrapper, 'Movies').findAll('td').at(1)
    expect(usernameCell?.text()).toBe('@movies')
    const usernameLink = usernameCell?.find('a')
    expect(usernameLink?.attributes('href')).toBe('https://t.me/s/movies')
    expect(usernameLink?.attributes('target')).toBe('_blank')
    const inaccessibleUsernameCell = channelRow(wrapper, 'Anime').findAll('td').at(1)
    expect(inaccessibleUsernameCell?.text()).toBe('@animehub')
    expect(inaccessibleUsernameCell?.find('a').exists()).toBe(false)
    const webAccessCell = channelRow(wrapper, 'Movies').findAll('td').at(8)
    expect(webAccessCell?.text()).toBe('可访问')
    const webAccessLink = webAccessCell?.find('a')
    expect(webAccessLink?.attributes('href')).toBe('https://t.me/s/movies')
    expect(webAccessLink?.attributes('target')).toBe('_blank')
    expect(wrapper.text()).toContain('同步')
    expect(wrapper.text()).toContain('监听')
    expect(wrapper.find('.channel-search').exists()).toBe(true)
    expect(wrapper.find('.type-filter').exists()).toBe(true)
    expect(wrapper.find('.sync-state-filter').exists()).toBe(true)
    expect(wrapper.find('.listen-state-filter').exists()).toBe(true)
    expect(wrapper.find('.web-access-filter').exists()).toBe(true)
    expect(wrapper.find('.sort-select').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('频道类')
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

  it('refreshes without passing the click event as account_id', async () => {
    const wrapper = mountChannelsView()
    await flushPromises()
    vi.mocked(apiGet).mockClear()

    await wrapper.find('.page-header button').trigger('click')
    await flushPromises()

    expect(apiGet).toHaveBeenCalledWith('/api/channels')
  })

  it('searches filters and sorts channel rows from table headers', async () => {
    const wrapper = mountChannelsView()
    await flushPromises()

    await wrapper.find('[data-sort-key="indexed"]').trigger('click')
    expect(channelTitles(wrapper)).toEqual(['Saved', 'Anime', 'Movies', 'Docs'])
    await wrapper.find('[data-sort-key="indexed"]').trigger('click')
    expect(channelTitles(wrapper)).toEqual(['Docs', 'Movies', 'Anime', 'Saved'])
    await wrapper.find('[data-sort-key="username"]').trigger('click')
    expect(channelTitles(wrapper)).toEqual(['Saved', 'Anime', 'Docs', 'Movies'])

    await wrapper.find('.channel-search').setValue('anime')
    expect(channelTitles(wrapper)).toEqual(['Anime'])
    await wrapper.find('.channel-search').setValue('')

    await wrapper.find('.type-filter').setValue('channel')
    expect(channelTitles(wrapper)).toEqual(['Movies'])
    await wrapper.find('.type-filter').setValue('saved_messages')
    expect(channelTitles(wrapper)).toEqual(['Saved'])
    await wrapper.find('.type-filter').setValue('')

    await wrapper.find('.sync-state-filter').setValue('failed')
    expect(channelTitles(wrapper)).toEqual(['Docs'])
    await wrapper.find('.sync-state-filter').setValue('')

    await wrapper.find('.listen-state-filter').setValue('enabled')
    expect(channelTitles(wrapper)).toEqual(['Anime'])
    await wrapper.find('.listen-state-filter').setValue('')

    await wrapper.find('.web-access-filter').setValue('accessible')
    expect(channelTitles(wrapper)).toEqual(['Movies'])
    await wrapper.find('.web-access-filter').setValue('inaccessible')
    expect(channelTitles(wrapper)).toEqual(['Anime'])
    await wrapper.find('.web-access-filter').setValue('unknown')
    expect(channelTitles(wrapper)).toEqual(['Saved', 'Docs'])
  })

  it('checks public channel web access from row and batch actions', async () => {
    const wrapper = mountChannelsView()
    await flushPromises()

    await channelRow(wrapper, 'Movies')
      .findAll('button')
      .find((button) => button.text() === '检测')
      ?.trigger('click')
    expect(apiPost).toHaveBeenCalledWith('/api/channels/web-access/check', { channel_ids: [1] })

    const savedCheckButton = channelRow(wrapper, 'Saved')
      .findAll('button')
      .find((button) => button.text() === '检测')
    expect(savedCheckButton?.attributes('disabled')).toBeDefined()

    await wrapper.find('.batch-web-access-check').trigger('click')
    expect(apiPost).toHaveBeenCalledWith('/api/channels/web-access/check', { channel_ids: [1, 3] })

    await wrapper.find('.type-filter').setValue('saved_messages')
    expect(wrapper.find('.batch-web-access-check').attributes('disabled')).toBeDefined()
  })

  it('does not show row action loading while only the list is loading', async () => {
    const wrapper = mountChannelsView()
    await flushPromises()

    const moviesButtons = channelRow(wrapper, 'Movies').findAll('button')
    expect(moviesButtons.find((button) => button.text() === '同步')?.attributes('data-loading')).toBe('false')
    expect(moviesButtons.find((button) => button.text() === '检测')?.attributes('data-loading')).toBe('false')
    expect(moviesButtons.find((button) => button.text() === '监听')?.attributes('data-loading')).toBe('false')
  })

  it('syncs history and enables listening from row actions', async () => {
    const wrapper = mountChannelsView()
    await flushPromises()

    await channelRow(wrapper, 'Movies')
      .findAll('button')
      .find((button) => button.text() === '同步')
      ?.trigger('click')
    expect(wrapper.find('.sync-modal').text()).toContain('同步记录最大条数')
    await wrapper.find('.sync-modal input').setValue('250')
    await wrapper.findAll('button').find((button) => button.text() === '开始同步')?.trigger('click')
    await channelRow(wrapper, 'Movies')
      .findAll('button')
      .find((button) => button.text() === '监听')
      ?.trigger('click')

    expect(apiPost).toHaveBeenCalledWith('/api/channels/sync', { channel_ids: [1], max_messages: 250 })
    expect(apiPatch).toHaveBeenCalledWith('/api/channels/1/control', {
      history_sync_enabled: false,
      sync_profile: 'Normal',
      listen_enabled: true,
      remote_search_allowed: true
    })
  })
})
