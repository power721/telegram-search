<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import ResourceFilters from '@/components/resources/ResourceFilters.vue'
import ResourceTable from '@/components/resources/ResourceTable.vue'
import { useResourcesStore } from '@/stores/resources'

const pageSizeOptions = [20, 50, 100]
const resources = useResourcesStore()
const keyword = ref('')
const category = ref('')
const pageSize = ref(50)
const offset = ref(0)

const page = computed(() => Math.floor(offset.value / pageSize.value) + 1)
const canGoPrevious = computed(() => offset.value > 0)
const canGoNext = computed(() => offset.value + pageSize.value < resources.total)
const allCount = computed(() => {
  const groupedTotal = Object.values(resources.grouped).reduce((total, count) => total + count, 0)
  return groupedTotal || resources.total
})

const labels: Record<string, string> = {
  cloud_drive: '网盘',
  magnet: '磁力',
  ed2k: 'ED2K',
  http: 'HTTP',
  files: '文件'
}
const resourceTypes = computed(() => [
  { key: '', label: '全部', count: allCount.value },
  ...Object.entries(labels).map(([key, label]) => ({
    key,
    label,
    count: resources.grouped[key] ?? 0
  }))
])

async function load() {
  await resources.load({
    keyword: keyword.value,
    category: category.value,
    limit: pageSize.value,
    offset: offset.value
  })
}

async function resetAndLoad() {
  offset.value = 0
  await load()
}

async function selectCategory(value: string) {
  category.value = value
  await resetAndLoad()
}

async function previousPage() {
  if (!canGoPrevious.value) return
  offset.value = Math.max(0, offset.value - pageSize.value)
  await load()
}

async function nextPage() {
  if (!canGoNext.value) return
  offset.value += pageSize.value
  await load()
}

async function changePageSize(event: Event) {
  pageSize.value = Number((event.target as HTMLSelectElement).value)
  offset.value = 0
  await load()
}

onMounted(() => {
  void load()
})
</script>

<template>
  <section class="page-section">
    <div class="page-header">
      <div>
        <p class="page-kicker">Telegram 资源库</p>
        <h1 class="page-title">资源</h1>
      </div>
    </div>

    <div class="resource-types">
      <button
        v-for="type in resourceTypes"
        :key="type.key || 'all'"
        :class="{ active: category === type.key }"
        type="button"
        @click="selectCategory(type.key)"
      >
        <span>{{ type.label }}</span>
        <strong>{{ type.count }}</strong>
      </button>
    </div>

    <ResourceFilters
      v-model:category="category"
      v-model:keyword="keyword"
      class="filters"
      @submit="resetAndLoad"
    />
    <p v-if="resources.error" class="error-text">{{ resources.error }}</p>
    <ResourceTable class="table" :items="resources.items" />
    <div class="pagination">
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
        :disabled="!canGoPrevious || resources.loading"
        type="button"
        @click="previousPage"
      >
        上一页
      </button>
      <span>第 {{ page }} 页，共 {{ resources.total }} 条</span>
      <button
        aria-label="下一页"
        :disabled="!canGoNext || resources.loading"
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

.resource-types {
  display: grid;
  gap: 10px;
  grid-template-columns: repeat(6, minmax(0, 1fr));
  margin-bottom: 14px;
}

.resource-types button {
  background: #ffffff;
  border: 1px solid #d9dee7;
  border-radius: 8px;
  display: flex;
  justify-content: space-between;
  min-height: 48px;
  padding: 10px 12px;
}

.resource-types button.active {
  border-color: #18a058;
}

.filters,
.table {
  margin-top: 14px;
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

@media (max-width: 900px) {
  .resource-types {
    grid-template-columns: 1fr;
  }
}
</style>
