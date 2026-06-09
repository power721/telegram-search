import { defineStore } from 'pinia'
import { apiDelete, apiGet, apiPost } from '@/api/client'
import type { Task, TasksResponse } from '@/api/types'

export interface TaskFilters {
  status?: string
  type?: string
  q?: string
  sort?: string
  order?: 'asc' | 'desc'
  limit?: number
  offset?: number
}

export interface TaskDeleteManyResult {
  deleted: number
  rejected_ids: number[]
  missing_ids: number[]
}

function buildTasksPath(filters: TaskFilters = {}) {
  const params = new URLSearchParams()
  if (filters.status) params.set('status', filters.status)
  if (filters.type) params.set('type', filters.type)
  if (filters.q) params.set('q', filters.q)
  if (filters.sort) params.set('sort', filters.sort)
  if (filters.order) params.set('order', filters.order)
  params.set('limit', String(filters.limit ?? 50))
  if (filters.offset) params.set('offset', String(filters.offset))
  return `/api/tasks?${params.toString()}`
}

export const useTasksStore = defineStore('tasks', {
  state: () => ({
    items: [] as Task[],
    total: 0,
    selected: null as Task | null,
    loading: false,
    error: ''
  }),
  actions: {
    async loadTasks(filters: TaskFilters = {}) {
      return this.withLoading(async () => {
        const response = await apiGet<TasksResponse>(buildTasksPath(filters))
        this.items = Array.isArray(response.items) ? response.items : []
        this.total = response.total ?? this.items.length
        return this.items
      })
    },
    async loadTask(id: number) {
      return this.withLoading(async () => {
        this.selected = await apiGet<Task>(`/api/tasks/${id}`)
        this.applyTask(this.selected)
        return this.selected
      })
    },
    async retryTask(id: number) {
      return this.runAction(id, 'retry')
    },
    async cancelTask(id: number) {
      return this.runAction(id, 'cancel')
    },
    async pauseTask(id: number) {
      return this.runAction(id, 'pause')
    },
    async resumeTask(id: number) {
      return this.runAction(id, 'resume')
    },
    async deleteTask(id: number) {
      return this.withLoading(async () => {
        await apiDelete<{ deleted: boolean }>(`/api/tasks/${id}`)
        this.removeTasks([id])
      })
    },
    async deleteTasks(ids: number[]) {
      return this.withLoading(async () => {
        const result = await apiPost<TaskDeleteManyResult>('/api/tasks/bulk-delete', { ids })
        const blocked = new Set([...(result.rejected_ids ?? []), ...(result.missing_ids ?? [])])
        this.removeTasks(ids.filter((id) => !blocked.has(id)))
        return result
      })
    },
    applyTask(task: Task) {
      const index = this.items.findIndex((item) => item.id === task.id)
      if (index >= 0) {
        this.items[index] = task
      } else {
        this.items.unshift(task)
      }
      if (this.selected?.id === task.id) {
        this.selected = task
      }
    },
    removeTasks(ids: number[]) {
      const targets = new Set(ids)
      const before = this.items.length
      this.items = this.items.filter((task) => !targets.has(task.id))
      this.total = Math.max(0, this.total - (before - this.items.length))
      if (this.selected && targets.has(this.selected.id)) {
        this.selected = null
      }
    },
    async runAction(id: number, action: 'retry' | 'cancel' | 'pause' | 'resume') {
      return this.withLoading(async () => {
        const task = await apiPost<Task>(`/api/tasks/${id}/${action}`)
        this.applyTask(task)
        return task
      })
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
