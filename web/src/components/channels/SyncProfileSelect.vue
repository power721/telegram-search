<script setup lang="ts">
import type { SyncProfile } from '@/api/types'

const value = defineModel<SyncProfile>('value', { required: true })

const options = [
  { label: '快速', value: 'Quick' },
  { label: '普通', value: 'Normal' },
  { label: '深度', value: 'Deep' },
  { label: '完整', value: 'Full' }
]
</script>

<template>
  <div class="profile-select">
    <span v-for="option in options" :key="option.value" class="profile-label">{{ option.label }}</span>
    <n-select v-model:value="value" :options="options" />
  </div>
</template>

<style scoped>
.profile-select {
  display: grid;
  gap: 8px;
}

.profile-label {
  display: none;
}
</style>
