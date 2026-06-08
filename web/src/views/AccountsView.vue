<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useDialog, useMessage } from 'naive-ui'
import type { TelegramAccount } from '@/api/types'
import { useTelegramStore } from '@/stores/telegram'

const telegram = useTelegramStore()
const dialog = useDialog()
const message = useMessage()

const loginDialogVisible = ref(false)
const loginPhone = ref('')
const loginCode = ref('')
const loginPassword = ref('')
const loginCodeSent = ref(false)

const metadataText = computed(() => {
  const sync = telegram.loginResult?.metadata_sync
  if (!sync) return ''
  if (sync.status === 'succeeded') return `元数据同步成功：${sync.channel_count} 个频道`
  if (sync.status === 'failed') return `元数据同步失败：${sync.error ?? '未知错误'}`
  return `元数据同步状态：${sync.status}`
})

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

function needsLogin(account: TelegramAccount) {
  return account.status === 'LOGIN_REQUIRED'
}

function openTelegramLogin(account?: TelegramAccount) {
  loginPhone.value = account?.phone ?? ''
  loginCode.value = ''
  loginPassword.value = ''
  loginCodeSent.value = false
  telegram.phone = loginPhone.value
  telegram.passwordRequired = false
  telegram.loginResult = null
  loginDialogVisible.value = true
}

async function sendCode() {
  try {
    await telegram.sendCode(loginPhone.value)
    loginCodeSent.value = true
    message.success('验证码已发送')
  } catch (error) {
    message.error(error instanceof Error ? error.message : '无法发送验证码')
  }
}

async function signIn() {
  try {
    const response = await telegram.signIn(loginCode.value)
    if (response.account) {
      finishLogin()
    }
  } catch (error) {
    message.error(error instanceof Error ? error.message : '无法登录')
  }
}

async function submitPassword() {
  try {
    const response = await telegram.submitPassword(loginPassword.value)
    if (response.account) {
      finishLogin()
    }
  } catch (error) {
    message.error(error instanceof Error ? error.message : '无法提交密码')
  }
}

function finishLogin() {
  loginDialogVisible.value = false
  message.success('Telegram 账号已连接')
}

async function logoutAccount(account: TelegramAccount) {
  await telegram.logoutAccount(account.id)
}

function confirmDeleteAccount(account: TelegramAccount) {
  dialog.warning({
    title: '删除账号',
    content: `确定删除 ${account.phone}？这会删除该账号及其索引数据。`,
    positiveText: '删除账号',
    positiveButtonProps: { type: 'error' },
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
      <div class="header-actions">
        <n-button :loading="telegram.loading" @click="telegram.loadAccounts">刷新</n-button>
        <n-button type="primary" @click="openTelegramLogin()">添加账号</n-button>
      </div>
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
                <n-button v-if="needsLogin(account)" size="small" type="primary" @click="openTelegramLogin(account)">
                  登录
                </n-button>
                <n-button v-else size="small" :loading="telegram.loading" @click="logoutAccount(account)">
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

    <n-modal v-model:show="loginDialogVisible">
      <section class="login-dialog">
        <p class="page-kicker">Telegram</p>
        <h2>Telegram 登录</h2>
        <n-form @submit.prevent>
          <n-form-item label="手机号">
            <n-input v-model:value="loginPhone" autocomplete="tel" />
          </n-form-item>
          <n-button type="primary" block :loading="telegram.loading" @click="sendCode">发送验证码</n-button>

          <div class="form-block">
            <n-form-item label="验证码">
              <n-input
                v-model:value="loginCode"
                inputmode="numeric"
                autocomplete="one-time-code"
                :disabled="!loginCodeSent"
              />
            </n-form-item>
            <n-button type="primary" block :disabled="!loginCodeSent" :loading="telegram.loading" @click="signIn">
              登录
            </n-button>
          </div>

          <div v-if="telegram.passwordRequired" class="form-block">
            <n-form-item label="两步验证密码">
              <n-input v-model:value="loginPassword" type="password" autocomplete="current-password" />
            </n-form-item>
            <n-button type="primary" block :loading="telegram.loading" @click="submitPassword">
              提交密码
            </n-button>
          </div>
        </n-form>
        <p v-if="metadataText" class="sync-result">{{ metadataText }}</p>
      </section>
    </n-modal>
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

.header-actions {
  display: flex;
  gap: 8px;
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

.login-dialog {
  background: #ffffff;
  border: 1px solid #d9dee7;
  border-radius: 8px;
  max-width: 420px;
  padding: 24px;
  width: min(420px, calc(100vw - 32px));
}

.login-dialog h2 {
  font-size: 22px;
  margin: 0 0 22px;
}

.form-block {
  border-top: 1px solid #edf0f5;
  margin-top: 16px;
  padding-top: 16px;
}

.sync-result {
  color: #475467;
  margin: 16px 0 0;
}

@media (max-width: 840px) {
  .page-header {
    align-items: stretch;
    flex-direction: column;
  }

  .header-actions {
    align-self: flex-start;
  }
}
</style>
