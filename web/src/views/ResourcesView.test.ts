import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiGet, apiPost } from '@/api/client'
import ResourcesView from './ResourcesView.vue'

const dialogWarning = vi.fn((options: { onPositiveClick?: () => void }) => {
  options.onPositiveClick?.()
})

vi.mock('naive-ui', async () => {
  const actual = await vi.importActual<typeof import('naive-ui')>('naive-ui')
  return {
    ...actual,
    useDialog: () => ({ warning: dialogWarning })
  }
})

vi.mock('@/api/client', () => ({
  apiGet: vi.fn((path: string) => {
    if (path === '/api/channels') {
      return Promise.resolve({
        items: [
          { id: 7, title: 'Movies', username: 'movies' },
          { id: 8, title: 'Docs', username: '' }
        ]
      })
    }
    if (path.startsWith('/api/resources/grouped')) {
      return Promise.resolve({ grouped: { cloud_drive: 1, magnet: 2, ed2k: 3, http: 4, files: 5 } })
    }
    return Promise.resolve({
      items: [
        {
          id: 'link:1',
          kind: 'link',
          type: 'aliyun',
          category: 'cloud_drive',
          title: 'Course Pack',
          url: 'https://example.com/course'
        }
      ],
      total: 75,
      grouped: { cloud_drive: 1, magnet: 2, ed2k: 3, http: 4, files: 5 }
    })
  }),
  apiPost: vi.fn().mockResolvedValue({ deleted: 1, missing_ids: [] })
}))

