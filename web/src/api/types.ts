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
  api_key_step_complete: boolean
  telegram_configured: boolean
  telegram_login_complete: boolean
  listen_rules_configured: boolean
  current_step: 'admin' | 'api_key' | 'telegram_api' | 'telegram_login' | 'listen_rules' | 'channel_selection' | 'complete'
}

export interface ListenRulesPayload {
  includes: string[]
  excludes: string[]
  message_types: string[]
  link_types: string[]
}

export interface APIKeyResponse {
  id: number
  name: string
  prefix: string
  key: string
  last_used_at?: string
  created_at?: string
  updated_at?: string
}

export type APIKeySetupResponse = APIKeyResponse

export interface WatchRulePayload extends ListenRulesPayload {
  channel_id: number
  enabled: boolean
}

export interface WatchRule extends WatchRulePayload {
  id: number
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
  phone?: string
  password_required?: boolean
  account?: TelegramAccount
  metadata_sync?: {
    status: string
    channel_count: number
    error?: string
  }
}

export interface TelegramQRLoginStartResponse {
  login_id: string
  status: 'pending'
  qr_url: string
  expires_at: string
}

export interface TelegramQRLoginStatusResponse extends TelegramLoginResponse {
  login_id: string
  status: 'pending' | 'online'
  qr_url?: string
  expires_at?: string
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
  indexed_message_count: number
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
  watch_rule?: WatchRule
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

export interface Task {
  id: number
  type: string
  status: string
  progress: number
  total: number
  message?: string
  error_code?: string
  error_message?: string
  retry_count: number
  next_run_at?: string
  payload_json?: string
  started_at?: string
  finished_at?: string
  created_at?: string
  updated_at?: string
}

export interface TasksResponse {
  items: Task[]
  total: number
}

export interface RuntimeEvent<T = unknown> {
  type: string
  payload?: T
  created_at: string
}

export interface LogFileInfo {
  name: string
  size: number
  mod_time?: string
}

export interface LogEntry {
  file: string
  time?: string
  level?: string
  message?: string
  caller?: string
  fields?: Record<string, unknown>
  raw: string
}

export interface LogsResponse {
  items: LogEntry[]
  total: number
  files: LogFileInfo[]
  limit: number
  offset: number
  order: 'asc' | 'desc'
}

export interface ListResult<T> {
  items: T[]
  total: number
}

export interface Link {
  id: number
  message_id: number
  type: string
  url: string
  password?: string
  note?: string
  source_snippet?: string
  category?: string
}

export interface MessageSearchResult {
  id: number
  channel_id: number
  telegram_channel_id?: number
  telegram_message_id: number
  text: string
  raw_json?: string
  date?: string
  channel_title?: string
  channel_username?: string
  links?: Link[]
  source?: 'local' | 'remote'
}

export interface LinkSearchResult extends Link {
  message_text?: string
  message_date?: string
  channel_id?: number
  telegram_channel_id?: number
  channel_title?: string
  channel_username?: string
  telegram_message_id?: number
  source?: 'local' | 'remote'
}

export interface FileSearchResult {
  id: number
  message_id: number
  file_name: string
  extension: string
  mime_type: string
  size_bytes: number
  category: string
  message_text?: string
  message_date?: string
  channel_id?: number
  telegram_channel_id?: number
  channel_title?: string
  channel_username?: string
  telegram_message_id?: number
  source?: 'local' | 'remote'
}

export interface ChannelSearchResult extends TelegramChannel {
  source?: 'local' | 'remote'
}

export interface GlobalSearchResult {
  messages: ListResult<MessageSearchResult>
  links: ListResult<LinkSearchResult>
  files: ListResult<FileSearchResult>
  channels: ListResult<ChannelSearchResult>
}

export interface RemoteSearchItem {
  source: 'remote'
  channel_id: number
  telegram_channel_id?: number
  channel_title: string
  channel_username?: string
  telegram_message_id: number
  text: string
  raw_json?: string
  date?: string
}

export interface RemoteSearchResults {
  task: RemoteSearchTask
  items: RemoteSearchItem[]
}

export interface ResourceItem {
  id: string
  kind: 'link' | 'file'
  type?: string
  category: string
  url?: string
  file_name?: string
  extension?: string
  mime_type?: string
  size_bytes?: number
  note?: string
  title?: string
  source_snippet?: string
  datetime?: string
  channel_id?: number
  telegram_channel_id?: number
  channel_title?: string
  channel_username?: string
  telegram_message_id?: number
}

export interface ResourcesResponse {
  items: ResourceItem[]
  total: number
  grouped: Record<string, number>
}

export interface ResourcesGroupedResponse {
  grouped: Record<string, number>
}

export interface LinksGroupedResponse {
  grouped: Record<string, number>
}
