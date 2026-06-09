<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useDialog } from 'naive-ui'
import type { Task } from '@/api/types'
import AppPagination from '@/components/common/AppPagination.vue'
import TaskDetailDrawer from '@/components/tasks/TaskDetailDrawer.vue'
import TaskTable from '@/components/tasks/TaskTable.vue'
import { useEventsStore } from '@/stores/events'
import { useTasksStore } from '@/stores/tasks'

const tasks = useTasksStore()
const events = useEventsStore()
const dialog = useDialog()
const detailOpen = ref(false)
const pageSizeOptions = [20, 50, 100]
const pageSize = ref(50)
const offset = ref(0)
const selectedTaskIds = ref<number[]>([])

const page = computed(() => Math.floor(offset.value / pageSize.value) + 1)

function loadPage() {
  selectedTaskIds.value = []
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

function toggleTaskSelection(task: Task, selected: boolean) {
  const next = new Set(selectedTaskIds.value)
  if (selected) {
    next.add(task.id)
  } else {
    next.delete(task.id)
  }
  selectedTaskIds.value = [...next]
}

function toggleCurrentPageSelection(selected: boolean) {
  selectedTaskIds.value = selected ? tasks.items.map((task) => task.id) : []
}

function confirmDeleteTask(task: Task) {
  dialog.warning({
    title: '删除任务',
    content: `确定删除任务 ${task.id}？`,
    positiveText: '删除任务',
    positiveButtonProps: { type: 'error' },
    negativeText: '取消',
    onPositiveClick: async () => {
      await tasks.deleteTask(task.id)
      await loadPage()
    }
  })
}

function confirmDeleteSelectedTasks() {
  const ids = [...selectedTaskIds.value]
  if (ids.length === 0) return
  dialog.warning({
    title: '删除任务',
    content: `确定删除选中的 ${ids.length} 个任务？运行中和取消中的任务不会被删除。`,
    positiveText: '删除任务',
    positiveButtonProps: { type: 'error' },
    negativeText: '取消',
    onPositiveClick: async () => {
      await tasks.deleteTasks(ids)
      await loadPage()
    }
  })
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
      <div class="header-actions">
        <n-button
          :disabled="selectedTaskIds.length === 0 || tasks.loading"
          type="error"
          ghost
          @click="confirmDeleteSelectedTasks"
        >
          删除选中
        </n-button>
        <n-button :loading="tasks.loading" @click="refreshTasks">刷新</n-button>
      </div>
    </div>

    <div v-if="tasks.error" class="error-strip">{{ tasks.error }}</div>

    <TaskTable
      :tasks="tasks.items"
      :loading="tasks.loading"
      :selected-ids="selectedTaskIds"
      @select="selectTask"
      @toggle-select="toggleTaskSelection"
      @toggle-select-all="toggleCurrentPageSelection"
      @retry="tasks.retryTask($event.id)"
      @cancel="tasks.cancelTask($event.id)"
      @pause="tasks.pauseTask($event.id)"
      @resume="tasks.resumeTask($event.id)"
      @delete="confirmDeleteTask"
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
<style scoped>
.header-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
</style>
