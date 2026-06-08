<script setup lang="ts">
const keyword = defineModel<string>('keyword', { required: true })
const category = defineModel<string>('category', { required: true })

const emit = defineEmits<{
  submit: []
}>()

const categories = [
  { label: '全部', value: '' },
  { label: '网盘', value: 'cloud_drive' },
  { label: '磁力', value: 'magnet' },
  { label: 'ED2K', value: 'ed2k' },
  { label: 'HTTP', value: 'http' },
  { label: '文件', value: 'files' }
]
</script>

<template>
  <form class="resource-filters" @submit.prevent="emit('submit')">
    <n-input v-model:value="keyword" clearable placeholder="搜索资源库" />
    <n-select
      v-model:value="category"
      :options="categories"
      class="category-select"
      label-field="label"
      value-field="value"
    />
    <n-button attr-type="submit" type="primary">搜索</n-button>
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
