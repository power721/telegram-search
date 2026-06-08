import { defineStore } from 'pinia'
import type { RuntimeEvent, Task } from '@/api/types'
import { useTasksStore } from '@/stores/tasks'

export const useEventsStore = defineStore('events', {
  state: () => ({
    connected: false,
    error: '',
    source: null as EventSource | null
  }),
  actions: {
    connect() {
      if (this.source) return
      if (typeof EventSource === 'undefined') return
      const source = new EventSource('/api/events')
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
        this.error = 'events stream disconnected'
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
