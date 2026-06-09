import { describe, expect, it } from 'vitest'
import { telegramChannelHref, telegramMessageHref } from './telegramLinks'

describe('telegramMessageHref', () => {
  it('uses channel username for public message links', () => {
    expect(telegramMessageHref({ channel_username: '@publicchannel', telegram_message_id: 42 })).toBe(
      'tg://resolve?domain=publicchannel&post=42'
    )
  })

  it('uses privatepost links for private channels', () => {
    expect(telegramMessageHref({ telegram_channel_id: 1001234567890, telegram_message_id: 42 })).toBe(
      'tg://privatepost?channel=1234567890&post=42'
    )
  })

  it('returns undefined without enough message location data', () => {
    expect(telegramMessageHref({ telegram_message_id: 42 })).toBeUndefined()
    expect(telegramMessageHref({ channel_username: 'publicchannel' })).toBeUndefined()
  })
})

describe('telegramChannelHref', () => {
  it('opens public channels through the Telegram app', () => {
    expect(telegramChannelHref({ username: '@publicchannel' })).toBe('tg://resolve?domain=publicchannel')
    expect(telegramChannelHref({ channel_username: 'otherchannel' })).toBe('tg://resolve?domain=otherchannel')
  })

  it('returns undefined without a channel username', () => {
    expect(telegramChannelHref({})).toBeUndefined()
  })
})
