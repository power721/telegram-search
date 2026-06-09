<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import ResourceFilters from '@/components/resources/ResourceFilters.vue'
import ResourceTable from '@/components/resources/ResourceTable.vue'
import { useChannelsStore } from '@/stores/channels'
import { useResourcesStore } from '@/stores/resources'

const pageSizeOptions = [20, 50, 100]
const resources = useResourcesStore()
const channels = useChannelsStore()
const keyword = ref('')
const category = ref('')
const channelId = ref<string | number>('')
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
const channelOptions = computed(() => [
  { label: '全部频道', value: '' },
  ...channels.items.map((channel) => ({
    label: channel.username ? `${channel.title} (@${channel.username})` : channel.title,
    value: channel.id
  }))
])

async function load() {
  await resources.load({
    keyword: keyword.value,
    category: category.value,
    channelId: typeof channelId.value === 'number' ? channelId.value : undefined,
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
  void channels.loadChannels()
  void load()
})
</script>

<template>
  <section class="page-section">
    <div class="page-header">
      <div>
        <p class="page-kicker">Telegram 资源库</p>
        <h1 class="page-title">资源</h1>
        <p class="page-subtitle">按类型、关键词和来源浏览已索引的链接与文件资源。</p>
      </div>
    </div>

    <div class="resource-types" role="tablist" aria-label="资源类型">
      <button
        v-for="type in resourceTypes"
        :key="type.key || 'all'"
        :aria-selected="category === type.key"
        :class="{ active: category === type.key }"
        role="tab"
        type="button"
        @click="selectCategory(type.key)"
      >
        <span>{{ type.label }}</span>
        <strong>{{ type.count }}</strong>
      </button>
    </div>

    <ResourceFilters
      v-model:category="category"
      v-model:channel-id="channelId"
      v-model:keyword="keyword"
      :channel-options="channelOptions"
      class="filters"
      @submit="resetAndLoad"
    />
    <p v-if="resources.error" class="error-text">{{ resources.error }}</p>
    <div v-if="resources.loading" class="table-panel resource-loading" aria-label="正在加载资源">
      <span class="skeleton-line" />
      <span class="skeleton-line" />
      <span class="skeleton-line short" />
    </div>
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
.resource-types {
  display: grid;
  gap: 8px;
  grid-template-columns: repeat(6, minmax(0, 1fr));
}

.resource-types button {
  background: var(--app-surface);
  border: 1px solid var(--app-border);
  border-radius: var(--app-radius);
  color: var(--app-text);
  display: flex;
  justify-content: space-between;
  min-height: 48px;
  padding: 10px 12px;
}

.resource-types button.active {
  background: var(--app-accent-subtle);
  border-color: color-mix(in srgb, var(--app-accent) 35%, var(--app-border));
  color: var(--app-heading);
}

.resource-types button:hover {
  border-color: var(--app-border-strong);
}

.resource-loading {
  display: grid;
  gap: 10px;
  padding: 14px;
}

.resource-loading .short {
  width: 58%;
}

.pagination label {
  align-items: center;
  display: inline-flex;
  gap: 6px;
}

@media (max-width: 900px) {
  .resource-types {
    grid-template-columns: 1fr;
  }
}
</style>
