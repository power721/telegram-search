<script setup lang="ts">
import { computed, ref, watch } from 'vue'

const props = withDefaults(
  defineProps<{
    page: number
    pageSize: number
    total: number
    pageSizeOptions?: number[]
    loading?: boolean
    showPageSize?: boolean
  }>(),
  {
    pageSizeOptions: () => [20, 50, 100],
    loading: false,
    showPageSize: true
  }
)

const emit = defineEmits<{
  'update:page': [page: number]
  'update:page-size': [pageSize: number]
}>()

const pageInput = ref(String(props.page))

const totalPages = computed(() => Math.max(1, Math.ceil(props.total / props.pageSize)))
const normalizedPage = computed(() => Math.min(Math.max(1, props.page), totalPages.value))
const canGoPrevious = computed(() => normalizedPage.value > 1)
const canGoNext = computed(() => normalizedPage.value < totalPages.value)
const visiblePages = computed(() => {
  const start = Math.max(1, normalizedPage.value - 5)
  const end = Math.min(totalPages.value, normalizedPage.value + 5)
  return Array.from({ length: end - start + 1 }, (_, index) => start + index)
})
const showFirstPage = computed(() => !visiblePages.value.includes(1))
const showLastPage = computed(() => !visiblePages.value.includes(totalPages.value))

watch(
  () => props.page,
  (page) => {
    pageInput.value = String(page)
  }
)

watch(totalPages, (pages) => {
  if (props.page > pages) {
    emit('update:page', pages)
  }
})

function goToPage(page: number) {
  const target = Math.min(Math.max(1, page), totalPages.value)
  pageInput.value = String(target)
  if (target !== props.page) {
    emit('update:page', target)
  }
}

function jumpToInputPage() {
  const page = Number.parseInt(pageInput.value, 10)
  if (!Number.isFinite(page)) {
    pageInput.value = String(normalizedPage.value)
    return
  }
  goToPage(page)
}

function changePageSize(event: Event) {
  emit('update:page-size', Number((event.target as HTMLSelectElement).value))
}
</script>

<template>
  <div class="pagination">
    <label v-if="showPageSize" class="pagination-size">
      每页
      <select aria-label="每页条数" :disabled="loading" :value="pageSize" @change="changePageSize">
        <option v-for="option in pageSizeOptions" :key="option" :value="option">
          {{ option }}
        </option>
      </select>
    </label>

    <button
      aria-label="上一页"
      :disabled="!canGoPrevious || loading"
      type="button"
      @click="goToPage(normalizedPage - 1)"
    >
      上一页
    </button>

    <div class="pagination-pages" aria-label="页码">
      <button
        v-if="showFirstPage"
        aria-label="首页"
        :disabled="loading"
        type="button"
        @click="goToPage(1)"
      >
        首页
      </button>
      <button
        v-for="pageNumber in visiblePages"
        :key="pageNumber"
        :aria-current="pageNumber === normalizedPage ? 'page' : undefined"
        :aria-label="`第 ${pageNumber} 页`"
        :class="{ active: pageNumber === normalizedPage }"
        :disabled="loading"
        type="button"
        @click="goToPage(pageNumber)"
      >
        {{ pageNumber }}
      </button>
      <button
        v-if="showLastPage"
        aria-label="尾页"
        :disabled="loading"
        type="button"
        @click="goToPage(totalPages)"
      >
        尾页
      </button>
    </div>

    <span class="pagination-summary">第 {{ normalizedPage }} / {{ totalPages }} 页，共 {{ total }} 条</span>

    <form class="pagination-jump" @submit.prevent="jumpToInputPage">
      <label>
        跳至
        <input
          v-model="pageInput"
          aria-label="跳转页码"
          :disabled="loading"
          inputmode="numeric"
          min="1"
          :max="totalPages"
          type="number"
        />
      </label>
      <button :disabled="loading" type="submit">跳转</button>
    </form>

    <button
      aria-label="下一页"
      :disabled="!canGoNext || loading"
      type="button"
      @click="goToPage(normalizedPage + 1)"
    >
      下一页
    </button>
  </div>
</template>
