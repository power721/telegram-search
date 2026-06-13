<script setup lang="ts">
import { ref } from 'vue'
import type { GlobalSearchResult, MediaURLs, RemoteSearchItem } from '@/api/types'
import { telegramChannelHref, telegramMessageHref } from '@/utils/telegramLinks'
import { vLazyLoad } from '@/directives/lazyLoad'

defineProps<{
  result: GlobalSearchResult | null
  remoteItems?: RemoteSearchItem[]
  loading?: boolean
}>()

const failedImageThumbs = ref(new Set<string>())

function mediaThumbKey(
  prefix: string,
  item: { id?: number | string; telegram_message_id?: number; media?: MediaURLs }
) {
  return `${prefix}-${item.id ?? item.telegram_message_id ?? 'item'}-${item.media?.image_url ?? ''}`
}

function showImageThumb(
  prefix: string,
  item: { id?: number | string; telegram_message_id?: number; media?: MediaURLs }
) {
  return Boolean(item.media?.image_url && !failedImageThumbs.value.has(mediaThumbKey(prefix, item)))
}

function showVideoThumb(item: { media?: MediaURLs }) {
  return Boolean(item.media?.video_url)
}

function markImageThumbFailed(
  prefix: string,
  item: { id?: number | string; telegram_message_id?: number; media?: MediaURLs }
) {
  failedImageThumbs.value = new Set(failedImageThumbs.value).add(mediaThumbKey(prefix, item))
}

function sourceLabel(source?: string) {
  const labels: Record<string, string> = {
    local: '本地',
    remote: '远程'
  }
  const label = labels[source ?? ''] ?? source
  return label || '本地'
}

function messageTextSegments(text: string) {
  const segments: Array<{ text: string; href?: string }> = []
  const urlPattern = /https?:\/\/[^\s<>"'，。！？；：、]+/gi
  let lastIndex = 0
  for (const match of text.matchAll(urlPattern)) {
    const rawHref = match[0]
    const trailing = rawHref.match(/[.,!?;:)\]}]+$/)?.[0] ?? ''
    const href = trailing ? rawHref.slice(0, -trailing.length) : rawHref
    const index = match.index ?? 0
    if (index > lastIndex) {
      segments.push({ text: text.slice(lastIndex, index) })
    }
    segments.push({ text: href, href })
    if (trailing) {
      segments.push({ text: trailing })
    }
    lastIndex = index + rawHref.length
  }
  if (lastIndex < text.length) {
    segments.push({ text: text.slice(lastIndex) })
  }
  return segments.length > 0 ? segments : [{ text }]
}

function messageHref(item: {
  channel_username?: string
  telegram_channel_id?: number
  telegram_message_id?: number
}) {
  return telegramMessageHref(item)
}

function channelHref(item: { username?: string }) {
  return telegramChannelHref(item)
}

function linkTitle(item: { media_title?: string; note?: string; url?: string }) {
  return item.media_title || item.note || item.url || '-'
}

