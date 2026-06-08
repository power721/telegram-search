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
