<script setup lang="ts">
import { useMessage } from 'naive-ui'
import QRCode from 'qrcode'
import { computed, nextTick, onBeforeUnmount, ref } from 'vue'
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
const loginMode = ref<'qr' | 'code'>('qr')
const qrCanvas = ref<HTMLCanvasElement | null>(null)
const qrLoginID = ref('')
const qrStatus = ref('')
let qrPolling: number | undefined

const metadataText = computed(() => {
  const sync = telegram.loginResult?.metadata_sync
  if (!sync) return ''
  if (sync.status === 'succeeded') return `元数据同步成功：${sync.channel_count} 个频道`
  if (sync.status === 'failed') return `元数据同步失败：${sync.error ?? '未知错误'}`
  return `元数据同步状态：${sync.status}`
})

function setLoginMode(mode: 'qr' | 'code') {
  loginMode.value = mode
  if (mode === 'code') {
    stopQRPolling()
  }
}

async function startQRLogin() {
  try {
    stopQRPolling()
    const response = await telegram.startQRLogin()
    qrLoginID.value = response.login_id
    qrStatus.value = response.status
    await renderQRCode(response.qr_url)
    await pollQRLogin()
  } catch (error) {
    message.error(error instanceof Error ? error.message : '无法生成二维码')
  }
}

async function renderQRCode(value?: string) {
  if (!value) return
  await nextTick()
  if (qrCanvas.value) {
    await QRCode.toCanvas(qrCanvas.value, value, { width: 220, margin: 1 })
  }
}

async function pollQRLogin() {
  if (!qrLoginID.value) return
  try {
    const response = await telegram.pollQRLogin(qrLoginID.value)
    qrStatus.value = response.status
    if (response.qr_url) {
      await renderQRCode(response.qr_url)
    }
    if (response.account) {
      await finish()
      return
    }
    stopQRPolling()
    qrPolling = window.setTimeout(() => {
      void pollQRLogin()
    }, 2000)
  } catch (error) {
    stopQRPolling()
    message.error(error instanceof Error ? error.message : '无法确认扫码状态')
  }
}

async function cancelQRLogin() {
  stopQRPolling()
  if (!qrLoginID.value) return
  try {
    await telegram.cancelQRLogin(qrLoginID.value)
    qrLoginID.value = ''
    qrStatus.value = ''
  } catch (error) {
    message.error(error instanceof Error ? error.message : '无法取消扫码登录')
  }
}

function stopQRPolling() {
  if (qrPolling !== undefined) {
    window.clearTimeout(qrPolling)
    qrPolling = undefined
  }
}

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
  stopQRPolling()
  await setup.load()
  message.success('Telegram 账号已连接')
  await router.push('/setup/listen-rules')
}

onBeforeUnmount(() => {
  stopQRPolling()
})
</script>

<template>
  <main class="setup-page">
    <section class="setup-panel">
      <p class="eyebrow">首次运行设置</p>
      <h1>Telegram 登录</h1>
      <n-button-group class="mode-switch">
        <n-button :type="loginMode === 'qr' ? 'primary' : 'default'" @click="setLoginMode('qr')">
          扫码登录
        </n-button>
        <n-button :type="loginMode === 'code' ? 'primary' : 'default'" @click="setLoginMode('code')">
          验证码登录
        </n-button>
      </n-button-group>

      <div v-if="loginMode === 'qr'" class="qr-login">
        <div class="qr-surface">
          <canvas v-show="qrLoginID" ref="qrCanvas" class="qr-canvas" />
          <div v-if="!qrLoginID" class="qr-placeholder">QR</div>
        </div>
        <n-button type="primary" block :loading="telegram.loading" @click="startQRLogin">生成二维码</n-button>
        <n-button v-if="qrLoginID" block @click="cancelQRLogin">取消</n-button>
        <p v-if="qrStatus" class="sync-result">扫码状态：{{ qrStatus }}</p>
      </div>

      <n-form v-else @submit.prevent>
        <n-form-item label="手机号">
          <n-input v-model:value="phone" autocomplete="tel" placeholder="请输入手机号码" />
        </n-form-item>
        <n-button type="primary" block :loading="telegram.loading" @click="sendCode">发送验证码</n-button>

        <div class="form-section">
          <n-form-item label="验证码">
            <n-input
              v-model:value="code"
              inputmode="numeric"
              autocomplete="one-time-code"
              placeholder="请输入验证码"
              :disabled="!codeSent"
            />
          </n-form-item>
          <n-button type="primary" block :disabled="!codeSent" :loading="telegram.loading" @click="signIn">
            登录
          </n-button>
        </div>

        <div v-if="telegram.passwordRequired" class="form-section">
          <n-form-item label="两步验证密码">
            <n-input v-model:value="password" type="password" autocomplete="current-password" placeholder="请输入密码" />
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
h1 {
  margin: 0 0 22px;
}

.mode-switch {
  display: grid;
  grid-template-columns: 1fr 1fr;
  margin-bottom: 18px;
  width: 100%;
}

.qr-login {
  display: grid;
  gap: 14px;
}

.qr-surface {
  align-items: center;
  aspect-ratio: 1;
  background: var(--app-surface-muted);
  border: 1px solid var(--app-border);
  border-radius: var(--app-radius);
  display: flex;
  justify-content: center;
  margin: 0 auto;
  max-width: 260px;
  width: 100%;
}

.qr-canvas {
  height: 220px;
  width: 220px;
}

.qr-placeholder {
  align-items: center;
  border: 1px dashed var(--app-border-strong);
  border-radius: var(--app-radius);
  color: var(--app-text-muted);
  display: flex;
  font-size: 20px;
  font-weight: 700;
  height: 120px;
  justify-content: center;
  width: 120px;
}

.sync-result {
  color: var(--app-text-muted);
  margin: 16px 0 0;
}
</style>
