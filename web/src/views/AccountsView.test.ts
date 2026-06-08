import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiDelete, apiGet, apiPost } from '@/api/client'
import AccountsView from './AccountsView.vue'

const push = vi.fn()
const dialogWarning = vi.fn((options: { onPositiveClick?: () => void }) => {
  options.onPositiveClick?.()
})
const messageSuccess = vi.fn()
const messageError = vi.fn()

vi.mock('vue-router', () => ({
  useRouter: () => ({ push })
}))

vi.mock('naive-ui', async () => {
  const actual = await vi.importActual<typeof import('naive-ui')>('naive-ui')
  return {
    ...actual,
    useDialog: () => ({ warning: dialogWarning }),
    useMessage: () => ({ success: messageSuccess, error: messageError })
  }
})

vi.mock('@/api/client', () => ({
  apiGet: vi.fn().mockResolvedValue({
    items: [
      {
        id: 1,
        phone: '+10000000000',
        telegram_user_id: 42,
        first_name: 'Ada',
        last_name: 'Lovelace',
        username: 'ada',
        status: 'ONLINE',
        last_online_at: '2026-06-08T02:00:00Z',
        last_error: ''
      },
      {
        id: 2,
        phone: '+10000000001',
        telegram_user_id: 43,
        first_name: 'Grace',
        last_name: 'Hopper',
        username: 'grace',
        status: 'LOGIN_REQUIRED',
        last_online_at: '',
        last_error: ''
      }
    ]
  }),
  apiPost: vi.fn((path: string) => {
    if (path === '/api/telegram/login/sign-in') {
      return Promise.resolve({
        status: 'ONLINE',
        account: { id: 2, phone: '+10000000001', status: 'ONLINE', last_error: '' },
        metadata_sync: { status: 'succeeded', channel_count: 3 }
      })
    }
    return Promise.resolve({
      id: 1,
      phone: '+10000000000',
      status: 'LOGIN_REQUIRED',
      last_error: ''
    })
  }),
  apiDelete: vi.fn().mockResolvedValue({ deleted: true })
}))

