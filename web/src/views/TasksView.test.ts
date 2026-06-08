import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import TasksView from './TasksView.vue'

vi.mock('@/api/client', () => ({
  apiGet: vi.fn().mockResolvedValue({
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

    expect(wrapper.text()).toContain('Tasks')
    expect(wrapper.text()).toContain('history_sync')
    expect(wrapper.text()).toContain('failed')
    expect(wrapper.text()).toContain('25 / 100')
    expect(wrapper.text()).toContain('temporary failure')
    expect(wrapper.text()).toContain('Retry')
    expect(wrapper.text()).toContain('Cancel')
    expect(wrapper.text()).toContain('Pause')
  })
})
