import { defineStore } from 'pinia'
import type { RuntimeEvent, Task } from '@/api/types'
import { useAPIKeyStore } from '@/stores/apiKey'
import { useTasksStore } from '@/stores/tasks'

export const useEventsStore = defineStore('events', {
  state: () => ({
    connected: false,
    error: '',
    source: null as EventSource | null
  }),
  actions: {
    async connect() {
      if (this.source) return
      if (typeof EventSource === 'undefined') return
      const apiKey = useAPIKeyStore()
      const current = apiKey.current ?? (await apiKey.load())
      const source = new EventSource(`/api/events?api_key=${encodeURIComponent(current.key)}`)
      source.addEventListener('task.updated', (event) => {
        const parsed = JSON.parse(event.data) as RuntimeEvent<Task>
        if (parsed.payload) {
          useTasksStore().applyTask(parsed.payload)
        }
      })
      source.onopen = () => {
        this.connected = true
        this.error = ''
      }
      source.onerror = () => {
        this.connected = false
        this.error = '事件流已断开'
      }
      this.source = source
    },
    disconnect() {
      this.source?.close()
      this.source = null
      this.connected = false
    }
  }
})
