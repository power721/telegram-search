<script setup lang="ts">
defineProps<{
  value?: boolean
  error?: string
}>()
</script>

<template>
  <n-tag v-if="error" size="small" type="error">检测失败</n-tag>
  <n-tag v-else-if="value === true" size="small" type="success">可访问</n-tag>
  <n-tag v-else-if="value === false" size="small" type="warning">不可访问</n-tag>
  <n-tag v-else size="small">未检测</n-tag>
</template>
