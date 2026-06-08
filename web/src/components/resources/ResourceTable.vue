<script setup lang="ts">
import type { ResourceItem } from '@/api/types'

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
</script>

<template>
  <div class="resource-table">
    <div class="table-head">
      <span>资源</span>
      <span>类型</span>
      <span>来源</span>
    </div>
    <article v-for="item in items" :key="item.id" class="table-row">
      <div>
        <strong>{{ itemLabel(item) }}</strong>
        <p v-if="item.url">
          <a :href="item.url" rel="noopener noreferrer" target="_blank">{{ item.url }}</a>
        </p>
        <p v-else>{{ item.file_name || '-' }}</p>
      </div>
      <span>{{ categoryLabel(item.category) }}</span>
      <span>{{ item.channel_title || 'Telegram' }}</span>
    </article>
  </div>
</template>

<style scoped>
.resource-table {
  background: #ffffff;
  border: 1px solid #d9dee7;
  border-radius: 8px;
  overflow: hidden;
  width: 100%;
}

.table-head,
.table-row {
  display: grid;
  gap: 12px;
  grid-template-columns: minmax(0, 1fr) 130px 160px;
  padding: 12px 14px;
}

.table-head {
  background: #f8fafc;
  color: #667085;
  font-size: 13px;
  font-weight: 600;
}

.table-row {
  border-top: 1px solid #eef1f5;
}

.table-row strong,
.table-row p,
.table-row a {
  overflow-wrap: anywhere;
}

.table-row p {
  color: #667085;
  margin: 4px 0 0;
}

.table-row a {
  color: #175cd3;
  text-decoration: underline;
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
