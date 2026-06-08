import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiGet } from '@/api/client'
import SearchView from './SearchView.vue'

vi.mock('@/api/client', () => ({
  apiGet: vi.fn(() =>
    Promise.resolve({
      messages: { items: [{ id: 1, text: 'ubuntu local result', source: 'local' }], total: 75 },
      links: { items: [{ id: 2, url: 'https://example.com', source: 'local' }], total: 75 },
      files: { items: [{ id: 3, file_name: 'ubuntu.iso', source: 'local' }], total: 1 },
      channels: { items: [{ id: 4, title: 'Ubuntu Channel', source: 'local' }], total: 1 }
    })
  ),
  apiPost: vi.fn()
}))

describe('SearchView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
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

  it('renders clickable links and loads the next search page with page size 50', async () => {
    const wrapper = mount(SearchView, {
      global: {
        stubs: {
          SearchFilters: {
            template: '<form @submit.prevent="$emit(\'submit\')"><input :value="query" @input="$emit(\'update:query\', $event.target.value)" /></form>',
            props: ['query'],
            emits: ['submit', 'update:query']
          }
        }
      }
    })
    await wrapper.get('input').setValue('ubuntu')
    await wrapper.get('form').trigger('submit')
    await new Promise((resolve) => setTimeout(resolve, 0))

    const link = wrapper.get('a[href="https://example.com"]')
    expect(link.attributes('target')).toBe('_blank')
    expect(link.attributes('rel')).toContain('noopener')

    await wrapper.get('button[aria-label="下一页"]').trigger('click')
    await new Promise((resolve) => setTimeout(resolve, 0))

    expect(apiGet).toHaveBeenCalledWith('/api/search/global?q=ubuntu&limit=50&offset=50')
    expect(wrapper.text()).toContain('第 2 页')
  })

  it('reloads search from page one when page size changes', async () => {
    const wrapper = mount(SearchView, {
      global: {
        stubs: {
          SearchFilters: {
            template: '<form @submit.prevent="$emit(\'submit\')"><input :value="query" @input="$emit(\'update:query\', $event.target.value)" /></form>',
            props: ['query'],
            emits: ['submit', 'update:query']
          }
        }
      }
    })
    await wrapper.get('input').setValue('ubuntu')
    await wrapper.get('form').trigger('submit')
    await new Promise((resolve) => setTimeout(resolve, 0))

    await wrapper.get('select[aria-label="每页条数"]').setValue('100')
    await new Promise((resolve) => setTimeout(resolve, 0))

    expect(apiGet).toHaveBeenCalledWith('/api/search/global?q=ubuntu&limit=100')
    expect(wrapper.text()).toContain('第 1 页')
  })
})
