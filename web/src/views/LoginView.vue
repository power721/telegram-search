<script setup lang="ts">
import { useMessage } from 'naive-ui'
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const message = useMessage()
const auth = useAuthStore()

const username = ref('admin')
const password = ref('')
const loading = ref(false)

async function submit() {
  loading.value = true
  try {
    await auth.login(username.value, password.value)
    await router.push('/')
  } catch (error) {
    message.error(error instanceof Error ? error.message : 'Login failed')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <main class="login-page">
    <section class="login-panel">
      <p class="eyebrow">Admin Console</p>
      <h1>Sign in to tg-search</h1>
      <n-form @submit.prevent="submit">
        <n-form-item label="Username">
          <n-input v-model:value="username" autocomplete="username" />
        </n-form-item>
        <n-form-item label="Password">
          <n-input v-model:value="password" type="password" autocomplete="current-password" />
        </n-form-item>
        <n-button type="primary" block :loading="loading" @click="submit">Sign In</n-button>
      </n-form>
    </section>
  </main>
</template>

<style scoped>
.login-page {
  align-items: center;
  display: flex;
  justify-content: center;
  min-height: 100vh;
  padding: 24px;
}

.login-panel {
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
</style>
