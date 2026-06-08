<script setup lang="ts">
import type { Task } from '@/api/types'

defineProps<{
  show: boolean
  task: Task | null
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
}>()

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
</script>

<template>
  <n-drawer :show="show" width="520" @update:show="emit('update:show', $event)">
    <n-drawer-content title="任务详情">
      <n-descriptions v-if="task" :column="1" bordered>
        <n-descriptions-item label="ID">{{ task.id }}</n-descriptions-item>
        <n-descriptions-item label="类型">{{ taskTypeLabel(task.type) }}</n-descriptions-item>
        <n-descriptions-item label="状态">{{ statusLabel(task.status) }}</n-descriptions-item>
        <n-descriptions-item label="进度">{{ task.progress }} / {{ task.total }}</n-descriptions-item>
        <n-descriptions-item label="消息">{{ task.message || '-' }}</n-descriptions-item>
        <n-descriptions-item label="错误">{{ task.error_message || '-' }}</n-descriptions-item>
        <n-descriptions-item label="重试次数">{{ task.retry_count }}</n-descriptions-item>
        <n-descriptions-item label="下次运行">{{ task.next_run_at || '-' }}</n-descriptions-item>
        <n-descriptions-item label="任务载荷">
          <pre>{{ task.payload_json || '{}' }}</pre>
        </n-descriptions-item>
      </n-descriptions>
    </n-drawer-content>
  </n-drawer>
</template>

<style scoped>
pre {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
}
</style>
