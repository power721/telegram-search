<script setup lang="ts">
import { ref } from 'vue'
import type { ResourceItem } from '@/api/types'
import { telegramMessageHref } from '@/utils/telegramLinks'

defineProps<{
  items: ResourceItem[]
}>()

const failedImageThumbs = ref(new Set<string>())

function showImageThumb(item: ResourceItem) {
  return Boolean(item.media?.image_url && !failedImageThumbs.value.has(item.id))
}

function showVideoThumb(item: ResourceItem) {
  return Boolean(item.media?.video_url && (!item.media?.image_url || failedImageThumbs.value.has(item.id)))
}

function markImageThumbFailed(item: ResourceItem) {
  failedImageThumbs.value = new Set(failedImageThumbs.value).add(item.id)
}

function categoryLabel(category: string) {
  const labels: Record<string, string> = {
    cloud_drive: '网盘',
    magnet: '磁力',
    ed2k: 'ED2K',
    http: 'HTTP',
    files: '文件'
  }
  return labels[category] ?? category
}

function itemLabel(item: ResourceItem) {
  return item.title || item.file_name || item.url || '-'
}

function messageHref(item: ResourceItem) {
  return telegramMessageHref(item)
}

function formatDate(value?: string) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return new Intl.DateTimeFormat('zh-CN', {
    dateStyle: 'medium',
    timeStyle: 'short'
  }).format(date)
}
</script>

<template>
  <div class="resource-table data-table">
    <div class="table-head sticky-head">
      <span>资源</span>
      <span>类型</span>
      <span>来源</span>
      <span>发布时间</span>
    </div>
    <div v-if="items.length === 0" class="empty-state">
      <strong>暂无资源</strong>
      <span>调整筛选条件或同步频道后，资源会显示在这里。</span>
    </div>
    <template v-for="item in items" :key="item.id">
      <article class="table-row">
        <div class="resource-cell">
          <img
            v-if="showImageThumb(item)"
            class="resource-thumb"
            :src="item.media?.image_url"
            alt=""
            loading="lazy"
            @error="markImageThumbFailed(item)"
          />
          <video
            v-else-if="showVideoThumb(item)"
            class="resource-thumb"
            :poster="item.media?.image_url"
            :src="item.media?.video_url"
            muted
            playsinline
            preload="metadata"
          ></video>
          <div class="resource-copy">
            <strong>{{ itemLabel(item) }}</strong>
            <p v-if="item.url">
              <a class="external-link" :href="item.url" rel="noopener noreferrer" target="_blank">{{ item.url }}</a>
            </p>
            <p v-else>{{ item.file_name || '-' }}</p>
          </div>
        </div>
        <span>{{ categoryLabel(item.category) }}</span>
        <span>
          <a
            v-if="messageHref(item)"
            class="channel-link"
            :href="messageHref(item)"
            rel="noopener noreferrer"
            target="_blank"
          >
            {{ item.channel_title || 'Telegram' }}
          </a>
          <template v-else>{{ item.channel_title || 'Telegram' }}</template>
        </span>
        <time :datetime="item.datetime">{{ formatDate(item.datetime) }}</time>
      </article>
    </template>
  </div>
</template>

<style scoped>
.resource-table {
  overflow: hidden;
  width: 100%;
}

.table-head,
.table-row {
  display: grid;
  gap: 12px;
  grid-template-columns: minmax(0, 1fr) 120px 150px 180px;
  padding: 8px 10px;
}

.table-row {
  align-items: center;
  border-top: 1px solid var(--app-border-subtle);
  color: inherit;
  text-decoration: none;
}

.table-row strong,
.table-row p,
.table-row time {
  overflow-wrap: anywhere;
}

.table-row p {
  color: var(--app-text-muted);
  margin: 4px 0 0;
}

.resource-cell {
  align-items: center;
  display: flex;
  gap: 10px;
  min-width: 0;
}

.resource-copy {
  min-width: 0;
}

.resource-thumb {
  aspect-ratio: 16 / 10;
  background: var(--app-bg-muted);
  border: 1px solid var(--app-border-subtle);
  border-radius: 6px;
  flex: 0 0 88px;
  height: 55px;
  object-fit: cover;
  width: 88px;
}

.external-link,
.channel-link {
  text-decoration: none;
}

.external-link {
  color: var(--app-accent);
  text-decoration: underline;
  text-underline-offset: 2px;
}

.external-link:hover {
  color: var(--app-accent-hover);
}

.channel-link {
  color: inherit;
}

.channel-link:hover {
  color: var(--app-accent);
}

.table-row time {
  color: var(--app-text-muted);
}

@media (max-width: 760px) {
  .table-head {
    display: none;
  }

  .table-row {
    grid-template-columns: 1fr;
  }
}
</style>
