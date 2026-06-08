<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useResourcesStore } from '@/stores/resources'
import { useStatusStore } from '@/stores/status'

const status = useStatusStore()
const resources = useResourcesStore()

onMounted(() => {
  void status.load()
  void resources.loadGrouped()
})

const cards = computed(() => [
  { label: 'Accounts', value: status.service?.accounts ?? 0 },
  { label: 'Channels', value: status.service?.channels ?? 0 },
  { label: 'Messages', value: status.service?.messages ?? 0 },
  { label: 'Links', value: status.service?.links ?? 0 }
])

const resourceTypes = computed(() => [
  { label: 'Cloud Drive', value: resources.grouped.cloud_drive ?? 0 },
  { label: 'Magnet', value: resources.grouped.magnet ?? 0 },
  { label: 'ED2K', value: resources.grouped.ed2k ?? 0 },
  { label: 'HTTP', value: resources.grouped.http ?? 0 },
  { label: 'Files', value: resources.grouped.files ?? 0 }
])

function formatBytes(value = 0) {
  if (value >= 1_000_000_000) return `${(value / 1_000_000_000).toFixed(1)} GB`
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1)} MB`
  if (value >= 1_000) return `${(value / 1_000).toFixed(1)} KB`
  return `${value} B`
}
</script>

<template>
  <section class="page-section">
    <div class="page-header">
      <div>
        <p class="page-kicker">Overview</p>
        <h1 class="page-title">Local Telegram Index</h1>
      </div>
      <n-input class="global-search" placeholder="Search messages, links, files, channels" />
    </div>

    <div class="metric-grid">
      <div v-for="card in cards" :key="card.label" class="metric-card">
        <span>{{ card.label }}</span>
        <strong>{{ card.value }}</strong>
      </div>
    </div>

    <div class="dashboard-grid">
      <section class="panel">
        <h2>Storage Usage</h2>
        <dl>
          <div>
            <dt>DB</dt>
            <dd>{{ formatBytes(status.storage?.db_bytes) }}</dd>
          </div>
          <div>
            <dt>Index</dt>
            <dd>{{ formatBytes(status.storage?.index_bytes) }}</dd>
          </div>
          <div>
            <dt>Media Cache</dt>
            <dd>{{ formatBytes(status.storage?.media_cache_bytes) }}</dd>
          </div>
          <div>
            <dt>Total</dt>
            <dd>{{ formatBytes(status.storage?.total_bytes) }}</dd>
          </div>
        </dl>
      </section>

      <section class="panel">
        <h2>Top Resource Types</h2>
        <div class="resource-types">
          <span v-for="item in resourceTypes" :key="item.label">
            {{ item.label }}
            <strong>{{ item.value }}</strong>
          </span>
        </div>
      </section>
    </div>
  </section>
</template>

<style scoped>
.page-header {
  align-items: center;
  display: flex;
  gap: 16px;
  justify-content: space-between;
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

.global-search {
  max-width: 420px;
}

.metric-grid {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  margin-bottom: 16px;
}

.metric-card,
.panel {
  background: #ffffff;
  border: 1px solid #d9dee7;
  border-radius: 8px;
  padding: 14px;
}

.metric-card span {
  color: #667085;
  display: block;
}

.metric-card strong {
  display: block;
  font-size: 24px;
  margin-top: 6px;
}

.dashboard-grid {
  display: grid;
  gap: 16px;
  grid-template-columns: 1fr 1fr;
}

h2 {
  font-size: 16px;
  margin: 0 0 12px;
}

dl {
  margin: 0;
}

dl div {
  display: flex;
  justify-content: space-between;
  padding: 7px 0;
}

dd {
  font-weight: 600;
  margin: 0;
}

.resource-types {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.resource-types span {
  border: 1px solid #d9dee7;
  border-radius: 6px;
  display: inline-flex;
  gap: 8px;
  padding: 6px 8px;
}

@media (max-width: 840px) {
  .page-header {
    align-items: stretch;
    flex-direction: column;
  }

  .metric-grid,
  .dashboard-grid {
    grid-template-columns: 1fr;
  }
}
</style>
