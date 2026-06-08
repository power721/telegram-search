<script setup lang="ts">
import { onMounted } from 'vue'
import { useDialog } from 'naive-ui'
import type { TelegramAccount } from '@/api/types'
import { useTelegramStore } from '@/stores/telegram'

const telegram = useTelegramStore()
const dialog = useDialog()

onMounted(() => {
  void telegram.loadAccounts()
})

function displayName(firstName: string, lastName: string, username: string) {
  const name = [firstName, lastName].filter(Boolean).join(' ')
  if (name) return name
  return username ? `@${username}` : '-'
}

function formatDate(value?: string) {
  if (!value) return '-'
  return new Intl.DateTimeFormat('zh-CN', {
    dateStyle: 'medium',
    timeStyle: 'short'
  }).format(new Date(value))
}

function statusLabel(status: string) {
  const labels: Record<string, string> = {
    ONLINE: '在线',
    LOGIN_REQUIRED: '需要登录',
    PASSWORD_REQUIRED: '需要密码',
    OFFLINE: '离线',
    ERROR: '异常'
  }
  return labels[status] ?? status
}

async function logoutAccount(account: TelegramAccount) {
  await telegram.logoutAccount(account.id)
}

function confirmDeleteAccount(account: TelegramAccount) {
  dialog.warning({
    title: '删除账号',
    content: `确定删除 ${account.phone}？这会删除该账号及其索引数据。`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: () => telegram.deleteAccount(account.id)
  })
}
</script>

<template>
  <section class="page-section">
    <div class="page-header">
      <div>
        <p class="page-kicker">Telegram</p>
        <h1 class="page-title">账号</h1>
      </div>
      <n-button :loading="telegram.loading" @click="telegram.loadAccounts">刷新</n-button>
    </div>

    <div class="table-panel">
      <table>
        <thead>
          <tr>
            <th>手机号</th>
            <th>名称</th>
            <th>状态</th>
            <th>最后在线</th>
            <th>最后错误</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="account in telegram.accounts" :key="account.id">
            <td>{{ account.phone }}</td>
            <td>{{ displayName(account.first_name, account.last_name, account.username) }}</td>
            <td>
              <n-tag size="small" :type="account.status === 'ONLINE' ? 'success' : 'default'">
                {{ statusLabel(account.status) }}
              </n-tag>
            </td>
            <td>{{ formatDate(account.last_online_at) }}</td>
            <td>{{ account.last_error || '-' }}</td>
            <td>
              <div class="action-buttons">
                <n-button size="small" :loading="telegram.loading" @click="logoutAccount(account)">
                  登出
                </n-button>
                <n-button
                  size="small"
                  type="error"
                  ghost
                  :loading="telegram.loading"
                  @click="confirmDeleteAccount(account)"
                >
                  删除
                </n-button>
              </div>
            </td>
          </tr>
          <tr v-if="telegram.accounts.length === 0">
            <td colspan="6" class="empty-cell">暂无账号</td>
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

.table-panel {
  background: #ffffff;
  border: 1px solid #d9dee7;
  border-radius: 8px;
  overflow-x: auto;
}

table {
  border-collapse: collapse;
  min-width: 760px;
  width: 100%;
}

th,
td {
  border-bottom: 1px solid #edf0f5;
  padding: 11px 12px;
  text-align: left;
  vertical-align: middle;
}

th {
  color: #667085;
  font-size: 13px;
  font-weight: 600;
}

tbody tr:last-child td {
  border-bottom: 0;
}

.action-buttons {
  display: flex;
  gap: 8px;
  white-space: nowrap;
}

.empty-cell {
  color: #667085;
  text-align: center;
}

@media (max-width: 840px) {
  .page-header {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
