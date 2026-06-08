export interface ErrorEnvelope {
  error: {
    code: string
    message: string
  }
}

export interface SetupStatus {
  complete: boolean
  admin_configured: boolean
  api_key_configured: boolean
  telegram_configured: boolean
}

export interface User {
  id: number
  username: string
  role: string
  last_login_at?: string
  created_at?: string
  updated_at?: string
}

export interface ServiceStatus {
  service: string
  accounts: number
  channels: number
  messages: number
  links: number
  account_states: Record<string, number>
}

export interface StorageUsage {
  db_bytes: number
  index_bytes: number
  media_cache_bytes: number
  total_bytes: number
  max_db_bytes: number
  max_media_bytes: number
  db_over_quota: boolean
  media_over_quota: boolean
}

export interface TelegramAPISettingsResponse {
  configured: boolean
  app_id: number
  app_hash_set: boolean
}

export interface TelegramAccount {
  id: number
  phone: string
  telegram_user_id: number
  first_name: string
  last_name: string
  username: string
  status: string
  session_path?: string
  last_online_at?: string
  last_error: string
}

export interface TelegramLoginResponse {
  status: string
  password_required?: boolean
  account?: TelegramAccount
  metadata_sync?: {
    status: string
    channel_count: number
    error?: string
  }
}

export interface TelegramAccountsResponse {
  items: TelegramAccount[]
}

export type SyncProfile = 'Quick' | 'Normal' | 'Deep' | 'Full'

export interface TelegramChannel {
  id: number
  account_id: number
  telegram_channel_id: number
  access_hash: number
  title: string
  username: string
  type: string
  member_count: number
  description: string
  avatar_state: string
  sync_state: string
  listen_state: string
  history_sync_enabled: boolean
  sync_profile: SyncProfile
  listen_enabled: boolean
  remote_search_allowed: boolean
  last_message_id: number
  last_sync_time?: string
  web_access?: boolean
  web_access_checked_at?: string
  web_access_error: string
}

export interface ChannelControlPayload {
  history_sync_enabled: boolean
  sync_profile: SyncProfile
  listen_enabled: boolean
  remote_search_allowed: boolean
}

export interface ChannelsResponse {
  items: TelegramChannel[]
}

export interface WebAccessCheckResponse {
  items: Array<{
    channel_id: number
    web_access: boolean
    checked_at: string
    web_access_error: string
  }>
}

export interface ChannelAnalysis {
  channel: TelegramChannel
  control: ChannelControlPayload
  watch_rule?: {
    id: number
    channel_id: number
    enabled: boolean
    includes: string[]
    excludes: string[]
    message_types: string[]
    link_types: string[]
  }
  indexed_counts: {
    messages: number
    links: number
    files: number
  }
}

export interface RemoteSearchTask {
  id: number
  account_id: number
  channel_id: number
  query: string
  status: string
  source: string
  expires_at: string
}
