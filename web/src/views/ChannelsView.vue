<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import type { TelegramChannel } from '@/api/types'
import WebAccessBadge from '@/components/channels/WebAccessBadge.vue'
import { useChannelsStore } from '@/stores/channels'

const channels = useChannelsStore()
const showSyncModal = ref(false)
const syncTarget = ref<TelegramChannel | null>(null)
const syncMaxMessages = ref<number | null>(1000)
const searchQuery = ref('')
const typeFilter = ref('')
const syncStateFilter = ref('')
const listenStateFilter = ref('')
const webAccessFilter = ref('')
const sortKey = ref('title_asc')
const canConfirmSync = computed(() => Number.isInteger(syncMaxMessages.value) && Number(syncMaxMessages.value) > 0)

const typeOptions = [
  { label: '全部类型', value: '' },
  { label: '频道', value: 'channel' },
  { label: '群组', value: 'group' },
  { label: '超级群组', value: 'supergroup' },
  { label: '保存的消息', value: 'saved_messages' }
]

const syncStateOptions = [
  { label: '全部同步状态', value: '' },
  { label: '仅元数据', value: 'metadata_only' },
  { label: '待同步', value: 'pending' },
  { label: '同步中', value: 'syncing' },
  { label: '已同步', value: 'synced' },
  { label: '同步失败', value: 'failed' },
  { label: '未启用', value: 'disabled' }
]

const listenStateOptions = [
  { label: '全部监听状态', value: '' },
  { label: '已启用', value: 'enabled' },
  { label: '未启用', value: 'disabled' },
  { label: '监听异常', value: 'error' }
]

const webAccessOptions = [
  { label: '全部网页访问', value: '' },
  { label: '可访问', value: 'accessible' },
  { label: '不可访问', value: 'inaccessible' },
  { label: '未检测', value: 'unknown' },
  { label: '检测失败', value: 'error' }
]

const sortOptions = [
  { label: '标题 A-Z', value: 'title_asc' },
  { label: '标题 Z-A', value: 'title_desc' },
  { label: '用户名 A-Z', value: 'username_asc' },
  { label: '已索引消息多到少', value: 'indexed_desc' },
  { label: '已索引消息少到多', value: 'indexed_asc' }
]

const filteredChannels = computed(() => {
  const query = searchQuery.value.trim().toLowerCase()
  return channels.items
    .filter((channel) => {
      if (query) {
        const haystack = `${channel.title} ${channel.username}`.toLowerCase()
        if (!haystack.includes(query)) return false
      }
      if (typeFilter.value && channel.type !== typeFilter.value) return false
      if (syncStateFilter.value && channel.sync_state !== syncStateFilter.value) return false
      if (listenStateFilter.value && channel.listen_state !== listenStateFilter.value) return false
      if (webAccessFilter.value && webAccessState(channel) !== webAccessFilter.value) return false
      return true
    })
    .sort(compareChannels)
})
const visibleWebCheckChannelIds = computed(() =>
  filteredChannels.value
    .filter((channel) => canCheckWebAccess(channel))
    .map((channel) => channel.id)
    .sort((left, right) => left - right)
)

onMounted(() => {
  void channels.loadChannels()
})

function username(channel: TelegramChannel) {
  return channel.username ? `@${channel.username}` : '-'
}

function channelTypeLabel(type: string) {
  const labels: Record<string, string> = {
    channel: '频道',
    group: '群组',
    supergroup: '超级群组',
    saved_messages: '保存的消息'
  }
  return labels[type] ?? type
}

function syncStateLabel(state: string) {
  const labels: Record<string, string> = {
    metadata_only: '仅元数据',
    pending: '待同步',
    syncing: '同步中',
    synced: '已同步',
    failed: '同步失败',
    disabled: '未启用'
  }
  return labels[state] ?? state
}

function listenStateLabel(state: string) {
  const labels: Record<string, string> = {
    enabled: '已启用',
    disabled: '未启用',
    error: '监听异常'
  }
  return labels[state] ?? state
}

function webAccessState(channel: TelegramChannel) {
  if (channel.web_access_error) return 'error'
  if (channel.web_access === true) return 'accessible'
  if (channel.web_access === false) return 'inaccessible'
  return 'unknown'
}

function webAccessUrl(channel: TelegramChannel) {
  if (channel.web_access !== true || !channel.username) return ''
  return `https://t.me/s/${encodeURIComponent(channel.username)}`
}

function canCheckWebAccess(channel: TelegramChannel) {
  return channel.type !== 'saved_messages' && Boolean(channel.username)
}

function compareChannels(left: TelegramChannel, right: TelegramChannel) {
  switch (sortKey.value) {
    case 'title_desc':
      return compareText(right.title, left.title)
    case 'username_asc':
      return compareText(left.username, right.username)
    case 'indexed_desc':
      return right.indexed_message_count - left.indexed_message_count || compareText(left.title, right.title)
    case 'indexed_asc':
      return left.indexed_message_count - right.indexed_message_count || compareText(left.title, right.title)
    case 'title_asc':
    default:
      return compareText(left.title, right.title)
  }
}

function compareText(left: string, right: string) {
  return left.localeCompare(right, 'zh-Hans-CN', { numeric: true, sensitivity: 'base' })
}

function syncHistory(channel: TelegramChannel) {
  syncTarget.value = channel
  syncMaxMessages.value = 1000
  showSyncModal.value = true
}

function closeSyncModal() {
  showSyncModal.value = false
  syncTarget.value = null
}

async function confirmSyncHistory() {
  if (!syncTarget.value || !canConfirmSync.value || syncMaxMessages.value === null) return
  await channels.syncChannels([syncTarget.value.id], syncMaxMessages.value)
  closeSyncModal()
}

