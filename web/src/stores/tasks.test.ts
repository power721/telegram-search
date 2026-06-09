import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiDelete, apiGet, apiPost } from '@/api/client'
import { useTasksStore } from './tasks'

vi.mock('@/api/client', () => ({
  apiGet: vi.fn((path: string) => {
    if (path === '/api/tasks?limit=50') {
      return Promise.resolve({
        total: 1,
        items: [
          {
            id: 1,
            type: 'history_sync',
            status: 'failed',
            progress: 20,
            total: 100,
            error_message: 'temporary failure',
            retry_count: 0
          }
        ]
      })
    }
    if (path === '/api/tasks/1') {
      return Promise.resolve({ id: 1, type: 'history_sync', status: 'failed' })
    }
    return Promise.reject(new Error(`unexpected GET ${path}`))
  }),
  apiPost: vi.fn((path: string) => {
    if (path === '/api/tasks/bulk-delete') {
      return Promise.resolve({ deleted: 1, rejected_ids: [3], missing_ids: [] })
    }
    return Promise.resolve({ id: 1, status: path.split('/').pop() })
  }),
  apiDelete: vi.fn(() => Promise.resolve({ deleted: true }))
}))

describe('tasks store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('loads task list and calls task action endpoints', async () => {
    const store = useTasksStore()
    await store.loadTasks()
    await store.loadTask(1)
    await store.retryTask(1)
    await store.cancelTask(1)
    await store.pauseTask(1)
    await store.resumeTask(1)

    expect(store.items).toHaveLength(1)
    expect(store.selected?.id).toBe(1)
    expect(apiGet).toHaveBeenCalledWith('/api/tasks?limit=50')
    expect(apiGet).toHaveBeenCalledWith('/api/tasks/1')
    expect(apiPost).toHaveBeenCalledWith('/api/tasks/1/retry')
    expect(apiPost).toHaveBeenCalledWith('/api/tasks/1/cancel')
    expect(apiPost).toHaveBeenCalledWith('/api/tasks/1/pause')
    expect(apiPost).toHaveBeenCalledWith('/api/tasks/1/resume')
  })

  it('deletes single and selected tasks', async () => {
    const store = useTasksStore()
    store.items = [
      { id: 1, type: 'history_sync', status: 'failed', progress: 0, total: 0, retry_count: 0 },
      { id: 2, type: 'history_sync', status: 'succeeded', progress: 1, total: 1, retry_count: 0 },
      { id: 3, type: 'history_sync', status: 'running', progress: 0, total: 0, retry_count: 0 }
    ]
    store.total = 3

    await store.deleteTask(1)
    const result = await store.deleteTasks([2, 3])

    expect(apiDelete).toHaveBeenCalledWith('/api/tasks/1')
    expect(apiPost).toHaveBeenCalledWith('/api/tasks/bulk-delete', { ids: [2, 3] })
    expect(result).toEqual({ deleted: 1, rejected_ids: [3], missing_ids: [] })
    expect(store.items.map((task) => task.id)).toEqual([3])
    expect(store.total).toBe(1)
  })

  it('keeps task items as an empty array when the API returns null items', async () => {
    vi.mocked(apiGet).mockResolvedValueOnce({ items: null, total: 0 } as never)
    const store = useTasksStore()

    await store.loadTasks()

    expect(store.items).toEqual([])
  })

  it('passes page offsets and stores task totals', async () => {
    vi.mocked(apiGet).mockResolvedValueOnce({ items: [], total: 75 } as never)
    const store = useTasksStore()

    await store.loadTasks({ limit: 50, offset: 50 })

    expect(apiGet).toHaveBeenCalledWith('/api/tasks?limit=50&offset=50')
    expect(store.total).toBe(75)
  })
})
