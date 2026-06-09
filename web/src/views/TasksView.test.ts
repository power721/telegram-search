import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiGet } from '@/api/client'
import TasksView from './TasksView.vue'

vi.mock('@/api/client', () => ({
  apiGet: vi.fn().mockResolvedValue({
    total: 75,
    items: [
      {
        id: 1,
        type: 'history_sync',
        status: 'failed',
        progress: 25,
        total: 100,
        error_message: 'temporary failure',
        retry_count: 2,
        next_run_at: '2026-06-08T13:00:00Z'
      },
      {
        id: 2,
        type: 'web_access_detection',
        status: 'running',
        progress: 4,
        total: 10,
        message: 'checking'
      }
    ]
  }),
  apiPost: vi.fn()
}))

describe('TasksView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('renders task status, progress, errors and actions', async () => {
    const wrapper = mount(TasksView, {
      global: {
        stubs: {
          'n-button': { template: '<button><slot /></button>' },
          'n-tag': { template: '<span><slot /></span>' },
          'n-drawer': true,
          'n-drawer-content': true,
          'n-descriptions': { template: '<div><slot /></div>' },
          'n-descriptions-item': { template: '<div><slot /></div>' }
        }
      }
    })
    await flushPromises()

    expect(wrapper.text()).toContain('任务')
    expect(wrapper.text()).toContain('历史同步')
    expect(wrapper.text()).toContain('失败')
    expect(wrapper.text()).toContain('25 / 100')
    expect(wrapper.text()).toContain('temporary failure')
    expect(wrapper.text()).toContain('重试')
    expect(wrapper.text()).toContain('取消')
    expect(wrapper.text()).toContain('暂停')
  })

  it('loads the next tasks page with page size 50', async () => {
    const wrapper = mount(TasksView, {
      global: {
        stubs: {
          'n-button': { template: '<button :disabled="disabled"><slot /></button>', props: ['disabled'] },
          'n-tag': { template: '<span><slot /></span>' },
          'n-drawer': true,
          'n-drawer-content': true,
          'n-descriptions': { template: '<div><slot /></div>' },
          'n-descriptions-item': { template: '<div><slot /></div>' }
        }
      }
    })
    await flushPromises()

    await wrapper.get('button[aria-label="下一页"]').trigger('click')
    await flushPromises()

    expect(apiGet).toHaveBeenCalledWith('/api/tasks?limit=50&offset=50')
    expect(wrapper.text()).toContain('第 2 / 2 页')
  })

  it('reloads tasks from page one when page size changes', async () => {
    const wrapper = mount(TasksView, {
      global: {
        stubs: {
          'n-button': { template: '<button :disabled="disabled"><slot /></button>', props: ['disabled'] },
          'n-tag': { template: '<span><slot /></span>' },
          'n-drawer': true,
          'n-drawer-content': true,
          'n-descriptions': { template: '<div><slot /></div>' },
          'n-descriptions-item': { template: '<div><slot /></div>' }
        }
      }
    })
    await flushPromises()

    await wrapper.get('select[aria-label="每页条数"]').setValue('20')
    await flushPromises()

    expect(apiGet).toHaveBeenCalledWith('/api/tasks?limit=20')
    expect(wrapper.text()).toContain('第 1 / 4 页')
  })

  it('jumps to a typed tasks page', async () => {
    const wrapper = mount(TasksView, {
      global: {
        stubs: {
          'n-button': { template: '<button :disabled="disabled"><slot /></button>', props: ['disabled'] },
          'n-tag': { template: '<span><slot /></span>' },
          'n-drawer': true,
          'n-drawer-content': true,
          'n-descriptions': { template: '<div><slot /></div>' },
          'n-descriptions-item': { template: '<div><slot /></div>' }
        }
      }
    })
    await flushPromises()

    await wrapper.get('input[aria-label="跳转页码"]').setValue('2')
    await wrapper.get('form.pagination-jump').trigger('submit')
    await flushPromises()

    expect(apiGet).toHaveBeenCalledWith('/api/tasks?limit=50&offset=50')
  })
})
