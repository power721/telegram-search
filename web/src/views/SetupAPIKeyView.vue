<script setup lang="ts">
import { useMessage } from 'naive-ui'
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useSetupStore } from '@/stores/setup'

const router = useRouter()
const message = useMessage()
const setup = useSetupStore()

const name = ref('default')
const createdKey = ref('')

async function createKey() {
  try {
    const response = await setup.createAPIKey(name.value)
    createdKey.value = response.key
    message.success('API key created')
  } catch (error) {
    message.error(error instanceof Error ? error.message : 'Could not create API key')
  }
}

async function skip() {
  try {
    await setup.skipAPIKey()
    await router.push('/setup/telegram-api')
  } catch (error) {
    message.error(error instanceof Error ? error.message : 'Could not skip API key')
  }
}
</script>

<template>
  <main class="setup-page">
    <section class="setup-panel">
      <p class="eyebrow">First Run Setup</p>
      <h1>API Key</h1>
      <n-form @submit.prevent="createKey">
        <n-form-item label="Name">
          <n-input v-model:value="name" autocomplete="off" />
        </n-form-item>
        <div class="actions">
          <n-button type="primary" :loading="setup.loading" @click="createKey">Create Key</n-button>
          <n-button :loading="setup.loading" @click="skip">Skip</n-button>
        </div>
        <div v-if="createdKey" class="key-result">
          <p>API key</p>
          <code>{{ createdKey }}</code>
          <n-button type="primary" @click="router.push('/setup/telegram-api')">Continue</n-button>
        </div>
      </n-form>
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

.actions {
  display: flex;
  gap: 10px;
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
