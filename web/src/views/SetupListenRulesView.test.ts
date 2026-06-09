import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiPost } from '@/api/client'
import SetupListenRulesView from './SetupListenRulesView.vue'

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
  apiGet: vi.fn().mockResolvedValue({}),
  apiPost: vi.fn().mockResolvedValue({
    complete: false,
    listen_rules_configured: true,
    current_step: 'channel_selection'
  })
}))

describe('SetupListenRulesView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('saves default listen rules and moves to channel selection', async () => {
    const wrapper = mount(SetupListenRulesView, {
      global: {
        stubs: {
          'n-form': { template: '<form><slot /></form>' },
          'n-form-item': { props: ['label'], template: '<label>{{ label }}<slot /></label>' },
          'n-input': true,
          'n-checkbox-group': { template: '<div><slot /></div>' },
          'n-checkbox': { props: ['value'], template: '<label><slot /></label>' },
          'n-button': { emits: ['click'], template: `<button @click="$emit('click')"><slot /></button>` }
        }
      }
    })

    await wrapper.find('button').trigger('click')
    await flushPromises()

    expect(apiPost).toHaveBeenCalledWith('/api/setup/listen-rules', {
      includes: [],
      excludes: [],
      message_types: ['link', 'text', 'image', 'video', 'audio'],
      link_types: ['cloud_drive', 'magnet', 'ed2k', 'other'],
      ignored_link_patterns: ['t.me', 'toapp.mypikpak.com', 'telegra.ph', 'www.themoviedb.org']
    })
    expect(push).toHaveBeenCalledWith('/setup/channels')
  })
})
