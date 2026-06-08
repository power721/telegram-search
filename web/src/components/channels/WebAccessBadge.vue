<script setup lang="ts">
defineProps<{
  value?: boolean
  error?: string
}>()
</script>

<template>
  <span v-if="error" class="status-pill status-danger">检测失败</span>
  <span v-else-if="value === true" class="status-pill status-success">可访问</span>
  <span v-else-if="value === false" class="status-pill status-warning">不可访问</span>
  <span v-else class="status-pill status-muted">未检测</span>
</template>