describe('AccountsView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('renders account columns', async () => {
    const wrapper = mountAccountsView()
    await flushPromises()

    expect(wrapper.text()).toContain('账号')
    expect(wrapper.text()).toContain('手机号')
    expect(wrapper.text()).toContain('状态')
    expect(wrapper.text()).toContain('最后在线')
    expect(wrapper.text()).toContain('操作')
    expect(wrapper.text()).toContain('添加账号')
    expect(wrapper.text()).toContain('登出')
    expect(wrapper.text()).toContain('登录')
    expect(wrapper.text()).toContain('删除')
  })

  it('opens telegram login dialog when adding or reconnecting an account', async () => {
    const wrapper = mountAccountsView()
    await flushPromises()

    const addButton = wrapper.findAll('button').find((button) => button.text() === '添加账号')
    expect(addButton).toBeTruthy()
    await addButton!.trigger('click')

    expect(wrapper.text()).toContain('Telegram 登录')
    expect((wrapper.find('input[autocomplete="tel"]').element as HTMLInputElement).value).toBe('')

    const loginButton = wrapper.findAll('button').find((button) => button.text() === '登录')
    expect(loginButton).toBeTruthy()
    await loginButton!.trigger('click')

    expect(push).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('Telegram 登录')
    expect((wrapper.find('input[autocomplete="tel"]').element as HTMLInputElement).value).toBe('+10000000001')
  })

  it('closes the telegram login dialog from close and cancel controls', async () => {
    const wrapper = mountAccountsView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text() === '添加账号')!.trigger('click')
    expect(wrapper.text()).toContain('Telegram 登录')
    expect(wrapper.find('[aria-label="关闭 Telegram 登录"]').exists()).toBe(true)

    await wrapper.findAll('button').find((button) => button.text() === '取消')!.trigger('click')
    expect(wrapper.text()).not.toContain('Telegram 登录')
  })

  it('logs in from the account dialog without navigating to setup', async () => {
    const wrapper = mountAccountsView()
    await flushPromises()

    const loginButton = wrapper.findAll('button').find((button) => button.text() === '登录')
    expect(loginButton).toBeTruthy()
    await loginButton!.trigger('click')

    await wrapper.findAll('button').find((button) => button.text() === '发送验证码')!.trigger('click')
    await flushPromises()
    const dialogLoginButtons = wrapper.findAll('button').filter((button) => button.text() === '登录')
    await dialogLoginButtons.at(-1)!.trigger('click')
    await flushPromises()

    expect(apiPost).toHaveBeenCalledWith('/api/telegram/login/send-code', { phone: '+10000000001' })
    expect(apiPost).toHaveBeenCalledWith('/api/telegram/login/sign-in', {
      phone: '+10000000001',
      code: ''
    })
    expect(push).not.toHaveBeenCalled()
    expect(apiGet).toHaveBeenCalledWith('/api/accounts')
    expect(messageSuccess).toHaveBeenCalledWith('Telegram 账号已连接')
  })

  it('uses the phone and code typed in the account login dialog', async () => {
    const wrapper = mountAccountsView()
    await flushPromises()

    const loginButton = wrapper.findAll('button').find((button) => button.text() === '登录')
    expect(loginButton).toBeTruthy()
    await loginButton!.trigger('click')

    const phoneInput = wrapper.find('input[autocomplete="tel"]')
    await phoneInput.setValue('+19999999999')
    await wrapper.findAll('button').find((button) => button.text() === '发送验证码')!.trigger('click')
    await flushPromises()

    await phoneInput.setValue('+18888888888')
    await wrapper.find('input[autocomplete="one-time-code"]').setValue('12345')
    const dialogLoginButtons = wrapper.findAll('button').filter((button) => button.text() === '登录')
    await dialogLoginButtons.at(-1)!.trigger('click')
    await flushPromises()

    expect(apiPost).toHaveBeenCalledWith('/api/telegram/login/send-code', { phone: '+19999999999' })
    expect(apiPost).toHaveBeenCalledWith('/api/telegram/login/sign-in', {
      phone: '+18888888888',
      code: '12345'
    })
  })

  it('logs out an account from the action column', async () => {
    const wrapper = mountAccountsView()
    await flushPromises()

    const logoutButton = wrapper.findAll('button').find((button) => button.text() === '登出')
    expect(logoutButton).toBeTruthy()
    await logoutButton!.trigger('click')
    await flushPromises()

    expect(apiPost).toHaveBeenCalledWith('/api/accounts/1/logout')
    expect(apiGet).toHaveBeenCalledWith('/api/accounts')
  })

  it('confirms before deleting an account', async () => {
    const wrapper = mountAccountsView()
    await flushPromises()

    const deleteButton = wrapper.findAll('button').find((button) => button.text() === '删除')
    expect(deleteButton).toBeTruthy()
    await deleteButton!.trigger('click')
    await flushPromises()

    expect(dialogWarning).toHaveBeenCalledWith(
      expect.objectContaining({
        positiveText: '删除账号',
        positiveButtonProps: expect.objectContaining({ type: 'error' })
      })
    )
    expect(apiDelete).toHaveBeenCalledWith('/api/accounts/1')
    expect(apiGet).toHaveBeenCalledWith('/api/accounts')
  })
})

function mountAccountsView() {
  return mount(AccountsView, {
    global: {
      stubs: {
        NButton: {
          emits: ['click'],
          template: '<button v-bind="$attrs" @click="$emit(\'click\', $event)"><slot /></button>'
        },
        NForm: {
          template: '<form><slot /></form>'
        },
        NFormItem: {
          props: ['label'],
          template: '<label>{{ label }}<slot /></label>'
        },
        NInput: {
          props: ['value', 'autocomplete'],
          emits: ['update:value'],
          template:
            '<input :value="value" :autocomplete="autocomplete" @input="$emit(\'update:value\', $event.target.value)" />'
        },
        NModal: {
          props: ['show'],
          template: '<div v-if="show"><slot /></div>'
        },
        NCard: {
          template: '<section><slot /></section>'
        },
        NTag: {
          template: '<span><slot /></span>'
        }
      }
    }
  })
}
