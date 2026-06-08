import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiGet, setAPIKey } from '@/api/client'
import { useEventsStore } from './events'
import { useTasksStore } from './tasks'

type Listener = (event: MessageEvent<string>) => void

class FakeEventSource {
  static instances: FakeEventSource[] = []
  readonly url: string
  listeners = new Map<string, Listener[]>()
  closed = false

  constructor(url: string) {
    this.url = url
    FakeEventSource.instances.push(this)
  }

  addEventListener(type: string, listener: Listener) {
    const listeners = this.listeners.get(type) ?? []
    listeners.push(listener)
    this.listeners.set(type, listeners)
  }

  close() {
    this.closed = true
  }

  emit(type: string, payload: unknown) {
    for (const listener of this.listeners.get(type) ?? []) {
      listener(new MessageEvent(type, { data: JSON.stringify(payload) }))
    }
  }
}

vi.mock('@/api/client', () => ({
  apiGet: vi.fn().mockResolvedValue({
    id: 1,
    name: 'default',
    prefix: '12345678',
    key: '12345678123456781234567812345678'
  }),
  setAPIKey: vi.fn()
}))

describe('events store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    FakeEventSource.instances = []
    vi.stubGlobal('EventSource', FakeEventSource)
    vi.clearAllMocks()
  })

  it('opens the events stream with api key and applies task updates', async () => {
    const tasks = useTasksStore()
    tasks.items = [{ id: 1, type: 'history_sync', status: 'running', progress: 1, total: 100 } as never]
    const events = useEventsStore()

    await events.connect()
    const source = FakeEventSource.instances[0]
    source.emit('task.updated', {
      type: 'task.updated',
      payload: { id: 1, type: 'history_sync', status: 'succeeded', progress: 100, total: 100 },
      created_at: '2026-06-08T12:00:00Z'
    })

    expect(apiGet).toHaveBeenCalledWith('/api/settings/api-key')
    expect(setAPIKey).toHaveBeenCalledWith('12345678123456781234567812345678')
    expect(source.url).toBe('/api/events?api_key=12345678123456781234567812345678')
    expect(tasks.items[0].status).toBe('succeeded')
    events.disconnect()
    expect(source.closed).toBe(true)
  })
})
