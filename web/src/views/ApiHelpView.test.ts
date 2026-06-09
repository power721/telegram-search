import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import ApiHelpView from './ApiHelpView.vue'

describe('ApiHelpView', () => {
  it('documents header-only api key authentication', () => {
    const wrapper = mount(ApiHelpView)
    const text = wrapper.text()

    expect(text).toContain('X-API-Key: YOUR_API_KEY')
    expect(text).toContain('Authorization: YOUR_API_KEY')
    expect(text).not.toContain('Bearer')
    expect(text).not.toContain('api_key=YOUR_API_KEY')
  })
})
