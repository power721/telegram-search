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

  it('emits row selection, page selection, and delete events', async () => {
    const items = [
      {
        id: 'link:https://example.com/course',
        kind: 'link' as const,
        category: 'cloud_drive',
        title: 'Course Pack',
        url: 'https://example.com/course'
      }
    ]
    const wrapper = mount(ResourceTable, {
      props: {
        items,
        selectedIds: []
      }
    })

    await wrapper.get('input[aria-label="选择资源 Course Pack"]').setValue(true)
    await wrapper.get('input[aria-label="选择当前页全部资源"]').setValue(true)
    await wrapper.findAll('button').find((button) => button.text() === '删除')!.trigger('click')

    expect(wrapper.emitted('toggleSelect')?.[0]).toEqual([items[0], true])
    expect(wrapper.emitted('toggleSelectAll')?.[0]).toEqual([true])
    expect(wrapper.emitted('delete')?.[0]).toEqual([items[0]])
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

  it('shows formatted file size in the metadata row for file resources', () => {
    const wrapper = mount(ResourceTable, {
      props: {
        items: [
          {
            id: 'file:ubuntu.iso',
            kind: 'file',
            category: 'files',
            file_name: 'ubuntu.iso',
            size_bytes: 5000
          }
        ]
      }
    })

    expect(wrapper.text()).not.toContain('大小')
    expect(wrapper.find('.media-meta').text()).toBe('4.9 KB')
  })

  it('shows the specific cloud drive type for cloud drive resources', () => {
    const wrapper = mount(ResourceTable, {
      props: {
        items: [
          {
            id: 'link:https://www.alipan.com/s/course',
            kind: 'link',
            type: 'aliyun',
            category: 'cloud_drive',
            title: 'Course Pack',
            url: 'https://www.alipan.com/s/course'
          },
          {
            id: 'link:magnet',
            kind: 'link',
            type: 'magnet',
            category: 'magnet',
            title: 'Magnet Resource',
            url: 'magnet:?xt=urn:btih:abc'
          },
          {
            id: 'link:legacy-cloud-drive',
            kind: 'link',
            type: 'url',
            category: 'cloud_drive',
            title: 'Legacy Cloud Resource',
            url: 'https://pan.quark.cn/s/legacy'
          }
        ]
      }
    })

    expect(wrapper.text()).toContain('阿里云盘')
    expect(wrapper.text()).toContain('磁力')
    expect(wrapper.text()).toContain('网盘')
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
              image_url: '/i/77'
            }
          }
        ]
      }
    })

    const image = wrapper.find('img.resource-thumb')
    expect(image.exists()).toBe(true)
    expect(image.attributes('src')).toBe('/i/77')
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
              image_url: '/i/77'
            }
          }
        ]
      }
    })

    const preview = wrapper.find('.resource-thumb-frame img.resource-thumb-preview')
    expect(preview.exists()).toBe(true)
    expect(preview.attributes('src')).toBe('/i/77')
    expect(preview.attributes('aria-hidden')).toBe('true')
    expect(resourceTableSource).toMatch(/--resource-thumb-preview-width:\s*600px;/)
    expect(resourceTableSource).toMatch(/img\.resource-thumb\s*\{[\s\S]*max-height:\s*55px;[\s\S]*max-width:\s*88px;[\s\S]*object-fit:\s*contain;/)
    expect(resourceTableSource).toMatch(/\.resource-thumb-preview\s*\{[\s\S]*max-height:\s*calc\(100vh - 32px\);[\s\S]*object-fit:\s*contain;[\s\S]*width:\s*auto;/)
    expect(resourceTableSource).toMatch(/\.resource-thumb-frame:hover\s+\.resource-thumb-preview\s*\{[\s\S]*opacity:\s*1;/)
  })

  it('renders a video placeholder when only a video URL is available', () => {
    const wrapper = mount(ResourceTable, {
      props: {
        items: [
          {
            id: 'file:video',
            kind: 'file',
            category: 'files',
            file_name: 'clip.mp4',
            media: {
              video_url: '/v/77'
            }
          }
        ]
      }
    })

    expect(wrapper.find('button.resource-thumb-button').exists()).toBe(true)
    expect(wrapper.find('.resource-video-placeholder').exists()).toBe(true)
    expect(wrapper.find('video.resource-thumb').exists()).toBe(false)
    expect(resourceTableSource).toMatch(/\.resource-video-placeholder\s*\{[\s\S]*linear-gradient/)
    expect(resourceTableSource).toMatch(/\.resource-thumb-button::after\s*\{[\s\S]*border-left:\s*11px solid #fff;/)
  })

  it('falls back to a video placeholder when an image thumbnail fails', async () => {
    const wrapper = mount(ResourceTable, {
      props: {
        items: [
          {
            id: 'file:video',
            kind: 'file',
            category: 'files',
            file_name: 'clip.mp4',
            media: {
              image_url: '/i/77',
              video_url: '/v/77'
            }
          }
        ]
      }
    })

    await wrapper.find('img.resource-thumb').trigger('error')

    expect(wrapper.find('.resource-video-placeholder').exists()).toBe(true)
    expect(wrapper.find('video.resource-thumb').exists()).toBe(false)
  })

  it('opens a video player dialog when clicking a video resource thumbnail', async () => {
    const wrapper = mount(ResourceTable, {
      props: {
        items: [
          {
            id: 'file:video',
            kind: 'file',
            category: 'files',
            file_name: 'clip.mp4',
            media: {
              image_url: '/i/77',
              video_url: '/v/77'
            }
          }
        ]
      }
    })

    await wrapper.find('button.resource-thumb-button').trigger('click')

    const player = wrapper.find('video.video-player')
    const dialog = wrapper.find('.video-player-dialog')
    expect(dialog.text()).toContain('clip.mp4')
    expect(player.exists()).toBe(true)
    expect(player.attributes('src')).toBe('/v/77')
    expect(player.attributes('poster')).toBe('/i/77')
    expect(player.attributes('controls')).toBeDefined()
    expect(player.attributes('autoplay')).toBeDefined()
    expect(resourceTableSource).toContain(':block-scroll="false"')
    expect(resourceTableSource).toMatch(/\.video-player-dialog\s*\{[\s\S]*width:\s*1200px;/)

    await wrapper.find('[aria-label="最大化播放窗口"]').trigger('click')

    expect(wrapper.find('.video-player-dialog').classes()).toContain('is-maximized')
    expect(wrapper.find('[aria-label="还原播放窗口"]').exists()).toBe(true)
    expect(resourceTableSource).toMatch(/\.video-player-dialog\.is-maximized\s*\{[\s\S]*width:\s*calc\(100vw - 24px\);/)

    await wrapper.find('[aria-label="关闭视频播放"]').trigger('click')

    expect(wrapper.find('video.video-player').exists()).toBe(false)
  })
})
