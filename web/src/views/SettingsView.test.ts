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
    if (path === '/api/storage/usage') {
      return Promise.resolve({
        db_bytes: 100,
        index_bytes: 200,
        media_cache_bytes: 300,
        total_bytes: 600,
        max_db_bytes: 15_000_000_000,
        max_media_bytes: 25_000_000_000,
        db_over_quota: false,
        media_over_quota: false
      })
    }
    if (path === '/api/settings/telegram-api') {
      return Promise.resolve({ configured: true, app_id: 123456, app_hash_set: true })
    }
    if (path === '/api/settings/runtime') {
      return Promise.resolve({
        sync: {
          workers: 5,
          history_batch_size: 100,
          telegram_request_interval: '2s'
        },
        storage: {
          max_db_size: 15_000_000_000,
          max_media_cache: 25_000_000_000
        },
        telegram: {
          proxy: '',
          reconnect_timeout: '5m0s',
          dial_timeout: '10s',
          rate_limit: {
            enabled: true,
            rate_per_second: 10,
            burst: 5
          },
          stream: {
            concurrency: 2,
            buffers: 4,
            chunk_timeout: '20s'
          },
          media: {
            concurrency: 2
          }
        }
      })
    }
    if (path === '/api/settings/version') {
      return Promise.resolve({
        current_version: 'v1.2.3',
        update_available: false
      })
    }
    if (path === '/api/settings/system-info') {
      return Promise.resolve({
        name: 'Linux',
        version: '6.8.0-124-generic',
        architecture: 'amd64',
        go_version: 'go1.25.0',
        cpu_count: 8,
        hostname: 'tg-search-host'
      })
    }
    if (path === '/api/settings/version?check_update=true') {
      return Promise.resolve({
        current_version: 'v1.2.3',
        latest_version: 'v1.2.4',
        latest_url: 'https://github.com/power721/tg-search/releases/tag/v1.2.4',
        update_available: true
      })
    }
    return Promise.resolve({
      id: 1,
      name: 'default',
      prefix: '12345678',
      key: '12345678123456781234567812345678',
      usage_count: 7,
      created_at: '2026-06-08T00:00:00Z'
    })
  }),
  apiPost: vi.fn().mockResolvedValue({
    id: 2,
    name: 'default',
    prefix: '87654321',
    key: '87654321876543218765432187654321',
    usage_count: 0,
    created_at: '2026-06-08T01:00:00Z'
  }),
  apiPut: vi.fn((path: string) => {
    if (path === '/api/settings/telegram-api') {
      return Promise.resolve({ configured: true, app_id: 654321, app_hash_set: true })
    }
    if (path === '/api/settings/runtime') {
      return Promise.resolve({
        sync: {
          workers: 8,
          history_batch_size: 250,
          telegram_request_interval: '1500ms'
        },
        storage: {
          max_db_size: 30_000_000_000,
          max_media_cache: 40_000_000_000
        },
        telegram: {
          proxy: 'socks5://127.0.0.1:1080',
          reconnect_timeout: '6m0s',
          dial_timeout: '15s',
          rate_limit: {
            enabled: false,
            rate_per_second: 12,
            burst: 6
          },
          stream: {
            concurrency: 4,
            buffers: 8,
            chunk_timeout: '30s'
          },
          media: {
            concurrency: 3
          }
        }
      })
    }
    return Promise.resolve({ id: 1, username: 'root', role: 'admin' })
  }),
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
  },
  'n-tabs': {
    props: ['value'],
    emits: ['update:value'],
    template: `
      <div class="n-tabs" :data-active-tab="value">
        <slot />
      </div>
    `
  },
  'n-tab-pane': {
    props: ['name', 'tab'],
    template: `<section class="n-tab-pane" :data-tab-name="name"><h2>{{ tab }}</h2><slot /></section>`
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
    expect(wrapper.get('[data-testid="api-key-usage-count"]').text()).toBe('7')

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

  it('renders Chinese placeholders for admin credential inputs', async () => {
    const wrapper = mount(SettingsView, {
      global: {
        stubs
      }
    })
    await flushPromises()

    expect(wrapper.get<HTMLInputElement>('[data-testid="admin-username-input"]').element.placeholder).toBe('请输入用户名')
    expect(wrapper.get<HTMLInputElement>('[data-testid="current-password-input"]').element.placeholder).toBe('请输入密码')
  })

  it('renders storage limits from the storage usage API', async () => {
    const wrapper = mount(SettingsView, {
      global: {
        stubs
      }
    })
    await flushPromises()

    expect(apiGet).toHaveBeenCalledWith('/api/storage/usage')
    expect(wrapper.text()).toContain('15.0 GB')
    expect(wrapper.text()).toContain('25.0 GB')
  })

  it('shows current version and checks GitHub release updates', async () => {
    const wrapper = mount(SettingsView, {
      global: {
        stubs
      }
    })
    await flushPromises()

    expect(apiGet).toHaveBeenCalledWith('/api/settings/version')
    expect(wrapper.get('[data-testid="current-version"]').text()).toBe('v1.2.3')
    expect(wrapper.text()).toContain('尚未检查')

    await wrapper.get('[data-testid="check-version"]').trigger('click')
    await flushPromises()

    expect(apiGet).toHaveBeenCalledWith('/api/settings/version?check_update=true')
    expect(wrapper.text()).toContain('发现新版本 v1.2.4')
  })

  it('renders system information from the settings API', async () => {
    const wrapper = mount(SettingsView, {
      global: {
        stubs
      }
    })
    await flushPromises()

    expect(apiGet).toHaveBeenCalledWith('/api/settings/system-info')
    expect(wrapper.get('[data-testid="system-name"]').text()).toBe('Linux')
    expect(wrapper.text()).toContain('6.8.0-124-generic')
    expect(wrapper.text()).toContain('amd64')
    expect(wrapper.text()).toContain('tg-search-host')
    expect(wrapper.text()).toContain('8')
    expect(wrapper.text()).not.toContain('Go 版本')
    expect(wrapper.text()).not.toContain('go1.25.0')
  })

  it('groups settings into four tabs', async () => {
    const wrapper = mount(SettingsView, {
      global: {
        stubs
      }
    })
    await flushPromises()

    const panes = wrapper.findAll('.n-tab-pane')
    expect(panes.map((pane) => pane.attributes('data-tab-name'))).toEqual(['security', 'storage', 'runtime', 'system'])
    expect(panes.map((pane) => pane.find('h2').text())).toEqual(['账号与安全', '存储', '运行参数', '系统'])
  })

  it('keeps API key and Telegram API panels in the right security column', async () => {
    const wrapper = mount(SettingsView, {
      global: {
        stubs
      }
    })
    await flushPromises()

    const leftPanels = wrapper.findAll('.security-column-left > .panel')
    const rightPanels = wrapper.findAll('.security-column-right > .panel')

    expect(leftPanels.map((panel) => panel.find('h2').text())).toEqual(['管理员账号'])
    expect(rightPanels.map((panel) => panel.find('h2').text())).toEqual(['API 密钥', 'Telegram API'])
  })

  it('loads and saves runtime settings from the runtime tab', async () => {
    const wrapper = mount(SettingsView, {
      global: {
        stubs
      }
    })
    await flushPromises()

    expect(apiGet).toHaveBeenCalledWith('/api/settings/runtime')
    expect(wrapper.get<HTMLInputElement>('[data-testid="runtime-workers-input"]').element.value).toBe('5')
    expect(wrapper.get<HTMLInputElement>('[data-testid="runtime-max-db-size-input"]').element.value).toBe('15000000000')
    expect(wrapper.get<HTMLInputElement>('[data-testid="runtime-rate-enabled-input"]').element.checked).toBe(true)

    await wrapper.get('[data-testid="runtime-workers-input"]').setValue('8')
    await wrapper.get('[data-testid="runtime-history-batch-size-input"]').setValue('250')
    await wrapper.get('[data-testid="runtime-request-interval-input"]').setValue('1500ms')
    await wrapper.get('[data-testid="runtime-max-db-size-input"]').setValue('30000000000')
    await wrapper.get('[data-testid="runtime-max-media-cache-input"]').setValue('40000000000')
    await wrapper.get('[data-testid="runtime-proxy-input"]').setValue('socks5://127.0.0.1:1080')
    await wrapper.get('[data-testid="runtime-reconnect-timeout-input"]').setValue('6m')
    await wrapper.get('[data-testid="runtime-dial-timeout-input"]').setValue('15s')
    await wrapper.get('[data-testid="runtime-rate-enabled-input"]').setValue(false)
    await wrapper.get('[data-testid="runtime-rate-per-second-input"]').setValue('12')
    await wrapper.get('[data-testid="runtime-rate-burst-input"]').setValue('6')
    await wrapper.get('[data-testid="runtime-stream-concurrency-input"]').setValue('4')
    await wrapper.get('[data-testid="runtime-stream-buffers-input"]').setValue('8')
    await wrapper.get('[data-testid="runtime-stream-timeout-input"]').setValue('30s')
    await wrapper.get('[data-testid="runtime-media-concurrency-input"]').setValue('3')
    await wrapper.get('[data-testid="save-runtime-settings"]').trigger('click')
    await flushPromises()

    expect(apiPut).toHaveBeenCalledWith('/api/settings/runtime', {
      sync: {
        workers: 8,
        history_batch_size: 250,
        telegram_request_interval: '1500ms'
      },
      storage: {
        max_db_size: 30_000_000_000,
        max_media_cache: 40_000_000_000
      },
      telegram: {
        proxy: 'socks5://127.0.0.1:1080',
        reconnect_timeout: '6m',
        dial_timeout: '15s',
        rate_limit: {
          enabled: false,
          rate_per_second: 12,
          burst: 6
        },
        stream: {
          concurrency: 4,
          buffers: 8,
          chunk_timeout: '30s'
        },
        media: {
          concurrency: 3
        }
      }
    })
  })

  it('updates Telegram API credentials from the settings page', async () => {
    const wrapper = mount(SettingsView, {
      global: {
        stubs
      }
    })
    await flushPromises()

    expect(apiGet).toHaveBeenCalledWith('/api/settings/telegram-api')
    expect(wrapper.get<HTMLInputElement>('[data-testid="telegram-app-id-input"]').element.value).toBe('123456')

    await wrapper.get('[data-testid="telegram-app-id-input"]').setValue('654321')
    await wrapper.get('[data-testid="telegram-app-hash-input"]').setValue('new-hash-secret')
    await wrapper.get('[data-testid="save-telegram-api"]').trigger('click')
    await flushPromises()

    expect(apiPut).toHaveBeenCalledWith('/api/settings/telegram-api', {
      app_id: 654321,
      app_hash: 'new-hash-secret'
    })
  })

  it('does not show default Telegram API credentials as saved settings', async () => {
    vi.mocked(apiGet).mockImplementation((path: string) => {
      if (path === '/api/auth/me') {
        return Promise.resolve({ id: 1, username: 'admin', role: 'admin' })
      }
      if (path === '/api/storage/usage') {
        return Promise.resolve({
          db_bytes: 100,
          index_bytes: 200,
          media_cache_bytes: 300,
          total_bytes: 600,
          max_db_bytes: 15_000_000_000,
          max_media_bytes: 25_000_000_000,
          db_over_quota: false,
          media_over_quota: false
        })
      }
      if (path === '/api/settings/telegram-api') {
        return Promise.resolve({ configured: false, app_id: 0, app_hash_set: false })
      }
      if (path === '/api/settings/runtime') {
        return Promise.resolve({
          sync: {
            workers: 5,
            history_batch_size: 100,
            telegram_request_interval: '2s'
          },
          storage: {
            max_db_size: 15_000_000_000,
            max_media_cache: 25_000_000_000
          },
          telegram: {
            proxy: '',
            reconnect_timeout: '5m0s',
            dial_timeout: '10s',
            rate_limit: {
              enabled: true,
              rate_per_second: 10,
              burst: 5
            },
            stream: {
              concurrency: 2,
              buffers: 4,
              chunk_timeout: '20s'
            },
            media: {
              concurrency: 2
            }
          }
        })
      }
      if (path === '/api/settings/version') {
        return Promise.resolve({
          current_version: 'dev',
          update_available: false
        })
      }
      if (path === '/api/settings/system-info') {
        return Promise.resolve({
          name: 'Linux',
          version: '6.8.0-124-generic',
          architecture: 'amd64',
          go_version: 'go1.25.0',
          cpu_count: 8,
          hostname: 'tg-search-host'
        })
      }
      if (path === '/api/settings/version?check_update=true') {
        return Promise.resolve({
          current_version: 'dev',
          latest_version: 'v1.2.4',
          latest_url: 'https://github.com/power721/tg-search/releases/tag/v1.2.4',
          update_available: false
        })
      }
      return Promise.resolve({
        id: 1,
        name: 'default',
        prefix: '12345678',
        key: '12345678123456781234567812345678',
        usage_count: 7,
        created_at: '2026-06-08T00:00:00Z'
      })
    })

    const wrapper = mount(SettingsView, {
      global: {
        stubs
      }
    })
    await flushPromises()

    expect(wrapper.get<HTMLInputElement>('[data-testid="telegram-app-id-input"]').element.value).toBe('')
    expect(wrapper.get<HTMLInputElement>('[data-testid="telegram-app-hash-input"]').element.placeholder).toBe('请输入 App Hash')
    expect(wrapper.text()).not.toContain('26375241')
  })
})
