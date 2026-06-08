import { defineStore } from 'pinia'
import { apiGet, apiPost } from '@/api/client'
import type { GlobalSearchResult, RemoteSearchResults, RemoteSearchTask } from '@/api/types'

export interface SearchFilters {
  query?: string
  accountId?: number
  channelId?: number
  linkType?: string
  fileType?: string
  limit?: number
}

function buildSearchPath(path: string, filters: SearchFilters) {
  const params = new URLSearchParams()
  const query = filters.query?.trim()
  if (query) params.set('q', query)
  if (filters.accountId) params.set('account_id', String(filters.accountId))
  if (filters.channelId) params.set('channel_id', String(filters.channelId))
  if (filters.linkType) params.set('link_type', filters.linkType)
  if (filters.fileType) params.set('file_type', filters.fileType)
  params.set('limit', String(filters.limit ?? 50))
  return `${path}?${params.toString()}`
}

export const useSearchStore = defineStore('search', {
  state: () => ({
    global: null as GlobalSearchResult | null,
    remoteTask: null as RemoteSearchTask | null,
    remoteResults: null as RemoteSearchResults | null,
    loading: false,
    error: ''
  }),
  actions: {
    async searchGlobal(query: string, filters: Omit<SearchFilters, 'query'> = {}) {
      return this.withLoading(async () => {
        this.global = await apiGet<GlobalSearchResult>(
          buildSearchPath('/api/search/global', { ...filters, query })
        )
        return this.global
      })
    },
    async createRemoteSearch(channelId: number, query: string) {
      return this.withLoading(async () => {
        this.remoteTask = await apiPost<RemoteSearchTask>('/api/search/remote', {
          channel_id: channelId,
          query
        })
        return this.remoteTask
      })
    },
    async loadRemoteResults(taskId: number) {
      return this.withLoading(async () => {
        this.remoteResults = await apiGet<RemoteSearchResults>(`/api/search/remote/${taskId}`)
        return this.remoteResults
      })
    },
    async withLoading<T>(fn: () => Promise<T>): Promise<T> {
      this.loading = true
      this.error = ''
      try {
        return await fn()
      } catch (error) {
        this.error = error instanceof Error ? error.message : 'Request failed'
        throw error
      } finally {
        this.loading = false
      }
    }
  }
})
