<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { ChannelControlPayload, TelegramChannel } from '@/api/types'
import SyncProfileSelect from './SyncProfileSelect.vue'

const props = defineProps<{
  show: boolean
  channel: TelegramChannel | null
  loading?: boolean
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
  save: [payload: ChannelControlPayload]
}>()

const form = ref<ChannelControlPayload>({
  history_sync_enabled: false,
  sync_profile: 'Normal',
  listen_enabled: false,
  remote_search_allowed: true
})

const title = computed(() => props.channel?.title ?? '频道控制')

watch(
  () => props.channel,
  (channel) => {
    if (!channel) return
    form.value = {
      history_sync_enabled: channel.history_sync_enabled,
      sync_profile: channel.sync_profile,
      listen_enabled: channel.listen_enabled,
      remote_search_allowed: channel.remote_search_allowed
    }
  },
  { immediate: true }
)

function save() {
  if (form.value.sync_profile === 'Full' && !window.confirm('确定使用完整同步档位？')) {
    return
  }
  emit('save', { ...form.value })
}
</script>

<template>
  <n-drawer :show="show" width="420" @update:show="emit('update:show', $event)">
    <n-drawer-content :title="title">
      <div class="control-form">
        <label>
          历史同步
          <n-switch v-model:value="form.history_sync_enabled" />
        </label>
        <label>
          同步档位
          <SyncProfileSelect v-model:value="form.sync_profile" />
        </label>
        <label>
          监听
          <n-switch v-model:value="form.listen_enabled" />
        </label>
        <label>
          远程搜索
          <n-switch v-model:value="form.remote_search_allowed" />
        </label>
        <n-button type="primary" :loading="loading" @click="save">保存</n-button>
      </div>
    </n-drawer-content>
  </n-drawer>
</template>

<style scoped>
.control-form {
  display: grid;
  gap: 16px;
}

label {
  color: var(--app-text);
  display: grid;
  gap: 8px;
}
</style>
