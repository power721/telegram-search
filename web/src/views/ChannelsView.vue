<script setup lang="ts">
import { onMounted, ref } from 'vue'
import type { ChannelControlPayload, TelegramChannel } from '@/api/types'
import ChannelControlDrawer from '@/components/channels/ChannelControlDrawer.vue'
import WebAccessBadge from '@/components/channels/WebAccessBadge.vue'
import { useChannelsStore } from '@/stores/channels'

const channels = useChannelsStore()
const drawerOpen = ref(false)
const remoteQuery = ref('')

onMounted(() => {
  void channels.loadChannels()
})

function username(channel: TelegramChannel) {
  return channel.username ? `@${channel.username}` : '-'
}

function edit(channel: TelegramChannel) {
  channels.selected = channel
  drawerOpen.value = true
}

async function saveControl(payload: ChannelControlPayload) {
  if (!channels.selected) return
  await channels.updateControl(channels.selected.id, payload)
  drawerOpen.value = false
}

async function remoteSearch(channel: TelegramChannel) {
  if (!remoteQuery.value.trim()) return
  await channels.createRemoteSearch(channel.id, remoteQuery.value)
  remoteQuery.value = ''
}
</script>

<template>
  <section class="page-section">
    <div class="page-header">
      <div>
        <p class="page-kicker">Telegram</p>
        <h1 class="page-title">Channels</h1>
      </div>
      <n-button :loading="channels.loading" @click="channels.loadChannels">Refresh</n-button>
    </div>

    <div class="profile-legend">
      <span>Quick</span>
      <span>Normal</span>
      <span>Deep</span>
      <span>Full</span>
    </div>

    <div class="table-panel">
      <table>
        <thead>
          <tr>
            <th>Title</th>
            <th>Username</th>
            <th>Type</th>
            <th>Sync State</th>
            <th>Listen State</th>
            <th>Profile</th>
            <th>Web Access</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="channel in channels.items" :key="channel.id">
            <td>{{ channel.title }}</td>
            <td>{{ username(channel) }}</td>
            <td>{{ channel.type }}</td>
            <td>{{ channel.sync_state }}</td>
            <td>{{ channel.listen_state }}</td>
            <td>{{ channel.sync_profile }}</td>
            <td><WebAccessBadge :value="channel.web_access" :error="channel.web_access_error" /></td>
            <td class="actions">
              <n-button size="small" @click="channels.analyzeChannel(channel.id)">Analyze</n-button>
              <n-button size="small" @click="channels.checkWebAccess([channel.id])">Check Web Access</n-button>
              <n-button size="small" @click="edit(channel)">Edit Controls</n-button>
              <n-input v-model:value="remoteQuery" class="remote-input" size="small" placeholder="Remote search" />
              <n-button size="small" @click="remoteSearch(channel)">Remote Search</n-button>
            </td>
          </tr>
          <tr v-if="channels.items.length === 0">
            <td colspan="8" class="empty-cell">No channels</td>
          </tr>
        </tbody>
      </table>
    </div>

    <ChannelControlDrawer
      v-model:show="drawerOpen"
      :channel="channels.selected"
      :loading="channels.loading"
      @save="saveControl"
    />
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
  min-width: 360px;
}

.remote-input {
  max-width: 150px;
}

.empty-cell {
  color: #667085;
  text-align: center;
}
</style>
