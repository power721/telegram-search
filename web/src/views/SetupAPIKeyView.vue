<script setup lang="ts">
import { useMessage } from 'naive-ui'
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useSetupStore } from '@/stores/setup'

const router = useRouter()
const message = useMessage()
const setup = useSetupStore()

const createdKey = ref('')

async function ensureKey() {
  try {
    const response = await setup.createAPIKey()
    createdKey.value = response.key
    message.success('API 密钥已自动生成')
  } catch (error) {
    message.error(error instanceof Error ? error.message : '无法生成 API 密钥')
  }
}

onMounted(ensureKey)
</script>

<template>
  <main class="setup-page">
    <section class="setup-panel">
      <p class="eyebrow">首次运行设置</p>
      <h1>API 密钥</h1>
      <div v-if="createdKey" class="key-result form-section">
        <p>API 密钥</p>
        <code>{{ createdKey }}</code>
        <n-button type="primary" @click="router.push('/setup/telegram-api')">继续</n-button>
      </div>
      <n-button v-else type="primary" :loading="setup.loading" disabled>正在生成密钥</n-button>
    </section>
  </main>
</template>

<style scoped>
h1 {
  margin: 0 0 22px;
}

.key-result {
  display: grid;
  gap: 10px;
  margin-top: 16px;
}

.key-result p {
  color: var(--app-text-muted);
  margin: 0;
}

code {
  background: var(--app-surface-muted);
  border: 1px solid var(--app-border);
  border-radius: var(--app-radius);
  color: var(--app-text);
  overflow-wrap: anywhere;
  padding: 8px;
}
</style>
