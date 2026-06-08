<script setup lang="ts">
import { useMessage } from 'naive-ui'
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useSetupStore } from '@/stores/setup'

const router = useRouter()
const message = useMessage()
const setup = useSetupStore()

const username = ref('admin')
const password = ref('')
const loading = ref(false)

async function submit() {
  loading.value = true
  try {
    await setup.createAdmin(username.value, password.value)
    message.success('Admin account created')
    await router.push('/login')
  } catch (error) {
    message.error(error instanceof Error ? error.message : 'Could not create admin account')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <main class="setup-page">
    <section class="setup-panel">
      <p class="eyebrow">First Run Setup</p>
      <h1>Create admin account</h1>
      <n-form @submit.prevent="submit">
        <n-form-item label="Username">
          <n-input v-model:value="username" autocomplete="username" />
        </n-form-item>
        <n-form-item label="Password">
          <n-input v-model:value="password" type="password" autocomplete="new-password" />
        </n-form-item>
        <n-button type="primary" block :loading="loading" @click="submit">Create Admin</n-button>
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
</style>
