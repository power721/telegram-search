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
    message.success('管理员账号已创建')
    await router.push('/login')
  } catch (error) {
    message.error(error instanceof Error ? error.message : '无法创建管理员账号')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <main class="setup-page">
    <section class="setup-panel">
      <p class="eyebrow">首次运行设置</p>
      <h1>创建管理员账号</h1>
      <n-form @submit.prevent="submit">
        <n-form-item label="用户名">
          <n-input v-model:value="username" autocomplete="username" />
        </n-form-item>
        <n-form-item label="密码">
          <n-input v-model:value="password" type="password" autocomplete="new-password" />
        </n-form-item>
        <n-button type="primary" block :loading="loading" @click="submit">创建管理员</n-button>
      </n-form>
    </section>
  </main>
</template>

<style scoped>
h1 {
  margin: 0 0 22px;
}
</style>
