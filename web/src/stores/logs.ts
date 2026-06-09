import { defineStore } from 'pinia'
import { apiDownload, apiGet } from '@/api/client'
import type { LogEntry, LogFileInfo, LogsResponse } from '@/api/types'

export interface LogFilters {
  file?: string
  level?: string
  query?: string
  order?: 'asc' | 'desc'
  limit?: number
  offset?: number
}

function buildLogsPath(filters: LogFilters = {}) {
  const params = new URLSearchParams()
  if (filters.file) params.set('file', filters.file)
  if (filters.level) params.set('level', filters.level)
  if (filters.query) params.set('q', filters.query)
  params.set('order', filters.order ?? 'desc')
  params.set('limit', String(filters.limit ?? 200))
  if (filters.offset) params.set('offset', String(filters.offset))
  return `/api/logs?${params.toString()}`
}

export const useLogsStore = defineStore('logs', {
  state: () => ({
    items: [] as LogEntry[],
    files: [] as LogFileInfo[],
    total: 0,
    order: 'desc' as 'asc' | 'desc',
    loading: false,
    downloading: false,
    error: ''
  }),
  actions: {
    async load(filters: LogFilters = {}) {
      return this.withLoading(async () => {
        const response = await apiGet<LogsResponse>(buildLogsPath(filters))
        this.items = Array.isArray(response.items) ? response.items : []
        this.files = Array.isArray(response.files) ? response.files : []
        this.total = response.total ?? this.items.length
        this.order = response.order ?? filters.order ?? 'desc'
        return this.items
      })
    },
    async download(file: string) {
      this.downloading = true
      this.error = ''
      try {
        return await apiDownload(`/api/logs/${encodeURIComponent(file)}/download`)
      } catch (error) {
        this.error = error instanceof Error ? error.message : '下载日志失败'
        throw error
      } finally {
        this.downloading = false
      }
    },
    async withLoading<T>(fn: () => Promise<T>): Promise<T> {
      this.loading = true
      this.error = ''
      try {
        return await fn()
      } catch (error) {
        this.error = error instanceof Error ? error.message : '请求日志失败'
        throw error
      } finally {
        this.loading = false
      }
    }
  }
})
