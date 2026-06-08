import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiGet } from '@/api/client'
import { useResourcesStore } from './resources'

vi.mock('@/api/client', () => ({
  apiGet: vi.fn()
}))

describe('useResourcesStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(apiGet).mockReset()
  })

  it('loads resources and grouped counts', async () => {
    vi.mocked(apiGet)
      .mockResolvedValueOnce({
        items: [{ id: 'link:1', kind: 'link', category: 'cloud_drive', title: 'Course' }],
        total: 1,
        grouped: { cloud_drive: 1, magnet: 0, ed2k: 0, http: 0, files: 0 }
      })
      .mockResolvedValueOnce({
        grouped: { cloud_drive: 1, magnet: 2, ed2k: 0, http: 3, files: 4 }
      })
    const store = useResourcesStore()

    await store.load({ keyword: 'course', category: 'cloud_drive' })
    await store.loadGrouped()

    expect(apiGet).toHaveBeenNthCalledWith(
      1,
      '/api/resources?q=course&category=cloud_drive&limit=50'
    )
    expect(store.items[0].title).toBe('Course')
    expect(store.grouped.files).toBe(4)
  })
})
