import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiGet, apiPost } from '@/api/client'
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

vi.mock('qrcode', () => ({
  default: { toCanvas: vi.fn(() => Promise.resolve()) }
}))

vi.mock('@/api/client', () => ({
  apiPost: vi.fn((path: string) => {
    if (path === '/api/telegram/login/qr/start') {
      return Promise.resolve({
        login_id: 'login-1',
        status: 'pending',
        qr_url: 'tg://login?token=one',
        expires_at: new Date(Date.now() + 60_000).toISOString()
      })
    }
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
    if (path === '/api/telegram/login/qr/login-1') {
      return Promise.resolve({
        login_id: 'login-1',
        status: 'online',
        account: { id: 1, phone: '+10000000000', status: 'ONLINE' },
        metadata_sync: { status: 'succeeded', channel_count: 0 }
      })
    }
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

  it('renders qr login by default and can switch to code login', async () => {
    const wrapper = mount(SetupTelegramLoginView, {
      global: {
        stubs: {
          'n-form': { template: '<form><slot /></form>' },
          'n-form-item': { props: ['label'], template: '<label>{{ label }}<slot /></label>' },
          'n-input': true,
          'n-button': { emits: ['click'], template: `<button @click="$emit('click')"><slot /></button>` },
          'n-button-group': { template: '<div><slot /></div>' }
        }
      }
    })
    expect(wrapper.text()).toContain('Telegram 登录')
    expect(wrapper.text()).toContain('扫码登录')
    expect(wrapper.text()).toContain('生成二维码')

    const codeButton = wrapper.findAll('button').find((button) => button.text() === '验证码登录')
    await codeButton?.trigger('click')

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
          'n-button': { emits: ['click'], template: `<button @click="$emit('click')"><slot /></button>` },
          'n-button-group': { template: '<div><slot /></div>' }
        }
      }
    })

    const codeModeButton = wrapper.findAll('button').find((button) => button.text() === '验证码登录')
    await codeModeButton?.trigger('click')
    await flushPromises()
    const sendCodeButton = wrapper.findAll('button').find((button) => button.text() === '发送验证码')
    await sendCodeButton?.trigger('click')
    await flushPromises()
    const signInButton = wrapper.findAll('button').find((button) => button.text() === '登录')
    await signInButton?.trigger('click')
    await flushPromises()

    expect(apiPost).toHaveBeenCalledWith('/api/telegram/login/sign-in', { phone: '', code: '' })
    expect(apiPost).not.toHaveBeenCalledWith('/api/setup/complete')
    expect(push).toHaveBeenCalledWith('/setup/listen-rules')
  })

  it('starts qr login and finishes after poll succeeds', async () => {
    const wrapper = mount(SetupTelegramLoginView, {
      global: {
        stubs: {
          'n-form': { template: '<form><slot /></form>' },
          'n-form-item': { props: ['label'], template: '<label>{{ label }}<slot /></label>' },
          'n-input': true,
          'n-button': { emits: ['click'], template: `<button @click="$emit('click')"><slot /></button>` },
          'n-button-group': { template: '<div><slot /></div>' }
        }
      }
    })

    const startButton = wrapper.findAll('button').find((button) => button.text() === '生成二维码')
    await startButton?.trigger('click')
    await flushPromises()

    expect(apiPost).toHaveBeenCalledWith('/api/telegram/login/qr/start', {})
    expect(apiGet).toHaveBeenCalledWith('/api/telegram/login/qr/login-1')
    expect(push).toHaveBeenCalledWith('/setup/listen-rules')
  })
})
