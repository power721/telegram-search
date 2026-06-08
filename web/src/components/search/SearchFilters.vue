<script setup lang="ts">
const query = defineModel<string>('query', { required: true })

const emit = defineEmits<{
  submit: []
}>()
</script>

<template>
  <form class="search-filters" @submit.prevent="emit('submit')">
    <n-input v-model:value="query" clearable placeholder="Search messages, links, files, channels" />
    <n-button attr-type="submit" type="primary">Search</n-button>
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
