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
  { label: '快速 - 最近 100 条', value: 'Quick' },
  { label: '普通 - 最近 1000 条', value: 'Normal' },
  { label: '深度 - 最近 10000 条', value: 'Deep' },
  { label: '完整 - 全部历史', value: 'Full' }
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
    message.success('设置完成')
    await router.push('/')
  } catch (error) {
    message.error(error instanceof Error ? error.message : '无法保存频道选择')
  }
}
</script>

<template>
  <main class="setup-page">
    <section class="setup-panel">
      <div class="page-header">
        <div>
          <p class="eyebrow">首次运行设置</p>
          <h1>选择频道</h1>
        </div>
        <n-button :loading="channels.loading" @click="channels.loadChannels">刷新</n-button>
      </div>

      <div class="table-panel">
        <table>
          <thead>
            <tr>
              <th>监听</th>
              <th>标题</th>
              <th>用户名</th>
              <th>成员数</th>
              <th>描述</th>
              <th>历史同步</th>
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
              <td colspan="6" class="empty-cell">未找到频道</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="actions">
        <n-button type="primary" :loading="channels.loading || setup.loading" @click="finish">
          保存并开始
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
