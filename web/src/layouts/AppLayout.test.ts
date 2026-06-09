import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import AppLayout from './AppLayout.vue'

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

describe('AppLayout', () => {
  it('provides a fixed enterprise dashboard shell with sidebar, toolbar and constrained content', () => {
    const wrapper = mount(AppLayout)

    expect(wrapper.find('.app-sidebar').classes()).toContain('is-fixed')
    expect(wrapper.find('.app-toolbar').exists()).toBe(true)
    expect(wrapper.find('.content-frame').exists()).toBe(true)
    expect(wrapper.find('.nav-item.active span').text()).toBe('资源')
    const labels = wrapper.findAll('.nav-item span').map((item) => item.text())
    expect(labels.slice(-2)).toEqual(['设置', 'API'])
    expect(wrapper.text()).toContain('TG Search')
    expect(wrapper.text()).toContain('本地索引控制台')
  })
})
