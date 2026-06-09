import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import SetupAdminView from './SetupAdminView.vue'

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

describe('SetupAdminView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('renders Chinese placeholders for admin account inputs', () => {
    const wrapper = mount(SetupAdminView, {
      global: {
        stubs: {
          'n-form': { template: '<form><slot /></form>' },
          'n-form-item': { props: ['label'], template: '<label>{{ label }}<slot /></label>' },
          'n-input': {
            props: ['value', 'type', 'autocomplete', 'placeholder'],
            template: '<input :type="type || \'text\'" :value="value" :autocomplete="autocomplete" :placeholder="placeholder" />'
          },
          'n-button': { template: '<button><slot /></button>' }
        }
      }
    })

    expect(wrapper.get<HTMLInputElement>('input[autocomplete="username"]').element.placeholder).toBe('请输入用户名')
    expect(wrapper.get<HTMLInputElement>('input[autocomplete="new-password"]').element.placeholder).toBe('请输入密码')
  })
})
