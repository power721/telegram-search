import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import SetupAPIKeyView from './SetupAPIKeyView.vue'

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() })
}))

vi.mock('naive-ui', async () => {
  const actual = await vi.importActual<typeof import('naive-ui')>('naive-ui')
  return {
    ...actual,
    useMessage: () => ({ error: vi.fn(), success: vi.fn() })
  }
})

vi.mock('@/api/client', () => ({
  apiGet: vi.fn().mockResolvedValue({ current_step: 'telegram_api' }),
  apiPost: vi.fn().mockResolvedValue({
    id: 1,
    name: 'default',
    prefix: '12345678',
    key: '12345678123456781234567812345678'
  })
}))

describe('SetupAPIKeyView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('shows the one-time api key after creation', async () => {
    const wrapper = mount(SetupAPIKeyView, {
      global: {
        stubs: {
          'n-form': { template: '<form><slot /></form>' },
          'n-form-item': { props: ['label'], template: '<label>{{ label }}<slot /></label>' },
          'n-input': true,
          'n-button': { emits: ['click'], template: `<button @click="$emit('click')"><slot /></button>` }
        }
      }
    })

    await wrapper.find('button').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('12345678123456781234567812345678')
  })
})
