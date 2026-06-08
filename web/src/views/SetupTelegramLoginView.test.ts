import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import SetupTelegramLoginView from './SetupTelegramLoginView.vue'

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
  apiPost: vi.fn().mockResolvedValue({ status: 'LOGIN_REQUIRED' }),
  apiGet: vi.fn()
}))

describe('SetupTelegramLoginView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('renders telegram login fields', () => {
    const wrapper = mount(SetupTelegramLoginView, {
      global: {
        stubs: {
          'n-form': { template: '<form><slot /></form>' },
          'n-form-item': { props: ['label'], template: '<label>{{ label }}<slot /></label>' },
          'n-input': true,
          'n-button': { template: '<button><slot /></button>' }
        }
      }
    })
    expect(wrapper.text()).toContain('Telegram Login')
    expect(wrapper.text()).toContain('Phone')
    expect(wrapper.text()).toContain('Code')
  })
})
