<script setup lang="ts">
import { useMessage } from 'naive-ui'
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import type { TelegramChannel } from '@/api/types'
import { useChannelsStore } from '@/stores/channels'
import { useSetupStore } from '@/stores/setup'

const router = useRouter()
const message = useMessage()
const channels = useChannelsStore()
const setup = useSetupStore()

const selected = reactive<Record<number, boolean>>({})
const page = ref(1)
const pageSize = 50

const visibleChannels = computed(() => {
  const start = (page.value - 1) * pageSize
  return channels.items.slice(start, start + pageSize)
})

const totalPages = computed(() => Math.max(1, Math.ceil(channels.items.length / pageSize)))
const selectedCount = computed(() => channels.items.filter((channel) => selected[channel.id]).length)

onMounted(async () => {
  await loadChannels()
})

async function loadChannels() {
  await channels.loadChannels()
  for (const channel of channels.items) {
    selected[channel.id] = channel.history_sync_enabled || channel.listen_enabled
  }
  if (page.value > totalPages.value) {
    page.value = totalPages.value
  }
}

function username(channel: TelegramChannel) {
  return channel.username ? `@${channel.username}` : '-'
}

function channelStatus(channel: TelegramChannel) {
  if (channel.web_access_error) return '封禁/不可用'
  if (channel.web_access === true) return '网页可访问'
  if (channel.username) return '公开'
  return '私有'
}

function channelStatusClass(channel: TelegramChannel) {
  if (channel.web_access_error) return 'status-bad'
  if (channel.web_access === true) return 'status-good'
  if (channel.username) return 'status-info'
  return 'status-muted'
}

function previousPage() {
  page.value = Math.max(1, page.value - 1)
}

function nextPage() {
  page.value = Math.min(totalPages.value, page.value + 1)
}

async function finish() {
  const selectedIds = channels.items.filter((channel) => selected[channel.id]).map((channel) => channel.id)
  try {
    if (selectedIds.length > 0) {
      await channels.updateControls(selectedIds, {
        history_sync_enabled: true,
        sync_profile: 'Normal',
        listen_enabled: true,
        remote_search_allowed: true
      })
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
        <n-button :loading="channels.loading" @click="loadChannels">刷新</n-button>
      </div>
      <div class="summary">
        <span>{{ channels.items.length }} 个频道</span>
        <span>已选择 {{ selectedCount }} 个</span>
        <span>第 {{ page }} / {{ totalPages }} 页</span>
      </div>

      <div class="table-panel">
        <table>
          <thead>
            <tr>
              <th>监听</th>
              <th>标题</th>
              <th>用户名</th>
              <th>状态</th>
              <th>成员数</th>
              <th>描述</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="channel in visibleChannels" :key="channel.id">
              <td>
                <n-checkbox v-model:checked="selected[channel.id]" />
              </td>
              <td class="title-cell">{{ channel.title }}</td>
              <td>{{ username(channel) }}</td>
              <td>
                <span class="status-badge" :class="channelStatusClass(channel)">
                  {{ channelStatus(channel) }}
                </span>
              </td>
              <td>{{ channel.member_count }}</td>
              <td class="description-cell" :title="channel.description">{{ channel.description || '-' }}</td>
            </tr>
            <tr v-if="channels.items.length === 0">
              <td colspan="6" class="empty-cell">未找到频道</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="pagination">
        <n-button :disabled="page <= 1" @click="previousPage">上一页</n-button>
        <n-button :disabled="page >= totalPages" @click="nextPage">下一页</n-button>
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
  max-width: 1440px;
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

.summary {
  color: #475467;
  display: flex;
  flex-wrap: wrap;
  font-size: 13px;
  gap: 16px;
  margin-bottom: 12px;
}

.table-panel {
  background: #ffffff;
  border: 1px solid #d9dee7;
  border-radius: 8px;
  overflow: hidden;
}

table {
  border-collapse: collapse;
  table-layout: fixed;
  width: 100%;
}

th:nth-child(1),
td:nth-child(1) {
  width: 56px;
}

th:nth-child(2),
td:nth-child(2) {
  width: 28%;
}

th:nth-child(3),
td:nth-child(3) {
  width: 160px;
}

th:nth-child(4),
td:nth-child(4) {
  width: 120px;
}

th:nth-child(5),
td:nth-child(5) {
  width: 100px;
}

th,
td {
  border-bottom: 1px solid #edf0f5;
  padding: 10px 12px;
  text-align: left;
  vertical-align: top;
}

.title-cell,
.description-cell {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

th {
  color: #667085;
  font-size: 13px;
  font-weight: 600;
}

.status-badge {
  border: 1px solid #d0d5dd;
  border-radius: 999px;
  display: inline-block;
  font-size: 12px;
  line-height: 20px;
  padding: 0 8px;
  white-space: nowrap;
}

.status-good {
  background: #ecfdf3;
  border-color: #abefc6;
  color: #067647;
}

.status-info {
  background: #eff8ff;
  border-color: #b2ddff;
  color: #175cd3;
}

.status-bad {
  background: #fef3f2;
  border-color: #fecdca;
  color: #b42318;
}

.status-muted {
  background: #f9fafb;
  color: #475467;
}

.empty-cell {
  color: #667085;
  text-align: center;
}

.pagination {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
  margin-top: 12px;
}

.actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}
</style>
