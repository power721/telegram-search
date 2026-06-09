import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import TaskTable from './TaskTable.vue'

describe('TaskTable', () => {
  it('renders task timestamps with seconds', () => {
    const wrapper = mount(TaskTable, {
      props: {
        tasks: [
          {
            id: 1,
            type: 'history_sync',
            status: 'flood_wait',
            progress: 4,
            total: 10,
            retry_count: 1,
            created_at: '2026-06-08T11:00:42Z',
            next_run_at: '2026-06-08T11:30:17Z'
          }
        ]
      },
      global: {
        stubs: {
          'n-button': { template: '<button><slot /></button>' }
        }
      }
    })

    expect(wrapper.text()).toContain(':42')
    expect(wrapper.text()).toContain(':17')
  })
})
