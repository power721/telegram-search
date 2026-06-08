<script setup lang="ts">
const query = defineModel<string>('query', { required: true })

const emit = defineEmits<{
  submit: []
}>()
</script>

<template>
  <form class="search-filters" @submit.prevent="emit('submit')">
    <n-input v-model:value="query" clearable placeholder="搜索消息、链接、文件、频道" />
    <n-button attr-type="submit" type="primary">搜索</n-button>
  </form>
</template>

<style scoped>
.search-filters {
  display: grid;
  gap: 10px;
  grid-template-columns: minmax(0, 1fr) auto;
}

@media (max-width: 640px) {
  .search-filters {
    grid-template-columns: 1fr;
  }
}
</style>
