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
  return item.media_title || item.title || item.file_name || item.url || '-'
}

function mediaMetaParts(item: ResourceItem) {
  return [
    item.media_year,
    item.media_season,
    item.media_episode,
    item.media_quality,
    item.media_size,
    item.media_category,
    item.media_tmdb_id ? `TMDB ${item.media_tmdb_id}` : ''
  ].filter(Boolean)
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
          <span v-if="showImageThumb(item)" class="resource-thumb-frame">
            <img
              class="resource-thumb"
              :src="item.media?.image_url"
              alt=""
              loading="lazy"
              @error="markImageThumbFailed(item)"
            />
            <img class="resource-thumb-preview" :src="item.media?.image_url" alt="" aria-hidden="true" />
          </span>
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
            <p v-if="mediaMetaParts(item).length > 0" class="media-meta">{{ mediaMetaParts(item).join(' · ') }}</p>
            <p v-if="item.media_tags" class="media-tags">{{ item.media_tags }}</p>
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
  --resource-thumb-preview-width: 480px;
  overflow: visible;
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
  position: relative;
  text-decoration: none;
}

.table-row:hover {
  z-index: 3;
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

.media-meta,
.media-tags {
  font-size: 12px;
  line-height: 1.35;
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

.resource-thumb-frame {
  flex: 0 0 88px;
  height: 55px;
  position: relative;
  width: 88px;
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

.resource-thumb-preview {
  aspect-ratio: 16 / 10;
  background: var(--app-bg-muted);
  border: 1px solid var(--app-border);
  border-radius: 8px;
  box-shadow: 0 14px 34px rgba(15, 23, 42, 0.18);
  height: auto;
  left: calc(100% + 10px);
  max-width: min(var(--resource-thumb-preview-width), calc(100vw - 32px));
  opacity: 0;
  object-fit: cover;
  pointer-events: none;
  position: absolute;
  top: 50%;
  transform: translateY(-50%) scale(0.96);
  transform-origin: left center;
  transition:
    opacity 0.16s ease,
    transform 0.16s ease;
  visibility: hidden;
  width: var(--resource-thumb-preview-width);
  z-index: 20;
}

.resource-thumb-frame:hover .resource-thumb-preview {
  opacity: 1;
  transform: translateY(-50%) scale(1);
  visibility: visible;
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

  .resource-thumb-preview {
    left: 0;
    top: calc(100% + 8px);
    transform: scale(0.96);
    transform-origin: top left;
    width: min(var(--resource-thumb-preview-width), calc(100vw - 32px));
  }

  .resource-thumb-frame:hover .resource-thumb-preview {
    transform: scale(1);
  }
}
</style>
