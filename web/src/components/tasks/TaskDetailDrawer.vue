<script setup lang="ts">
import type { Task } from '@/api/types'

defineProps<{
  show: boolean
  task: Task | null
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
}>()
</script>

<template>
  <n-drawer :show="show" width="520" @update:show="emit('update:show', $event)">
    <n-drawer-content title="Task Detail">
      <n-descriptions v-if="task" :column="1" bordered>
        <n-descriptions-item label="ID">{{ task.id }}</n-descriptions-item>
        <n-descriptions-item label="Type">{{ task.type }}</n-descriptions-item>
        <n-descriptions-item label="Status">{{ task.status }}</n-descriptions-item>
        <n-descriptions-item label="Progress">{{ task.progress }} / {{ task.total }}</n-descriptions-item>
        <n-descriptions-item label="Message">{{ task.message || '-' }}</n-descriptions-item>
        <n-descriptions-item label="Error">{{ task.error_message || '-' }}</n-descriptions-item>
        <n-descriptions-item label="Retry Count">{{ task.retry_count }}</n-descriptions-item>
        <n-descriptions-item label="Next Run">{{ task.next_run_at || '-' }}</n-descriptions-item>
        <n-descriptions-item label="Payload">
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
