<script setup lang="ts">
import type { Task } from '@/api/types'

defineProps<{
  tasks: Task[]
  loading?: boolean
  selectedIds?: number[]
}>()

const emit = defineEmits<{
  select: [task: Task]
  toggleSelect: [task: Task, selected: boolean]
  toggleSelectAll: [selected: boolean]
  retry: [task: Task]
  cancel: [task: Task]
  pause: [task: Task]
  resume: [task: Task]
  delete: [task: Task]
}>()

function progressLabel(task: Task) {
  if (task.total > 0) return `${task.progress} / ${task.total}`
  return `${task.progress}`
}

function taskTypeLabel(type: string) {
  const labels: Record<string, string> = {
    backup: '备份',
    channel_analysis: '频道分析',
    gap_recovery: '缺口恢复',
    history_sync: '历史同步',
    listener_recovery: '监听恢复',
    metadata_sync: '元数据同步',
    remote_search: '远程搜索',
    web_access_detection: '网页访问检测'
  }
  return labels[type] ?? type
}

function statusLabel(status: string) {
  const labels: Record<string, string> = {
    queued: '排队中',
    running: '运行中',
    canceling: '取消中',
    canceled: '已取消',
    paused: '已暂停',
    failed: '失败',
    succeeded: '成功',
    flood_wait: '等待限流解除',
    reconnecting: '重连中'
  }
  return labels[status] ?? status
}

function statusClass(status: string) {
  if (status === 'succeeded') return 'status-success'
  if (['running', 'reconnecting'].includes(status)) return 'status-info'
  if (['queued', 'paused', 'flood_wait', 'canceling'].includes(status)) return 'status-warning'
  if (['failed', 'canceled'].includes(status)) return 'status-danger'
  return 'status-muted'
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

function canDelete(task: Task) {
  return !['running', 'canceling'].includes(task.status)
}

function isSelected(task: Task, selectedIds: number[] = []) {
  return selectedIds.includes(task.id)
}

function allSelected(tasks: Task[], selectedIds: number[] = []) {
  return tasks.length > 0 && tasks.every((task) => selectedIds.includes(task.id))
}
</script>

<template>
  <div class="table-panel">
    <table class="data-table">
      <thead>
        <tr>
          <th>
            <input
              aria-label="选择当前页任务"
              :checked="allSelected(tasks, selectedIds)"
              type="checkbox"
              @change="emit('toggleSelectAll', ($event.target as HTMLInputElement).checked)"
            />
          </th>
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
        <tr v-if="loading">
          <td colspan="9">
            <div class="loading-stack" aria-label="正在加载任务">
              <span class="skeleton-line" />
              <span class="skeleton-line" />
              <span class="skeleton-line short" />
            </div>
          </td>
        </tr>
        <tr v-for="task in tasks" :key="task.id">
          <td>
            <input
              :aria-label="`选择任务 ${task.id}`"
              :checked="isSelected(task, selectedIds)"
              type="checkbox"
              @change="emit('toggleSelect', task, ($event.target as HTMLInputElement).checked)"
            />
          </td>
          <td>{{ task.id }}</td>
          <td>{{ taskTypeLabel(task.type) }}</td>
          <td><span class="status-pill" :class="statusClass(task.status)">{{ statusLabel(task.status) }}</span></td>
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
            <n-button v-if="canDelete(task)" size="small" type="error" ghost @click="emit('delete', task)">删除</n-button>
          </td>
        </tr>
        <tr v-if="!loading && tasks.length === 0">
          <td colspan="9">
            <div class="empty-state">
              <strong>暂无任务</strong>
              <span>同步、检测、清理等后台任务会显示在这里。</span>
            </div>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<style scoped>
.table-panel {
  overflow-x: auto;
}

table {
  min-width: 980px;
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
  text-align: center;
}

.loading-stack {
  display: grid;
  gap: 8px;
  padding: 8px 0;
}

.loading-stack .short {
  width: 58%;
}
</style>
