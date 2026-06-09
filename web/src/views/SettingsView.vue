<script setup lang="ts">
import { useMessage } from 'naive-ui'
import { onMounted, ref, watch } from 'vue'
import { apiGet, apiPut } from '@/api/client'
import type { StorageUsage, SystemInfoResponse, TelegramAPISettingsResponse, VersionInfoResponse } from '@/api/types'
import { useAPIKeyStore } from '@/stores/apiKey'
import { useAuthStore } from '@/stores/auth'

const message = useMessage()
const apiKey = useAPIKeyStore()
const auth = useAuthStore()
const showAPIKey = ref(false)
const credentialsUsername = ref('')
const currentPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const credentialsLoading = ref(false)
const storageUsage = ref<StorageUsage | null>(null)
const telegramSettings = ref<TelegramAPISettingsResponse | null>(null)
const telegramAppID = ref('')
const telegramAppHash = ref('')
const telegramLoading = ref(false)
const versionInfo = ref<VersionInfoResponse | null>(null)
const versionLoading = ref(false)
const versionError = ref('')
const systemInfo = ref<SystemInfoResponse | null>(null)

onMounted(() => {
  apiKey.load().catch((error) => {
    message.error(error instanceof Error ? error.message : '无法加载 API 密钥')
  })
  loadStorageUsage()
  loadTelegramSettings()
  loadVersionInfo(false)
  loadSystemInfo()
  if (!auth.loaded) {
    auth.loadMe().catch(() => {
      message.error('无法加载当前管理员')
    })
  }
})

watch(
  () => auth.user?.username,
  (username) => {
    credentialsUsername.value = username ?? credentialsUsername.value
  },
  { immediate: true }
)

async function updateCredentials() {
  const username = credentialsUsername.value.trim()
  if (!username) {
    message.error('用户名不能为空')
    return
  }
  if (!currentPassword.value) {
    message.error('请输入当前密码')
    return
  }
  if (newPassword.value && newPassword.value.length < 8) {
    message.error('新密码至少 8 位')
    return
  }
  if (newPassword.value !== confirmPassword.value) {
    message.error('两次输入的新密码不一致')
    return
  }
  credentialsLoading.value = true
  try {
    await auth.updateCredentials(username, currentPassword.value, newPassword.value)
    credentialsUsername.value = auth.user?.username ?? username
    currentPassword.value = ''
    newPassword.value = ''
    confirmPassword.value = ''
    message.success('管理员账号已更新')
  } catch (error) {
    message.error(error instanceof Error ? error.message : '无法更新管理员账号')
  } finally {
    credentialsLoading.value = false
  }
}

async function loadStorageUsage() {
  try {
    storageUsage.value = await apiGet<StorageUsage>('/api/storage/usage')
  } catch (error) {
    message.error(error instanceof Error ? error.message : '无法加载存储限额')
  }
}

async function loadTelegramSettings() {
  try {
    telegramSettings.value = await apiGet<TelegramAPISettingsResponse>('/api/settings/telegram-api')
    telegramAppID.value = telegramSettings.value.app_id ? String(telegramSettings.value.app_id) : ''
  } catch (error) {
    message.error(error instanceof Error ? error.message : '无法加载 Telegram API')
  }
}

async function loadVersionInfo(checkUpdate = true) {
  versionLoading.value = true
  versionError.value = ''
  try {
    versionInfo.value = await apiGet<VersionInfoResponse>(checkUpdate ? '/api/settings/version?check_update=true' : '/api/settings/version')
  } catch (error) {
    versionError.value = error instanceof Error ? error.message : '无法检查更新'
    message.error(versionError.value)
  } finally {
    versionLoading.value = false
  }
}

async function loadSystemInfo() {
  try {
    systemInfo.value = await apiGet<SystemInfoResponse>('/api/settings/system-info')
  } catch (error) {
    message.error(error instanceof Error ? error.message : '无法加载系统信息')
  }
}

async function updateTelegramAPI() {
  const appID = Number(telegramAppID.value)
  const appHash = telegramAppHash.value.trim()
  if (!Number.isInteger(appID) || appID <= 0) {
    message.error('App ID 必须大于 0')
    return
  }
  if (!appHash) {
    message.error('请输入 App Hash')
    return
  }
  telegramLoading.value = true
  try {
    telegramSettings.value = await apiPut<TelegramAPISettingsResponse>('/api/settings/telegram-api', {
      app_id: appID,
      app_hash: appHash
    })
    telegramAppID.value = String(telegramSettings.value.app_id)
    telegramAppHash.value = ''
    message.success('Telegram API 已更新')
  } catch (error) {
    message.error(error instanceof Error ? error.message : '无法保存 Telegram API')
  } finally {
    telegramLoading.value = false
  }
}

