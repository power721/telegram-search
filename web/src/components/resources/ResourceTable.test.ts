import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import resourceTableSource from './ResourceTable.vue?raw'
import ResourceTable from './ResourceTable.vue'

describe('ResourceTable', () => {
  it('fills the available horizontal space', () => {
    expect(resourceTableSource).toMatch(/\.resource-table\s*\{[\s\S]*\bwidth:\s*100%;/)
  })

  it('shows each resource message publish time', () => {
    const publishedAt = '2026-06-08T04:30:00Z'
    const wrapper = mount(ResourceTable, {
      props: {
        items: [
          {
            id: 'link:https://example.com/course',
            kind: 'link',
            category: 'cloud_drive',
            title: 'Course Pack',
            url: 'https://example.com/course',
            datetime: publishedAt
          }
        ]
      }
    })

    const expected = new Intl.DateTimeFormat('zh-CN', {
      dateStyle: 'medium',
      timeStyle: 'short'
    }).format(new Date(publishedAt))

    expect(wrapper.text()).toContain('发布时间')
    expect(wrapper.text()).toContain(expected)
  })
})
