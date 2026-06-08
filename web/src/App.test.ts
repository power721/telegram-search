import { mount } from '@vue/test-utils'
import naive from 'naive-ui'
import { describe, expect, it, vi } from 'vitest'
import App from './App.vue'

describe('App', () => {
  it('provides Naive UI dialog context', () => {
    const wrapper = mount(App, {
      global: {
        stubs: {
          RouterView: { template: '<main />' },
          NConfigProvider: { template: '<section><slot /></section>' },
          NMessageProvider: { template: '<section><slot /></section>' },
          NDialogProvider: { template: '<section data-test="dialog-provider"><slot /></section>' }
        }
      }
    })

    expect(wrapper.find('[data-test="dialog-provider"]').exists()).toBe(true)
  })

  it('renders Naive inputs with valid theme colors', () => {
    const wrapper = mount(App, {
      global: {
        plugins: [naive],
        stubs: {
          RouterView: { template: '<n-input data-test="themed-input" />' }
        }
      }
    })

    expect(wrapper.find('[data-test="themed-input"]').exists()).toBe(true)
  })

  it('uses dark Naive button colors when the OS color scheme is dark', () => {
    mockColorScheme(true)

    const wrapper = mount(App, {
      global: {
        plugins: [naive],
        stubs: {
          RouterView: { template: '<n-button data-test="default-action">刷新</n-button>' }
        }
      }
    })

    const style = wrapper.find('[data-test="default-action"]').attributes('style')
    expect(style).toContain('--n-color: #161b22')
    expect(style).toContain('--n-text-color: #e6edf3')
  })

  it('uses dark Naive descriptions header colors when the OS color scheme is dark', () => {
    mockColorScheme(true)

    const wrapper = mount(App, {
      global: {
        plugins: [naive],
        stubs: {
          RouterView: {
            template: `
              <n-descriptions data-test="task-description" :column="1" bordered>
                <n-descriptions-item label="ID">1</n-descriptions-item>
              </n-descriptions>
            `
          }
        }
      }
    })

    const headerStyle = wrapper.find('[data-test="task-description"]').attributes('style')
    expect(headerStyle).toContain('--n-th-color: #21262d')
    expect(headerStyle).toContain('--n-th-text-color: #f0f6fc')
  })
})

function mockColorScheme(matchesDark: boolean) {
  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: query.includes('prefers-color-scheme: dark') ? matchesDark : false,
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn()
    }))
  })
}
