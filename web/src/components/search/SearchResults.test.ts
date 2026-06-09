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

  it('links messages, links, and files to their Telegram message positions', () => {
    const result = {
      messages: {
        items: [
          {
            id: 1,
            text: 'public message',
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

    const hrefs = wrapper.findAll('a.result-row').map((link) => link.attributes('href'))
    expect(hrefs).toContain('tg://resolve?domain=publicchannel&post=42')
    expect(hrefs).toContain('tg://privatepost?channel=1234567890&post=43')
    expect(hrefs).toContain('tg://resolve?domain=files&post=44')
  })
})
