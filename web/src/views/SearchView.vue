<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import SearchFilters from '@/components/search/SearchFilters.vue'
import SearchResults from '@/components/search/SearchResults.vue'
import { useSearchStore } from '@/stores/search'

const pageSizeOptions = [20, 50, 100]
const search = useSearchStore()
const query = ref('')
const pageSize = ref(50)
const offset = ref(0)

const page = computed(() => Math.floor(offset.value / pageSize.value) + 1)
const total = computed(() => {
  const result = search.global
  if (!result) return 0
  return Math.max(
    result.messages.total,
    result.links.total,
    result.files.total,
    result.channels.total
  )
})
const canGoPrevious = computed(() => offset.value > 0)
const canGoNext = computed(() => offset.value + pageSize.value < total.value)

async function runSearch() {
  if (!query.value.trim()) return
  await search.searchGlobal(query.value, { limit: pageSize.value, offset: offset.value })
}

async function submitSearch() {
  offset.value = 0
  await runSearch()
}

async function previousPage() {
  if (!canGoPrevious.value) return
  offset.value = Math.max(0, offset.value - pageSize.value)
  await runSearch()
}

async function nextPage() {
  if (!canGoNext.value) return
  offset.value += pageSize.value
  await runSearch()
}

async function changePageSize(event: Event) {
  pageSize.value = Number((event.target as HTMLSelectElement).value)
  offset.value = 0
  await runSearch()
}

onMounted(() => {
  void runSearch()
})
</script>

<template>
  <section class="page-section">
    <div class="page-header">
      <div>
        <p class="page-kicker">全局搜索</p>
        <h1 class="page-title">搜索</h1>
      </div>
    </div>

    <SearchFilters v-model:query="query" @submit="submitSearch" />
    <p v-if="search.error" class="error-text">{{ search.error }}</p>
    <SearchResults
      class="results"
      :remote-items="search.remoteResults?.items"
      :result="search.global"
    />
    <div v-if="search.global" class="pagination">
      <label>
        每页
        <select aria-label="每页条数" :value="pageSize" @change="changePageSize">
          <option v-for="option in pageSizeOptions" :key="option" :value="option">
            {{ option }}
          </option>
        </select>
      </label>
      <button
        aria-label="上一页"
        :disabled="!canGoPrevious || search.loading"
        type="button"
        @click="previousPage"
      >
        上一页
      </button>
      <span>第 {{ page }} 页，共 {{ total }} 条</span>
      <button
        aria-label="下一页"
        :disabled="!canGoNext || search.loading"
        type="button"
        @click="nextPage"
      >
        下一页
      </button>
    </div>
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

.pagination {
  align-items: center;
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  justify-content: flex-end;
  margin-top: 14px;
}

.pagination label {
  align-items: center;
  color: #667085;
  display: inline-flex;
  gap: 6px;
}

.pagination select {
  background: #ffffff;
  border: 1px solid #d9dee7;
  border-radius: 6px;
  padding: 7px 8px;
}

.pagination button {
  background: #ffffff;
  border: 1px solid #d9dee7;
  border-radius: 6px;
  padding: 7px 10px;
}

.pagination button:disabled {
  color: #98a2b3;
  cursor: not-allowed;
}

.pagination span {
  color: #667085;
}
</style>
