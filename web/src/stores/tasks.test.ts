import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiGet, apiPost } from '@/api/client'
import { useTasksStore } from './tasks'

vi.mock('@/api/client', () => ({
  apiGet: vi.fn((path: string) => {
    if (path === '/api/tasks') {
      return Promise.resolve({
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
  apiPost: vi.fn((path: string) => Promise.resolve({ id: 1, status: path.split('/').pop() }))
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
    expect(apiGet).toHaveBeenCalledWith('/api/tasks')
    expect(apiGet).toHaveBeenCalledWith('/api/tasks/1')
    expect(apiPost).toHaveBeenCalledWith('/api/tasks/1/retry')
    expect(apiPost).toHaveBeenCalledWith('/api/tasks/1/cancel')
    expect(apiPost).toHaveBeenCalledWith('/api/tasks/1/pause')
    expect(apiPost).toHaveBeenCalledWith('/api/tasks/1/resume')
  })

  it('keeps task items as an empty array when the API returns null items', async () => {
    vi.mocked(apiGet).mockResolvedValueOnce({ items: null } as never)
    const store = useTasksStore()

    await store.loadTasks()

    expect(store.items).toEqual([])
  })
})