async function regenerate() {
  try {
    await apiKey.regenerate()
    showAPIKey.value = false
    message.success('API 密钥已重新生成')
  } catch (error) {
    message.error(error instanceof Error ? error.message : '无法重新生成 API 密钥')
  }
}

function toggleAPIKeyVisibility() {
  showAPIKey.value = !showAPIKey.value
}

function formatTime(value?: string) {
  return value ? new Date(value).toLocaleString() : '-'
}

function formatCount(value = 0) {
  return value.toLocaleString()
}

function formatBytes(value = 0) {
  if (value >= 1_000_000_000) return `${(value / 1_000_000_000).toFixed(1)} GB`
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1)} MB`
  if (value >= 1_000) return `${(value / 1_000).toFixed(1)} KB`
  return `${value} B`
}

function versionStatusText() {
  if (versionError.value) return '检查失败'
  if (!versionInfo.value?.latest_version) return '尚未检查'
  if (versionInfo.value.update_available) return `发现新版本 ${versionInfo.value.latest_version}`
  return '已是最新版本'
}
</script>

<template>
  <section class="page-section">
    <div class="page-header">
      <div>
        <p class="page-kicker">配置</p>
        <h1 class="page-title">设置</h1>
        <p class="page-subtitle">管理管理员账号、存储限额和浏览器 API 密钥。</p>
      </div>
    </div>
    <div class="settings-grid">
      <div class="settings-column settings-column-left">
        <section class="panel admin-panel">
          <h2>管理员账号</h2>
          <n-form class="credential-form" @submit.prevent="updateCredentials">
            <n-form-item label="用户名">
              <n-input
                v-model:value="credentialsUsername"
                data-testid="admin-username-input"
                autocomplete="username"
                placeholder="请输入用户名"
              />
            </n-form-item>
            <n-form-item label="当前密码">
              <n-input
                v-model:value="currentPassword"
                data-testid="current-password-input"
                type="password"
                autocomplete="current-password"
                placeholder="请输入密码"
              />
            </n-form-item>
            <n-form-item label="新密码">
              <n-input
                v-model:value="newPassword"
                data-testid="new-password-input"
                type="password"
                autocomplete="new-password"
                placeholder="留空则不修改"
              />
            </n-form-item>
            <n-form-item label="确认新密码">
              <n-input
                v-model:value="confirmPassword"
                data-testid="confirm-password-input"
                type="password"
                autocomplete="new-password"
                placeholder="留空则不修改"
              />
            </n-form-item>
            <div class="form-actions">
              <n-button
                data-testid="save-admin-credentials"
                type="primary"
                :loading="credentialsLoading"
                @click="updateCredentials"
              >
                保存
              </n-button>
            </div>
          </n-form>
        </section>
        <section class="panel storage-panel">
          <h2>存储</h2>
          <dl>
            <div>
              <dt>最大数据库容量</dt>
              <dd>{{ formatBytes(storageUsage?.max_db_bytes) }}</dd>
            </div>
            <div>
              <dt>最大媒体缓存</dt>
              <dd>{{ formatBytes(storageUsage?.max_media_bytes) }}</dd>
            </div>
          </dl>
        </section>
        <section class="panel version-panel">
          <div class="panel-header">
            <h2>版本</h2>
            <n-button data-testid="check-version" size="small" type="primary" :loading="versionLoading" @click="loadVersionInfo()">
              检查更新
            </n-button>
          </div>
          <dl>
            <div>
              <dt>当前版本</dt>
              <dd data-testid="current-version">{{ versionInfo?.current_version ?? '-' }}</dd>
            </div>
            <div>
              <dt>更新状态</dt>
              <dd>{{ versionStatusText() }}</dd>
            </div>
          </dl>
          <a
            v-if="versionInfo?.latest_url"
            class="version-link"
            :href="versionInfo.latest_url"
            target="_blank"
            rel="noreferrer"
          >
            查看 GitHub Release
          </a>
        </section>
      </div>
      <div class="settings-column settings-column-right">
        <section class="panel api-key-panel">
          <div class="panel-header">
            <h2>API 密钥</h2>
            <n-button data-testid="regenerate-api-key" size="small" type="primary" :loading="apiKey.loading" @click="regenerate">
              重新生成
            </n-button>
          </div>
          <dl v-if="apiKey.current">
            <div>
              <dt>创建时间</dt>
              <dd>{{ formatTime(apiKey.current.created_at) }}</dd>
            </div>
            <div>
              <dt>最后使用</dt>
              <dd>{{ formatTime(apiKey.current.last_used_at) }}</dd>
            </div>
            <div>
              <dt>使用次数</dt>
              <dd data-testid="api-key-usage-count">{{ formatCount(apiKey.current.usage_count) }}</dd>
            </div>
          </dl>
          <div v-if="apiKey.current" class="api-key-field">
            <input
              data-testid="api-key-input"
              class="api-key-input"
              :type="showAPIKey ? 'text' : 'password'"
              :value="apiKey.current.key"
              readonly
              autocomplete="off"
            />
            <n-button
              data-testid="toggle-api-key-visibility"
              size="small"
              secondary
              @click="toggleAPIKeyVisibility"
            >
              {{ showAPIKey ? '隐藏' : '显示' }}
            </n-button>
          </div>
          <div v-else class="loading-stack" aria-label="正在加载 API 密钥">
            <span class="skeleton-line" />
            <span class="skeleton-line short" />
          </div>
        </section>
        <section class="panel telegram-panel">
          <h2>Telegram API</h2>
          <n-form class="telegram-form" @submit.prevent="updateTelegramAPI">
            <n-form-item label="App ID">
              <n-input
                v-model:value="telegramAppID"
                data-testid="telegram-app-id-input"
                inputmode="numeric"
                placeholder="请输入 App ID"
              />
            </n-form-item>
            <n-form-item label="App Hash">
              <n-input
                v-model:value="telegramAppHash"
                data-testid="telegram-app-hash-input"
                type="password"
                autocomplete="off"
                :placeholder="telegramSettings?.app_hash_set ? '已设置，输入新 Hash 保存' : '请输入 App Hash'"
              />
            </n-form-item>
            <div class="form-actions">
              <n-button
                data-testid="save-telegram-api"
                type="primary"
                :loading="telegramLoading"
                @click="updateTelegramAPI"
              >
                保存
              </n-button>
            </div>
          </n-form>
        </section>
        <section class="panel system-panel">
          <h2>系统</h2>
          <dl>
            <div>
              <dt>名称</dt>
              <dd data-testid="system-name">{{ systemInfo?.name || '-' }}</dd>
            </div>
            <div>
              <dt>版本</dt>
              <dd>{{ systemInfo?.version || '-' }}</dd>
            </div>
            <div>
              <dt>架构</dt>
              <dd>{{ systemInfo?.architecture || '-' }}</dd>
            </div>
            <div>
              <dt>主机名</dt>
              <dd>{{ systemInfo?.hostname || '-' }}</dd>
            </div>
            <div>
              <dt>CPU</dt>
              <dd>{{ systemInfo?.cpu_count ?? '-' }}</dd>
            </div>
            <div>
              <dt>Go 版本</dt>
              <dd>{{ systemInfo?.go_version || '-' }}</dd>
            </div>
          </dl>
        </section>
      </div>
    </div>
  </section>
</template>

<style scoped>
.settings-grid {
  align-items: start;
  display: grid;
  column-gap: 16px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.settings-column {
  display: grid;
  gap: 16px;
}

.credential-form {
  display: grid;
}

.telegram-form {
  display: grid;
}

.form-actions {
  display: flex;
  justify-content: flex-end;
}

.panel-header h2 {
  margin: 0;
}

.api-key-panel {
  display: grid;
  gap: 12px;
}

.version-panel {
  display: grid;
  gap: 12px;
}

.system-panel {
  display: grid;
  gap: 12px;
}

.version-link {
  color: var(--app-primary);
  font-weight: 600;
  text-decoration: none;
}

.version-link:hover {
  text-decoration: underline;
}

dl {
  margin: 0;
}

dl div {
  display: flex;
  justify-content: space-between;
  padding: 7px 0;
}

dd {
  font-weight: 600;
  margin: 0;
}

.api-key-field {
  align-items: center;
  background: var(--app-surface-muted);
  border: 1px solid var(--app-border);
  border-radius: var(--app-radius);
  display: grid;
  gap: 8px;
  grid-template-columns: minmax(0, 1fr) auto;
  padding: 8px;
}

.api-key-input {
  background: transparent;
  border: 0;
  color: var(--app-text);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', monospace;
  font-size: 14px;
  min-width: 0;
  overflow-wrap: anywhere;
  outline: 0;
  width: 100%;
}

.loading-stack {
  display: grid;
  gap: 8px;
}

.loading-stack .short {
  width: 58%;
}

@media (max-width: 840px) {
  .settings-grid {
    row-gap: 16px;
    grid-template-columns: 1fr;
  }

  .settings-column {
    display: contents;
  }

  .admin-panel {
    order: 1;
  }

  .api-key-panel {
    order: 4;
  }

  .storage-panel {
    order: 2;
  }

  .telegram-panel {
    order: 5;
  }

  .version-panel {
    order: 3;
  }

  .system-panel {
    order: 6;
  }
}

@media (max-width: 520px) {
  .api-key-field {
    grid-template-columns: 1fr;
  }
}
</style>
