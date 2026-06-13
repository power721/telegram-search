<script setup lang="ts">
import { computed, ref } from 'vue'
import type { ResourceItem } from '@/api/types'
import { telegramMessageHref } from '@/utils/telegramLinks'
import { vLazyLoad } from '@/directives/lazyLoad'

const props = withDefaults(defineProps<{
  items: ResourceItem[]
  selectedIds?: string[]
}>(), {
  selectedIds: () => []
})

const emit = defineEmits<{
  delete: [item: ResourceItem]
  toggleSelect: [item: ResourceItem, selected: boolean]
  toggleSelectAll: [selected: boolean]
}>()

const failedImageThumbs = ref(new Set<string>())
const activeVideo = ref<ResourceItem | null>(null)
const videoDialogVisible = ref(false)
const isVideoMaximized = ref(false)
const selectedSet = computed(() => new Set(props.selectedIds))
const selectedItemsCount = computed(() => props.items.filter((item) => selectedSet.value.has(item.id)).length)
const allCurrentPageSelected = computed(() => props.items.length > 0 && selectedItemsCount.value === props.items.length)
const partlyCurrentPageSelected = computed(() => selectedItemsCount.value > 0 && !allCurrentPageSelected.value)

function toggleResourceSelection(item: ResourceItem, event: Event) {
  emit('toggleSelect', item, (event.target as HTMLInputElement).checked)
}

function toggleCurrentPageSelection(event: Event) {
  emit('toggleSelectAll', (event.target as HTMLInputElement).checked)
}

function showImageThumb(item: ResourceItem) {
  return Boolean(item.media?.image_url && !failedImageThumbs.value.has(item.id))
}

