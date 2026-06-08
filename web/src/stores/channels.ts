import { defineStore } from 'pinia'
import { apiDelete, apiGet, apiPatch, apiPost, apiPut } from '@/api/client'
import type {
  ChannelAnalysis,
  ChannelControlPayload,
  ChannelsResponse,
  ListenRulesPayload,
  RemoteSearchTask,
  TelegramChannel,
  WatchRule,
  WatchRulePayload,
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
    async updateControls(channelIds: number[], payload: ChannelControlPayload) {
      return this.withLoading(async () => {
        const response = await apiPatch<ChannelsResponse>('/api/channels/control', {
          channel_ids: channelIds,
          control: payload
        })
        for (const item of response.items) {
          this.replaceChannel(item)
        }
        return response.items
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
    async syncChannels(channelIds: number[], maxMessages?: number) {
      return this.withLoading(async () => {
        return apiPost('/api/channels/sync', {
          channel_ids: channelIds,
          ...(maxMessages ? { max_messages: maxMessages } : {})
        })
      })
    },
    async createWatchRule(payload: WatchRulePayload) {
      return this.withLoading(async () => {
        return apiPost<WatchRule>('/api/watch-rules', payload)
      })
    },
    async updateWatchRule(ruleId: number, payload: WatchRulePayload) {
      return this.withLoading(async () => {
        return apiPut<WatchRule>(`/api/watch-rules/${ruleId}`, payload)
      })
    },
    async deleteWatchRule(ruleId: number) {
      return this.withLoading(async () => {
        return apiDelete<{ deleted: boolean }>(`/api/watch-rules/${ruleId}`)
      })
    },
    async loadGlobalListenRules() {
      return this.withLoading(async () => {
        return apiGet<ListenRulesPayload>('/api/listen-rules')
      })
    },
    async updateGlobalListenRules(payload: ListenRulesPayload) {
      return this.withLoading(async () => {
        return apiPut<ListenRulesPayload>('/api/listen-rules', payload)
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
        this.error = error instanceof Error ? error.message : '请求失败'
        throw error
      } finally {
        this.loading = false
      }
    }
  }
})
