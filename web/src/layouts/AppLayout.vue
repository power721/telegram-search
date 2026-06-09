<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, RouterView, useRoute } from 'vue-router'

const route = useRoute()

const navItems = [
  { label: '首页', eyebrow: '概览', to: '/', name: 'home' },
  { label: '搜索', eyebrow: '检索', to: '/search', name: 'search' },
  { label: '频道', eyebrow: '频道管理', to: '/channels', name: 'channels' },
  { label: '资源', eyebrow: '资源库', to: '/resources', name: 'resources' },
  { label: '账号', eyebrow: '账号管理', to: '/accounts', name: 'accounts' },
  { label: '任务', eyebrow: '任务队列', to: '/tasks', name: 'tasks' },
  { label: '日志', eyebrow: '运行日志', to: '/logs', name: 'logs' },
  { label: '设置', eyebrow: '系统设置', to: '/settings', name: 'settings' }
]

const activeName = computed(() => String(route.name ?? 'home'))
const activeItem = computed(() => navItems.find((item) => item.name === activeName.value) ?? navItems[0])
</script>

<template>
  <div class="app-shell">
    <aside class="app-sidebar is-fixed" aria-label="主导航">
      <div class="brand-block">
        <div class="brand-mark" aria-hidden="true">tg</div>
        <div>
          <div class="brand">TG Search</div>
          <div class="brand-subtitle">本地索引控制台</div>
        </div>
      </div>
      <nav class="nav-list">
        <RouterLink
          v-for="item in navItems"
          :key="item.name"
          :to="item.to"
          class="nav-item"
          :class="{ active: activeName === item.name }"
        >
          <span>{{ item.label }}</span>
          <small>{{ item.eyebrow }}</small>
        </RouterLink>
      </nav>
      <div class="sidebar-footer">
        <span class="status-pill status-success">本地运行</span>
      </div>
    </aside>
    <div class="app-workspace">
      <header class="app-toolbar">
        <div>
          <p class="toolbar-kicker">{{ activeItem.eyebrow }}</p>
          <h1>{{ activeItem.label }}</h1>
        </div>
        <div class="toolbar-actions">
          <span class="toolbar-chip">SQLite FTS5</span>
          <span class="toolbar-chip">私有索引</span>
        </div>
      </header>
      <main class="app-main">
        <div class="content-frame">
          <RouterView />
        </div>
      </main>
    </div>
  </div>
</template>

<style scoped>
.app-shell {
  min-height: 100vh;
}

.app-sidebar {
  background: var(--app-surface);
  border-right: 1px solid var(--app-border);
  display: flex;
  flex-direction: column;
  gap: 18px;
  height: 100vh;
  left: 0;
  padding: 16px 12px;
  top: 0;
  width: var(--app-sidebar-width);
  z-index: 20;
}

.app-sidebar.is-fixed {
  position: fixed;
}

.brand-block {
  align-items: center;
  display: grid;
  gap: 10px;
  grid-template-columns: 34px minmax(0, 1fr);
  padding: 0 6px 10px;
}

.brand-mark {
  align-items: center;
  background: var(--app-heading);
  border-radius: 6px;
  color: var(--app-surface);
  display: inline-flex;
  font-size: 14px;
  font-weight: 750;
  height: 34px;
  justify-content: center;
  letter-spacing: 0;
  width: 34px;
}

.brand {
  color: var(--app-heading);
  font-size: 16px;
  font-weight: 700;
  line-height: 1.2;
}

.brand-subtitle {
  color: var(--app-text-muted);
  font-size: 13px;
  line-height: 1.3;
  margin-top: 2px;
}

.nav-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.nav-item {
  border: 1px solid transparent;
  border-radius: var(--app-radius);
  color: var(--app-text);
  display: grid;
  gap: 1px;
  min-height: 44px;
  padding: 7px 10px;
  text-decoration: none;
}

.nav-item:hover {
  background: var(--app-surface-muted);
  border-color: var(--app-border-subtle);
}

.nav-item span {
  font-size: 15px;
  font-weight: 600;
  line-height: 1.35;
}

.nav-item small {
  color: var(--app-text-muted);
  font-size: 12px;
  line-height: 1.2;
}

.nav-item.active {
  background: var(--app-accent-subtle);
  border-color: color-mix(in srgb, var(--app-accent) 28%, var(--app-border));
  color: var(--app-heading);
  font-weight: 600;
}

.sidebar-footer {
  border-top: 1px solid var(--app-border-subtle);
  margin-top: auto;
  padding: 14px 6px 0;
}

.app-workspace {
  margin-left: var(--app-sidebar-width);
  min-width: 0;
}

.app-toolbar {
  align-items: center;
  background: color-mix(in srgb, var(--app-surface) 92%, transparent);
  border-bottom: 1px solid var(--app-border);
  display: flex;
  gap: 16px;
  height: var(--app-toolbar-height);
  justify-content: space-between;
  padding: 0 24px;
  position: sticky;
  top: 0;
  z-index: 10;
}

.toolbar-kicker {
  color: var(--app-text-muted);
  font-size: 12px;
  font-weight: 700;
  line-height: 1;
  margin: 0 0 3px;
}

.app-toolbar h1 {
  color: var(--app-heading);
  font-size: 16px;
  font-weight: 650;
  line-height: 1.2;
  margin: 0;
}

.toolbar-chip {
  border: 1px solid var(--app-border);
  border-radius: 999px;
  color: var(--app-text-muted);
  font-size: 13px;
  line-height: 22px;
  padding: 0 8px;
}

.app-main {
  min-width: 0;
  padding: 20px 24px 32px;
}

.content-frame {
  margin: 0 auto;
  max-width: var(--app-content-max);
  min-width: 0;
  width: 100%;
}

@media (max-width: 860px) {
  .app-sidebar {
    height: auto;
    overflow-x: auto;
    width: 100%;
  }

  .app-sidebar.is-fixed {
    position: sticky;
  }

  .brand-block,
  .sidebar-footer {
    display: none;
  }

  .nav-list {
    flex-direction: row;
  }

  .nav-item {
    min-width: 92px;
  }

  .app-workspace {
    margin-left: 0;
  }

  .app-toolbar {
    padding: 0 16px;
  }

  .toolbar-actions {
    display: none;
  }

  .app-main {
    padding: 16px;
  }
}
</style>
