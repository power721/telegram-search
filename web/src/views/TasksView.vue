<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import type { Task } from '@/api/types'
import AppPagination from '@/components/common/AppPagination.vue'
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

async function changePage(pageNumber: number) {
  offset.value = (pageNumber - 1) * pageSize.value
  await loadPage()
}

async function changePageSize(value: number) {
  pageSize.value = value
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

    <AppPagination
      :loading="tasks.loading"
      :page="page"
      :page-size="pageSize"
      :page-size-options="pageSizeOptions"
      :total="tasks.total"
      @update:page="changePage"
      @update:page-size="changePageSize"
    />

    <TaskDetailDrawer v-model:show="detailOpen" :task="tasks.selected" />
  </section>
</template>
