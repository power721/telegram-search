import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiDelete, apiGet, apiPost } from '@/api/client'
import AccountsView from './AccountsView.vue'

const dialogWarning = vi.fn((options: { onPositiveClick?: () => void }) => {
  options.onPositiveClick?.()
})

vi.mock('naive-ui', async () => {
  const actual = await vi.importActual<typeof import('naive-ui')>('naive-ui')
  return {
    ...actual,
    useDialog: () => ({ warning: dialogWarning })
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
      }
    ]
  }),
  apiPost: vi.fn().mockResolvedValue({
    id: 1,
    phone: '+10000000000',
    status: 'LOGIN_REQUIRED',
    last_error: ''
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
    expect(wrapper.text()).toContain('登出')
    expect(wrapper.text()).toContain('删除')
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

    expect(dialogWarning).toHaveBeenCalled()
    expect(apiDelete).toHaveBeenCalledWith('/api/accounts/1')
    expect(apiGet).toHaveBeenCalledWith('/api/accounts')
  })
})

function mountAccountsView() {
  return mount(AccountsView, {
    global: {
      stubs: {
        NButton: {
          template: '<button v-bind="$attrs" @click="$emit(\'click\', $event)"><slot /></button>'
        },
        NTag: {
          template: '<span><slot /></span>'
        }
      }
    }
  })
}
