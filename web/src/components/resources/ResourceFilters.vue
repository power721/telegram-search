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
    <label class="filter-label" for="resource-keyword">关键词</label>
    <n-input id="resource-keyword" v-model:value="keyword" clearable placeholder="搜索资源库" />
    <label class="filter-label" for="resource-category">类型</label>
    <n-select
      id="resource-category"
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
  grid-template-columns: auto minmax(0, 1fr) auto 180px auto;
}

@media (max-width: 760px) {
  .resource-filters {
    grid-template-columns: 1fr;
  }
}
</style>
