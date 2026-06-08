<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useResourcesStore } from '@/stores/resources'
import { useStatusStore } from '@/stores/status'
import { useTasksStore } from '@/stores/tasks'

const status = useStatusStore()
const resources = useResourcesStore()
const tasks = useTasksStore()

onMounted(() => {
  void status.load()
  void resources.loadGrouped()
  void resources.loadLinkTypesGrouped()
  void tasks.loadTasks()
})

const cards = computed(() => [
  { label: '账号', value: status.service?.accounts ?? 0 },
  { label: '频道', value: status.service?.channels ?? 0 },
  { label: '消息', value: status.service?.messages ?? 0 },
  { label: '链接', value: status.service?.links ?? 0 },
  { label: '任务', value: tasks.total }
])

const resourceTypes = computed(() => [
  { label: '网盘', value: resources.grouped.cloud_drive ?? 0 },
  { label: '磁力', value: resources.grouped.magnet ?? 0 },
  { label: 'ED2K', value: resources.grouped.ed2k ?? 0 },
  { label: 'HTTP', value: resources.grouped.http ?? 0 },
  { label: '文件', value: resources.grouped.files ?? 0 }
])

const linkTypes = computed(() =>
  Object.entries(resources.linkTypesGrouped)
    .map(([type, value]) => ({ label: linkTypeLabel(type), type, value }))
    .sort((a, b) => b.value - a.value || a.label.localeCompare(b.label))
)

const failedTasks = computed(() =>
  tasks.items.filter((task) => task.status === 'failed').slice(0, 4)
)

function formatBytes(value = 0) {
  if (value >= 1_000_000_000) return `${(value / 1_000_000_000).toFixed(1)} GB`
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1)} MB`
  if (value >= 1_000) return `${(value / 1_000).toFixed(1)} KB`
  return `${value} B`
}

function taskTypeLabel(type: string) {
  const labels: Record<string, string> = {
    history_sync: '历史同步',
    web_access_detection: '网页访问检测',
    metadata_sync: '元数据同步',
    cleanup: '清理'
  }
  return labels[type] ?? type
}

function linkTypeLabel(type: string) {
  const labels: Record<string, string> = {
    '115': '115',
    '123': '123',
    aliyun: '阿里云盘',
    baidu: '百度网盘',
    ed2k: 'ED2K',
    guangya: '光亚盘',
    magnet: '磁力',
    mobile: '移动云盘',
    pikpak: 'PikPak',
    quark: '夸克',
    tianyi: '天翼云盘',
    uc: 'UC',
    url: '普通链接',
    xunlei: '迅雷云盘'
  }
  return labels[type] ?? type
}
</script>

<template>
  <section class="page-section">
    <div class="page-header">
      <div>
        <p class="page-kicker">概览</p>
        <h1 class="page-title">本地 Telegram 索引</h1>
      </div>
    </div>

    <div class="metric-grid">
      <div v-for="card in cards" :key="card.label" class="metric-card">
        <span>{{ card.label }}</span>
        <strong>{{ card.value }}</strong>
      </div>
    </div>

    <div class="dashboard-grid">
      <section class="panel">
        <h2>存储使用</h2>
        <dl>
          <div>
            <dt>数据库</dt>
            <dd>{{ formatBytes(status.storage?.db_bytes) }}</dd>
          </div>
          <div>
            <dt>索引</dt>
            <dd>{{ formatBytes(status.storage?.index_bytes) }}</dd>
          </div>
          <div>
            <dt>媒体缓存</dt>
            <dd>{{ formatBytes(status.storage?.media_cache_bytes) }}</dd>
          </div>
          <div>
            <dt>总计</dt>
            <dd>{{ formatBytes(status.storage?.total_bytes) }}</dd>
          </div>
        </dl>
      </section>

      <section class="panel">
        <h2>资源类型统计</h2>
        <div class="resource-types">
          <span v-for="item in resourceTypes" :key="item.label">
            {{ item.label }}
            <strong>{{ item.value }}</strong>
          </span>
        </div>
      </section>

      <section class="panel">
        <h2>链接类型统计</h2>
        <div v-if="linkTypes.length === 0" class="muted">暂无链接类型统计</div>
        <div v-else class="resource-types">
          <span v-for="item in linkTypes" :key="item.type">
            {{ item.label }}
            <strong>{{ item.value }}</strong>
          </span>
        </div>
      </section>

      <section class="panel">
        <h2>最近任务错误</h2>
        <div v-if="failedTasks.length === 0" class="muted">暂无最近任务错误</div>
        <ul v-else class="task-errors">
          <li v-for="task in failedTasks" :key="task.id">
            <span>{{ taskTypeLabel(task.type) }}</span>
            <strong>{{ task.error_message || task.message || task.status }}</strong>
          </li>
        </ul>
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

.metric-grid {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(5, minmax(0, 1fr));
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

.muted {
  color: #667085;
}

.task-errors {
  display: grid;
  gap: 8px;
  list-style: none;
  margin: 0;
  padding: 0;
}

.task-errors li {
  border: 1px solid #edf0f5;
  border-radius: 6px;
  padding: 8px;
}

.task-errors span {
  color: #667085;
  display: block;
  font-size: 13px;
}

.task-errors strong {
  display: block;
  margin-top: 3px;
  overflow-wrap: anywhere;
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
