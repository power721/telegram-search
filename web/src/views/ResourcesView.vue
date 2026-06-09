<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useDialog } from 'naive-ui'
import type { ResourceItem } from '@/api/types'
import AppPagination from '@/components/common/AppPagination.vue'
import ResourceFilters from '@/components/resources/ResourceFilters.vue'
import ResourceTable from '@/components/resources/ResourceTable.vue'
import { useChannelsStore } from '@/stores/channels'
import { useResourcesStore } from '@/stores/resources'

const pageSizeOptions = [20, 50, 100]
const resources = useResourcesStore()
const channels = useChannelsStore()
const dialog = useDialog()
const keyword = ref('')
const category = ref('')
const channelId = ref<string | number>('')
const pageSize = ref(50)
const offset = ref(0)
const selectedResourceIds = ref<string[]>([])

const page = computed(() => Math.floor(offset.value / pageSize.value) + 1)
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

async function refreshResources() {
  selectedResourceIds.value = []
  await load()
}

async function resetAndLoad() {
  offset.value = 0
  await load()
}

async function selectCategory(value: string) {
  category.value = value
  await resetAndLoad()
}

async function changePage(pageNumber: number) {
  offset.value = (pageNumber - 1) * pageSize.value
  await load()
}

async function changePageSize(value: number) {
  pageSize.value = value
  offset.value = 0
  await load()
}

function toggleResourceSelection(item: ResourceItem, selected: boolean) {
  const next = new Set(selectedResourceIds.value)
  if (selected) {
    next.add(item.id)
  } else {
    next.delete(item.id)
  }
  selectedResourceIds.value = [...next]
}

function toggleCurrentPageSelection(selected: boolean) {
  selectedResourceIds.value = selected ? resources.items.map((item) => item.id) : []
}

function confirmDeleteResource(item: ResourceItem) {
  dialog.warning({
    title: '删除资源',
    content: `确定删除 ${resourceLabel(item)}？`,
    positiveText: '删除资源',
    positiveButtonProps: { type: 'error' },
    negativeText: '取消',
    onPositiveClick: async () => {
      await resources.deleteResource(item.id)
      selectedResourceIds.value = selectedResourceIds.value.filter((id) => id !== item.id)
      await load()
    }
  })
}

function confirmDeleteSelectedResources() {
  const ids = [...selectedResourceIds.value]
  if (ids.length === 0) return
  dialog.warning({
    title: '删除资源',
    content: `确定删除选中的 ${ids.length} 个资源？`,
    positiveText: '删除资源',
    positiveButtonProps: { type: 'error' },
    negativeText: '取消',
    onPositiveClick: async () => {
      await resources.deleteResources(ids)
      selectedResourceIds.value = []
      await load()
    }
  })
}

function resourceLabel(item: ResourceItem) {
  return item.media?.title || item.title || item.file_name || item.url || item.id
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
      <div class="header-actions">
        <n-button
          :disabled="selectedResourceIds.length === 0 || resources.loading"
          ghost
          type="error"
          @click="confirmDeleteSelectedResources"
        >
          删除选中
        </n-button>
        <n-button aria-label="刷新资源" :loading="resources.loading" @click="refreshResources">刷新</n-button>
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
    <ResourceTable
      class="table"
      :items="resources.items"
      :selected-ids="selectedResourceIds"
      @delete="confirmDeleteResource"
      @toggle-select="toggleResourceSelection"
      @toggle-select-all="toggleCurrentPageSelection"
    />
    <AppPagination
      :loading="resources.loading"
      :page="page"
      :page-size="pageSize"
      :page-size-options="pageSizeOptions"
      :total="resources.total"
      @update:page="changePage"
      @update:page-size="changePageSize"
    />
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

@media (max-width: 900px) {
  .resource-types {
    grid-template-columns: 1fr;
  }
}
</style>