function linkMetaParts(item: {
  media_year?: string
  media_season?: string
  media_episode?: string
  media_quality?: string
  media_size?: string
  media_category?: string
  media_tmdb_id?: string
}) {
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
</script>

<template>
  <div class="search-results">
    <section class="result-section">
      <header>
        <h2>消息</h2>
        <span>{{ result?.messages.total ?? 0 }}</span>
      </header>
      <div v-if="loading" class="loading-stack" aria-label="正在加载消息结果">
        <span class="skeleton-line" />
        <span class="skeleton-line" />
        <span class="skeleton-line short" />
      </div>
      <template v-for="item in result?.messages.items ?? []" :key="`m-${item.id}`">
        <article class="result-row">
          <div class="result-content">
            <span v-if="showImageThumb('message', item)" class="search-thumb-frame">
              <img
                v-lazy-load
                class="search-thumb"
                :data-src="item.media?.image_url"
                alt=""
                @error="markImageThumbFailed('message', item)"
              />
              <img
                v-lazy-load
                class="search-thumb-preview"
                :data-src="item.media?.image_url"
                alt=""
                aria-hidden="true"
              />
            </span>
            <video
              v-else-if="showVideoThumb(item)"
              class="search-thumb"
              :poster="item.media?.image_url"
              :src="item.media?.video_url"
              muted
              playsinline
              preload="none"
            ></video>
            <div class="result-copy">
              <strong>
                <a
                  v-if="messageHref(item)"
                  class="title-link"
                  :href="messageHref(item)"
                  rel="noopener noreferrer"
                  target="_blank"
                >
                  {{ item.channel_title || 'Telegram' }}
                </a>
                <template v-else>{{ item.channel_title || 'Telegram' }}</template>
              </strong>
              <p class="message-text">
                <template v-for="(segment, index) in messageTextSegments(item.text)" :key="index">
                  <a
                    v-if="segment.href"
                    class="external-link"
                    :href="segment.href"
                    rel="noopener noreferrer"
                    target="_blank"
                  >
                    {{ segment.text }}
                  </a>
                  <template v-else>{{ segment.text }}</template>
                </template>
              </p>
              <small class="status-pill status-info">{{ sourceLabel(item.source) }}</small>
            </div>
          </div>
        </article>
      </template>
      <template v-for="item in remoteItems ?? []" :key="`r-${item.telegram_message_id}`">
        <article class="result-row">
          <div class="result-content">
            <span v-if="showImageThumb('remote', item)" class="search-thumb-frame">
              <img
                v-lazy-load
                class="search-thumb"
                :data-src="item.media?.image_url"
                alt=""
                @error="markImageThumbFailed('remote', item)"
              />
              <img
                v-lazy-load
                class="search-thumb-preview"
                :data-src="item.media?.image_url"
                alt=""
                aria-hidden="true"
              />
            </span>
            <video
              v-else-if="showVideoThumb(item)"
              class="search-thumb"
              :poster="item.media?.image_url"
              :src="item.media?.video_url"
              muted
              playsinline
              preload="none"
            ></video>
            <div class="result-copy">
              <strong>
                <a
                  v-if="messageHref(item)"
                  class="title-link"
                  :href="messageHref(item)"
                  rel="noopener noreferrer"
                  target="_blank"
                >
                  {{ item.channel_title || '远程结果' }}
                </a>
                <template v-else>{{ item.channel_title || '远程结果' }}</template>
              </strong>
              <p class="message-text">
                <template v-for="(segment, index) in messageTextSegments(item.text)" :key="index">
                  <a
                    v-if="segment.href"
                    class="external-link"
                    :href="segment.href"
                    rel="noopener noreferrer"
                    target="_blank"
                  >
                    {{ segment.text }}
                  </a>
                  <template v-else>{{ segment.text }}</template>
                </template>
              </p>
              <small class="status-pill status-warning">{{ sourceLabel(item.source) }}</small>
            </div>
          </div>
        </article>
      </template>
      <div
        v-if="!loading && (result?.messages.items?.length ?? 0) === 0 && (remoteItems?.length ?? 0) === 0"
        class="empty-state"
      >
        <strong>暂无消息结果</strong>
        <span>尝试更具体的关键词，或先同步相关频道。</span>
      </div>
    </section>

    <section class="result-section">
      <header>
        <h2>链接</h2>
        <span>{{ result?.links.total ?? 0 }}</span>
      </header>
      <div v-if="loading" class="loading-stack" aria-label="正在加载链接结果">
        <span class="skeleton-line" />
        <span class="skeleton-line short" />
      </div>
      <template v-for="item in result?.links.items ?? []" :key="`l-${item.id}`">
        <article class="result-row">
          <div class="result-content">
            <span v-if="showImageThumb('link', item)" class="search-thumb-frame">
              <img
                v-lazy-load
                class="search-thumb"
                :data-src="item.media?.image_url"
                alt=""
                @error="markImageThumbFailed('link', item)"
              />
              <img
                v-lazy-load
                class="search-thumb-preview"
                :data-src="item.media?.image_url"
                alt=""
                aria-hidden="true"
              />
            </span>
            <video
              v-else-if="showVideoThumb(item)"
              class="search-thumb"
              :poster="item.media?.image_url"
              :src="item.media?.video_url"
              muted
              playsinline
              preload="none"
            ></video>
            <div class="result-copy">
              <strong>
                <a
                  v-if="messageHref(item)"
                  class="title-link"
                  :href="messageHref(item)"
                  rel="noopener noreferrer"
                  target="_blank"
                >
                  {{ linkTitle(item) }}
                </a>
                <template v-else>{{ linkTitle(item) }}</template>
              </strong>
              <p>
                <a class="external-link" :href="item.url" rel="noopener noreferrer" target="_blank">{{ item.url }}</a>
              </p>
              <p v-if="linkMetaParts(item).length > 0" class="link-meta">{{ linkMetaParts(item).join(' · ') }}</p>
              <p v-if="item.media_tags" class="link-meta">{{ item.media_tags }}</p>
              <small class="status-pill status-info">{{ sourceLabel(item.source) }}</small>
            </div>
          </div>
        </article>
      </template>
      <div v-if="!loading && (result?.links.items?.length ?? 0) === 0" class="empty-state">
        <strong>暂无链接结果</strong>
        <span>资源链接会在本地索引后出现在这里。</span>
      </div>
    </section>

    <section class="result-section">
      <header>
        <h2>文件</h2>
        <span>{{ result?.files.total ?? 0 }}</span>
      </header>
      <div v-if="loading" class="loading-stack" aria-label="正在加载文件结果">
        <span class="skeleton-line" />
        <span class="skeleton-line short" />
      </div>
      <template v-for="item in result?.files.items ?? []" :key="`f-${item.id}`">
        <article class="result-row">
          <div class="result-content">
            <span v-if="showImageThumb('file', item)" class="search-thumb-frame">
              <img
                v-lazy-load
                class="search-thumb"
                :data-src="item.media?.image_url"
                alt=""
                @error="markImageThumbFailed('file', item)"
              />
              <img
                v-lazy-load
                class="search-thumb-preview"
                :data-src="item.media?.image_url"
                alt=""
                aria-hidden="true"
              />
            </span>
            <video
              v-else-if="showVideoThumb(item)"
              class="search-thumb"
              :poster="item.media?.image_url"
              :src="item.media?.video_url"
              muted
              playsinline
              preload="none"
            ></video>
            <div class="result-copy">
              <strong>
                <a
                  v-if="messageHref(item)"
                  class="title-link"
                  :href="messageHref(item)"
                  rel="noopener noreferrer"
                  target="_blank"
                >
                  {{ item.file_name }}
                </a>
                <template v-else>{{ item.file_name }}</template>
              </strong>
              <p>{{ item.extension }} {{ item.mime_type }}</p>
              <small class="status-pill status-info">{{ sourceLabel(item.source) }}</small>
            </div>
          </div>
        </article>
      </template>
      <div v-if="!loading && (result?.files.items?.length ?? 0) === 0" class="empty-state">
        <strong>暂无文件结果</strong>
        <span>文件元数据会在历史同步后进入搜索。</span>
      </div>
    </section>

    <section class="result-section">
      <header>
        <h2>频道</h2>
        <span>{{ result?.channels.total ?? 0 }}</span>
      </header>
      <div v-if="loading" class="loading-stack" aria-label="正在加载频道结果">
        <span class="skeleton-line" />
        <span class="skeleton-line short" />
      </div>
      <article v-for="item in result?.channels.items ?? []" :key="`c-${item.id}`" class="result-row">
        <strong>
          <a
            v-if="channelHref(item)"
            class="title-link"
            :href="channelHref(item)"
            rel="noopener noreferrer"
            target="_blank"
          >
            {{ item.title }}
          </a>
          <template v-else>{{ item.title }}</template>
        </strong>
        <p>@{{ item.username || '私有频道' }}</p>
        <small class="status-pill status-info">{{ sourceLabel(item.source) }}</small>
      </article>
      <div v-if="!loading && (result?.channels.items?.length ?? 0) === 0" class="empty-state">
        <strong>暂无频道结果</strong>
        <span>频道元数据同步完成后可以被搜索。</span>
      </div>
    </section>
  </div>
</template>

<style scoped>
.search-results {
  --search-thumb-preview-width: 600px;
  display: grid;
  gap: 14px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  overflow: visible;
}

.result-section {
  min-height: 148px;
  overflow: visible;
  padding: 12px;
}

header {
  align-items: center;
  display: flex;
  justify-content: space-between;
  margin-bottom: 10px;
}

h2 {
  font-size: 16px;
  margin: 0;
}

header span,
small {
  color: var(--app-text-muted);
}

.result-row {
  border-top: 1px solid var(--app-border-subtle);
  color: inherit;
  display: block;
  padding: 10px 0;
  position: relative;
  text-decoration: none;
}

.result-row:hover {
  z-index: 3;
}

.result-row:first-of-type {
  border-top: 0;
}

.result-row strong,
.result-row p,
.result-row a {
  overflow-wrap: anywhere;
}

.result-row p {
  color: var(--app-text-muted);
  margin: 4px 0;
}

.result-content {
  align-items: center;
  display: flex;
  gap: 10px;
  min-width: 0;
}

.result-copy {
  min-width: 0;
}

.link-meta {
  font-size: 12px;
  line-height: 1.35;
}

.search-thumb-frame {
  align-items: center;
  display: flex;
  flex: 0 0 88px;
  height: 55px;
  justify-content: center;
  position: relative;
  width: 88px;
}

.search-thumb {
  aspect-ratio: 16 / 10;
  background: var(--app-bg-muted);
  border: 1px solid var(--app-border-subtle);
  border-radius: 6px;
  flex: 0 0 88px;
  height: 55px;
  object-fit: cover;
  width: 88px;
}

.search-thumb-preview {
  background: var(--app-bg-muted);
  border: 1px solid var(--app-border);
  border-radius: 8px;
  box-shadow: 0 14px 34px rgba(15, 23, 42, 0.18);
  display: block;
  height: auto;
  left: calc(100% + 10px);
  max-height: calc(100vh - 32px);
  max-width: min(var(--search-thumb-preview-width), calc(100vw - 32px));
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

.search-thumb-frame:hover .search-thumb-preview {
  opacity: 1;
  transform: translateY(-50%) scale(1);
  visibility: visible;
}

.loading-stack {
  display: grid;
  gap: 8px;
  padding: 8px 0;
}

.loading-stack .short {
  width: 62%;
}

.external-link {
  color: var(--app-accent);
  text-decoration: underline;
  text-underline-offset: 2px;
}

.title-link {
  color: inherit;
  text-decoration: none;
}

.title-link:hover,
.external-link:hover {
  color: var(--app-accent-hover);
}

@media (max-width: 900px) {
  .search-results {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 760px) {
  .search-thumb-preview {
    left: 0;
    top: calc(100% + 8px);
    transform: scale(0.96);
    transform-origin: top left;
  }

  .search-thumb-frame:hover .search-thumb-preview {
    transform: scale(1);
  }
}
</style>
