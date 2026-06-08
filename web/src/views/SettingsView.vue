<script setup lang="ts">
import { useMessage } from 'naive-ui'
import { onMounted, ref } from 'vue'
import { useAPIKeyStore } from '@/stores/apiKey'

const message = useMessage()
const apiKey = useAPIKeyStore()
const showAPIKey = ref(false)

onMounted(() => {
  apiKey.load().catch((error) => {
    message.error(error instanceof Error ? error.message : '无法加载 API 密钥')
  })
})

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
</script>

<template>
  <section class="page-section">
    <p class="page-kicker">配置</p>
    <h1 class="page-title">设置</h1>
    <div class="settings-grid">
      <section class="panel">
        <h2>存储</h2>
        <dl>
          <div>
            <dt>最大数据库容量</dt>
            <dd>10 GB</dd>
          </div>
          <div>
            <dt>最大媒体缓存</dt>
            <dd>20 GB</dd>
          </div>
        </dl>
      </section>
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
        <p v-else>正在加载 API 密钥</p>
      </section>
    </div>
  </section>
</template>

<style scoped>
.page-kicker {
  color: #667085;
  margin: 0 0 4px;
}

.page-title {
  font-size: 24px;
  margin: 0 0 18px;
}

.settings-grid {
  display: grid;
  gap: 16px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.panel {
  background: #ffffff;
  border: 1px solid #d9dee7;
  border-radius: 8px;
  padding: 14px;
}

h2 {
  font-size: 16px;
  margin: 0 0 12px;
}

.panel-header {
  align-items: center;
  display: flex;
  gap: 12px;
  justify-content: space-between;
}

.panel-header h2 {
  margin: 0;
}

.api-key-panel {
  display: grid;
  gap: 12px;
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

p {
  color: #667085;
  margin: 0;
}

.api-key-field {
  align-items: center;
  background: #f6f8fb;
  border: 1px solid #d9dee7;
  border-radius: 6px;
  display: grid;
  gap: 8px;
  grid-template-columns: minmax(0, 1fr) auto;
  padding: 8px;
}

.api-key-input {
  background: transparent;
  border: 0;
  color: #101828;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', monospace;
  font-size: 13px;
  min-width: 0;
  overflow-wrap: anywhere;
  outline: 0;
  width: 100%;
}

@media (max-width: 840px) {
  .settings-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 520px) {
  .api-key-field {
    grid-template-columns: 1fr;
  }
}
</style>
