<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import type { Task } from '@/api/types'
import TaskDetailDrawer from '@/components/tasks/TaskDetailDrawer.vue'
import TaskTable from '@/components/tasks/TaskTable.vue'
import { useEventsStore } from '@/stores/events'
import { useTasksStore } from '@/stores/tasks'

const tasks = useTasksStore()
const events = useEventsStore()
const detailOpen = ref(false)
const pageSizeOptions = [20, 50, 100]
const pageSize = ref(50)
const offset = ref(0)

const page = computed(() => Math.floor(offset.value / pageSize.value) + 1)
const canGoPrevious = computed(() => offset.value > 0)
const canGoNext = computed(() => offset.value + pageSize.value < tasks.total)

function loadPage() {
  return tasks.loadTasks({ limit: pageSize.value, offset: offset.value })
}

onMounted(() => {
  void loadPage()
  events.connect()
})

onUnmounted(() => {
  events.disconnect()
})

function selectTask(task: Task) {
  tasks.selected = task
  detailOpen.value = true
}

async function refreshTasks() {
  await loadPage()
}

async function previousPage() {
  if (!canGoPrevious.value) return
  offset.value = Math.max(0, offset.value - pageSize.value)
  await loadPage()
}

async function nextPage() {
  if (!canGoNext.value) return
  offset.value += pageSize.value
  await loadPage()
}

async function changePageSize(event: Event) {
  pageSize.value = Number((event.target as HTMLSelectElement).value)
  offset.value = 0
  await loadPage()
}
</script>

<template>
  <section class="page-section">
    <div class="page-header">
      <div>
        <p class="page-kicker">运行状态</p>
        <h1 class="page-title">任务</h1>
        <p class="page-subtitle">查看后台同步、检测、重试和限流恢复任务。</p>
      </div>
      <n-button :loading="tasks.loading" @click="refreshTasks">刷新</n-button>
    </div>

    <div v-if="tasks.error" class="error-strip">{{ tasks.error }}</div>

    <TaskTable
      :tasks="tasks.items"
      :loading="tasks.loading"
      @select="selectTask"
      @retry="tasks.retryTask($event.id)"
      @cancel="tasks.cancelTask($event.id)"
      @pause="tasks.pauseTask($event.id)"
      @resume="tasks.resumeTask($event.id)"
    />

    <div class="pagination">
      <label>
        每页
        <select aria-label="每页条数" :value="pageSize" @change="changePageSize">
          <option v-for="option in pageSizeOptions" :key="option" :value="option">
            {{ option }}
          </option>
        </select>
      </label>
      <button
        aria-label="上一页"
        :disabled="!canGoPrevious || tasks.loading"
        type="button"
        @click="previousPage"
      >
        上一页
      </button>
      <span>第 {{ page }} 页，共 {{ tasks.total }} 条</span>
      <button
        aria-label="下一页"
        :disabled="!canGoNext || tasks.loading"
        type="button"
        @click="nextPage"
      >
        下一页
      </button>
    </div>

    <TaskDetailDrawer v-model:show="detailOpen" :task="tasks.selected" />
  </section>
</template>

<style scoped>
.pagination label {
  align-items: center;
  display: inline-flex;
  gap: 6px;
}
</style>
