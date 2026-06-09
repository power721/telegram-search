<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import AppPagination from '@/components/common/AppPagination.vue'
import type { LogEntry } from '@/api/types'
import { useLogsStore } from '@/stores/logs'

const logs = useLogsStore()
const file = ref('')
const level = ref('')
const query = ref('')
const order = ref<'asc' | 'desc'>('desc')
const pageSizeOptions = [100, 200, 500, 1000]
const pageSize = ref(200)
const offset = ref(0)

const page = computed(() => Math.floor(offset.value / pageSize.value) + 1)
const fileOptions = computed(() => [
  { label: '全部日志', value: '' },
  ...logs.files.map((item) => ({
    label: `${logFileLabel(item.name)}${item.size ? ` (${formatBytes(item.size)})` : ''}`,
    value: item.name
  }))
])
const levelOptions = [
  { label: '全部级别', value: '' },
  { label: 'Debug', value: 'debug' },
  { label: 'Info', value: 'info' },
  { label: 'Warn', value: 'warn' },
  { label: 'Error', value: 'error' }
]
const orderOptions = [
  { label: '最新在前', value: 'desc' },
  { label: '最早在前', value: 'asc' }
]
const selectedFileLabel = computed(() => (file.value ? logFileLabel(file.value) : '全部日志'))

function load() {
  return logs.load({
    file: file.value,
    level: level.value,
    query: query.value.trim(),
    order: order.value,
    limit: pageSize.value,
    offset: offset.value
  })
}

async function resetAndLoad() {
  offset.value = 0
  await load()
}

function selectFile(value: string) {
  file.value = value
  void resetAndLoad()
}

function selectLevel(value: string) {
  level.value = value
  void resetAndLoad()
}

function selectOrder(value: 'asc' | 'desc') {
  order.value = value
  void resetAndLoad()
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

async function downloadSelectedLog() {
  if (!file.value) {
    logs.error = '请选择一个具体日志文件后再下载'
    return
  }
  const blob = await logs.download(file.value)
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = file.value
  link.click()
  URL.revokeObjectURL(url)
}

function logFileLabel(name: string) {
  const labels: Record<string, string> = {
    'app.log': '应用日志',
    'sync.log': '同步日志',
    'telegram.log': 'Telegram 日志',
    'error.log': '错误日志'
  }
  return labels[name] ?? name
}

function formatBytes(bytes: number) {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let value = bytes
  let unit = 0
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024
    unit += 1
  }
  return `${value >= 10 || unit === 0 ? value.toFixed(0) : value.toFixed(1)} ${units[unit]}`
}

function formatTime(value?: string) {
  if (!value) return '-'
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false
  }).format(new Date(value))
}

function levelClass(entry: LogEntry) {
  return `level-${(entry.level || 'raw').toLowerCase()}`
}

function fieldsText(entry: LogEntry) {
  return entry.fields ? JSON.stringify(entry.fields) : ''
}

onMounted(() => {
  void load()
})
</script>