async function checkWebAccess(channel: TelegramChannel) {
  if (!canCheckWebAccess(channel)) return
  await channels.checkWebAccess([channel.id])
}

async function batchCheckWebAccess() {
  if (visibleWebCheckChannelIds.value.length === 0) return
  await channels.checkWebAccess(visibleWebCheckChannelIds.value)
}

async function enableListening(channel: TelegramChannel) {
  await channels.updateControl(channel.id, {
    history_sync_enabled: channel.history_sync_enabled,
    sync_profile: channel.sync_profile,
    listen_enabled: true,
    remote_search_allowed: channel.remote_search_allowed
  })
}
</script>

<template>
  <section class="page-section">
    <div class="page-header">
      <div>
        <p class="page-kicker">Telegram</p>
        <h1 class="page-title">频道</h1>
      </div>
      <n-button :loading="channels.loading" @click="channels.loadChannels">刷新</n-button>
    </div>

    <div class="channel-toolbar">
      <n-input v-model:value="searchQuery" class="channel-search" clearable placeholder="搜索标题或用户名" />
      <n-select v-model:value="typeFilter" class="type-filter" :options="typeOptions" />
      <n-select v-model:value="syncStateFilter" class="sync-state-filter" :options="syncStateOptions" />
      <n-select v-model:value="listenStateFilter" class="listen-state-filter" :options="listenStateOptions" />
      <n-select v-model:value="webAccessFilter" class="web-access-filter" :options="webAccessOptions" />
      <n-select v-model:value="sortKey" class="sort-select" :options="sortOptions" />
      <n-button
        class="batch-web-access-check"
        :disabled="visibleWebCheckChannelIds.length === 0"
        :loading="channels.loading"
        @click="batchCheckWebAccess"
      >
        批量检测
      </n-button>
    </div>

    <div class="table-panel">
      <table>
        <thead>
          <tr>
            <th>标题</th>
            <th>用户名</th>
            <th>类型</th>
            <th>同步状态</th>
            <th>监听状态</th>
            <th>已索引消息</th>
            <th>网页访问</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="channel in filteredChannels" :key="channel.id">
            <td>{{ channel.title }}</td>
            <td>{{ username(channel) }}</td>
            <td>{{ channelTypeLabel(channel.type) }}</td>
            <td>{{ syncStateLabel(channel.sync_state) }}</td>
            <td>{{ listenStateLabel(channel.listen_state) }}</td>
            <td>{{ channel.indexed_message_count }}</td>
            <td :title="channel.web_access_error || undefined">
              <a
                v-if="webAccessUrl(channel)"
                class="web-access-link"
                :href="webAccessUrl(channel)"
                target="_blank"
                rel="noreferrer"
              >
                <WebAccessBadge :value="channel.web_access" :error="channel.web_access_error" />
              </a>
              <WebAccessBadge v-else :value="channel.web_access" :error="channel.web_access_error" />
            </td>
            <td class="actions">
              <n-button size="small" :loading="channels.loading" @click="syncHistory(channel)">同步</n-button>
              <n-button
                size="small"
                :disabled="!canCheckWebAccess(channel)"
                :loading="channels.loading"
                @click="checkWebAccess(channel)"
              >
                检测
              </n-button>
              <n-button size="small" :loading="channels.loading" @click="enableListening(channel)">监听</n-button>
            </td>
          </tr>
          <tr v-if="filteredChannels.length === 0">
            <td colspan="8" class="empty-cell">暂无频道</td>
          </tr>
        </tbody>
      </table>
    </div>

    <n-modal v-model:show="showSyncModal">
      <n-card class="sync-modal" :bordered="false">
        <h2 class="sync-modal-title">同步记录最大条数</h2>
        <n-input-number v-model:value="syncMaxMessages" :min="1" :precision="0" />
        <div class="sync-modal-actions">
          <n-button @click="closeSyncModal">取消</n-button>
          <n-button type="primary" :disabled="!canConfirmSync" :loading="channels.loading" @click="confirmSyncHistory">
            开始同步
          </n-button>
        </div>
      </n-card>
    </n-modal>
  </section>
</template>

<style scoped>
.page-header {
  align-items: center;
  display: flex;
  gap: 16px;
  justify-content: space-between;
  margin-bottom: 14px;
}

.page-kicker {
  color: #667085;
  margin: 0 0 4px;
}

.page-title {
  font-size: 24px;
  margin: 0;
}

.table-panel {
  background: #ffffff;
  border: 1px solid #d9dee7;
  border-radius: 8px;
  overflow-x: auto;
}

.channel-toolbar {
  display: grid;
  gap: 10px;
  grid-template-columns: minmax(220px, 1.4fr) repeat(5, minmax(130px, 1fr)) auto;
  margin-bottom: 12px;
}

table {
  border-collapse: collapse;
  min-width: 940px;
  width: 100%;
}

th,
td {
  border-bottom: 1px solid #edf0f5;
  padding: 10px 12px;
  text-align: left;
  vertical-align: top;
}

th {
  color: #667085;
  font-size: 13px;
  font-weight: 600;
}

.actions {
  align-items: center;
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  min-width: 120px;
}

.web-access-link {
  color: inherit;
  display: inline-flex;
  text-decoration: none;
}

.sync-modal {
  max-width: 420px;
  width: calc(100vw - 32px);
}

.sync-modal-title {
  font-size: 18px;
  font-weight: 600;
  margin: 0 0 14px;
}

.sync-modal-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
  margin-top: 16px;
}

.empty-cell {
  color: #667085;
  text-align: center;
}

@media (max-width: 1120px) {
  .channel-toolbar {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 640px) {
  .channel-toolbar {
    grid-template-columns: 1fr;
  }
}
</style>
