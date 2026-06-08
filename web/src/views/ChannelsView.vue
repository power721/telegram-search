<script setup lang="ts">
import { onMounted } from 'vue'
import type { TelegramChannel } from '@/api/types'
import WebAccessBadge from '@/components/channels/WebAccessBadge.vue'
import { useChannelsStore } from '@/stores/channels'

const channels = useChannelsStore()

onMounted(() => {
  void channels.loadChannels()
})

function username(channel: TelegramChannel) {
  return channel.username ? `@${channel.username}` : '-'
}

function syncProfileLabel(profile: string) {
  const labels: Record<string, string> = {
    Quick: '快速',
    Normal: '普通',
    Deep: '深度',
    Full: '完整'
  }
  return labels[profile] ?? profile
}

function channelTypeLabel(type: string) {
  const labels: Record<string, string> = {
    channel: '频道类',
    group: '群组',
    supergroup: '超级群组'
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

function webAccessText(channel: TelegramChannel) {
  if (channel.web_access_error) return '检测失败'
  if (channel.web_access === true) return '可访问'
  if (channel.web_access === false) return '不可访问'
  return '未检测'
}

async function syncHistory(channel: TelegramChannel) {
  await channels.syncChannels([channel.id])
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

    <div class="profile-legend">
      <span>快速</span>
      <span>普通</span>
      <span>深度</span>
      <span>完整</span>
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
            <th>同步档位</th>
            <th>网页访问</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="channel in channels.items" :key="channel.id">
            <td>{{ channel.title }}</td>
            <td>{{ username(channel) }}</td>
            <td>{{ channelTypeLabel(channel.type) }}</td>
            <td>{{ syncStateLabel(channel.sync_state) }}</td>
            <td>{{ listenStateLabel(channel.listen_state) }}</td>
            <td>{{ syncProfileLabel(channel.sync_profile) }}</td>
            <td :title="channel.web_access_error || undefined">
              <WebAccessBadge :value="channel.web_access" :error="channel.web_access_error" />
              <span class="web-access-text">{{ webAccessText(channel) }}</span>
            </td>
            <td class="actions">
              <n-button size="small" :loading="channels.loading" @click="syncHistory(channel)">同步</n-button>
              <n-button size="small" :loading="channels.loading" @click="enableListening(channel)">监听</n-button>
            </td>
          </tr>
          <tr v-if="channels.items.length === 0">
            <td colspan="8" class="empty-cell">暂无频道</td>
          </tr>
        </tbody>
      </table>
    </div>
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

.profile-legend {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 12px;
}

.profile-legend span {
  border: 1px solid #d9dee7;
  border-radius: 6px;
  color: #354052;
  padding: 5px 8px;
}

.table-panel {
  background: #ffffff;
  border: 1px solid #d9dee7;
  border-radius: 8px;
  overflow-x: auto;
}

table {
  border-collapse: collapse;
  min-width: 980px;
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

.web-access-text {
  margin-left: 6px;
}

.empty-cell {
  color: #667085;
  text-align: center;
}
</style>
