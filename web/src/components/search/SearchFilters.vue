<script setup lang="ts">
const query = defineModel<string>('query', { required: true })

const emit = defineEmits<{
  submit: []
}>()
</script>

<template>
  <form class="search-filters" @submit.prevent="emit('submit')">
    <label class="filter-label" for="global-search-query">查询</label>
    <n-input
      id="global-search-query"
      v-model:value="query"
      clearable
      placeholder="搜索消息、链接、文件、频道"
    />
    <n-button attr-type="submit" type="primary">搜索</n-button>
  </form>
</template>

<style scoped>
.search-filters {
  grid-template-columns: auto minmax(0, 1fr) auto;
}

@media (max-width: 640px) {
  .search-filters {
    grid-template-columns: 1fr;
  }
}
</style>
