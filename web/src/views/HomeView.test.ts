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
    if (path === '/api/tasks') {
      return Promise.resolve({
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
    expect(wrapper.text()).toContain('Storage Usage')
    expect(wrapper.text()).toContain('4.3 GB')
    expect(wrapper.text()).toContain('Top Resource Types')
    expect(wrapper.text()).toContain('Cloud Drive')
    expect(wrapper.text()).toContain('Recent Task Errors')
    expect(wrapper.text()).toContain('temporary failure')
  })
})
