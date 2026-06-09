import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiGet, apiPost, apiPut, setAPIKey } from '@/api/client'
import SettingsView from './SettingsView.vue'

vi.mock('naive-ui', async () => {
  const actual = await vi.importActual<typeof import('naive-ui')>('naive-ui')
  return {
    ...actual,
    useMessage: () => ({ error: vi.fn(), success: vi.fn() })
  }
})

vi.mock('@/api/client', () => ({
  apiGet: vi.fn((path: string) => {
    if (path === '/api/auth/me') {
      return Promise.resolve({ id: 1, username: 'admin', role: 'admin' })
    }
    return Promise.resolve({
      id: 1,
      name: 'default',
      prefix: '12345678',
      key: '12345678123456781234567812345678',
      created_at: '2026-06-08T00:00:00Z'
    })
  }),
  apiPost: vi.fn().mockResolvedValue({
    id: 2,
    name: 'default',
    prefix: '87654321',
    key: '87654321876543218765432187654321',
    created_at: '2026-06-08T01:00:00Z'
  }),
  apiPut: vi.fn().mockResolvedValue({ id: 1, username: 'root', role: 'admin' }),
  setAPIKey: vi.fn()
}))

const stubs = {
  'n-form': { template: '<form><slot /></form>' },
  'n-form-item': { props: ['label'], template: '<label>{{ label }}<slot /></label>' },
  'n-input': {
    props: ['value', 'type', 'autocomplete', 'placeholder'],
    emits: ['update:value'],
    template: `
      <input
        :data-testid="$attrs['data-testid']"
        :type="type || 'text'"
        :value="value"
        :autocomplete="autocomplete"
        :placeholder="placeholder"
        @input="$emit('update:value', $event.target.value)"
      />
    `
  },
  'n-button': {
    emits: ['click'],
    template: `<button :data-testid="$attrs['data-testid']" @click="$emit('click')"><slot /></button>`
  }
}

describe('SettingsView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('loads and masks the full api key without rendering the prefix field', async () => {
    const wrapper = mount(SettingsView, {
      global: {
        stubs
      }
    })
    await flushPromises()

    expect(apiGet).toHaveBeenCalledWith('/api/settings/api-key')
    expect(setAPIKey).toHaveBeenCalledWith('12345678123456781234567812345678')
    expect(wrapper.text()).not.toContain('前缀')
    expect(wrapper.text()).not.toContain('12345678123456781234567812345678')

    const input = wrapper.get<HTMLInputElement>('[data-testid="api-key-input"]')
    expect(input.element.type).toBe('password')
    expect(input.element.value).toBe('12345678123456781234567812345678')

    await wrapper.get('[data-testid="toggle-api-key-visibility"]').trigger('click')
    expect(input.element.type).toBe('text')
  })

  it('regenerates and keeps the replacement key masked', async () => {
    const wrapper = mount(SettingsView, {
      global: {
        stubs
      }
    })
    await flushPromises()
    await wrapper.get('[data-testid="regenerate-api-key"]').trigger('click')
    await flushPromises()

    expect(apiPost).toHaveBeenCalledWith('/api/settings/api-key/regenerate')
    expect(setAPIKey).toHaveBeenCalledWith('87654321876543218765432187654321')
    expect(wrapper.text()).not.toContain('87654321876543218765432187654321')

    const input = wrapper.get<HTMLInputElement>('[data-testid="api-key-input"]')
    expect(input.element.type).toBe('password')
    expect(input.element.value).toBe('87654321876543218765432187654321')
  })

  it('updates admin credentials from the settings page', async () => {
    const wrapper = mount(SettingsView, {
      global: {
        stubs
      }
    })
    await flushPromises()

    await wrapper.get('[data-testid="admin-username-input"]').setValue('root')
    await wrapper.get('[data-testid="current-password-input"]').setValue('secret123')
    await wrapper.get('[data-testid="new-password-input"]').setValue('newsecret123')
    await wrapper.get('[data-testid="confirm-password-input"]').setValue('newsecret123')
    await wrapper.get('[data-testid="save-admin-credentials"]').trigger('click')
    await flushPromises()

    expect(apiPut).toHaveBeenCalledWith('/api/settings/admin', {
      username: 'root',
      current_password: 'secret123',
      new_password: 'newsecret123'
    })
    expect(wrapper.get<HTMLInputElement>('[data-testid="current-password-input"]').element.value).toBe('')
    expect(wrapper.get<HTMLInputElement>('[data-testid="new-password-input"]').element.value).toBe('')
  })
})
