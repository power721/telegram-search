import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AccountsView from './AccountsView.vue'

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
  })
}))

describe('AccountsView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('renders account columns', () => {
    const wrapper = mount(AccountsView)
    expect(wrapper.text()).toContain('Accounts')
    expect(wrapper.text()).toContain('Phone')
    expect(wrapper.text()).toContain('Status')
    expect(wrapper.text()).toContain('Last Online')
  })
})
