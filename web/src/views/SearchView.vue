<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import AppPagination from '@/components/common/AppPagination.vue'
import SearchFilters from '@/components/search/SearchFilters.vue'
import SearchResults from '@/components/search/SearchResults.vue'
import { useSearchStore } from '@/stores/search'

const pageSizeOptions = [20, 50, 100]
const search = useSearchStore()
const route = useRoute()
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

async function runSearch() {
  if (!query.value.trim()) return
  await search.searchGlobal(query.value, { limit: pageSize.value, offset: offset.value })
}

async function submitSearch() {
  offset.value = 0
  await runSearch()
}

async function changePage(pageNumber: number) {
  offset.value = (pageNumber - 1) * pageSize.value
  await runSearch()
}

async function changePageSize(value: number) {
  pageSize.value = value
  offset.value = 0
  await runSearch()
}

onMounted(() => {
  const routeQuery = route?.query?.q
  if (typeof routeQuery === 'string') {
    query.value = routeQuery
  }
  void runSearch()
})
</script>

<template>
  <section class="page-section">
    <div class="page-header">
      <div>
        <p class="page-kicker">全局搜索</p>
        <h1 class="page-title">搜索</h1>
        <p class="page-subtitle">一次查询消息、链接、文件和频道，结果按本地索引来源分组。</p>
      </div>
    </div>

    <SearchFilters v-model:query="query" @submit="submitSearch" />
    <p v-if="search.error" class="error-text">{{ search.error }}</p>
    <SearchResults
      class="results"
      :loading="search.loading"
      :remote-items="search.remoteResults?.items"
      :result="search.global"
    />
    <AppPagination
      v-if="search.global"
      :loading="search.loading"
      :page="page"
      :page-size="pageSize"
      :page-size-options="pageSizeOptions"
      :total="total"
      @update:page="changePage"
      @update:page-size="changePageSize"
    />
  </section>
</template>

<style scoped>
.results {
  min-width: 0;
}
</style>
