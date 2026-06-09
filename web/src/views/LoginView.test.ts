import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import LoginView from './LoginView.vue'

const push = vi.fn()

vi.mock('vue-router', () => ({
  useRouter: () => ({ push })
}))

vi.mock('naive-ui', async () => {
  const actual = await vi.importActual<typeof import('naive-ui')>('naive-ui')
  return {
    ...actual,
    useMessage: () => ({ error: vi.fn(), success: vi.fn() })
  }
})

vi.mock('@/api/client', () => ({
  apiPost: vi.fn().mockResolvedValue({ id: 1, username: 'admin', role: 'admin' }),
  apiGet: vi.fn()
}))

describe('LoginView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    push.mockReset()
  })

  it('renders the login heading', () => {
    const wrapper = mount(LoginView)
    expect(wrapper.text()).toContain('登录 TG Search')
  })
})
