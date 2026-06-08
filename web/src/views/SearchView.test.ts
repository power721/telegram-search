import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import SearchView from './SearchView.vue'

vi.mock('@/api/client', () => ({
  apiGet: vi.fn(() =>
    Promise.resolve({
      messages: { items: [{ id: 1, text: 'ubuntu local result', source: 'local' }], total: 1 },
      links: { items: [{ id: 2, url: 'https://example.com', source: 'local' }], total: 1 },
      files: { items: [{ id: 3, file_name: 'ubuntu.iso', source: 'local' }], total: 1 },
      channels: { items: [{ id: 4, title: 'Ubuntu Channel', source: 'local' }], total: 1 }
    })
  ),
  apiPost: vi.fn()
}))

describe('SearchView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('renders grouped search sections and source labels', async () => {
    const wrapper = mount(SearchView)
    await new Promise((resolve) => setTimeout(resolve, 0))

    expect(wrapper.text()).toContain('消息')
    expect(wrapper.text()).toContain('链接')
    expect(wrapper.text()).toContain('文件')
    expect(wrapper.text()).toContain('频道')
    expect(wrapper.text()).not.toContain('Messages')
    expect(wrapper.text()).not.toContain('Channels')
  })
})
