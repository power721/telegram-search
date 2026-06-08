<script setup lang="ts">
import { onMounted, ref } from 'vue'
import SearchFilters from '@/components/search/SearchFilters.vue'
import SearchResults from '@/components/search/SearchResults.vue'
import { useSearchStore } from '@/stores/search'

const search = useSearchStore()
const query = ref('')

async function runSearch() {
  if (!query.value.trim()) return
  await search.searchGlobal(query.value)
}

onMounted(() => {
  void runSearch()
})
</script>

<template>
  <section class="page-section">
    <div class="page-header">
      <div>
        <p class="page-kicker">Global Search</p>
        <h1 class="page-title">Search</h1>
      </div>
    </div>

    <SearchFilters v-model:query="query" @submit="runSearch" />
    <p v-if="search.error" class="error-text">{{ search.error }}</p>
    <SearchResults
      class="results"
      :remote-items="search.remoteResults?.items"
      :result="search.global"
    />
  </section>
</template>

<style scoped>
.page-header {
  margin-bottom: 18px;
}

.page-kicker {
  color: #667085;
  margin: 0 0 4px;
}

.page-title {
  font-size: 24px;
  margin: 0;
}

.results {
  margin-top: 16px;
}

.error-text {
  color: #b42318;
}
</style>
