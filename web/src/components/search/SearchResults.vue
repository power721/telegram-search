<script setup lang="ts">
import type { GlobalSearchResult, RemoteSearchItem } from '@/api/types'

defineProps<{
  result: GlobalSearchResult | null
  remoteItems?: RemoteSearchItem[]
}>()
</script>

<template>
  <div class="search-results">
    <section class="result-section">
      <header>
        <h2>Messages</h2>
        <span>{{ result?.messages.total ?? 0 }}</span>
      </header>
      <article v-for="item in result?.messages.items ?? []" :key="`m-${item.id}`" class="result-row">
        <strong>{{ item.channel_title || 'Telegram' }}</strong>
        <p>{{ item.text }}</p>
        <small>{{ item.source || 'local' }}</small>
      </article>
      <article v-for="item in remoteItems ?? []" :key="`r-${item.telegram_message_id}`" class="result-row">
        <strong>{{ item.channel_title || 'Remote' }}</strong>
        <p>{{ item.text }}</p>
        <small>{{ item.source }}</small>
      </article>
    </section>

    <section class="result-section">
      <header>
        <h2>Links</h2>
        <span>{{ result?.links.total ?? 0 }}</span>
      </header>
      <article v-for="item in result?.links.items ?? []" :key="`l-${item.id}`" class="result-row">
        <strong>{{ item.note || item.url }}</strong>
        <p>{{ item.url }}</p>
        <small>{{ item.source || 'local' }}</small>
      </article>
    </section>

    <section class="result-section">
      <header>
        <h2>Files</h2>
        <span>{{ result?.files.total ?? 0 }}</span>
      </header>
      <article v-for="item in result?.files.items ?? []" :key="`f-${item.id}`" class="result-row">
        <strong>{{ item.file_name }}</strong>
        <p>{{ item.extension }} {{ item.mime_type }}</p>
        <small>{{ item.source || 'local' }}</small>
      </article>
    </section>

    <section class="result-section">
      <header>
        <h2>Channels</h2>
        <span>{{ result?.channels.total ?? 0 }}</span>
      </header>
      <article v-for="item in result?.channels.items ?? []" :key="`c-${item.id}`" class="result-row">
        <strong>{{ item.title }}</strong>
        <p>@{{ item.username || 'private' }}</p>
        <small>{{ item.source || 'local' }}</small>
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
.result-row p {
  overflow-wrap: anywhere;
}

.result-row p {
  color: #475467;
  margin: 4px 0;
}

@media (max-width: 900px) {
  .search-results {
    grid-template-columns: 1fr;
  }
}
</style>
