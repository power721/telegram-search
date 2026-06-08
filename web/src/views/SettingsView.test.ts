import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiGet, apiPost, setAPIKey } from '@/api/client'
import SettingsView from './SettingsView.vue'

vi.mock('naive-ui', async () => {
  const actual = await vi.importActual<typeof import('naive-ui')>('naive-ui')
  return {
    ...actual,
    useMessage: () => ({ error: vi.fn(), success: vi.fn() })
  }
})

vi.mock('@/api/client', () => ({
  apiGet: vi.fn().mockResolvedValue({
    id: 1,
    name: 'default',
    prefix: '12345678',
    key: '12345678123456781234567812345678',
    created_at: '2026-06-08T00:00:00Z'
  }),
  apiPost: vi.fn().mockResolvedValue({
    id: 2,
    name: 'default',
    prefix: '87654321',
    key: '87654321876543218765432187654321',
    created_at: '2026-06-08T01:00:00Z'
  }),
  setAPIKey: vi.fn()
}))

describe('SettingsView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('loads and masks the full api key without rendering the prefix field', async () => {
    const wrapper = mount(SettingsView, {
      global: {
        stubs: {
          'n-button': {
            emits: ['click'],
            template: `<button :data-testid="$attrs['data-testid']" @click="$emit('click')"><slot /></button>`
          }
        }
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
        stubs: {
          'n-button': {
            emits: ['click'],
            template: `<button :data-testid="$attrs['data-testid']" @click="$emit('click')"><slot /></button>`
          }
        }
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
})
