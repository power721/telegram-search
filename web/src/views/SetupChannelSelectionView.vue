<script setup lang="ts">
import { useMessage } from 'naive-ui'
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import type { TelegramChannel } from '@/api/types'
import AppPagination from '@/components/common/AppPagination.vue'
import { useChannelsStore } from '@/stores/channels'
import { useSetupStore } from '@/stores/setup'

const router = useRouter()
const message = useMessage()
const channels = useChannelsStore()
const setup = useSetupStore()

const selected = reactive<Record<number, boolean>>({})
const page = ref(1)
const pageSize = 50
const emptyRefreshAttempts = ref(0)
const maxEmptyRefreshAttempts = 10
let emptyRefreshTimer: number | undefined

const visibleChannels = computed(() => {
  const start = (page.value - 1) * pageSize
  return channels.items.slice(start, start + pageSize)
})

const totalPages = computed(() => Math.max(1, Math.ceil(channels.items.length / pageSize)))
const selectedCount = computed(() => channels.items.filter((channel) => selected[channel.id]).length)

onMounted(async () => {
  await loadChannels()
})

onUnmounted(() => {
  stopEmptyRefresh()
})

async function loadChannels() {
  await channels.loadChannels()
  for (const channel of channels.items) {
    selected[channel.id] = channel.history_sync_enabled || channel.listen_enabled
  }
  if (page.value > totalPages.value) {
    page.value = totalPages.value
  }
  scheduleEmptyRefresh()
}

function scheduleEmptyRefresh() {
  stopEmptyRefresh()
  if (channels.items.length > 0 || emptyRefreshAttempts.value >= maxEmptyRefreshAttempts) {
    return
  }
  emptyRefreshTimer = window.setTimeout(async () => {
    if (channels.loading) {
      scheduleEmptyRefresh()
      return
    }
    emptyRefreshAttempts.value += 1
    try {
      await loadChannels()
    } catch {
      scheduleEmptyRefresh()
    }
  }, 3000)
}

function stopEmptyRefresh() {
  if (emptyRefreshTimer) {
    window.clearTimeout(emptyRefreshTimer)
    emptyRefreshTimer = undefined
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
  if (channel.web_access_error) return 'status-danger'
  if (channel.web_access === true) return 'status-success'
  if (channel.username) return 'status-info'
  return 'status-muted'
}

function changePage(pageNumber: number) {
  page.value = Math.min(Math.max(1, pageNumber), totalPages.value)
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
    <section class="setup-panel setup-wide">
      <div class="page-header">
        <div>
          <p class="eyebrow">首次运行设置</p>
          <h1>选择频道</h1>
          <p class="page-subtitle">选择首次启用索引和实时监听的频道。</p>
        </div>
        <n-button :loading="channels.loading" @click="loadChannels">刷新</n-button>
      </div>
      <div class="summary">
        <span>{{ channels.items.length }} 个频道</span>
        <span>已选择 {{ selectedCount }} 个</span>
        <span>第 {{ page }} / {{ totalPages }} 页</span>
      </div>

      <div class="table-panel">
        <table class="data-table">
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
            <tr v-if="channels.loading && channels.items.length === 0">
              <td colspan="6">
                <div class="loading-stack" aria-label="正在加载频道">
                  <span class="skeleton-line" />
                  <span class="skeleton-line" />
                  <span class="skeleton-line short" />
                </div>
              </td>
            </tr>
            <tr v-for="channel in visibleChannels" :key="channel.id">
              <td>
                <n-checkbox v-model:checked="selected[channel.id]" />
              </td>
              <td class="title-cell">{{ channel.title }}</td>
              <td>{{ username(channel) }}</td>
              <td>
                <span class="status-pill" :class="channelStatusClass(channel)">
                  {{ channelStatus(channel) }}
                </span>
              </td>
              <td>{{ channel.member_count }}</td>
              <td class="description-cell" :title="channel.description">{{ channel.description || '-' }}</td>
            </tr>
            <tr v-if="!channels.loading && channels.items.length === 0">
              <td colspan="6">
                <div class="empty-state">
                  <strong>正在更新频道列表</strong>
                  <span>后台正在同步当前账号可访问的频道，稍后会自动刷新，也可以手动点击刷新。</span>
                  <span v-if="emptyRefreshAttempts < maxEmptyRefreshAttempts" class="empty-state-note">
                    已自动刷新 {{ emptyRefreshAttempts }} / {{ maxEmptyRefreshAttempts }} 次
                  </span>
                  <span v-else class="empty-state-note">如果仍为空，请确认 Telegram 账号已登录并稍后手动刷新。</span>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <AppPagination
        :loading="channels.loading"
        :page="page"
        :page-size="pageSize"
        :show-page-size="false"
        :total="channels.items.length"
        @update:page="changePage"
      />

      <div class="actions">
        <n-button type="primary" :loading="channels.loading || setup.loading" @click="finish">
          保存并开始
        </n-button>
      </div>
    </section>
  </main>
</template>

<style scoped>
.setup-wide {
  max-width: 1440px;
}

h1 {
  margin: 0;
}

.summary {
  color: var(--app-text-muted);
  display: flex;
  flex-wrap: wrap;
  font-size: 14px;
  gap: 16px;
  margin: 16px 0 12px;
}

table {
  table-layout: fixed;
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

.title-cell,
.description-cell {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

.loading-stack {
  display: grid;
  gap: 8px;
  padding: 8px 0;
}

.loading-stack .short {
  width: 58%;
}
</style>
