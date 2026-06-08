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

function taskTypeLabel(type: string) {
  const labels: Record<string, string> = {
    history_sync: '历史同步',
    web_access_detection: '网页访问检测',
    metadata_sync: '元数据同步',
    cleanup: '清理'
  }
  return labels[type] ?? type
}

function statusLabel(status: string) {
  const labels: Record<string, string> = {
    pending: '待处理',
    running: '运行中',
    paused: '已暂停',
    failed: '失败',
    succeeded: '成功',
    completed: '已完成',
    cancelled: '已取消',
    flood_wait: '等待限流解除',
    reconnecting: '重连中'
  }
  return labels[status] ?? status
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
          <th>类型</th>
          <th>状态</th>
          <th>进度</th>
          <th>重试次数</th>
          <th>下次运行</th>
          <th>消息</th>
          <th>操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="task in tasks" :key="task.id">
          <td>{{ task.id }}</td>
          <td>{{ taskTypeLabel(task.type) }}</td>
          <td><n-tag size="small">{{ statusLabel(task.status) }}</n-tag></td>
          <td>{{ progressLabel(task) }}</td>
          <td>{{ task.retry_count }}</td>
          <td>{{ task.next_run_at || '-' }}</td>
          <td class="message-cell">{{ task.error_message || task.message || '-' }}</td>
          <td class="actions">
            <n-button size="small" @click="emit('select', task)">详情</n-button>
            <n-button v-if="canRetry(task)" size="small" @click="emit('retry', task)">重试</n-button>
            <n-button v-if="canCancel(task)" size="small" @click="emit('cancel', task)">取消</n-button>
            <n-button v-if="canPause(task)" size="small" @click="emit('pause', task)">暂停</n-button>
            <n-button v-if="canResume(task)" size="small" @click="emit('resume', task)">恢复</n-button>
          </td>
        </tr>
        <tr v-if="tasks.length === 0">
          <td colspan="8" class="empty-cell">暂无任务</td>
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
