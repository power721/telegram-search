import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import HomeView from './HomeView.vue'

vi.mock('@/api/client', () => ({
  apiGet: vi.fn((path: string) => {
    if (path === '/api/status') {
      return Promise.resolve({
        service: 'ok',
        accounts: 1,
        channels: 2,
        messages: 100,
        links: 30,
        account_states: { ONLINE: 1 }
      })
    }
    if (path === '/api/storage/usage') {
      return Promise.resolve({
        db_bytes: 3_200_000_000,
        index_bytes: 1_100_000_000,
        media_cache_bytes: 0,
        total_bytes: 4_300_000_000,
        max_db_bytes: 10_000_000_000,
        max_media_bytes: 20_000_000_000,
        db_over_quota: false,
        media_over_quota: false
      })
    }
    if (path === '/api/resources/grouped') {
      return Promise.resolve({
        grouped: { cloud_drive: 2, magnet: 1, ed2k: 0, http: 3, files: 4 }
      })
    }
    if (path === '/api/links/grouped') {
      return Promise.resolve({
        grouped: { aliyun: 2, quark: 3, magnet: 1 }
      })
    }
    if (path.startsWith('/api/tasks')) {
      return Promise.resolve({
        total: 6,
        items: [{ id: 1, type: 'history_sync', status: 'failed', error_message: 'temporary failure' }]
      })
    }
    return Promise.reject(new Error('unexpected path'))
  })
}))

describe('HomeView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('renders storage usage', async () => {
    const wrapper = mount(HomeView)
    await new Promise((resolve) => setTimeout(resolve, 0))
    expect(wrapper.text()).toContain('存储使用')
    expect(wrapper.text()).toContain('4.3 GB')
    expect(wrapper.text()).toContain('资源类型统计')
    expect(wrapper.text()).toContain('网盘')
    expect(wrapper.text()).toContain('链接类型统计')
    expect(wrapper.text()).toContain('夸克')
    expect(wrapper.text()).toContain('任务')
    expect(wrapper.text()).toContain('6')
    expect(wrapper.text()).toContain('最近任务错误')
    expect(wrapper.text()).toContain('temporary failure')
    expect(wrapper.find('.home-search').exists()).toBe(true)
    expect(wrapper.get('input[name="q"]').attributes('placeholder')).toBe('搜索消息、链接、文件、频道')
  })
})
