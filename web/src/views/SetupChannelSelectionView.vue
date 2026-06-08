<script setup lang="ts">
import { useMessage } from 'naive-ui'
import { onMounted, reactive } from 'vue'
import { useRouter } from 'vue-router'
import type { SyncProfile, TelegramChannel } from '@/api/types'
import { useChannelsStore } from '@/stores/channels'
import { useSetupStore } from '@/stores/setup'

const router = useRouter()
const message = useMessage()
const channels = useChannelsStore()
const setup = useSetupStore()

const selected = reactive<Record<number, boolean>>({})
const profiles = reactive<Record<number, SyncProfile>>({})

const profileOptions = [
  { label: 'Quick - latest 100', value: 'Quick' },
  { label: 'Normal - latest 1000', value: 'Normal' },
  { label: 'Deep - latest 10000', value: 'Deep' },
  { label: 'Full - all history', value: 'Full' }
]

onMounted(async () => {
  await channels.loadChannels()
  for (const channel of channels.items) {
    selected[channel.id] = channel.history_sync_enabled || channel.listen_enabled
    profiles[channel.id] = channel.sync_profile || 'Normal'
  }
  const ids = channels.items.map((channel) => channel.id)
  if (ids.length > 0) {
    void channels.checkWebAccess(ids)
  }
})

function username(channel: TelegramChannel) {
  return channel.username ? `@${channel.username}` : '-'
}

async function finish() {
  const selectedIds = channels.items.filter((channel) => selected[channel.id]).map((channel) => channel.id)
  try {
    for (const channelID of selectedIds) {
      await channels.updateControl(channelID, {
        history_sync_enabled: true,
        sync_profile: profiles[channelID] || 'Normal',
        listen_enabled: true,
        remote_search_allowed: true
      })
      await channels.createWatchRule({
        channel_id: channelID,
        enabled: true,
        ...setup.listenRules
      })
    }
    if (selectedIds.length > 0) {
      await channels.syncChannels(selectedIds)
    }
    await setup.completeSetup()
    message.success('Setup complete')
    await router.push('/')
  } catch (error) {
    message.error(error instanceof Error ? error.message : 'Could not save channel selection')
  }
}
</script>

<template>
  <main class="setup-page">
    <section class="setup-panel">
      <div class="page-header">
        <div>
          <p class="eyebrow">First Run Setup</p>
          <h1>Select Channels</h1>
        </div>
        <n-button :loading="channels.loading" @click="channels.loadChannels">Refresh</n-button>
      </div>

      <div class="table-panel">
        <table>
          <thead>
            <tr>
              <th>Listen</th>
              <th>Title</th>
              <th>Username</th>
              <th>Members</th>
              <th>Description</th>
              <th>History</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="channel in channels.items" :key="channel.id">
              <td>
                <n-checkbox v-model:checked="selected[channel.id]" />
              </td>
              <td>{{ channel.title }}</td>
              <td>{{ username(channel) }}</td>
              <td>{{ channel.member_count }}</td>
              <td>{{ channel.description }}</td>
              <td>
                <n-select
                  v-model:value="profiles[channel.id]"
                  :options="profileOptions"
                  :disabled="!selected[channel.id]"
                />
              </td>
            </tr>
            <tr v-if="channels.items.length === 0">
              <td colspan="6" class="empty-cell">No channels found</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="actions">
        <n-button type="primary" :loading="channels.loading || setup.loading" @click="finish">
          Save and Start
        </n-button>
      </div>
    </section>
  </main>
</template>

<style scoped>
.setup-page {
  min-height: 100vh;
  padding: 24px;
}

.setup-panel {
  margin: 0 auto;
  max-width: 1120px;
}

.page-header {
  align-items: center;
  display: flex;
  justify-content: space-between;
  margin-bottom: 16px;
}

.eyebrow {
  color: #667085;
  font-size: 13px;
  margin: 0 0 8px;
}

h1 {
  font-size: 24px;
  margin: 0;
}

.table-panel {
  background: #ffffff;
  border: 1px solid #d9dee7;
  border-radius: 8px;
  overflow-x: auto;
}

table {
  border-collapse: collapse;
  min-width: 900px;
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

.empty-cell {
  color: #667085;
  text-align: center;
}

.actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}
</style>
