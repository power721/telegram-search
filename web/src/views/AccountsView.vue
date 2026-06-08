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
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short'
  }).format(new Date(value))
}

async function logoutAccount(account: TelegramAccount) {
  await telegram.logoutAccount(account.id)
}

function confirmDeleteAccount(account: TelegramAccount) {
  dialog.warning({
    title: 'Delete account',
    content: `Delete ${account.phone}? This removes the account and its indexed data.`,
    positiveText: 'Delete',
    negativeText: 'Cancel',
    onPositiveClick: () => telegram.deleteAccount(account.id)
  })
}
</script>

<template>
  <section class="page-section">
    <div class="page-header">
      <div>
        <p class="page-kicker">Telegram</p>
        <h1 class="page-title">Accounts</h1>
      </div>
      <n-button :loading="telegram.loading" @click="telegram.loadAccounts">Refresh</n-button>
    </div>

    <div class="table-panel">
      <table>
        <thead>
          <tr>
            <th>Phone</th>
            <th>Name</th>
            <th>Status</th>
            <th>Last Online</th>
            <th>Last Error</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="account in telegram.accounts" :key="account.id">
            <td>{{ account.phone }}</td>
            <td>{{ displayName(account.first_name, account.last_name, account.username) }}</td>
            <td>
              <n-tag size="small" :type="account.status === 'ONLINE' ? 'success' : 'default'">
                {{ account.status }}
              </n-tag>
            </td>
            <td>{{ formatDate(account.last_online_at) }}</td>
            <td>{{ account.last_error || '-' }}</td>
            <td>
              <div class="action-buttons">
                <n-button size="small" :loading="telegram.loading" @click="logoutAccount(account)">
                  Logout
                </n-button>
                <n-button
                  size="small"
                  type="error"
                  ghost
                  :loading="telegram.loading"
                  @click="confirmDeleteAccount(account)"
                >
                  Delete
                </n-button>
              </div>
            </td>
          </tr>
          <tr v-if="telegram.accounts.length === 0">
            <td colspan="6" class="empty-cell">No accounts</td>
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
