import { defineStore } from 'pinia'
import { apiGet } from '@/api/client'
import type { ResourceItem, ResourcesGroupedResponse, ResourcesResponse } from '@/api/types'

export interface ResourceFilters {
  keyword?: string
  type?: string
  category?: string
  extension?: string
  limit?: number
}

function buildResourcePath(path: string, filters: ResourceFilters = {}) {
  const params = new URLSearchParams()
  const keyword = filters.keyword?.trim()
  if (keyword) params.set('q', keyword)
  if (filters.type) params.set('type', filters.type)
  if (filters.category) params.set('category', filters.category)
  if (filters.extension) params.set('extension', filters.extension)
  params.set('limit', String(filters.limit ?? 50))
  return `${path}?${params.toString()}`
}

export const useResourcesStore = defineStore('resources', {
  state: () => ({
    items: [] as ResourceItem[],
    total: 0,
    grouped: {} as Record<string, number>,
    loading: false,
    error: ''
  }),
  actions: {
    async load(filters: ResourceFilters = {}) {
      return this.withLoading(async () => {
        const response = await apiGet<ResourcesResponse>(buildResourcePath('/api/resources', filters))
        this.items = response.items
        this.total = response.total
        this.grouped = response.grouped
        return response
      })
    },
    async loadGrouped(filters: ResourceFilters = {}) {
      return this.withLoading(async () => {
        const response = await apiGet<ResourcesGroupedResponse>(
          buildResourcePath('/api/resources/grouped', filters)
        )
        this.grouped = response.grouped
        return response.grouped
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
