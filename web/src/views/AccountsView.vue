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

function statusClass(status: string) {
  if (status === 'ONLINE') return 'status-success'
  if (status === 'LOGIN_REQUIRED' || status === 'PASSWORD_REQUIRED') return 'status-warning'
  if (status === 'ERROR') return 'status-danger'
  return 'status-muted'
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

function closeTelegramLogin() {
  loginDialogVisible.value = false
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
    telegram.phone = loginPhone.value
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
    telegram.phone = loginPhone.value
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
        <p class="page-subtitle">管理 Telegram 会话状态、重新登录和账号删除。</p>
      </div>
      <div class="header-actions">
        <n-button :loading="telegram.loading" @click="telegram.loadAccounts">刷新</n-button>
        <n-button type="primary" @click="openTelegramLogin()">添加账号</n-button>
      </div>
    </div>

    <div class="table-panel">
      <table class="data-table">
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
          <tr v-if="telegram.loading && telegram.accounts.length === 0">
            <td colspan="6">
              <div class="loading-stack" aria-label="正在加载账号">
                <span class="skeleton-line" />
                <span class="skeleton-line short" />
              </div>
            </td>
          </tr>
          <tr v-for="account in telegram.accounts" :key="account.id">
            <td>{{ account.phone }}</td>
            <td>{{ displayName(account.first_name, account.last_name, account.username) }}</td>
            <td>
              <span class="status-pill" :class="statusClass(account.status)">{{ statusLabel(account.status) }}</span>
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
          <tr v-if="!telegram.loading && telegram.accounts.length === 0">
            <td colspan="6">
              <div class="empty-state">
                <strong>暂无账号</strong>
                <span>添加 Telegram 账号后即可同步频道元数据。</span>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <n-modal v-model:show="loginDialogVisible" :mask-closable="false">
      <n-card class="login-dialog" :bordered="false">
        <div class="login-dialog-header">
          <div>
            <p class="page-kicker">Telegram</p>
            <h2>Telegram 登录</h2>
          </div>
          <n-button
            aria-label="关闭 Telegram 登录"
            circle
            quaternary
            size="small"
            :disabled="telegram.loading"
            @click="closeTelegramLogin"
          >
            ×
          </n-button>
        </div>
        <n-form @submit.prevent>
          <n-form-item label="手机号">
            <n-input v-model:value="loginPhone" autocomplete="tel" placeholder="请输入手机号码" />
          </n-form-item>
          <n-button type="primary" block :loading="telegram.loading" @click="sendCode">发送验证码</n-button>

          <div class="form-section">
            <n-form-item label="验证码">
              <n-input
                v-model:value="loginCode"
                inputmode="numeric"
                autocomplete="one-time-code"
                placeholder="请输入验证码"
                :disabled="!loginCodeSent"
              />
            </n-form-item>
            <n-button type="primary" block :disabled="!loginCodeSent" :loading="telegram.loading" @click="signIn">
              登录
            </n-button>
          </div>

          <div v-if="telegram.passwordRequired" class="form-section">
            <n-form-item label="两步验证密码">
              <n-input
                v-model:value="loginPassword"
                type="password"
                autocomplete="current-password"
                placeholder="请输入密码"
              />
            </n-form-item>
            <n-button type="primary" block :loading="telegram.loading" @click="submitPassword">
              提交密码
            </n-button>
          </div>
        </n-form>
        <p v-if="metadataText" class="sync-result">{{ metadataText }}</p>
        <div class="login-dialog-actions">
          <n-button :disabled="telegram.loading" @click="closeTelegramLogin">取消</n-button>
        </div>
      </n-card>
    </n-modal>
  </section>
</template>

<style scoped>
table {
  min-width: 760px;
}

.action-buttons {
  display: flex;
  gap: 8px;
  white-space: nowrap;
}

.login-dialog {
  max-width: 420px;
  width: min(420px, calc(100vw - 32px));
}

.login-dialog-header {
  align-items: flex-start;
  display: flex;
  gap: 12px;
  justify-content: space-between;
}

.login-dialog h2 {
  font-size: 22px;
  margin: 0 0 22px;
}

.login-dialog-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

.sync-result {
  color: var(--app-text-muted);
  margin: 16px 0 0;
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
