<script setup lang="ts">
const keyword = defineModel<string>('keyword', { required: true })
const category = defineModel<string>('category', { required: true })

const emit = defineEmits<{
  submit: []
}>()

const categories = [
  { label: 'All', value: '' },
  { label: 'Cloud Drive', value: 'cloud_drive' },
  { label: 'Magnet', value: 'magnet' },
  { label: 'ED2K', value: 'ed2k' },
  { label: 'HTTP', value: 'http' },
  { label: 'Files', value: 'files' }
]
</script>

<template>
  <form class="resource-filters" @submit.prevent="emit('submit')">
    <n-input v-model:value="keyword" clearable placeholder="Search resource library" />
    <n-select
      v-model:value="category"
      :options="categories"
      class="category-select"
      label-field="label"
      value-field="value"
    />
    <n-button attr-type="submit" type="primary">Search</n-button>
  </form>
</template>

<style scoped>
.resource-filters {
  display: grid;
  gap: 10px;
  grid-template-columns: minmax(0, 1fr) 180px auto;
}

@media (max-width: 760px) {
  .resource-filters {
    grid-template-columns: 1fr;
  }
}
</style>