<template>
  <section class="page-section">
    <div class="page-header">
      <div>
        <p class="page-kicker">运行诊断</p>
        <h1 class="page-title">日志</h1>
        <p class="page-subtitle">查看、搜索、筛选和下载运行日志。</p>
      </div>
      <div class="header-actions">
        <n-button :loading="logs.loading" @click="load">刷新</n-button>
        <n-button :loading="logs.downloading" :disabled="!file" type="primary" @click="downloadSelectedLog">
          下载日志
        </n-button>
      </div>
    </div>

    <form class="log-filters" @submit.prevent="resetAndLoad">
      <label>
        <span class="filter-label">日志文件</span>
        <n-select :value="file" :options="fileOptions" @update:value="selectFile" />
      </label>
      <label>
        <span class="filter-label">级别</span>
        <n-select :value="level" :options="levelOptions" @update:value="selectLevel" />
      </label>
      <label>
        <span class="filter-label">关键词</span>
        <n-input v-model:value="query" clearable placeholder="搜索消息、字段或原始日志" />
      </label>
      <label>
        <span class="filter-label">顺序</span>
        <n-select :value="order" :options="orderOptions" @update:value="selectOrder" />
      </label>
      <n-button attr-type="submit" type="primary">搜索</n-button>
    </form>

    <div class="log-summary">
      <span>{{ selectedFileLabel }}</span>
      <strong>{{ logs.total }}</strong>
      <span>条匹配日志</span>
    </div>

    <div v-if="logs.error" class="error-strip">{{ logs.error }}</div>

    <div class="table-panel log-table">
      <table class="data-table">
        <thead>
          <tr>
            <th class="time-col">时间</th>
            <th>文件</th>
            <th>级别</th>
            <th>消息</th>
            <th>调用方</th>
            <th>字段</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="logs.loading">
            <td colspan="6" class="empty-cell">正在加载日志</td>
          </tr>
          <tr v-else-if="logs.items.length === 0">
            <td colspan="6" class="empty-cell">没有匹配的日志</td>
          </tr>
          <template v-else>
            <tr v-for="entry in logs.items" :key="`${entry.file}-${entry.time ?? ''}-${entry.raw}`">
              <td class="time-col">{{ formatTime(entry.time) }}</td>
              <td>{{ logFileLabel(entry.file) }}</td>
              <td>
                <span class="level-pill" :class="levelClass(entry)">{{ entry.level || 'raw' }}</span>
              </td>
              <td>
                <div class="message-cell">{{ entry.message || entry.raw }}</div>
                <code class="raw-line">{{ entry.raw }}</code>
              </td>
              <td class="caller-cell">{{ entry.caller || '-' }}</td>
              <td class="fields-cell">{{ fieldsText(entry) || '-' }}</td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>

    <AppPagination
      :loading="logs.loading"
      :page="page"
      :page-size="pageSize"
      :page-size-options="pageSizeOptions"
      :total="logs.total"
      @update:page="changePage"
      @update:page-size="changePageSize"
    />
  </section>
</template>

<style scoped>
.log-filters {
  align-items: end;
  background: var(--app-surface);
  border: 1px solid var(--app-border);
  border-radius: var(--app-radius);
  display: grid;
  gap: 10px;
  grid-template-columns: minmax(150px, 1fr) minmax(120px, 0.7fr) minmax(220px, 1.8fr) minmax(130px, 0.8fr) auto;
  padding: 10px;
}

.log-filters label {
  display: grid;
  gap: 6px;
  min-width: 0;
}

.log-summary {
  align-items: center;
  color: var(--app-text-muted);
  display: flex;
  gap: 6px;
}

.log-summary strong {
  color: var(--app-heading);
}

.log-table {
  max-height: calc(100vh - 300px);
}

.time-col {
  min-width: 126px;
  white-space: nowrap;
}

.level-pill {
  border: 1px solid var(--app-border);
  border-radius: 999px;
  color: var(--app-text-muted);
  display: inline-flex;
  font-size: 12px;
  font-weight: 650;
  line-height: 20px;
  padding: 0 8px;
  text-transform: uppercase;
}

.level-info {
  background: var(--app-accent-subtle);
  color: var(--app-accent);
}

.level-warn {
  background: var(--app-warning-bg);
  color: var(--app-warning);
}

.level-error,
.level-dpanic,
.level-panic,
.level-fatal {
  background: var(--app-danger-bg);
  color: var(--app-danger);
}

.message-cell {
  color: var(--app-heading);
  font-weight: 600;
  max-width: 520px;
  overflow-wrap: anywhere;
}

.raw-line {
  color: var(--app-text-muted);
  display: block;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  margin-top: 4px;
  max-width: 620px;
  overflow-wrap: anywhere;
  white-space: normal;
}

.caller-cell,
.fields-cell {
  color: var(--app-text-muted);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  max-width: 260px;
  overflow-wrap: anywhere;
}

.empty-cell {
  color: var(--app-text-muted);
  padding: 28px 10px;
  text-align: center;
}

@media (max-width: 1100px) {
  .log-filters {
    grid-template-columns: 1fr 1fr;
  }
}

@media (max-width: 720px) {
  .log-filters {
    grid-template-columns: 1fr;
  }
}
</style>
