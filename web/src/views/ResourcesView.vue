<script setup lang="ts">
import { onMounted, ref } from 'vue'
import ResourceFilters from '@/components/resources/ResourceFilters.vue'
import ResourceTable from '@/components/resources/ResourceTable.vue'
import { useResourcesStore } from '@/stores/resources'

const resources = useResourcesStore()
const keyword = ref('ubuntu')
const category = ref('')

const labels: Record<string, string> = {
  cloud_drive: 'Cloud Drive',
  magnet: 'Magnet',
  ed2k: 'ED2K',
  http: 'HTTP',
  files: 'Files'
}

async function load() {
  await resources.load({ keyword: keyword.value, category: category.value })
}

onMounted(() => {
  void load()
  void resources.loadGrouped()
})
</script>

<template>
  <section class="page-section">
    <div class="page-header">
      <div>
        <p class="page-kicker">Telegram Resource Library</p>
        <h1 class="page-title">Resources</h1>
      </div>
    </div>

    <div class="resource-types">
      <button
        v-for="(label, key) in labels"
        :key="key"
        :class="{ active: category === key }"
        type="button"
        @click="
          category = key;
          load()
        "
      >
        <span>{{ label }}</span>
        <strong>{{ resources.grouped[key] ?? 0 }}</strong>
      </button>
    </div>

    <ResourceFilters
      v-model:category="category"
      v-model:keyword="keyword"
      class="filters"
      @submit="load"
    />
    <p v-if="resources.error" class="error-text">{{ resources.error }}</p>
    <ResourceTable class="table" :items="resources.items" />
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
  grid-template-columns: repeat(5, minmax(0, 1fr));
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

@media (max-width: 900px) {
  .resource-types {
    grid-template-columns: 1fr;
  }
}
</style>
