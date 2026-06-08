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
      <div v-if="createdKey" class="key-result">
        <p>API 密钥</p>
        <code>{{ createdKey }}</code>
        <n-button type="primary" @click="router.push('/setup/telegram-api')">继续</n-button>
      </div>
      <n-button v-else type="primary" :loading="setup.loading" disabled>正在生成密钥</n-button>
    </section>
  </main>
</template>

<style scoped>
.setup-page {
  align-items: center;
  display: flex;
  justify-content: center;
  min-height: 100vh;
  padding: 24px;
}

.setup-panel {
  background: #ffffff;
  border: 1px solid #d9dee7;
  border-radius: 8px;
  max-width: 420px;
  padding: 24px;
  width: 100%;
}

.eyebrow {
  color: #667085;
  font-size: 13px;
  margin: 0 0 8px;
}

h1 {
  font-size: 24px;
  margin: 0 0 22px;
}

.key-result {
  border-top: 1px solid #edf0f5;
  display: grid;
  gap: 10px;
  margin-top: 16px;
  padding-top: 16px;
}

.key-result p {
  color: #667085;
  margin: 0;
}

code {
  background: #f6f8fb;
  border: 1px solid #d9dee7;
  border-radius: 6px;
  color: #101828;
  overflow-wrap: anywhere;
  padding: 8px;
}
</style>
