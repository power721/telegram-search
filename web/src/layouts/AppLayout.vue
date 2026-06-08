<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, RouterView, useRoute } from 'vue-router'

const route = useRoute()

const navItems = [
  { label: 'Home', to: '/', name: 'home' },
  { label: 'Search', to: '/search', name: 'search' },
  { label: 'Channels', to: '/channels', name: 'channels' },
  { label: 'Resources', to: '/resources', name: 'resources' },
  { label: 'Accounts', to: '/accounts', name: 'accounts' },
  { label: 'Tasks', to: '/tasks', name: 'tasks' },
  { label: 'Settings', to: '/settings', name: 'settings' }
]

const activeName = computed(() => String(route.name ?? 'home'))
</script>

<template>
  <div class="app-shell">
    <aside class="app-sidebar">
      <div class="brand">tg-search</div>
      <nav class="nav-list">
        <RouterLink
          v-for="item in navItems"
          :key="item.name"
          :to="item.to"
          class="nav-item"
          :class="{ active: activeName === item.name }"
        >
          {{ item.label }}
        </RouterLink>
      </nav>
    </aside>
    <main class="app-main">
      <RouterView />
    </main>
  </div>
</template>

<style scoped>
.app-shell {
  display: grid;
  min-height: 100vh;
  grid-template-columns: 232px minmax(0, 1fr);
}

.app-sidebar {
  border-right: 1px solid #d9dee7;
  background: #ffffff;
  padding: 20px 14px;
}

.brand {
  font-size: 18px;
  font-weight: 700;
  margin: 0 10px 20px;
}

.nav-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.nav-item {
  border-radius: 6px;
  color: #354052;
  padding: 9px 10px;
  text-decoration: none;
}

.nav-item.active {
  background: #e8eef8;
  color: #172033;
  font-weight: 600;
}

.app-main {
  min-width: 0;
  padding: 24px;
}
</style>
