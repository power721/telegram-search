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
const ignoredLinkPatterns = ref('t.me, toapp.mypikpak.com, telegra.ph, www.themoviedb.org')
const messageTypes = ref(['link', 'text', 'image', 'video', 'audio'])
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
      link_types: linkTypes.value,
      ignored_link_patterns: terms(ignoredLinkPatterns.value)
    })
    message.success('监听规则已保存')
    await router.push('/setup/channels')
  } catch (error) {
    message.error(error instanceof Error ? error.message : '无法保存监听规则')
  }
}
</script>

<template>
  <main class="setup-page">
    <section class="setup-panel">
      <p class="eyebrow">首次运行设置</p>
      <h1>监听规则</h1>
      <n-form @submit.prevent="submit">
        <n-form-item label="包含关键词">
          <n-input v-model:value="includes" placeholder="多个关键词用英文逗号分隔" />
        </n-form-item>
        <n-form-item label="排除关键词">
          <n-input v-model:value="excludes" placeholder="多个关键词用英文逗号分隔" />
        </n-form-item>
        <n-form-item label="忽略链接">
          <n-input v-model:value="ignoredLinkPatterns" placeholder="t.me, *.t.me, example.com" />
        </n-form-item>
        <n-form-item label="消息类型">
          <n-checkbox-group v-model:value="messageTypes">
            <n-checkbox value="link">链接</n-checkbox>
            <n-checkbox value="image">图片</n-checkbox>
            <n-checkbox value="video">视频</n-checkbox>
            <n-checkbox value="audio">音频</n-checkbox>
            <n-checkbox value="file">文件</n-checkbox>
            <n-checkbox value="text">文本</n-checkbox>
          </n-checkbox-group>
        </n-form-item>
        <n-form-item label="链接类型">
          <n-checkbox-group v-model:value="linkTypes">
            <n-checkbox value="cloud_drive">网盘</n-checkbox>
            <n-checkbox value="magnet">磁力</n-checkbox>
            <n-checkbox value="ed2k">ED2K</n-checkbox>
            <n-checkbox value="other">其他</n-checkbox>
          </n-checkbox-group>
        </n-form-item>
        <n-button type="primary" block :loading="setup.loading" @click="submit">继续</n-button>
      </n-form>
    </section>
  </main>
</template>

<style scoped>
.setup-panel {
  max-width: 520px;
}

h1 {
  margin: 0 0 22px;
}
</style>
