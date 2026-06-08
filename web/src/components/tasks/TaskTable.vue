<script setup lang="ts">
import type { Task } from '@/api/types'

defineProps<{
  tasks: Task[]
  loading?: boolean
}>()

const emit = defineEmits<{
  select: [task: Task]
  retry: [task: Task]
  cancel: [task: Task]
  pause: [task: Task]
  resume: [task: Task]
}>()

function progressLabel(task: Task) {
  if (task.total > 0) return `${task.progress} / ${task.total}`
  return `${task.progress}`
}

function canRetry(task: Task) {
  return ['failed', 'flood_wait', 'reconnecting'].includes(task.status)
}

function canCancel(task: Task) {
  return ['running', 'paused', 'flood_wait', 'reconnecting'].includes(task.status)
}

function canPause(task: Task) {
  return task.status === 'running'
}

function canResume(task: Task) {
  return task.status === 'paused'
}
</script>

<template>
  <div class="table-panel">
    <table>
      <thead>
        <tr>
          <th>ID</th>
          <th>Type</th>
          <th>Status</th>
          <th>Progress</th>
          <th>Retry</th>
          <th>Next Run</th>
          <th>Message</th>
          <th>Actions</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="task in tasks" :key="task.id">
          <td>{{ task.id }}</td>
          <td>{{ task.type }}</td>
          <td><n-tag size="small">{{ task.status }}</n-tag></td>
          <td>{{ progressLabel(task) }}</td>
          <td>{{ task.retry_count }}</td>
          <td>{{ task.next_run_at || '-' }}</td>
          <td class="message-cell">{{ task.error_message || task.message || '-' }}</td>
          <td class="actions">
            <n-button size="small" @click="emit('select', task)">Details</n-button>
            <n-button v-if="canRetry(task)" size="small" @click="emit('retry', task)">Retry</n-button>
            <n-button v-if="canCancel(task)" size="small" @click="emit('cancel', task)">Cancel</n-button>
            <n-button v-if="canPause(task)" size="small" @click="emit('pause', task)">Pause</n-button>
            <n-button v-if="canResume(task)" size="small" @click="emit('resume', task)">Resume</n-button>
          </td>
        </tr>
        <tr v-if="tasks.length === 0">
          <td colspan="8" class="empty-cell">No tasks</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<style scoped>
.table-panel {
  background: #ffffff;
  border: 1px solid #d9dee7;
  border-radius: 8px;
  overflow-x: auto;
}

table {
  border-collapse: collapse;
  min-width: 980px;
  width: 100%;
}

th,
td {
  border-bottom: 1px solid #edf0f5;
  padding: 10px 12px;
  text-align: left;
  vertical-align: top;
}

th {
  color: #667085;
  font-size: 13px;
  font-weight: 600;
}

.message-cell {
  max-width: 260px;
  overflow-wrap: anywhere;
}

.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  min-width: 260px;
}

.empty-cell {
  color: #667085;
  text-align: center;
}
</style>
