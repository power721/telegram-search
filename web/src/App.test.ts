import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
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
})
