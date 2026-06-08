<script setup lang="ts">
import type { ResourceItem } from '@/api/types'

defineProps<{
  items: ResourceItem[]
}>()
</script>

<template>
  <div class="resource-table">
    <div class="table-head">
      <span>Resource</span>
      <span>Type</span>
      <span>Source</span>
    </div>
    <article v-for="item in items" :key="item.id" class="table-row">
      <div>
        <strong>{{ item.title || item.file_name || item.url }}</strong>
        <p>{{ item.url || item.file_name }}</p>
      </div>
      <span>{{ item.category }}</span>
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
.table-row p {
  overflow-wrap: anywhere;
}

.table-row p {
  color: #667085;
  margin: 4px 0 0;
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
