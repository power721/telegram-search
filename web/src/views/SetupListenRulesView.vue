<script setup lang="ts">
import { useMessage } from 'naive-ui'
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useSetupStore } from '@/stores/setup'

const router = useRouter()
const message = useMessage()
const setup = useSetupStore()

const includes = ref('')
const excludes = ref('')
const messageTypes = ref(['link', 'text'])
const linkTypes = ref(['cloud_drive', 'magnet', 'ed2k', 'other'])

function terms(value: string) {
  return value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
}

async function submit() {
  try {
    await setup.saveListenRules({
      includes: terms(includes.value),
      excludes: terms(excludes.value),
      message_types: messageTypes.value,
      link_types: linkTypes.value
    })
    message.success('Listen rules saved')
    await router.push('/setup/channels')
  } catch (error) {
    message.error(error instanceof Error ? error.message : 'Could not save listen rules')
  }
}
</script>

<template>
  <main class="setup-page">
    <section class="setup-panel">
      <p class="eyebrow">First Run Setup</p>
      <h1>Listen Rules</h1>
      <n-form @submit.prevent="submit">
        <n-form-item label="Includes">
          <n-input v-model:value="includes" placeholder="Comma separated keywords" />
        </n-form-item>
        <n-form-item label="Excludes">
          <n-input v-model:value="excludes" placeholder="Comma separated keywords" />
        </n-form-item>
        <n-form-item label="Message Types">
          <n-checkbox-group v-model:value="messageTypes">
            <n-checkbox value="link">Links</n-checkbox>
            <n-checkbox value="video">Video</n-checkbox>
            <n-checkbox value="audio">Audio</n-checkbox>
            <n-checkbox value="file">Files</n-checkbox>
            <n-checkbox value="text">Text</n-checkbox>
          </n-checkbox-group>
        </n-form-item>
        <n-form-item label="Link Types">
          <n-checkbox-group v-model:value="linkTypes">
            <n-checkbox value="cloud_drive">Cloud Drive</n-checkbox>
            <n-checkbox value="magnet">Magnet</n-checkbox>
            <n-checkbox value="ed2k">ED2K</n-checkbox>
            <n-checkbox value="other">Other</n-checkbox>
          </n-checkbox-group>
        </n-form-item>
        <n-button type="primary" block :loading="setup.loading" @click="submit">Continue</n-button>
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
  max-width: 520px;
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
