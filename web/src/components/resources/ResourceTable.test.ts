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

  it('styles external links as visible links', () => {
    expect(resourceTableSource).toMatch(/\.external-link\s*\{[\s\S]*color:\s*var\(--app-accent\);/)
    expect(resourceTableSource).toMatch(/\.external-link\s*\{[\s\S]*text-decoration:\s*underline;/)
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

  it('renders nested media metadata', () => {
    const wrapper = mount(ResourceTable, {
      props: {
        items: [
          {
            id: 'link:https://example.com/course',
            kind: 'link',
            category: 'cloud_drive',
            title: 'Fallback Title',
            url: 'https://example.com/course',
            media: {
              title: 'Course Pack',
              year: '2026',
              season: 'S01',
              episode: 'E02',
              quality: '4K',
              size: '12GB',
              category: 'course',
              tmdb_id: '12345',
              tags: 'linux,release'
            }
          }
        ]
      }
    })

    expect(wrapper.text()).toContain('Course Pack')
    expect(wrapper.text()).toContain('2026 · S01 · E02 · 4K · 12GB · course · TMDB 12345')
    expect(wrapper.text()).toContain('linux,release')
  })

  it('opens the Telegram message position from the resource title and channel column', () => {
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

    expect(wrapper.find('a.table-row').exists()).toBe(false)
    expect(wrapper.find('a.title-link').attributes('href')).toBe('tg://resolve?domain=resources&post=77')
    expect(wrapper.find('a.channel-link').attributes('href')).toBe('tg://resolve?domain=resources&post=77')
    expect(wrapper.find('a.external-link').attributes('href')).toBe('https://example.com/course')
    for (const link of wrapper.findAll('a.title-link, a.channel-link')) {
      expect(link.attributes('target')).toBe('_blank')
      expect(link.attributes('rel')).toContain('noopener')
    }
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

  it('renders an enlarged hover preview for image thumbnails', () => {
    const wrapper = mount(ResourceTable, {
      props: {
        items: [
          {
            id: 'link:https://example.com/course',
            kind: 'link',
            category: 'cloud_drive',
            title: 'Course Pack',
            media: {
              image_url: '/i/resources/77'
            }
          }
        ]
      }
    })

    const preview = wrapper.find('.resource-thumb-frame img.resource-thumb-preview')
    expect(preview.exists()).toBe(true)
    expect(preview.attributes('src')).toBe('/i/resources/77')
    expect(preview.attributes('aria-hidden')).toBe('true')
    expect(resourceTableSource).toMatch(/--resource-thumb-preview-width:\s*600px;/)
    expect(resourceTableSource).toMatch(/\.resource-thumb-frame:hover\s+\.resource-thumb-preview\s*\{[\s\S]*opacity:\s*1;/)
  })

  it('renders video previews when only a video URL is available', () => {
    const wrapper = mount(ResourceTable, {
      props: {
        items: [
          {
            id: 'file:video',
            kind: 'file',
            category: 'files',
            file_name: 'clip.mp4',
            media: {
              video_url: '/v/resources/77'
            }
          }
        ]
      }
    })

    const video = wrapper.find('video.resource-thumb')
    expect(video.exists()).toBe(true)
    expect(video.attributes('src')).toBe('/v/resources/77')
    expect(video.attributes('preload')).toBe('metadata')
  })

  it('falls back to video preview when an image thumbnail fails', async () => {
    const wrapper = mount(ResourceTable, {
      props: {
        items: [
          {
            id: 'file:video',
            kind: 'file',
            category: 'files',
            file_name: 'clip.mp4',
            media: {
              image_url: '/i/resources/77',
              video_url: '/v/resources/77'
            }
          }
        ]
      }
    })

    await wrapper.find('img.resource-thumb').trigger('error')

    const video = wrapper.find('video.resource-thumb')
    expect(video.exists()).toBe(true)
    expect(video.attributes('src')).toBe('/v/resources/77')
    expect(video.attributes('poster')).toBe('/i/resources/77')
  })
})
