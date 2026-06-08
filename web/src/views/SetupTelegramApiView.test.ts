import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiGet, apiPost } from '@/api/client'
import SetupTelegramApiView from './SetupTelegramApiView.vue'

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
  apiPost: vi.fn().mockResolvedValue({ configured: true, app_id: 123456, app_hash_set: true }),
  apiGet: vi.fn().mockResolvedValue({
    complete: false,
    admin_configured: true,
    api_key_configured: true,
    telegram_configured: true
  })
}))

describe('SetupTelegramApiView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    push.mockReset()
    vi.clearAllMocks()
  })

  it('renders telegram api setup fields', () => {
    const wrapper = mount(SetupTelegramApiView, {
      global: {
        stubs: {
          'n-form': { template: '<form><slot /></form>' },
          'n-form-item': { props: ['label'], template: '<label>{{ label }}<slot /></label>' },
          'n-input': true,
          'n-input-number': true,
          'n-button': { template: '<button v-bind="$attrs"><slot /></button>' }
        }
      }
    })
    expect(wrapper.text()).toContain('Telegram API')
    expect(wrapper.text()).toContain('App ID')
    expect(wrapper.text()).toContain('App Hash')
  })

  it('refreshes setup status before moving to telegram login', async () => {
    const wrapper = mount(SetupTelegramApiView, {
      global: {
        stubs: {
          'n-form': { template: '<form><slot /></form>' },
          'n-form-item': { props: ['label'], template: '<label>{{ label }}<slot /></label>' },
          'n-input': true,
          'n-input-number': true,
          'n-button': { template: '<button v-bind="$attrs"><slot /></button>' }
        }
      }
    })

    await wrapper.find('button').trigger('click')
    await flushPromises()

    expect(apiPost).toHaveBeenCalledWith('/api/setup/telegram-api', {
      app_id: 0,
      app_hash: ''
    })
    expect(apiGet).toHaveBeenCalledWith('/api/setup/status')
    expect(push).toHaveBeenCalledWith('/setup/telegram-login')
  })
})
