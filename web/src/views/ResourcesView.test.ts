import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiGet } from '@/api/client'
import ResourcesView from './ResourcesView.vue'

vi.mock('@/api/client', () => ({
  apiGet: vi.fn((path: string) => {
    if (path.startsWith('/api/resources/grouped')) {
      return Promise.resolve({ grouped: { cloud_drive: 1, magnet: 2, ed2k: 3, http: 4, files: 5 } })
    }
    return Promise.resolve({
      items: [
        {
          id: 'link:1',
          kind: 'link',
          category: 'cloud_drive',
          title: 'Course Pack',
          url: 'https://example.com/course'
        }
      ],
      total: 75,
      grouped: { cloud_drive: 1, magnet: 2, ed2k: 3, http: 4, files: 5 }
    })
  })
}))

describe('ResourcesView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('renders resource filters and table', async () => {
    const wrapper = mount(ResourcesView)
    await new Promise((resolve) => setTimeout(resolve, 0))

    for (const label of ['网盘', '磁力', 'ED2K', 'HTTP', '文件']) {
      expect(wrapper.text()).toContain(label)
    }
    expect(wrapper.text()).toContain('Course Pack')
    expect(wrapper.text()).not.toContain('Cloud Drive')
    expect(wrapper.text()).not.toContain('Files')

    const link = wrapper.get('a[href="https://example.com/course"]')
    expect(link.attributes('target')).toBe('_blank')
    expect(link.attributes('rel')).toContain('noopener')
  })

  it('loads the next resources page with page size 50', async () => {
    const wrapper = mount(ResourcesView)
    await new Promise((resolve) => setTimeout(resolve, 0))

    await wrapper.get('button[aria-label="下一页"]').trigger('click')
    await new Promise((resolve) => setTimeout(resolve, 0))

    expect(apiGet).toHaveBeenCalledWith('/api/resources?limit=50&offset=50')
    expect(wrapper.text()).toContain('第 2 页')
  })

  it('reloads resources from page one when page size changes', async () => {
    const wrapper = mount(ResourcesView)
    await new Promise((resolve) => setTimeout(resolve, 0))

    await wrapper.get('select[aria-label="每页条数"]').setValue('100')
    await new Promise((resolve) => setTimeout(resolve, 0))

    expect(apiGet).toHaveBeenCalledWith('/api/resources?limit=100')
    expect(wrapper.text()).toContain('第 1 页')
  })
})
