<script setup lang="ts">
defineProps<{
  value?: boolean
  error?: string
}>()
</script>

<template>
  <n-tag v-if="error" size="small" type="error">Error</n-tag>
  <n-tag v-else-if="value === true" size="small" type="success">Web Access</n-tag>
  <n-tag v-else-if="value === false" size="small" type="warning">No Web Access</n-tag>
  <n-tag v-else size="small">Unknown</n-tag>
</template>
