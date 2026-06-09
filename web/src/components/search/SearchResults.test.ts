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

  it('does not link result rows to Telegram message positions', () => {
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
      channels: { items: [], total: 0 }
    } as unknown as GlobalSearchResult

    const wrapper = mount(SearchResults, {
      props: { result }
    })

    expect(wrapper.find('a.result-row').exists()).toBe(false)
    expect(wrapper.findAll('a').map((link) => link.attributes('href'))).not.toContain(
      'tg://resolve?domain=publicchannel&post=42'
    )
    expect(wrapper.findAll('a').map((link) => link.attributes('href'))).not.toContain(
      'tg://privatepost?channel=1234567890&post=43'
    )
    expect(wrapper.findAll('a').map((link) => link.attributes('href'))).not.toContain(
      'tg://resolve?domain=files&post=44'
    )
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
})