describe('ResourcesView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    vi.mocked(apiPost).mockResolvedValue({ deleted: 1, missing_ids: [] })
  })

  it('renders resource filters and table', async () => {
    const wrapper = mountResourcesView()
    await new Promise((resolve) => setTimeout(resolve, 0))

    for (const label of ['全部', '网盘', '磁力', 'ED2K', 'HTTP', '文件']) {
      expect(wrapper.text()).toContain(label)
    }
    expect(wrapper.text()).toContain('Course Pack')
    expect(wrapper.text()).toContain('阿里云盘')
    expect(wrapper.text()).toContain('全部频道')
    expect(wrapper.text()).toContain('Movies (@movies)')
    expect(wrapper.text()).toContain('Docs')
    expect(wrapper.text()).not.toContain('Cloud Drive')
    expect(wrapper.text()).not.toContain('Files')

    const link = wrapper.get('a[href="https://example.com/course"]')
    expect(link.attributes('target')).toBe('_blank')
    expect(link.attributes('rel')).toContain('noopener')
  })

  it('loads the next resources page with page size 50', async () => {
    const wrapper = mountResourcesView()
    await new Promise((resolve) => setTimeout(resolve, 0))

    await wrapper.get('button[aria-label="下一页"]').trigger('click')
    await new Promise((resolve) => setTimeout(resolve, 0))

    expect(apiGet).toHaveBeenCalledWith('/api/resources?limit=50&offset=50')
    expect(wrapper.text()).toContain('第 2 / 2 页')
  })

  it('clears the resource type filter from the all button', async () => {
    const wrapper = mountResourcesView()
    await new Promise((resolve) => setTimeout(resolve, 0))

    await wrapper.findAll('.resource-types button').find((button) => button.text().includes('网盘'))!.trigger('click')
    await new Promise((resolve) => setTimeout(resolve, 0))
    await wrapper.findAll('.resource-types button').find((button) => button.text().includes('全部'))!.trigger('click')
    await new Promise((resolve) => setTimeout(resolve, 0))

    expect(apiGet).toHaveBeenCalledWith('/api/resources?category=cloud_drive&limit=50')
    expect(apiGet).toHaveBeenCalledWith('/api/resources?limit=50')
    expect(wrapper.findAll('.resource-types button').at(0)!.classes()).toContain('active')
  })

  it('reloads resources from page one when page size changes', async () => {
    const wrapper = mountResourcesView()
    await new Promise((resolve) => setTimeout(resolve, 0))

    await wrapper.get('select[aria-label="每页条数"]').setValue('100')
    await new Promise((resolve) => setTimeout(resolve, 0))

    expect(apiGet).toHaveBeenCalledWith('/api/resources?limit=100')
    expect(wrapper.text()).toContain('第 1 / 1 页')
  })

  it('jumps to a typed resources page', async () => {
    const wrapper = mountResourcesView()
    await new Promise((resolve) => setTimeout(resolve, 0))

    await wrapper.get('input[aria-label="跳转页码"]').setValue('2')
    await wrapper.get('form.pagination-jump').trigger('submit')
    await new Promise((resolve) => setTimeout(resolve, 0))

    expect(apiGet).toHaveBeenCalledWith('/api/resources?limit=50&offset=50')
  })

  it('refreshes the current resources page and clears selected resources', async () => {
    const wrapper = mountResourcesView()
    await new Promise((resolve) => setTimeout(resolve, 0))

    await wrapper.get('input[aria-label="跳转页码"]').setValue('2')
    await wrapper.get('form.pagination-jump').trigger('submit')
    await new Promise((resolve) => setTimeout(resolve, 0))
    await wrapper.get('input[aria-label="选择资源 Course Pack"]').setValue(true)

    expect((wrapper.get('input[aria-label="选择资源 Course Pack"]').element as HTMLInputElement).checked).toBe(true)

    await wrapper.get('button[aria-label="刷新资源"]').trigger('click')
    await new Promise((resolve) => setTimeout(resolve, 0))

    expect(apiGet).toHaveBeenCalledWith('/api/resources?limit=50&offset=50')
    expect((wrapper.get('input[aria-label="选择资源 Course Pack"]').element as HTMLInputElement).checked).toBe(false)
  })

  it('filters resources by channel from the channel dropdown', async () => {
    const wrapper = mountResourcesView()
    await new Promise((resolve) => setTimeout(resolve, 0))

    await wrapper.get('#resource-channel').setValue('7')
    await wrapper.find('.resource-filters').trigger('submit')
    await new Promise((resolve) => setTimeout(resolve, 0))

    expect(apiGet).toHaveBeenCalledWith('/api/channels')
    expect(apiGet).toHaveBeenCalledWith('/api/resources?channel_id=7&limit=50')
    expect(wrapper.text()).toContain('第 1 / 2 页')
  })

  it('deletes one resource after confirmation', async () => {
    const wrapper = mountResourcesView()
    await new Promise((resolve) => setTimeout(resolve, 0))

    await wrapper.findAll('button').find((button) => button.text() === '删除')!.trigger('click')
    await new Promise((resolve) => setTimeout(resolve, 0))

    expect(dialogWarning).toHaveBeenCalledWith(
      expect.objectContaining({
        title: '删除资源',
        positiveText: '删除资源'
      })
    )
    expect(apiPost).toHaveBeenCalledWith('/api/resources/bulk-delete', { ids: ['link:1'] })
  })

  it('deletes selected resources after confirmation', async () => {
    const wrapper = mountResourcesView()
    await new Promise((resolve) => setTimeout(resolve, 0))

    await wrapper.get('input[aria-label="选择资源 Course Pack"]').setValue(true)
    await wrapper.findAll('button').find((button) => button.text() === '删除选中')!.trigger('click')
    await new Promise((resolve) => setTimeout(resolve, 0))

    expect(apiPost).toHaveBeenCalledWith('/api/resources/bulk-delete', { ids: ['link:1'] })
  })
})

function mountResourcesView() {
  return mount(ResourcesView, {
    global: {
      stubs: {
        NInput: {
          props: ['value'],
          emits: ['update:value'],
          template:
            '<input v-bind="$attrs" :value="value" @input="$emit(\'update:value\', $event.target.value)" />'
        },
        NSelect: {
          props: ['value', 'options'],
          emits: ['update:value'],
          template:
            '<select v-bind="$attrs" :value="value" @change="$emit(\'update:value\', $event.target.value === \'\' ? \'\' : Number($event.target.value) || $event.target.value)"><option v-for="option in options" :key="String(option.value)" :value="option.value">{{ option.label }}</option></select>'
        },
        NButton: {
          emits: ['click'],
          template: '<button v-bind="$attrs" @click="$emit(\'click\', $event)"><slot /></button>'
        }
      }
    }
  })
}
