import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import type { GlobalSearchResult } from '@/api/types'
import searchResultsSource from './SearchResults.vue?raw'
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

  it('styles external links as visible links', () => {
    expect(searchResultsSource).toMatch(/\.external-link\s*\{[\s\S]*color:\s*var\(--app-accent\);/)
    expect(searchResultsSource).toMatch(/\.external-link\s*\{[\s\S]*text-decoration:\s*underline;/)
  })

  it('opens Telegram message positions from message, link, and file titles', () => {
    const result = {
      messages: {
        items: [
          {
            id: 1,
            text: 'public message https://example.com/post',
            channel_username: 'publicchannel',
            telegram_message_id: 42
          }
        ],
        total: 1
      },
      links: {
        items: [
          {
            id: 2,
            message_id: 1,
            type: 'url',
            url: 'https://example.com/resource',
            telegram_channel_id: 1001234567890,
            telegram_message_id: 43
          }
        ],
        total: 1
      },
      files: {
        items: [
          {
            id: 3,
            message_id: 1,
            file_name: 'movie.mkv',
            extension: '.mkv',
            mime_type: 'video/x-matroska',
            size_bytes: 100,
            category: 'video',
            channel_username: 'files',
            telegram_message_id: 44
          }
        ],
        total: 1
      },
      channels: {
        items: [
          {
            id: 4,
            title: 'Public Channel',
            username: 'publicchannel'
          }
        ],
        total: 1
      }
    } as unknown as GlobalSearchResult

    const wrapper = mount(SearchResults, {
      props: { result }
    })

    expect(wrapper.find('a.result-row').exists()).toBe(false)
    const titleHrefs = wrapper.findAll('a.title-link').map((link) => link.attributes('href'))
    expect(titleHrefs).toEqual([
      'tg://resolve?domain=publicchannel&post=42',
      'tg://privatepost?channel=1234567890&post=43',
      'tg://resolve?domain=files&post=44',
      'tg://resolve?domain=publicchannel'
    ])
    for (const link of wrapper.findAll('a.title-link')) {
      expect(link.attributes('target')).toBe('_blank')
      expect(link.attributes('rel')).toContain('noopener')
    }
  })

  it('keeps external links clickable in message text and link results', () => {
    const result = {
      messages: {
        items: [
          {
            id: 1,
            text: 'watch https://example.com/post and (https://example.org/next).',
            channel_username: 'publicchannel',
            telegram_message_id: 42
          }
        ],
        total: 1
      },
      links: {
        items: [
          {
            id: 2,
            message_id: 1,
            type: 'url',
            url: 'https://example.com/resource',
            telegram_channel_id: 1001234567890,
            telegram_message_id: 43
          }
        ],
        total: 1
      },
      files: { items: [], total: 0 },
      channels: { items: [], total: 0 }
    } as unknown as GlobalSearchResult

    const wrapper = mount(SearchResults, {
      props: {
        result,
        remoteItems: [
          {
            source: 'remote',
            channel_id: 9,
            channel_title: 'Remote',
            telegram_message_id: 99,
            text: 'remote https://remote.example/item'
          }
        ]
      }
    })

    const hrefs = wrapper.findAll('a.external-link').map((link) => link.attributes('href'))
    expect(hrefs).toEqual([
      'https://example.com/post',
      'https://example.org/next',
      'https://remote.example/item',
      'https://example.com/resource'
    ])
  })

  it('renders enlarged hover previews for message, link, and file image results', () => {
    const result = {
      messages: {
        items: [
          {
            id: 1,
            text: 'photo result',
            media: {
              image_url: '/i/42'
            }
          }
        ],
        total: 1
      },
      links: {
        items: [
          {
            id: 3,
            message_id: 1,
            type: 'url',
            url: 'https://example.com/poster',
            media: {
              image_url: '/i/44'
            }
          }
        ],
        total: 1
      },
      files: {
        items: [
          {
            id: 2,
            message_id: 1,
            file_name: 'poster.jpg',
            extension: '.jpg',
            mime_type: 'image/jpeg',
            size_bytes: 100,
            category: 'image',
            media: {
              image_url: '/i/43'
            }
          }
        ],
        total: 1
      },
      channels: { items: [], total: 0 }
    } as unknown as GlobalSearchResult

    const wrapper = mount(SearchResults, {
      props: { result }
    })

    const thumbs = wrapper.findAll('img.search-thumb')
    expect(thumbs.map((image) => image.attributes('src'))).toEqual(['/i/42', '/i/44', '/i/43'])
    const previews = wrapper.findAll('.search-thumb-frame img.search-thumb-preview')
    expect(previews.map((image) => image.attributes('src'))).toEqual(['/i/42', '/i/44', '/i/43'])
    for (const preview of previews) {
      expect(preview.attributes('aria-hidden')).toBe('true')
    }
    expect(searchResultsSource).toMatch(/--search-thumb-preview-width:\s*600px;/)
    expect(searchResultsSource).toMatch(/\.search-thumb-frame:hover\s+\.search-thumb-preview\s*\{[\s\S]*opacity:\s*1;/)
  })

  it('falls back to video preview when an image thumbnail fails', async () => {
    const result = {
      messages: {
        items: [
          {
            id: 1,
            text: 'video result',
            media: {
              image_url: '/i/42',
              video_url: '/v/42'
            }
          }
        ],
        total: 1
      },
      links: { items: [], total: 0 },
      files: { items: [], total: 0 },
      channels: { items: [], total: 0 }
    } as unknown as GlobalSearchResult

    const wrapper = mount(SearchResults, {
      props: { result }
    })

    await wrapper.find('img.search-thumb').trigger('error')

    expect(wrapper.find('img.search-thumb').exists()).toBe(false)
    const video = wrapper.find('video.search-thumb')
    expect(video.exists()).toBe(true)
    expect(video.attributes('src')).toBe('/v/42')
  })
})