function showPlayableVideo(item: ResourceItem) {
  return Boolean(item.media?.video_url)
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

function cloudDriveTypeLabel(type?: string) {
  if (!type || type === 'url') return ''
  const labels: Record<string, string> = {
    '115': '115云盘',
    '123': '123网盘',
    aliyun: '阿里云盘',
    baidu: '百度网盘',
    guangya: '光鸭云盘',
    jianguoyun: '坚果云',
    lanzou: '蓝奏云',
    mobile: '移动云盘',
    pikpak: 'PikPak',
    quark: '夸克网盘',
    tianyi: '天翼云盘',
    uc: 'UC网盘',
    weiyun: '微云',
    xunlei: '迅雷云盘'
  }
  return labels[type] ?? type
}

function fileTypeLabel(item: ResourceItem) {
  const type = item.type?.trim()
  const labels: Record<string, string> = {
    image: '图片',
    video: '视频',
    audio: '音频',
    document: '文档',
    ebook: '文档',
    archive: '压缩包',
    software: '软件',
    file: '文件'
  }
  if (type && type !== 'files') return labels[type] ?? type
  const mimeType = item.mime_type?.toLowerCase() ?? ''
  const extension = item.extension?.toLowerCase() ?? ''
  if (mimeType.startsWith('image/') || ['.jpg', '.jpeg', '.png', '.webp', '.gif'].includes(extension)) return '图片'
  if (mimeType.startsWith('video/') || ['.mp4', '.mkv', '.avi', '.mov', '.webm'].includes(extension)) return '视频'
  if (mimeType.startsWith('audio/') || ['.mp3', '.m4a', '.ogg', '.opus', '.flac', '.wav'].includes(extension)) return '音频'
  if (['.zip', '.rar', '.7z', '.tar', '.gz', '.bz2', '.xz'].includes(extension)) return '压缩包'
  if (['.pdf', '.epub', '.mobi', '.doc', '.docx', '.xls', '.xlsx', '.ppt', '.pptx', '.txt', '.rtf', '.md', '.csv'].includes(extension)) return '文档'
  return ''
}

function resourceTypeLabel(item: ResourceItem) {
  if (item.kind === 'file' && item.category === 'files') return fileTypeLabel(item) || categoryLabel(item.category)
  if (item.category !== 'cloud_drive') return categoryLabel(item.category)
  return cloudDriveTypeLabel(item.type) || categoryLabel(item.category)
}

function itemLabel(item: ResourceItem) {
  return item.media?.title || item.title || item.file_name || item.url || '-'
}

function openVideoPlayer(item: ResourceItem) {
  if (!item.media?.video_url) return
  activeVideo.value = item
  isVideoMaximized.value = false
  videoDialogVisible.value = true
}

function closeVideoPlayer() {
  videoDialogVisible.value = false
  isVideoMaximized.value = false
  activeVideo.value = null
}

function handleVideoDialogVisibleUpdate(show: boolean) {
  if (show) {
    videoDialogVisible.value = true
    return
  }
  closeVideoPlayer()
}

function toggleVideoMaximized() {
  isVideoMaximized.value = !isVideoMaximized.value
}

function mediaMetaParts(item: ResourceItem) {
  return [
    item.media?.year,
    item.media?.season,
    item.media?.episode,
    item.media?.quality,
    item.media?.size || fileSizeLabel(item),
    item.media?.category,
    item.media?.tmdb_id ? `TMDB ${item.media.tmdb_id}` : ''
  ].filter(Boolean)
}

function fileSizeLabel(item: ResourceItem) {
  if (item.kind !== 'file' || !item.size_bytes) return ''
  return formatBytes(item.size_bytes)
}

function formatBytes(value?: number) {
  if (!value) return '-'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let bytes = value
  let unit = 0
  while (bytes >= 1024 && unit < units.length - 1) {
    bytes /= 1024
    unit += 1
  }
  return `${bytes >= 10 || unit === 0 ? bytes.toFixed(0) : bytes.toFixed(1)} ${units[unit]}`
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
      <label class="select-cell">
        <input
          :checked="allCurrentPageSelected"
          :disabled="items.length === 0"
          :indeterminate="partlyCurrentPageSelected"
          aria-label="选择当前页全部资源"
          type="checkbox"
          @change="toggleCurrentPageSelection"
        />
      </label>
      <span>资源</span>
      <span>类型</span>
      <span>来源</span>
      <span>发布时间</span>
      <span>操作</span>
    </div>
    <div v-if="items.length === 0" class="empty-state">
      <strong>暂无资源</strong>
      <span>调整筛选条件或同步频道后，资源会显示在这里。</span>
    </div>
    <template v-for="item in items" :key="item.id">
      <article class="table-row">
        <label class="select-cell">
          <input
            :aria-label="`选择资源 ${itemLabel(item)}`"
            :checked="selectedSet.has(item.id)"
            type="checkbox"
            @change="toggleResourceSelection(item, $event)"
          />
        </label>
        <div class="resource-cell">
          <button
            v-if="showPlayableVideo(item)"
            class="resource-thumb-button"
            type="button"
            :aria-label="`播放视频 ${itemLabel(item)}`"
            @click="openVideoPlayer(item)"
          >
            <img
              v-if="showImageThumb(item)"
              v-lazy-load
              class="resource-thumb"
              :data-src="item.media?.image_url"
              alt=""
              @error="markImageThumbFailed(item)"
            />
            <img
              v-if="showImageThumb(item)"
              v-lazy-load
              class="resource-thumb-preview"
              :data-src="item.media?.image_url"
              alt=""
              aria-hidden="true"
            />
            <span v-else class="resource-thumb resource-video-placeholder" aria-hidden="true"></span>
          </button>
          <span v-else-if="showImageThumb(item)" class="resource-thumb-frame">
            <img
              v-lazy-load
              class="resource-thumb"
              :data-src="item.media?.image_url"
              alt=""
              @error="markImageThumbFailed(item)"
            />
            <img
              v-lazy-load
              class="resource-thumb-preview"
              :data-src="item.media?.image_url"
              alt=""
              aria-hidden="true"
            />
          </span>
          <div class="resource-copy">
            <strong>
              <a
                v-if="messageHref(item)"
                class="title-link"
                :href="messageHref(item)"
                rel="noopener noreferrer"
                target="_blank"
              >
                {{ itemLabel(item) }}
              </a>
              <template v-else>{{ itemLabel(item) }}</template>
            </strong>
            <p v-if="item.url">
              <a class="external-link" :href="item.url" rel="noopener noreferrer" target="_blank">{{ item.url }}</a>
            </p>
            <p v-else>{{ item.file_name || '-' }}</p>
            <p v-if="mediaMetaParts(item).length > 0" class="media-meta">{{ mediaMetaParts(item).join(' · ') }}</p>
            <p v-if="item.media?.tags" class="media-tags">{{ item.media.tags }}</p>
          </div>
        </div>
        <span>{{ resourceTypeLabel(item) }}</span>
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
        <span class="resource-actions">
          <button class="delete-resource-button" type="button" @click="emit('delete', item)">删除</button>
        </span>
      </article>
    </template>
    <n-modal :block-scroll="false" :show="videoDialogVisible" @update:show="handleVideoDialogVisibleUpdate">
      <n-card
        v-if="activeVideo"
        class="video-player-dialog"
        :class="{ 'is-maximized': isVideoMaximized }"
        :bordered="false"
      >
        <div class="video-player-header">
          <h2>{{ itemLabel(activeVideo) }}</h2>
          <div class="video-player-actions">
            <n-button
              :aria-label="isVideoMaximized ? '还原播放窗口' : '最大化播放窗口'"
              circle
              quaternary
              size="small"
              @click="toggleVideoMaximized"
            >
              <svg
                v-if="isVideoMaximized"
                class="video-player-action-icon"
                viewBox="0 0 24 24"
                aria-hidden="true"
              >
                <path d="M9 3v6H3" />
                <path d="M15 21v-6h6" />
                <path d="M9 9 4 4" />
                <path d="m15 15 5 5" />
              </svg>
              <svg v-else class="video-player-action-icon" viewBox="0 0 24 24" aria-hidden="true">
                <path d="M8 3H3v5" />
                <path d="M16 3h5v5" />
                <path d="M21 16v5h-5" />
                <path d="M3 16v5h5" />
              </svg>
            </n-button>
            <n-button
              aria-label="关闭视频播放"
              class="video-player-close"
              circle
              quaternary
              size="small"
              @click="closeVideoPlayer"
            >
              <svg class="video-player-close-icon" viewBox="0 0 24 24" aria-hidden="true">
                <path d="M18 6 6 18" />
                <path d="m6 6 12 12" />
              </svg>
            </n-button>
          </div>
        </div>
        <video
          :key="activeVideo.id"
          class="video-player"
          :poster="activeVideo.media?.image_url"
          :src="activeVideo.media?.video_url"
          autoplay
          controls
          playsinline
          preload="metadata"
        ></video>
      </n-card>
    </n-modal>
  </div>
</template>

<style scoped>
.resource-table {
  --resource-thumb-preview-width: 600px;
  overflow: visible;
  width: 100%;
}

.table-head,
.table-row {
  display: grid;
  gap: 12px;
  grid-template-columns: 28px minmax(0, 1fr) 120px 150px 180px 72px;
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

.select-cell {
  align-items: center;
  display: flex;
  justify-content: center;
}

.select-cell input {
  accent-color: var(--app-accent);
  height: 16px;
  width: 16px;
}

.resource-copy {
  min-width: 0;
}

.resource-thumb-frame {
  align-items: center;
  display: flex;
  flex: 0 0 88px;
  height: 55px;
  justify-content: center;
  position: relative;
  width: 88px;
}

.resource-thumb-button {
  align-items: center;
  background: transparent;
  border: 0;
  cursor: pointer;
  display: flex;
  flex: 0 0 88px;
  height: 55px;
  justify-content: center;
  padding: 0;
  position: relative;
  width: 88px;
}

.resource-thumb-button:hover .resource-thumb {
  border-color: var(--app-accent);
}

.resource-thumb-button::before,
.resource-thumb-button::after {
  content: "";
  left: 50%;
  pointer-events: none;
  position: absolute;
  top: 50%;
  transform: translate(-50%, -50%);
  z-index: 1;
}

.resource-thumb-button::before {
  background: rgba(0, 0, 0, 0.52);
  border-radius: 999px;
  height: 28px;
  width: 28px;
}

.resource-thumb-button::after {
  border-bottom: 7px solid transparent;
  border-left: 11px solid #fff;
  border-top: 7px solid transparent;
  margin-left: 2px;
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

.resource-video-placeholder {
  background:
    linear-gradient(90deg, rgba(255, 255, 255, 0.12) 12.5%, transparent 12.5% 87.5%, rgba(255, 255, 255, 0.12) 87.5%),
    linear-gradient(135deg, #24292f, #57606a);
  display: block;
}

.resource-thumb-preview {
  background: var(--app-bg-muted);
  border: 1px solid var(--app-border);
  border-radius: 8px;
  box-shadow: 0 14px 34px rgba(15, 23, 42, 0.18);
  display: block;
  height: auto;
  left: calc(100% + 10px);
  max-height: calc(100vh - 32px);
  max-width: min(var(--resource-thumb-preview-width), calc(100vw - 32px));
  opacity: 0;
  object-fit: contain;
  pointer-events: none;
  position: absolute;
  top: 50%;
  transform: translateY(-50%) scale(0.96);
  transform-origin: left center;
  transition:
    opacity 0.16s ease,
    transform 0.16s ease;
  visibility: hidden;
  width: auto;
  z-index: 20;
}

.resource-thumb-frame:hover .resource-thumb-preview,
.resource-thumb-button:hover .resource-thumb-preview {
  opacity: 1;
  transform: translateY(-50%) scale(1);
  visibility: visible;
}

.external-link,
.title-link,
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

.title-link,
.channel-link {
  color: inherit;
}

.title-link:hover,
.channel-link:hover {
  color: var(--app-accent);
}

.table-row time {
  color: var(--app-text-muted);
}

.resource-actions {
  align-items: center;
  display: flex;
  justify-content: flex-end;
}

.delete-resource-button {
  background: transparent;
  border: 1px solid color-mix(in srgb, var(--app-danger, #d03050) 35%, var(--app-border));
  border-radius: 6px;
  color: var(--app-danger, #d03050);
  cursor: pointer;
  font: inherit;
  min-height: 30px;
  padding: 4px 10px;
}

.delete-resource-button:hover {
  background: color-mix(in srgb, var(--app-danger, #d03050) 9%, transparent);
}

.video-player-dialog {
  max-width: min(1200px, calc(100vw - 32px));
  width: 1200px;
}

.video-player-dialog.is-maximized {
  max-width: calc(100vw - 24px);
  width: calc(100vw - 24px);
}

.video-player-header {
  align-items: center;
  display: flex;
  gap: 12px;
  justify-content: space-between;
  margin-bottom: 12px;
}

.video-player-header h2 {
  color: var(--app-heading);
  font-size: 16px;
  font-weight: 650;
  line-height: 1.35;
  margin: 0;
  min-width: 0;
  overflow-wrap: anywhere;
}

.video-player-actions {
  align-items: center;
  display: flex;
  flex: 0 0 auto;
  gap: 6px;
}

.video-player-action-icon {
  fill: none;
  height: 16px;
  stroke: currentColor;
  stroke-linecap: round;
  stroke-linejoin: round;
  stroke-width: 2;
  width: 16px;
}

.video-player-close-icon {
  fill: none;
  height: 20px;
  stroke: currentColor;
  stroke-linecap: round;
  stroke-linejoin: round;
  stroke-width: 2.4;
  width: 20px;
}

.video-player {
  aspect-ratio: 16 / 9;
  background: #000;
  border-radius: 6px;
  display: block;
  max-height: calc(100vh - 180px);
  width: 100%;
}

.video-player-dialog.is-maximized .video-player {
  max-height: calc(100vh - 124px);
}

@media (max-width: 760px) {
  .table-head {
    display: none;
  }

  .table-row {
    grid-template-columns: 1fr;
  }

  .select-cell {
    justify-content: flex-start;
  }

  .select-cell input {
    height: 20px;
    width: 20px;
  }

  .resource-actions {
    justify-content: flex-start;
  }

  .resource-thumb-preview {
    left: 0;
    top: calc(100% + 8px);
    transform: scale(0.96);
    transform-origin: top left;
  }

  .resource-thumb-frame:hover .resource-thumb-preview,
  .resource-thumb-button:hover .resource-thumb-preview {
    transform: scale(1);
  }
}
</style>
