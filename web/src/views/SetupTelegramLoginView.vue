<script setup lang="ts">
import { useMessage } from 'naive-ui'
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useSetupStore } from '@/stores/setup'
import { useTelegramStore } from '@/stores/telegram'

const router = useRouter()
const message = useMessage()
const setup = useSetupStore()
const telegram = useTelegramStore()

const phone = ref('')
const code = ref('')
const password = ref('')
const codeSent = ref(false)

const metadataText = computed(() => {
  const sync = telegram.loginResult?.metadata_sync
  if (!sync) return ''
  if (sync.status === 'succeeded') return `元数据同步成功：${sync.channel_count} 个频道`
  if (sync.status === 'failed') return `元数据同步失败：${sync.error ?? '未知错误'}`
  return `元数据同步状态：${sync.status}`
})

async function sendCode() {
  try {
    await telegram.sendCode(phone.value)
    codeSent.value = true
    message.success('验证码已发送')
  } catch (error) {
    message.error(error instanceof Error ? error.message : '无法发送验证码')
  }
}

async function signIn() {
  try {
    const response = await telegram.signIn(code.value)
    if (response.account) {
      await finish()
    }
  } catch (error) {
    message.error(error instanceof Error ? error.message : '无法登录')
  }
}

async function submitPassword() {
  try {
    const response = await telegram.submitPassword(password.value)
    if (response.account) {
      await finish()
    }
  } catch (error) {
    message.error(error instanceof Error ? error.message : '无法提交密码')
  }
}

async function finish() {
  await setup.load()
  message.success('Telegram 账号已连接')
  await router.push('/setup/listen-rules')
}
</script>

<template>
  <main class="setup-page">
    <section class="setup-panel">
      <p class="eyebrow">首次运行设置</p>
      <h1>Telegram 登录</h1>
      <n-form @submit.prevent>
        <n-form-item label="手机号">
          <n-input v-model:value="phone" autocomplete="tel" />
        </n-form-item>
        <n-button type="primary" block :loading="telegram.loading" @click="sendCode">发送验证码</n-button>

        <div class="form-block">
          <n-form-item label="验证码">
            <n-input
              v-model:value="code"
              inputmode="numeric"
              autocomplete="one-time-code"
              :disabled="!codeSent"
            />
          </n-form-item>
          <n-button type="primary" block :disabled="!codeSent" :loading="telegram.loading" @click="signIn">
            登录
          </n-button>
        </div>

        <div v-if="telegram.passwordRequired" class="form-block">
          <n-form-item label="两步验证密码">
            <n-input v-model:value="password" type="password" autocomplete="current-password" />
          </n-form-item>
          <n-button type="primary" block :loading="telegram.loading" @click="submitPassword">
            提交密码
          </n-button>
        </div>
      </n-form>
      <p v-if="metadataText" class="sync-result">{{ metadataText }}</p>
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

.form-block {
  border-top: 1px solid #edf0f5;
  margin-top: 16px;
  padding-top: 16px;
}

.sync-result {
  color: #475467;
  margin: 16px 0 0;
}
</style>
