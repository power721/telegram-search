<script setup lang="ts">
import { useMessage } from 'naive-ui'
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useSetupStore } from '@/stores/setup'
import { useTelegramStore } from '@/stores/telegram'

const router = useRouter()
const message = useMessage()
const setup = useSetupStore()
const telegram = useTelegramStore()

const appID = ref<number | null>(null)
const appHash = ref('')

async function submit() {
  try {
    await telegram.saveTelegramAPI(Number(appID.value), appHash.value)
    await setup.load()
    message.success('Telegram API ready')
    await router.push('/setup/telegram-login')
  } catch (error) {
    message.error(error instanceof Error ? error.message : 'Could not save Telegram API')
  }
}
</script>

<template>
  <main class="setup-page">
    <section class="setup-panel">
      <p class="eyebrow">First Run Setup</p>
      <h1>Telegram API</h1>
      <n-form @submit.prevent="submit">
        <n-form-item label="App ID (Optional)">
          <n-input-number
            v-model:value="appID"
            class="full-width"
            placeholder="Use built-in"
            :show-button="false"
          />
        </n-form-item>
        <n-form-item label="App Hash (Optional)">
          <n-input
            v-model:value="appHash"
            type="password"
            autocomplete="off"
            placeholder="Use built-in"
          />
        </n-form-item>
        <n-button type="primary" block :loading="telegram.loading" @click="submit">Continue</n-button>
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

.full-width {
  width: 100%;
}
</style>
