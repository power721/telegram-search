<script setup lang="ts">
import type { ResourceItem } from '@/api/types'
import { telegramMessageHref } from '@/utils/telegramLinks'

defineProps<{
  items: ResourceItem[]
}>()

function categoryLabel(category: string) {
  const labels: Record<string, string> = {
    cloud_drive: '网盘',
    magnet: '磁力',
    ed2k: 'ED2K',
    http: 'HTTP',
    files: '文件'
  }
  return labels[category] ?? category
}

function itemLabel(item: ResourceItem) {
  return item.title || item.file_name || item.url || '-'
}

function resourceHref(item: ResourceItem) {
  return telegramMessageHref(item) ?? item.url
}

function formatDate(value?: string) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return new Intl.DateTimeFormat('zh-CN', {
    dateStyle: 'medium',
    timeStyle: 'short'
  }).format(date)
}
</script>

<template>
  <div class="resource-table data-table">
    <div class="table-head sticky-head">
      <span>资源</span>
      <span>类型</span>
      <span>来源</span>
      <span>发布时间</span>
    </div>
    <div v-if="items.length === 0" class="empty-state">
      <strong>暂无资源</strong>
      <span>调整筛选条件或同步频道后，资源会显示在这里。</span>
    </div>
    <template v-for="item in items" :key="item.id">
      <a
        v-if="resourceHref(item)"
        class="table-row resource-link"
        :href="resourceHref(item)"
        rel="noopener noreferrer"
        target="_blank"
      >
        <div>
          <strong>{{ itemLabel(item) }}</strong>
          <p v-if="item.url">{{ item.url }}</p>
          <p v-else>{{ item.file_name || '-' }}</p>
        </div>
        <span>{{ categoryLabel(item.category) }}</span>
        <span>{{ item.channel_title || 'Telegram' }}</span>
        <time :datetime="item.datetime">{{ formatDate(item.datetime) }}</time>
      </a>
      <article v-else class="table-row">
        <div>
          <strong>{{ itemLabel(item) }}</strong>
          <p v-if="item.url">{{ item.url }}</p>
          <p v-else>{{ item.file_name || '-' }}</p>
        </div>
        <span>{{ categoryLabel(item.category) }}</span>
        <span>{{ item.channel_title || 'Telegram' }}</span>
        <time :datetime="item.datetime">{{ formatDate(item.datetime) }}</time>
      </article>
    </template>
  </div>
</template>

<style scoped>
.resource-table {
  overflow: hidden;
  width: 100%;
}

.table-head,
.table-row {
  display: grid;
  gap: 12px;
  grid-template-columns: minmax(0, 1fr) 120px 150px 180px;
  padding: 8px 10px;
}

.table-row {
  border-top: 1px solid var(--app-border-subtle);
  color: inherit;
  text-decoration: none;
}

.table-row strong,
.table-row p,
.table-row time {
  overflow-wrap: anywhere;
}

.table-row p {
  color: var(--app-text-muted);
  margin: 4px 0 0;
}

.resource-link:hover strong {
  color: var(--app-accent);
}

.table-row time {
  color: var(--app-text-muted);
}

@media (max-width: 760px) {
  .table-head {
    display: none;
  }

  .table-row {
    grid-template-columns: 1fr;
  }
}
</style>
