import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiGet } from '@/api/client'
import LogsView from './LogsView.vue'

vi.mock('@/api/client', () => ({
  apiGet: vi.fn(() =>
    Promise.resolve({
      items: [
        {
          file: 'app.log',
          time: '2026-06-09T02:00:00Z',
          level: 'info',
          message: 'boot complete',
          caller: 'cmd/main.go:1',
          fields: { address: '127.0.0.1:9900' },
          raw: '{"msg":"boot complete"}'
        }
      ],
      files: [{ name: 'app.log', size: 120 }],
      total: 1,
      limit: 200,
      offset: 0,
      order: 'desc'
    })
  ),
  apiDownload: vi.fn()
}))

describe('LogsView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('renders logs and submits filter queries', async () => {
    const wrapper = mount(LogsView, {
      global: {
        stubs: {
          'n-button': {
            props: ['disabled'],
            emits: ['click'],
            template: '<button :disabled="disabled" @click="$emit(\'click\')"><slot /></button>'
          },
          'n-input': {
            props: ['value'],
            emits: ['update:value'],
            template: '<input :value="value" @input="$emit(\'update:value\', $event.target.value)" />'
          },
          'n-select': {
            props: ['value', 'options'],
            emits: ['update:value'],
            template:
              '<select :value="value" @change="$emit(\'update:value\', $event.target.value)"><option v-for="option in options" :key="option.value" :value="option.value">{{ option.label }}</option></select>'
          }
        }
      }
    })
    await flushPromises()

    expect(wrapper.text()).toContain('日志')
    expect(wrapper.text()).toContain('boot complete')
    expect(wrapper.text()).toContain('cmd/main.go:1')
    expect(wrapper.text()).toContain('127.0.0.1:9900')
    expect(wrapper.find('button:disabled').text()).toContain('下载日志')

    const selects = wrapper.findAll('select')
    await selects[0].setValue('app.log')
    await selects[1].setValue('info')
    await selects[2].setValue('asc')
    await wrapper.find('input').setValue('boot')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(apiGet).toHaveBeenCalledWith('/api/logs?file=app.log&level=info&q=boot&order=asc&limit=200')
  })
})
