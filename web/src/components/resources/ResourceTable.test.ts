import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import resourceTableSource from './ResourceTable.vue?raw'
import ResourceTable from './ResourceTable.vue'

describe('ResourceTable', () => {
  it('fills the available horizontal space', () => {
    expect(resourceTableSource).toMatch(/\.resource-table\s*\{[\s\S]*\bwidth:\s*100%;/)
  })

  it('uses the shared compact table pattern with sticky headers and empty state', () => {
    const wrapper = mount(ResourceTable, {
      props: {
        items: []
      }
    })

    expect(wrapper.find('.resource-table').classes()).toContain('data-table')
    expect(wrapper.find('.table-head').classes()).toContain('sticky-head')
    expect(wrapper.find('.empty-state').text()).toContain('暂无资源')
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

  it('opens the Telegram message position for a resource row', () => {
    const wrapper = mount(ResourceTable, {
      props: {
        items: [
          {
            id: 'link:https://example.com/course',
            kind: 'link',
            category: 'cloud_drive',
            title: 'Course Pack',
            url: 'https://example.com/course',
            channel_username: 'resources',
            telegram_message_id: 77,
            datetime: '2026-06-08T04:30:00Z'
          }
        ]
      }
    })

    expect(wrapper.find('a.table-row').attributes('href')).toBe('tg://resolve?domain=resources&post=77')
  })

  it('renders media thumbnails for resources with image URLs', () => {
    const wrapper = mount(ResourceTable, {
      props: {
        items: [
          {
            id: 'link:https://example.com/course',
            kind: 'link',
            category: 'cloud_drive',
            title: 'Course Pack',
            url: 'https://example.com/course',
            media: {
              image_url: '/i/resources/77'
            }
          }
        ]
      }
    })

    const image = wrapper.find('img.resource-thumb')
    expect(image.exists()).toBe(true)
    expect(image.attributes('src')).toBe('/i/resources/77')
  })
})
