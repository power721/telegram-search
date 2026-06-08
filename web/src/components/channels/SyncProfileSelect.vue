<script setup lang="ts">
import type { SyncProfile } from '@/api/types'

const value = defineModel<SyncProfile>('value', { required: true })

const options = [
  { label: 'Quick', value: 'Quick' },
  { label: 'Normal', value: 'Normal' },
  { label: 'Deep', value: 'Deep' },
  { label: 'Full', value: 'Full' }
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
