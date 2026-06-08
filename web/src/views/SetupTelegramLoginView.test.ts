import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiPost } from '@/api/client'
import SetupTelegramLoginView from './SetupTelegramLoginView.vue'

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
  apiPost: vi.fn((path: string) => {
    if (path === '/api/telegram/login/sign-in') {
      return Promise.resolve({
        status: 'ONLINE',
        account: { id: 1, phone: '+10000000000', status: 'ONLINE' },
        metadata_sync: { status: 'succeeded', channel_count: 3 }
      })
    }
    return Promise.resolve({ status: 'LOGIN_REQUIRED' })
  }),
  apiGet: vi.fn((path: string) => {
    if (path === '/api/accounts') return Promise.resolve({ items: [] })
    if (path === '/api/setup/status') {
      return Promise.resolve({
        complete: false,
        admin_configured: true,
        api_key_configured: false,
        api_key_step_complete: true,
        telegram_configured: true,
        telegram_login_complete: true,
        listen_rules_configured: false,
        current_step: 'listen_rules'
      })
    }
    return Promise.resolve({})
  })
}))

describe('SetupTelegramLoginView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
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
    expect(wrapper.text()).toContain('Telegram 登录')
    expect(wrapper.text()).toContain('手机号')
    expect(wrapper.text()).toContain('验证码')
  })

  it('continues to listen rules after login instead of completing setup', async () => {
    const wrapper = mount(SetupTelegramLoginView, {
      global: {
        stubs: {
          'n-form': { template: '<form><slot /></form>' },
          'n-form-item': { props: ['label'], template: '<label>{{ label }}<slot /></label>' },
          'n-input': true,
          'n-button': { emits: ['click'], template: `<button @click="$emit('click')"><slot /></button>` }
        }
      }
    })

    await wrapper.findAll('button')[0].trigger('click')
    await flushPromises()
    await wrapper.findAll('button')[1].trigger('click')
    await flushPromises()

    expect(apiPost).toHaveBeenCalledWith('/api/telegram/login/sign-in', { phone: '', code: '' })
    expect(apiPost).not.toHaveBeenCalledWith('/api/setup/complete')
    expect(push).toHaveBeenCalledWith('/setup/listen-rules')
  })
})
