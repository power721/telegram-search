import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ResourcesView from './ResourcesView.vue'

vi.mock('@/api/client', () => ({
  apiGet: vi.fn((path: string) => {
    if (path.startsWith('/api/resources/grouped')) {
      return Promise.resolve({ grouped: { cloud_drive: 1, magnet: 2, ed2k: 3, http: 4, files: 5 } })
    }
    return Promise.resolve({
      items: [{ id: 'link:1', kind: 'link', category: 'cloud_drive', title: 'Course Pack' }],
      total: 1,
      grouped: { cloud_drive: 1, magnet: 2, ed2k: 3, http: 4, files: 5 }
    })
  })
}))

describe('ResourcesView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('renders resource filters and table', async () => {
    const wrapper = mount(ResourcesView)
    await new Promise((resolve) => setTimeout(resolve, 0))

    for (const label of ['Cloud Drive', 'Magnet', 'ED2K', 'HTTP', 'Files']) {
      expect(wrapper.text()).toContain(label)
    }
    expect(wrapper.text()).toContain('Course Pack')
  })
})
