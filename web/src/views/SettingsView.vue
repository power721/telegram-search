<script setup lang="ts">
import { useMessage } from 'naive-ui'
import { onMounted, ref, watch } from 'vue'
import { apiGet, apiPut } from '@/api/client'
import type {
  RuntimeSettings,
  StorageUsage,
  SystemInfoResponse,
  TelegramAPISettingsResponse,
  VersionInfoResponse
} from '@/api/types'
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
const activeTab = ref('security')
const runtimeLoading = ref(false)
type SizeUnit = 'GB' | 'MB' | 'B'
const sizeUnitOptions: Array<{ label: string; value: SizeUnit }> = [
  { label: 'GB', value: 'GB' },
  { label: 'MB', value: 'MB' },
  { label: 'B', value: 'B' }
]
const sizeUnitMultipliers: Record<SizeUnit, number> = {
  GB: 1_000_000_000,
  MB: 1_000_000,
  B: 1
}
const runtimeForm = ref({
  workers: '',
  historyBatchSize: '',
  telegramRequestInterval: '',
  maxDBSize: '',
  maxDBSizeUnit: 'GB' as SizeUnit,
  maxMediaCache: '',
  maxMediaCacheUnit: 'GB' as SizeUnit,
  proxy: '',
  reconnectTimeout: '',
  dialTimeout: '',
  rateLimitEnabled: true,
  ratePerSecond: '',
  burst: '',
  streamConcurrency: '',
  streamBuffers: '',
  streamChunkTimeout: '',
  mediaConcurrency: ''
})

