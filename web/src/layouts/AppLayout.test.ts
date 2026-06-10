import { mount } from '@vue/test-utils'
import { defineComponent, nextTick } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AppLayout from './AppLayout.vue'

const wideContentStorageKey = 'tg-search:wide-content'

vi.mock('vue-router', () => ({
  RouterLink: {
    props: ['to'],
    template: '<a :href="to"><slot /></a>'
  },
  RouterView: {
    template: '<main data-test="route-view" />'
  },
  useRoute: () => ({ name: 'resources' })
}))

const NSwitchStub = defineComponent({
  props: {
    value: {
      type: Boolean,
      default: false
    }
  },
  emits: ['update:value'],
  template: `
    <button
      class="wide-switch-stub"
      type="button"
      v-bind="$attrs"
      :aria-pressed="String(value)"
      @click="$emit('update:value', !value)"
    />
  `
})

function mountLayout() {
  return mount(AppLayout, {
    global: {
      stubs: {
        'n-switch': NSwitchStub
      }
    }
  })
}

describe('AppLayout', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('provides a fixed enterprise dashboard shell with sidebar, toolbar and constrained content', () => {
    const wrapper = mountLayout()

    expect(wrapper.find('.app-sidebar').classes()).toContain('is-fixed')
    expect(wrapper.find('.app-toolbar').exists()).toBe(true)
    expect(wrapper.find('.content-frame').exists()).toBe(true)
    expect(wrapper.find('.nav-item.active span').text()).toBe('资源')
    const labels = wrapper.findAll('.nav-item span').map((item) => item.text())
    expect(labels.slice(-2)).toEqual(['设置', 'API'])
    expect(wrapper.text()).toContain('TG Search')
    expect(wrapper.text()).toContain('本地索引控制台')
  })

  it('toggles wide content mode and persists the preference', async () => {
    const wrapper = mountLayout()

    expect(wrapper.find('.content-frame').classes()).not.toContain('is-wide')

    await wrapper.find('.wide-switch-stub').trigger('click')
    await nextTick()

    expect(wrapper.find('.content-frame').classes()).toContain('is-wide')
    expect(localStorage.getItem(wideContentStorageKey)).toBe('true')
  })

  it('restores wide content mode from local storage', () => {
    localStorage.setItem(wideContentStorageKey, 'true')

    const wrapper = mountLayout()

    expect(wrapper.find('.content-frame').classes()).toContain('is-wide')
  })
})
