import { defineStore } from 'pinia'
import { apiGet, apiPatch, apiPost } from '@/api/client'
import type {
  ChannelAnalysis,
  ChannelControlPayload,
  ChannelsResponse,
  RemoteSearchTask,
  TelegramChannel,
  WebAccessCheckResponse
} from '@/api/types'

export const useChannelsStore = defineStore('channels', {
  state: () => ({
    items: [] as TelegramChannel[],
    selected: null as TelegramChannel | null,
    analysis: null as ChannelAnalysis | null,
    remoteTask: null as RemoteSearchTask | null,
    loading: false,
    error: ''
  }),
  actions: {
    async loadChannels(accountId?: number) {
      return this.withLoading(async () => {
        const suffix = accountId ? `?account_id=${accountId}` : ''
        const response = await apiGet<ChannelsResponse>(`/api/channels${suffix}`)
        this.items = response.items
        return this.items
      })
    },
    async updateControl(channelId: number, payload: ChannelControlPayload) {
      return this.withLoading(async () => {
        const updated = await apiPatch<TelegramChannel>(`/api/channels/${channelId}/control`, payload)
        this.replaceChannel(updated)
        return updated
      })
    },
    async checkWebAccess(channelIds: number[]) {
      return this.withLoading(async () => {
        const response = await apiPost<WebAccessCheckResponse>('/api/channels/web-access/check', {
          channel_ids: channelIds
        })
        await this.loadChannels()
        return response.items
      })
    },
    async analyzeChannel(channelId: number) {
      return this.withLoading(async () => {
        this.analysis = await apiPost<ChannelAnalysis>(`/api/channels/${channelId}/analyze`)
        return this.analysis
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
    replaceChannel(channel: TelegramChannel) {
      const index = this.items.findIndex((item) => item.id === channel.id)
      if (index >= 0) {
        this.items[index] = channel
      }
      if (this.selected?.id === channel.id) {
        this.selected = channel
      }
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