onMounted(() => {
  apiKey.load().catch((error) => {
    message.error(error instanceof Error ? error.message : '无法加载 API 密钥')
  })
  loadStorageUsage()
  loadTelegramSettings()
  loadRuntimeSettings()
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

async function loadRuntimeSettings() {
  try {
    const settings = await apiGet<RuntimeSettings>('/api/settings/runtime')
    fillRuntimeForm(settings)
  } catch (error) {
    message.error(error instanceof Error ? error.message : '无法加载运行参数')
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

async function updateRuntimeSettings() {
  let payload: RuntimeSettings
  try {
    payload = runtimePayload()
  } catch (error) {
    message.error(error instanceof Error ? error.message : '运行参数格式无效')
    return
  }
  runtimeLoading.value = true
  try {
    const saved = await apiPut<RuntimeSettings>('/api/settings/runtime', payload)
    fillRuntimeForm(saved)
    await loadStorageUsage()
    message.success('运行参数已保存，重启后生效')
  } catch (error) {
    message.error(error instanceof Error ? error.message : '无法保存运行参数')
  } finally {
    runtimeLoading.value = false
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

function splitSizeForInput(bytes: number) {
  if (bytes > 0 && bytes % sizeUnitMultipliers.GB === 0) {
    return { value: String(bytes / sizeUnitMultipliers.GB), unit: 'GB' as SizeUnit }
  }
  if (bytes > 0 && bytes % sizeUnitMultipliers.MB === 0) {
    return { value: String(bytes / sizeUnitMultipliers.MB), unit: 'MB' as SizeUnit }
  }
  return { value: String(bytes), unit: 'B' as SizeUnit }
}

function fillRuntimeForm(settings: RuntimeSettings) {
  const maxDBSize = splitSizeForInput(settings.storage.max_db_size)
  const maxMediaCache = splitSizeForInput(settings.storage.max_media_cache)
  runtimeForm.value = {
    workers: String(settings.sync.workers),
    historyBatchSize: String(settings.sync.history_batch_size),
    telegramRequestInterval: settings.sync.telegram_request_interval,
    maxDBSize: maxDBSize.value,
    maxDBSizeUnit: maxDBSize.unit,
    maxMediaCache: maxMediaCache.value,
    maxMediaCacheUnit: maxMediaCache.unit,
    proxy: settings.telegram.proxy,
    reconnectTimeout: settings.telegram.reconnect_timeout,
    dialTimeout: settings.telegram.dial_timeout,
    rateLimitEnabled: settings.telegram.rate_limit.enabled,
    ratePerSecond: String(settings.telegram.rate_limit.rate_per_second),
    burst: String(settings.telegram.rate_limit.burst),
    streamConcurrency: String(settings.telegram.stream.concurrency),
    streamBuffers: String(settings.telegram.stream.buffers),
    streamChunkTimeout: settings.telegram.stream.chunk_timeout,
    mediaConcurrency: String(settings.telegram.media.concurrency)
  }
}

function runtimePayload(): RuntimeSettings {
  return {
    sync: {
      workers: positiveInteger(runtimeForm.value.workers),
      history_batch_size: positiveInteger(runtimeForm.value.historyBatchSize),
      telegram_request_interval: runtimeForm.value.telegramRequestInterval.trim()
    },
    storage: {
      max_db_size: sizeLimitBytes(runtimeForm.value.maxDBSize, runtimeForm.value.maxDBSizeUnit),
      max_media_cache: sizeLimitBytes(runtimeForm.value.maxMediaCache, runtimeForm.value.maxMediaCacheUnit)
    },
    telegram: {
      proxy: runtimeForm.value.proxy.trim(),
      reconnect_timeout: runtimeForm.value.reconnectTimeout.trim(),
      dial_timeout: runtimeForm.value.dialTimeout.trim(),
      rate_limit: {
        enabled: runtimeForm.value.rateLimitEnabled,
        rate_per_second: positiveInteger(runtimeForm.value.ratePerSecond),
        burst: positiveInteger(runtimeForm.value.burst)
      },
      stream: {
        concurrency: positiveInteger(runtimeForm.value.streamConcurrency),
        buffers: positiveInteger(runtimeForm.value.streamBuffers),
        chunk_timeout: runtimeForm.value.streamChunkTimeout.trim()
      },
      media: {
        concurrency: positiveInteger(runtimeForm.value.mediaConcurrency)
      }
    }
  }
}

function positiveInteger(value: string) {
  const parsed = Number(value)
  if (!Number.isInteger(parsed) || parsed <= 0) {
    throw new Error('invalid positive integer')
  }
  return parsed
}

function sizeLimitBytes(value: string, unit: SizeUnit) {
  return positiveInteger(value) * sizeUnitMultipliers[unit]
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
        <p class="page-subtitle">管理账号、安全凭据、存储限额和运行参数。</p>
      </div>
    </div>
    <n-tabs v-model:value="activeTab" type="line" animated class="settings-tabs">
      <n-tab-pane name="security" tab="账号与安全">
        <div class="settings-panel-grid security-grid">
          <div class="security-column-left">
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
          </div>
          <div class="security-column-right">
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
          </div>
        </div>
      </n-tab-pane>

      <n-tab-pane name="storage" tab="存储">
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
          <n-form class="runtime-form storage-limit-form" @submit.prevent="updateRuntimeSettings">
            <n-form-item label="数据库容量上限">
              <div class="size-limit-row">
                <n-input
                  v-model:value="runtimeForm.maxDBSize"
                  data-testid="runtime-max-db-size-input"
                  inputmode="numeric"
                  placeholder="10"
                />
                <n-select
                  v-model:value="runtimeForm.maxDBSizeUnit"
                  data-testid="runtime-max-db-size-unit"
                  :options="sizeUnitOptions"
                />
              </div>
            </n-form-item>
            <n-form-item label="媒体缓存上限">
              <div class="size-limit-row">
                <n-input
                  v-model:value="runtimeForm.maxMediaCache"
                  data-testid="runtime-max-media-cache-input"
                  inputmode="numeric"
                  placeholder="20"
                />
                <n-select
                  v-model:value="runtimeForm.maxMediaCacheUnit"
                  data-testid="runtime-max-media-cache-unit"
                  :options="sizeUnitOptions"
                />
              </div>
            </n-form-item>
            <div class="form-actions">
              <n-button data-testid="save-runtime-storage" type="primary" :loading="runtimeLoading" @click="updateRuntimeSettings">
                保存
              </n-button>
            </div>
          </n-form>
        </section>
      </n-tab-pane>

      <n-tab-pane name="runtime" tab="运行参数">
        <section class="panel runtime-panel">
          <div class="panel-header">
            <h2>运行参数</h2>
            <span class="restart-note">保存后重启生效</span>
          </div>
          <n-form class="runtime-form" @submit.prevent="updateRuntimeSettings">
            <div class="runtime-section">
              <h3>同步</h3>
              <div class="runtime-grid">
                <n-form-item label="同步 workers">
                  <n-input v-model:value="runtimeForm.workers" data-testid="runtime-workers-input" inputmode="numeric" />
                </n-form-item>
                <n-form-item label="历史批量大小">
                  <n-input
                    v-model:value="runtimeForm.historyBatchSize"
                    data-testid="runtime-history-batch-size-input"
                    inputmode="numeric"
                  />
                </n-form-item>
                <n-form-item label="Telegram 请求间隔">
                  <n-input v-model:value="runtimeForm.telegramRequestInterval" data-testid="runtime-request-interval-input" />
                </n-form-item>
              </div>
            </div>

            <div class="runtime-section">
              <h3>Telegram 网络</h3>
              <div class="runtime-grid">
                <n-form-item label="代理">
                  <n-input v-model:value="runtimeForm.proxy" data-testid="runtime-proxy-input" placeholder="socks5://127.0.0.1:1080" />
                </n-form-item>
                <n-form-item label="重连超时">
                  <n-input v-model:value="runtimeForm.reconnectTimeout" data-testid="runtime-reconnect-timeout-input" />
                </n-form-item>
                <n-form-item label="拨号超时">
                  <n-input v-model:value="runtimeForm.dialTimeout" data-testid="runtime-dial-timeout-input" />
                </n-form-item>
              </div>
            </div>

            <div class="runtime-section">
              <h3>限速</h3>
              <label class="checkbox-row">
                <input
                  v-model="runtimeForm.rateLimitEnabled"
                  data-testid="runtime-rate-enabled-input"
                  type="checkbox"
                />
                启用 Telegram 请求限速
              </label>
              <div class="runtime-grid">
                <n-form-item label="每秒请求数">
                  <n-input v-model:value="runtimeForm.ratePerSecond" data-testid="runtime-rate-per-second-input" inputmode="numeric" />
                </n-form-item>
                <n-form-item label="突发容量">
                  <n-input v-model:value="runtimeForm.burst" data-testid="runtime-rate-burst-input" inputmode="numeric" />
                </n-form-item>
              </div>
            </div>

            <div class="runtime-section">
              <h3>媒体与流式读取</h3>
              <div class="runtime-grid">
                <n-form-item label="流式并发">
                  <n-input v-model:value="runtimeForm.streamConcurrency" data-testid="runtime-stream-concurrency-input" inputmode="numeric" />
                </n-form-item>
                <n-form-item label="预取缓冲数">
                  <n-input v-model:value="runtimeForm.streamBuffers" data-testid="runtime-stream-buffers-input" inputmode="numeric" />
                </n-form-item>
                <n-form-item label="分片超时">
                  <n-input v-model:value="runtimeForm.streamChunkTimeout" data-testid="runtime-stream-timeout-input" />
                </n-form-item>
                <n-form-item label="媒体下载并发">
                  <n-input v-model:value="runtimeForm.mediaConcurrency" data-testid="runtime-media-concurrency-input" inputmode="numeric" />
                </n-form-item>
              </div>
            </div>

            <div class="form-actions">
              <n-button data-testid="save-runtime-settings" type="primary" :loading="runtimeLoading" @click="updateRuntimeSettings">
                保存
              </n-button>
            </div>
          </n-form>
        </section>
      </n-tab-pane>

      <n-tab-pane name="system" tab="系统">
        <div class="settings-panel-grid">
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
            </dl>
          </section>
        </div>
      </n-tab-pane>
    </n-tabs>
  </section>
</template>

<style scoped>
.settings-tabs {
  width: 100%;
}

.settings-panel-grid {
  align-items: start;
  display: grid;
  gap: 16px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.security-column-left,
.security-column-right {
  display: grid;
  gap: 16px;
}

.credential-form,
.telegram-form,
.runtime-form {
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

.runtime-panel {
  display: grid;
  gap: 16px;
}

.storage-limit-form {
  margin-top: 16px;
}

.size-limit-row {
  display: grid;
  gap: 8px;
  grid-template-columns: minmax(0, 1fr) 92px;
  width: 100%;
}

.runtime-section {
  border-top: 1px solid var(--app-border);
  display: grid;
  gap: 10px;
  padding-top: 14px;
}

.runtime-section:first-child {
  border-top: 0;
  padding-top: 0;
}

.runtime-section h3 {
  color: var(--app-text);
  font-size: 15px;
  margin: 0;
}

.runtime-grid {
  display: grid;
  gap: 12px 16px;
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.restart-note {
  color: var(--app-text-muted);
  font-size: 13px;
}

.checkbox-row {
  align-items: center;
  color: var(--app-text);
  display: inline-flex;
  gap: 8px;
  min-height: 32px;
}

.checkbox-row input {
  height: 16px;
  margin: 0;
  width: 16px;
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

@media (max-width: 900px) {
  .settings-panel-grid,
  .runtime-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 520px) {
  .api-key-field {
    grid-template-columns: 1fr;
  }

  .size-limit-row {
    grid-template-columns: 1fr;
  }
}
</style>
