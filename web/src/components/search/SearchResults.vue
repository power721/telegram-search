<script setup lang="ts">
import type { GlobalSearchResult, RemoteSearchItem } from '@/api/types'

defineProps<{
  result: GlobalSearchResult | null
  remoteItems?: RemoteSearchItem[]
  loading?: boolean
}>()

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
          <strong>{{ item.channel_title || 'Telegram' }}</strong>
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
        </article>
      </template>
      <template v-for="item in remoteItems ?? []" :key="`r-${item.telegram_message_id}`">
        <article class="result-row">
          <strong>{{ item.channel_title || '远程结果' }}</strong>
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
          <strong>{{ item.note || item.url }}</strong>
          <p>
            <a class="external-link" :href="item.url" rel="noopener noreferrer" target="_blank">{{ item.url }}</a>
          </p>
          <small class="status-pill status-info">{{ sourceLabel(item.source) }}</small>
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
          <strong>{{ item.file_name }}</strong>
          <p>{{ item.extension }} {{ item.mime_type }}</p>
          <small class="status-pill status-info">{{ sourceLabel(item.source) }}</small>
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
        <strong>{{ item.title }}</strong>
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
  display: grid;
  gap: 14px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.result-section {
  min-height: 148px;
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
  text-decoration: none;
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

.loading-stack {
  display: grid;
  gap: 8px;
  padding: 8px 0;
}

.loading-stack .short {
  width: 62%;
}

.external-link {
  color: inherit;
  text-decoration: none;
}

.external-link:hover {
  color: var(--app-accent);
}

@media (max-width: 900px) {
  .search-results {
    grid-template-columns: 1fr;
  }
}
</style>
