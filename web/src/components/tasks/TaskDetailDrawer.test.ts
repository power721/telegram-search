import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import TaskDetailDrawer from './TaskDetailDrawer.vue'

describe('TaskDetailDrawer', () => {
  it('renders task lifecycle timestamps with seconds', () => {
    const wrapper = mount(TaskDetailDrawer, {
      props: {
        show: true,
        task: {
          id: 7,
          type: 'history_sync',
          status: 'succeeded',
          progress: 100,
          total: 100,
          retry_count: 0,
          created_at: '2026-06-08T11:00:42Z',
          started_at: '2026-06-08T11:05:17Z',
          finished_at: '2026-06-08T11:30:59Z',
          payload_json: '{}'
        }
      },
      global: {
        stubs: {
          'n-drawer': { template: '<div><slot /></div>' },
          'n-drawer-content': { template: '<section><slot /></section>' },
          'n-descriptions': { template: '<dl><slot /></dl>' },
          'n-descriptions-item': { template: '<div><dt>{{ label }}</dt><dd><slot /></dd></div>', props: ['label'] }
        }
      }
    })

    expect(wrapper.text()).toContain('创建时间')
    expect(wrapper.text()).toContain('开始时间')
    expect(wrapper.text()).toContain('结束时间')
    expect(wrapper.text()).toContain(':42')
    expect(wrapper.text()).toContain(':17')
    expect(wrapper.text()).toContain(':59')
  })
})
