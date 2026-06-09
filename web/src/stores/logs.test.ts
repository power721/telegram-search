import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiDownload, apiGet } from '@/api/client'
import { useLogsStore } from './logs'

vi.mock('@/api/client', () => ({
  apiGet: vi.fn(() =>
    Promise.resolve({
      items: [{ file: 'app.log', level: 'info', message: 'boot complete', raw: '{}' }],
      files: [{ name: 'app.log', size: 120 }],
      total: 1,
      limit: 200,
      offset: 0,
      order: 'desc'
    })
  ),
  apiDownload: vi.fn(() => Promise.resolve(new Blob(['log data'])))
}))

describe('logs store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('loads logs with filters and downloads a selected file', async () => {
    const store = useLogsStore()

    await store.load({ file: 'app.log', level: 'info', query: 'boot', order: 'asc', limit: 100, offset: 100 })
    const blob = await store.download('app.log')

    expect(apiGet).toHaveBeenCalledWith('/api/logs?file=app.log&level=info&q=boot&order=asc&limit=100&offset=100')
    expect(apiDownload).toHaveBeenCalledWith('/api/logs/app.log/download')
    expect(store.items).toHaveLength(1)
    expect(store.files[0].name).toBe('app.log')
    expect(store.total).toBe(1)
    expect(blob).toBeInstanceOf(Blob)
  })
})
