import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiGet, apiPost } from '@/api/client'
import TasksView from './TasksView.vue'

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
      },
      {
        id: 3,
        type: 'gap_recovery',
        status: 'queued',
        progress: 0,
        total: 4,
        retry_count: 0
      }
    ]
  }),
  apiPost: vi.fn((path: string) => {
    if (path === '/api/tasks/bulk-delete') {
      return Promise.resolve({ deleted: 2, rejected_ids: [], missing_ids: [] })
    }
    return Promise.resolve({ id: 1, status: path.split('/').pop() })
  })
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
    expect(wrapper.text()).toContain('消息同步')
    expect(wrapper.text()).toContain('排队中')
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

  it('confirms before deleting selected tasks', async () => {
    const wrapper = mount(TasksView, {
      global: {
        stubs: {
          'n-button': {
            template: '<button :disabled="disabled" @click="$emit(\'click\', $event)"><slot /></button>',
            props: ['disabled'],
            emits: ['click']
          },
          'n-tag': { template: '<span><slot /></span>' },
          'n-drawer': true,
          'n-drawer-content': true,
          'n-descriptions': { template: '<div><slot /></div>' },
          'n-descriptions-item': { template: '<div><slot /></div>' }
        }
      }
    })
    await flushPromises()

    const checkboxes = wrapper.findAll('input[type="checkbox"]')
    await checkboxes[1].setValue(true)
    await checkboxes[3].setValue(true)
    await wrapper.findAll('button').find((button) => button.text() === '删除选中')!.trigger('click')
    await flushPromises()

    expect(dialogWarning).toHaveBeenCalledWith(
      expect.objectContaining({
        positiveText: '删除任务',
        positiveButtonProps: expect.objectContaining({ type: 'error' })
      })
    )
    expect(apiPost).toHaveBeenCalledWith('/api/tasks/bulk-delete', { ids: [1, 3] })
    expect(apiGet).toHaveBeenCalledWith('/api/tasks?limit=50')
  })
})
