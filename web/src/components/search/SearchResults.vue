<script setup lang="ts">
import type { GlobalSearchResult, RemoteSearchItem } from '@/api/types'

defineProps<{
  result: GlobalSearchResult | null
  remoteItems?: RemoteSearchItem[]
}>()

function sourceLabel(source?: string) {
  const labels: Record<string, string> = {
    local: '本地',
    remote: '远程'
  }
  const label = labels[source ?? ''] ?? source
  return label || '本地'
}
</script>

<template>
  <div class="search-results">
    <section class="result-section">
      <header>
        <h2>消息</h2>
        <span>{{ result?.messages.total ?? 0 }}</span>
      </header>
      <article v-for="item in result?.messages.items ?? []" :key="`m-${item.id}`" class="result-row">
        <strong>{{ item.channel_title || 'Telegram' }}</strong>
        <p>{{ item.text }}</p>
        <small>{{ sourceLabel(item.source) }}</small>
      </article>
      <article v-for="item in remoteItems ?? []" :key="`r-${item.telegram_message_id}`" class="result-row">
        <strong>{{ item.channel_title || '远程结果' }}</strong>
        <p>{{ item.text }}</p>
        <small>{{ sourceLabel(item.source) }}</small>
      </article>
    </section>

    <section class="result-section">
      <header>
        <h2>链接</h2>
        <span>{{ result?.links.total ?? 0 }}</span>
      </header>
      <article v-for="item in result?.links.items ?? []" :key="`l-${item.id}`" class="result-row">
        <strong>{{ item.note || item.url }}</strong>
        <p>
          <a :href="item.url" rel="noopener noreferrer" target="_blank">{{ item.url }}</a>
        </p>
        <small>{{ sourceLabel(item.source) }}</small>
      </article>
    </section>

    <section class="result-section">
      <header>
        <h2>文件</h2>
        <span>{{ result?.files.total ?? 0 }}</span>
      </header>
      <article v-for="item in result?.files.items ?? []" :key="`f-${item.id}`" class="result-row">
        <strong>{{ item.file_name }}</strong>
        <p>{{ item.extension }} {{ item.mime_type }}</p>
        <small>{{ sourceLabel(item.source) }}</small>
      </article>
    </section>

    <section class="result-section">
      <header>
        <h2>频道</h2>
        <span>{{ result?.channels.total ?? 0 }}</span>
      </header>
      <article v-for="item in result?.channels.items ?? []" :key="`c-${item.id}`" class="result-row">
        <strong>{{ item.title }}</strong>
        <p>@{{ item.username || '私有频道' }}</p>
        <small>{{ sourceLabel(item.source) }}</small>
      </article>
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
  background: #ffffff;
  border: 1px solid #d9dee7;
  border-radius: 8px;
  min-height: 148px;
  padding: 14px;
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
  color: #667085;
}

.result-row {
  border-top: 1px solid #eef1f5;
  padding: 10px 0;
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
  color: #475467;
  margin: 4px 0;
}

.result-row a {
  color: #175cd3;
  text-decoration: underline;
}

@media (max-width: 900px) {
  .search-results {
    grid-template-columns: 1fr;
  }
}
</style>
