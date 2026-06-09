import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import type { GlobalSearchResult } from '@/api/types'
import SearchResults from './SearchResults.vue'

describe('SearchResults', () => {
  it('renders empty groups when API returns null items', () => {
    const result = {
      messages: { items: [], total: 0 },
      links: { items: null, total: 0 },
      files: { items: null, total: 0 },
      channels: { items: [], total: 0 }
    } as unknown as GlobalSearchResult

    const wrapper = mount(SearchResults, {
      props: { result }
    })

    expect(wrapper.text()).toContain('暂无链接结果')
    expect(wrapper.text()).toContain('暂无文件结果')
  })
})
