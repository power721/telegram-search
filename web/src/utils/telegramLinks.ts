export interface TelegramMessageLocation {
  channel_username?: string
  telegram_channel_id?: number
  telegram_message_id?: number
}

export interface TelegramChannelLocation {
  username?: string
  channel_username?: string
}

export function telegramMessageHref(location: TelegramMessageLocation) {
  const postID = location.telegram_message_id
  if (!postID) return undefined

  const username = location.channel_username?.trim().replace(/^@/, '')
  if (username) {
    return `tg://resolve?domain=${encodeURIComponent(username)}&post=${encodeURIComponent(String(postID))}`
  }

  const channelID = normalizePrivateChannelID(location.telegram_channel_id)
  if (!channelID) return undefined
  return `tg://privatepost?channel=${encodeURIComponent(channelID)}&post=${encodeURIComponent(String(postID))}`
}

export function telegramChannelHref(location: TelegramChannelLocation) {
  const username = (location.username || location.channel_username)?.trim().replace(/^@/, '')
  if (!username) return undefined
  return `tg://resolve?domain=${encodeURIComponent(username)}`
}

export function normalizePrivateChannelID(value?: number) {
  if (!value) return ''
  const raw = String(Math.trunc(Math.abs(value)))
  if (raw.startsWith('100') && raw.length > 10) {
    return raw.slice(3)
  }
  return raw
}
