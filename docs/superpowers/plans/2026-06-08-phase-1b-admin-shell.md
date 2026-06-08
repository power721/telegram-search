# Phase 1B Admin Shell Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first Vue admin shell for `tg-search`: setup-aware routing, login, authenticated layout, dashboard skeleton, storage usage display, and settings skeleton.

**Architecture:** Add a new `web/` Vite app that talks to the Go backend through REST APIs created in Phase 1A. Keep the UI as an operations console: stable left navigation, compact dashboard metrics, clear auth/setup states, and no marketing page. This phase builds the shell and foundation screens only; Telegram onboarding, channel management, Global Search, and Resource Library are later phase work.

**Tech Stack:** Vue 3, TypeScript, Vite, Naive UI, Pinia, Vue Router, UnoCSS, Vitest, Vue Test Utils, jsdom, npm.

---

## Prerequisite

Complete Phase 1A first:

[Phase 1A Foundation Plan](/home/harold/workspace/telegram-search/docs/superpowers/plans/2026-06-08-phase-1a-foundation.md)

Phase 1B assumes these backend APIs exist:

- `GET /api/setup/status`
- `POST /api/setup/admin`
- `POST /api/setup/api-key`
- `POST /api/setup/complete`
- `POST /api/auth/login`
- `POST /api/auth/logout`
- `GET /api/auth/me`
- `GET /api/status`
- `GET /api/storage/usage`

## Future Phase Index

This phase does not replace later plans. Continue from the product spec:

[tg-search Product Redesign Design](/home/harold/workspace/telegram-search/docs/superpowers/specs/2026-06-08-tg-search-product-redesign-design.md)

Remaining sequence:

- **Phase 1C Telegram Onboarding:** Telegram API setup, phone/code/2FA login, account state, metadata sync.
- **Phase 1D Channel Control:** channel table, Sync Profile selection, Web Access Detection, listen rules, remote-search entry point.
- **Phase 1E Index/Search/Resources:** message contents, sync cursors, FTS updates, Global Search, Telegram Resource Library.
- **Phase 1F Runtime Reliability:** persistent tasks, SSE, FloodWait, reconnect, gap recovery, retry/cancel/pause.
- **Phase 1G Packaging/Ops:** Docker, Compose, release docs, backup, logs, smoke tests.

## Scope

In scope:

- Create `web/` Vite Vue app.
- Add TypeScript, Pinia, Vue Router, Naive UI, UnoCSS, Vitest.
- Add API client with typed response helpers.
- Add setup/auth stores.
- Add setup-aware route guards.
- Add login page.
- Add first-run admin setup page.
- Add authenticated app shell with left navigation.
- Add Home dashboard skeleton with service status and storage usage.
- Add Settings skeleton with storage quota display placeholders.
- Add placeholder pages for Search, Channels, Resources, Accounts, Tasks.
- Add frontend tests for route guards, auth store, login page, and dashboard storage usage rendering.
- Add root npm scripts for common frontend commands.
- Document frontend development workflow.

Out of scope:

- Telegram API setup wizard steps.
- Telegram account login UI.
- Channel tables and actions.
- Sync Profile UI behavior.
- Global Search implementation.
- Resource Library implementation.
- SSE task stream.
- Docker frontend build integration.

## File Structure

- Create `package.json` at repository root with npm workspace scripts.
- Create `web/package.json`.
- Create `web/index.html`.
- Create `web/vite.config.ts`.
- Create `web/tsconfig.json`.
- Create `web/tsconfig.node.json`.
- Create `web/vitest.config.ts`.
- Create `web/uno.config.ts`.
- Create `web/src/main.ts`.
- Create `web/src/App.vue`.
- Create `web/src/styles/base.css`.
- Create `web/src/api/client.ts`.
- Create `web/src/api/types.ts`.
- Create `web/src/stores/auth.ts`.
- Create `web/src/stores/setup.ts`.
- Create `web/src/stores/status.ts`.
- Create `web/src/router/index.ts`.
- Create `web/src/layouts/AppLayout.vue`.
- Create `web/src/views/SetupAdminView.vue`.
- Create `web/src/views/LoginView.vue`.
- Create `web/src/views/HomeView.vue`.
- Create `web/src/views/SettingsView.vue`.
- Create `web/src/views/placeholders.ts`.
- Create `web/src/test/setup.ts`.
- Create tests under `web/src/**/*.test.ts`.
- Modify `.gitignore` for frontend build artifacts.
- Modify `README.md`.

## Task 1: Scaffold Web App

**Files:**

- Create: root `package.json`
- Create: `web/package.json`
- Create: `web/index.html`
- Create: `web/vite.config.ts`
- Create: `web/tsconfig.json`
- Create: `web/tsconfig.node.json`
- Create: `web/vitest.config.ts`
- Create: `web/uno.config.ts`
- Create: `web/src/main.ts`
- Create: `web/src/App.vue`
- Create: `web/src/styles/base.css`
- Create: `web/src/test/setup.ts`
- Modify: `.gitignore`

- [ ] **Step 1: Create root package scripts**

Create root `package.json`:

```json
{
  "name": "tg-search",
  "private": true,
  "scripts": {
    "web:dev": "npm --prefix web run dev",
    "web:build": "npm --prefix web run build",
    "web:test": "npm --prefix web run test",
    "web:typecheck": "npm --prefix web run typecheck"
  }
}
```

- [ ] **Step 2: Create frontend package**

Create `web/package.json`:

```json
{
  "name": "tg-search-web",
  "private": true,
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "dev": "vite --host 127.0.0.1",
    "build": "vue-tsc -b && vite build",
    "test": "vitest run",
    "typecheck": "vue-tsc -b"
  },
  "dependencies": {
    "@unocss/reset": "^0.65.4",
    "@vueuse/core": "^12.8.2",
    "naive-ui": "^2.41.0",
    "pinia": "^2.3.1",
    "vue": "^3.5.13",
    "vue-router": "^4.5.0"
  },
  "devDependencies": {
    "@vitejs/plugin-vue": "^5.2.1",
    "@vue/test-utils": "^2.4.6",
    "jsdom": "^25.0.1",
    "typescript": "^5.7.3",
    "@types/node": "^22.10.7",
    "unocss": "^0.65.4",
    "vite": "^6.0.11",
    "vitest": "^2.1.8",
    "vue-tsc": "^2.2.0"
  }
}
```

- [ ] **Step 3: Install frontend dependencies**

Run:

```bash
npm install --prefix web
```

Expected: `web/package-lock.json` is created and install succeeds.

- [ ] **Step 4: Create Vite config**

Create `web/vite.config.ts`:

```ts
import { fileURLToPath, URL } from 'node:url'
import vue from '@vitejs/plugin-vue'
import UnoCSS from 'unocss/vite'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [vue(), UnoCSS()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  },
  server: {
    host: '127.0.0.1',
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:9900',
        changeOrigin: true
      }
    }
  }
})
```

Create `web/vitest.config.ts`:

```ts
import { fileURLToPath, URL } from 'node:url'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vitest/config'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  },
  test: {
    environment: 'jsdom',
    globals: true,
    passWithNoTests: true,
    setupFiles: ['./src/test/setup.ts']
  }
})
```

Create `web/uno.config.ts`:

```ts
import { defineConfig, presetUno } from 'unocss'

export default defineConfig({
  presets: [presetUno()]
})
```

- [ ] **Step 5: Create TypeScript config**

Create `web/tsconfig.json`:

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "useDefineForClassFields": true,
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "strict": true,
    "jsx": "preserve",
    "resolveJsonModule": true,
    "isolatedModules": true,
    "esModuleInterop": true,
    "lib": ["ES2022", "DOM", "DOM.Iterable"],
    "types": ["vitest/globals"],
    "baseUrl": ".",
    "paths": {
      "@/*": ["src/*"]
    }
  },
  "include": ["src/**/*.ts", "src/**/*.d.ts", "src/**/*.tsx", "src/**/*.vue"],
  "references": [{ "path": "./tsconfig.node.json" }]
}
```

Create `web/tsconfig.node.json`:

```json
{
  "compilerOptions": {
    "composite": true,
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "allowSyntheticDefaultImports": true
  },
  "include": ["vite.config.ts", "vitest.config.ts", "uno.config.ts"]
}
```

- [ ] **Step 6: Create app shell placeholder**

Create `web/index.html`:

```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>tg-search</title>
  </head>
  <body>
    <div id="app"></div>
    <script type="module" src="/src/main.ts"></script>
  </body>
</html>
```

Create `web/src/App.vue`:

```vue
<template>
  <n-config-provider>
    <n-message-provider>
      <main class="bootstrap-screen">
        <h1>tg-search</h1>
        <p>Admin shell scaffold</p>
      </main>
    </n-message-provider>
  </n-config-provider>
</template>

<style scoped>
.bootstrap-screen {
  align-items: center;
  display: flex;
  flex-direction: column;
  justify-content: center;
  min-height: 100vh;
}
</style>
```

Create `web/src/main.ts`:

```ts
import '@unocss/reset/tailwind.css'
import 'uno.css'
import './styles/base.css'

import naive from 'naive-ui'
import { createPinia } from 'pinia'
import { createApp } from 'vue'
import App from './App.vue'

createApp(App).use(createPinia()).use(naive).mount('#app')
```

Create `web/src/styles/base.css`:

```css
:root {
  color: #1f2933;
  background: #f6f7f9;
  font-family:
    Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI",
    sans-serif;
}

body {
  margin: 0;
  min-width: 320px;
}

* {
  box-sizing: border-box;
}
```

Create `web/src/test/setup.ts`:

```ts
import { config } from '@vue/test-utils'

class ResizeObserverStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}

globalThis.ResizeObserver = ResizeObserverStub as typeof ResizeObserver

config.global.stubs = {
  transition: false,
  teleport: false
}
```

- [ ] **Step 7: Update `.gitignore`**

Add:

```gitignore
web/node_modules/
web/dist/
web/.vite/
```

- [ ] **Step 8: Run scaffold verification**

Run:

```bash
npm run web:typecheck
npm run web:test
```

Expected: typecheck passes; tests pass with no test files or empty suite behavior accepted by Vitest.

- [ ] **Step 9: Commit**

Run:

```bash
git add package.json web/package.json web/package-lock.json web/index.html web/vite.config.ts web/vitest.config.ts web/tsconfig.json web/tsconfig.node.json web/uno.config.ts web/src .gitignore
git commit -m "feat: scaffold vue admin shell"
```

## Task 2: Add API Client And Types

**Files:**

- Create: `web/src/api/types.ts`
- Create: `web/src/api/client.ts`
- Create: `web/src/api/client.test.ts`

- [ ] **Step 1: Add API client tests**

Create `web/src/api/client.test.ts`:

```ts
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError, apiGet, apiPost } from './client'

describe('api client', () => {
  const originalFetch = globalThis.fetch

  beforeEach(() => {
    vi.restoreAllMocks()
  })

  afterEach(() => {
    globalThis.fetch = originalFetch
  })

  it('returns JSON for successful GET requests', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ service: 'ok' })
    } as Response)

    await expect(apiGet('/api/status')).resolves.toEqual({ service: 'ok' })
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/status', {
      credentials: 'include',
      headers: { Accept: 'application/json' }
    })
  })

  it('throws ApiError for error envelopes', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      json: async () => ({ error: { code: 'bad_request', message: 'invalid' } })
    } as Response)

    await expect(apiPost('/api/auth/login', { username: 'a' })).rejects.toMatchObject({
      code: 'bad_request',
      message: 'invalid',
      status: 400
    })
  })
})
```

- [ ] **Step 2: Create API types**

Create `web/src/api/types.ts`:

```ts
export interface ErrorEnvelope {
  error: {
    code: string
    message: string
  }
}

export interface SetupStatus {
  complete: boolean
  admin_configured: boolean
  api_key_configured: boolean
  telegram_configured: boolean
}

export interface User {
  id: number
  username: string
  role: string
  last_login_at?: string
  created_at?: string
  updated_at?: string
}

export interface ServiceStatus {
  service: string
  accounts: number
  channels: number
  messages: number
  links: number
  account_states: Record<string, number>
}

export interface StorageUsage {
  db_bytes: number
  index_bytes: number
  media_cache_bytes: number
  total_bytes: number
  max_db_bytes: number
  max_media_bytes: number
  db_over_quota: boolean
  media_over_quota: boolean
}
```

- [ ] **Step 3: Create API client**

Create `web/src/api/client.ts`:

```ts
import type { ErrorEnvelope } from './types'

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    public readonly code: string,
    message: string
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

export async function apiGet<T>(path: string): Promise<T> {
  const response = await fetch(path, {
    credentials: 'include',
    headers: { Accept: 'application/json' }
  })
  return readResponse<T>(response)
}

export async function apiPost<T>(path: string, body?: unknown): Promise<T> {
  const response = await fetch(path, {
    method: 'POST',
    credentials: 'include',
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/json'
    },
    body: body === undefined ? undefined : JSON.stringify(body)
  })
  return readResponse<T>(response)
}

async function readResponse<T>(response: Response): Promise<T> {
  const data = await response.json().catch(() => undefined)
  if (!response.ok) {
    const envelope = data as ErrorEnvelope | undefined
    throw new ApiError(
      response.status,
      envelope?.error?.code ?? 'http_error',
      envelope?.error?.message ?? `request failed with ${response.status}`
    )
  }
  return data as T
}
```

- [ ] **Step 4: Run tests**

Run:

```bash
npm run web:test -- --run src/api/client.test.ts
```

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```bash
git add web/src/api
git commit -m "feat: add web api client"
```

## Task 3: Add Setup And Auth Stores

**Files:**

- Create: `web/src/stores/setup.ts`
- Create: `web/src/stores/auth.ts`
- Create: `web/src/stores/status.ts`
- Create: `web/src/stores/setup.test.ts`
- Create: `web/src/stores/auth.test.ts`

- [ ] **Step 1: Add setup store test**

Create `web/src/stores/setup.test.ts`:

```ts
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useSetupStore } from './setup'

vi.mock('@/api/client', () => ({
  apiGet: vi.fn().mockResolvedValue({
    complete: false,
    admin_configured: false,
    api_key_configured: false,
    telegram_configured: false
  }),
  apiPost: vi.fn().mockResolvedValue({ ok: true })
}))

describe('setup store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('loads setup status', async () => {
    const store = useSetupStore()
    await store.load()
    expect(store.status?.admin_configured).toBe(false)
    expect(store.loaded).toBe(true)
  })
})
```

- [ ] **Step 2: Add auth store test**

Create `web/src/stores/auth.test.ts`:

```ts
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useAuthStore } from './auth'

vi.mock('@/api/client', () => ({
  apiGet: vi.fn().mockResolvedValue({ id: 1, username: 'admin', role: 'admin' }),
  apiPost: vi.fn().mockResolvedValue({ id: 1, username: 'admin', role: 'admin' })
}))

describe('auth store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('logs in and stores current user', async () => {
    const store = useAuthStore()
    await store.login('admin', 'secret123')
    expect(store.user?.username).toBe('admin')
    expect(store.authenticated).toBe(true)
  })
})
```

- [ ] **Step 3: Implement stores**

Create `web/src/stores/setup.ts`:

```ts
import { defineStore } from 'pinia'
import { apiGet, apiPost } from '@/api/client'
import type { SetupStatus } from '@/api/types'

export const useSetupStore = defineStore('setup', {
  state: () => ({
    status: undefined as SetupStatus | undefined,
    loaded: false,
    loading: false
  }),
  actions: {
    async load() {
      this.loading = true
      try {
        this.status = await apiGet<SetupStatus>('/api/setup/status')
        this.loaded = true
      } finally {
        this.loading = false
      }
    },
    async createAdmin(username: string, password: string) {
      await apiPost('/api/setup/admin', { username, password })
      await this.load()
    }
  }
})
```

Create `web/src/stores/auth.ts`:

```ts
import { defineStore } from 'pinia'
import { apiGet, apiPost } from '@/api/client'
import type { User } from '@/api/types'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    user: undefined as User | undefined,
    loaded: false,
    loading: false
  }),
  getters: {
    authenticated: (state) => state.user !== undefined
  },
  actions: {
    async loadMe() {
      this.loading = true
      try {
        this.user = await apiGet<User>('/api/auth/me')
      } catch {
        this.user = undefined
      } finally {
        this.loaded = true
        this.loading = false
      }
    },
    async login(username: string, password: string) {
      this.user = await apiPost<User>('/api/auth/login', { username, password })
      this.loaded = true
    },
    async logout() {
      await apiPost('/api/auth/logout')
      this.user = undefined
      this.loaded = true
    }
  }
})
```

Create `web/src/stores/status.ts`:

```ts
import { defineStore } from 'pinia'
import { apiGet } from '@/api/client'
import type { ServiceStatus, StorageUsage } from '@/api/types'

export const useStatusStore = defineStore('status', {
  state: () => ({
    service: undefined as ServiceStatus | undefined,
    storage: undefined as StorageUsage | undefined,
    loading: false
  }),
  actions: {
    async load() {
      this.loading = true
      try {
        const [service, storage] = await Promise.all([
          apiGet<ServiceStatus>('/api/status'),
          apiGet<StorageUsage>('/api/storage/usage')
        ])
        this.service = service
        this.storage = storage
      } finally {
        this.loading = false
      }
    }
  }
})
```

- [ ] **Step 4: Run tests**

Run:

```bash
npm run web:test -- --run src/stores
```

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```bash
git add web/src/stores
git commit -m "feat: add setup and auth stores"
```

## Task 4: Add Router, Guards, And Layout

**Files:**

- Modify: `web/src/main.ts`
- Modify: `web/src/App.vue`
- Create: `web/src/router/index.ts`
- Create: `web/src/router/guards.test.ts`
- Create: `web/src/layouts/AppLayout.vue`
- Create: `web/src/views/placeholders.ts`

- [ ] **Step 1: Add route guard test**

Create `web/src/router/guards.test.ts`:

```ts
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { router } from './index'

vi.mock('@/api/client', () => ({
  apiGet: vi.fn((path: string) => {
    if (path === '/api/setup/status') {
      return Promise.resolve({
        complete: false,
        admin_configured: false,
        api_key_configured: false,
        telegram_configured: false
      })
    }
    return Promise.reject(new Error('unauthorized'))
  })
}))

describe('router guards', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('sends fresh installs to setup admin', async () => {
    await router.push('/')
    await router.isReady()
    expect(router.currentRoute.value.name).toBe('setup-admin')
  })
})
```

- [ ] **Step 2: Create placeholder views**

Create `web/src/views/placeholders.ts`:

```ts
import { defineComponent, h } from 'vue'

export function placeholderView(title: string) {
  return defineComponent({
    name: `${title.replace(/\s+/g, '')}View`,
    setup() {
      return () =>
        h('section', { class: 'page-section' }, [
          h('h1', { class: 'page-title' }, title),
          h('p', { class: 'page-muted' }, 'This section will be implemented in a later phase.')
        ])
    }
  })
}
```

- [ ] **Step 3: Create app layout**

Create `web/src/layouts/AppLayout.vue`:

```vue
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
```

- [ ] **Step 4: Create router**

Create `web/src/router/index.ts`:

```ts
import { createRouter, createWebHistory } from 'vue-router'
import AppLayout from '@/layouts/AppLayout.vue'
import { useAuthStore } from '@/stores/auth'
import { useSetupStore } from '@/stores/setup'
import { placeholderView } from '@/views/placeholders'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/setup/admin', name: 'setup-admin', component: placeholderView('Create Admin'), meta: { public: true } },
    { path: '/login', name: 'login', component: placeholderView('Login'), meta: { public: true } },
    {
      path: '/',
      component: AppLayout,
      children: [
        { path: '', name: 'home', component: placeholderView('Home') },
        { path: 'search', name: 'search', component: placeholderView('Search') },
        { path: 'channels', name: 'channels', component: placeholderView('Channels') },
        { path: 'resources', name: 'resources', component: placeholderView('Resources') },
        { path: 'accounts', name: 'accounts', component: placeholderView('Accounts') },
        { path: 'tasks', name: 'tasks', component: placeholderView('Tasks') },
        { path: 'settings', name: 'settings', component: placeholderView('Settings') }
      ]
    }
  ]
})

router.beforeEach(async (to) => {
  const setup = useSetupStore()
  const auth = useAuthStore()

  if (!setup.loaded) {
    await setup.load()
  }

  if (!setup.status?.admin_configured && to.name !== 'setup-admin') {
    return { name: 'setup-admin' }
  }

  if (setup.status?.admin_configured && to.name === 'setup-admin') {
    return { name: 'login' }
  }

  if (!auth.loaded) {
    await auth.loadMe()
  }

  if (!to.meta.public && !auth.authenticated) {
    return { name: 'login' }
  }

  if (to.name === 'login' && auth.authenticated) {
    return { name: 'home' }
  }

  return true
})
```

- [ ] **Step 5: Enable router in the app**

Replace `web/src/App.vue` with:

```vue
<template>
  <n-config-provider>
    <n-message-provider>
      <router-view />
    </n-message-provider>
  </n-config-provider>
</template>
```

Update `web/src/main.ts`:

```ts
import '@unocss/reset/tailwind.css'
import 'uno.css'
import './styles/base.css'

import naive from 'naive-ui'
import { createPinia } from 'pinia'
import { createApp } from 'vue'
import App from './App.vue'
import { router } from './router'

createApp(App).use(createPinia()).use(router).use(naive).mount('#app')
```

- [ ] **Step 6: Run tests**

Run:

```bash
npm run web:test -- --run src/router/guards.test.ts
```

Expected: PASS.

- [ ] **Step 7: Commit**

Run:

```bash
git add web/src/main.ts web/src/App.vue web/src/router web/src/layouts web/src/views/placeholders.ts
git commit -m "feat: add admin shell routing"
```

## Task 5: Add Setup Admin And Login Views

**Files:**

- Create: `web/src/views/SetupAdminView.vue`
- Create: `web/src/views/LoginView.vue`
- Create: `web/src/views/LoginView.test.ts`
- Modify: `web/src/router/index.ts`

- [ ] **Step 1: Add login view test**

Create `web/src/views/LoginView.test.ts`:

```ts
import { createPinia, setActivePinia } from 'pinia'
import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import LoginView from './LoginView.vue'

const push = vi.fn()

vi.mock('vue-router', () => ({
  useRouter: () => ({ push })
}))

vi.mock('naive-ui', async () => {
  const actual = await vi.importActual<typeof import('naive-ui')>('naive-ui')
  return {
    ...actual,
    useMessage: () => ({ error: vi.fn(), success: vi.fn() })
  }
})

vi.mock('@/api/client', () => ({
  apiPost: vi.fn().mockResolvedValue({ id: 1, username: 'admin', role: 'admin' }),
  apiGet: vi.fn()
}))

describe('LoginView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    push.mockReset()
  })

  it('renders the login heading', () => {
    const wrapper = mount(LoginView)
    expect(wrapper.text()).toContain('Sign in to tg-search')
  })
})
```

- [ ] **Step 2: Create setup admin view**

Create `web/src/views/SetupAdminView.vue`:

```vue
<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useMessage } from 'naive-ui'
import { useSetupStore } from '@/stores/setup'

const router = useRouter()
const message = useMessage()
const setup = useSetupStore()

const username = ref('admin')
const password = ref('')
const loading = ref(false)

async function submit() {
  loading.value = true
  try {
    await setup.createAdmin(username.value, password.value)
    message.success('Admin account created')
    await router.push('/login')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <main class="setup-page">
    <section class="setup-panel">
      <p class="eyebrow">First Run Setup</p>
      <h1>Create admin account</h1>
      <n-form @submit.prevent="submit">
        <n-form-item label="Username">
          <n-input v-model:value="username" autocomplete="username" />
        </n-form-item>
        <n-form-item label="Password">
          <n-input v-model:value="password" type="password" autocomplete="new-password" />
        </n-form-item>
        <n-button type="primary" block :loading="loading" @click="submit">Create Admin</n-button>
      </n-form>
    </section>
  </main>
</template>

<style scoped>
.setup-page {
  align-items: center;
  display: flex;
  min-height: 100vh;
  justify-content: center;
  padding: 24px;
}

.setup-panel {
  background: #fff;
  border: 1px solid #d9dee7;
  border-radius: 8px;
  max-width: 420px;
  padding: 24px;
  width: 100%;
}

.eyebrow {
  color: #667085;
  font-size: 13px;
  margin: 0 0 8px;
}
</style>
```

- [ ] **Step 3: Create login view**

Create `web/src/views/LoginView.vue`:

```vue
<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useMessage } from 'naive-ui'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const message = useMessage()
const auth = useAuthStore()

const username = ref('admin')
const password = ref('')
const loading = ref(false)

async function submit() {
  loading.value = true
  try {
    await auth.login(username.value, password.value)
    await router.push('/')
  } catch (error) {
    message.error(error instanceof Error ? error.message : 'Login failed')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <main class="login-page">
    <section class="login-panel">
      <p class="eyebrow">Admin Console</p>
      <h1>Sign in to tg-search</h1>
      <n-form @submit.prevent="submit">
        <n-form-item label="Username">
          <n-input v-model:value="username" autocomplete="username" />
        </n-form-item>
        <n-form-item label="Password">
          <n-input v-model:value="password" type="password" autocomplete="current-password" />
        </n-form-item>
        <n-button type="primary" block :loading="loading" @click="submit">Sign In</n-button>
      </n-form>
    </section>
  </main>
</template>

<style scoped>
.login-page {
  align-items: center;
  display: flex;
  min-height: 100vh;
  justify-content: center;
  padding: 24px;
}

.login-panel {
  background: #fff;
  border: 1px solid #d9dee7;
  border-radius: 8px;
  max-width: 420px;
  padding: 24px;
  width: 100%;
}

.eyebrow {
  color: #667085;
  font-size: 13px;
  margin: 0 0 8px;
}
</style>
```

- [ ] **Step 4: Replace setup and login placeholder routes**

In `web/src/router/index.ts`, add imports:

```ts
import LoginView from '@/views/LoginView.vue'
import SetupAdminView from '@/views/SetupAdminView.vue'
```

Replace:

```ts
{ path: '/setup/admin', name: 'setup-admin', component: placeholderView('Create Admin'), meta: { public: true } },
{ path: '/login', name: 'login', component: placeholderView('Login'), meta: { public: true } },
```

with:

```ts
{ path: '/setup/admin', name: 'setup-admin', component: SetupAdminView, meta: { public: true } },
{ path: '/login', name: 'login', component: LoginView, meta: { public: true } },
```

- [ ] **Step 5: Run tests**

Run:

```bash
npm run web:test -- --run src/views/LoginView.test.ts
```

Expected: PASS.

- [ ] **Step 6: Commit**

Run:

```bash
git add web/src/views/SetupAdminView.vue web/src/views/LoginView.vue web/src/views/LoginView.test.ts web/src/router/index.ts
git commit -m "feat: add setup and login views"
```

## Task 6: Add Home And Settings Skeletons

**Files:**

- Create: `web/src/views/HomeView.vue`
- Create: `web/src/views/HomeView.test.ts`
- Create: `web/src/views/SettingsView.vue`
- Modify: `web/src/router/index.ts`

- [ ] **Step 1: Add home view test**

Create `web/src/views/HomeView.test.ts`:

```ts
import { createPinia, setActivePinia } from 'pinia'
import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import HomeView from './HomeView.vue'

vi.mock('@/api/client', () => ({
  apiGet: vi.fn((path: string) => {
    if (path === '/api/status') {
      return Promise.resolve({
        service: 'ok',
        accounts: 1,
        channels: 2,
        messages: 100,
        links: 30,
        account_states: { ONLINE: 1 }
      })
    }
    if (path === '/api/storage/usage') {
      return Promise.resolve({
        db_bytes: 3200000000,
        index_bytes: 1100000000,
        media_cache_bytes: 0,
        total_bytes: 4300000000,
        max_db_bytes: 10000000000,
        max_media_bytes: 20000000000,
        db_over_quota: false,
        media_over_quota: false
      })
    }
    return Promise.reject(new Error('unexpected path'))
  })
}))

describe('HomeView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('renders storage usage', async () => {
    const wrapper = mount(HomeView)
    await new Promise((resolve) => setTimeout(resolve, 0))
    expect(wrapper.text()).toContain('Storage Usage')
    expect(wrapper.text()).toContain('4.3 GB')
  })
})
```

- [ ] **Step 2: Create Home view**

Create `web/src/views/HomeView.vue`:

```vue
<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useStatusStore } from '@/stores/status'

const status = useStatusStore()

onMounted(() => {
  void status.load()
})

const cards = computed(() => [
  { label: 'Accounts', value: status.service?.accounts ?? 0 },
  { label: 'Channels', value: status.service?.channels ?? 0 },
  { label: 'Messages', value: status.service?.messages ?? 0 },
  { label: 'Links', value: status.service?.links ?? 0 }
])

function formatBytes(value = 0) {
  if (value >= 1_000_000_000) return `${(value / 1_000_000_000).toFixed(1)} GB`
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1)} MB`
  if (value >= 1_000) return `${(value / 1_000).toFixed(1)} KB`
  return `${value} B`
}
</script>

<template>
  <section class="page-section">
    <div class="page-header">
      <div>
        <p class="page-kicker">Overview</p>
        <h1 class="page-title">Local Telegram Index</h1>
      </div>
      <n-input class="global-search" placeholder="Search messages, links, files, channels" />
    </div>

    <div class="metric-grid">
      <div v-for="card in cards" :key="card.label" class="metric-card">
        <span>{{ card.label }}</span>
        <strong>{{ card.value }}</strong>
      </div>
    </div>

    <div class="dashboard-grid">
      <section class="panel">
        <h2>Storage Usage</h2>
        <dl>
          <div><dt>DB</dt><dd>{{ formatBytes(status.storage?.db_bytes) }}</dd></div>
          <div><dt>Index</dt><dd>{{ formatBytes(status.storage?.index_bytes) }}</dd></div>
          <div><dt>Media Cache</dt><dd>{{ formatBytes(status.storage?.media_cache_bytes) }}</dd></div>
          <div><dt>Total</dt><dd>{{ formatBytes(status.storage?.total_bytes) }}</dd></div>
        </dl>
      </section>

      <section class="panel">
        <h2>Top Resource Types</h2>
        <div class="resource-types">
          <span>Cloud Drive</span>
          <span>Magnet</span>
          <span>ED2K</span>
          <span>HTTP</span>
          <span>Files</span>
        </div>
      </section>
    </div>
  </section>
</template>

<style scoped>
.page-header {
  align-items: center;
  display: flex;
  gap: 16px;
  justify-content: space-between;
  margin-bottom: 18px;
}

.page-kicker {
  color: #667085;
  margin: 0 0 4px;
}

.page-title {
  font-size: 24px;
  margin: 0;
}

.global-search {
  max-width: 420px;
}

.metric-grid {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  margin-bottom: 16px;
}

.metric-card,
.panel {
  background: #fff;
  border: 1px solid #d9dee7;
  border-radius: 8px;
  padding: 14px;
}

.metric-card span {
  color: #667085;
  display: block;
}

.metric-card strong {
  display: block;
  font-size: 24px;
  margin-top: 6px;
}

.dashboard-grid {
  display: grid;
  gap: 16px;
  grid-template-columns: 1fr 1fr;
}

dl div {
  display: flex;
  justify-content: space-between;
  padding: 7px 0;
}

.resource-types {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.resource-types span {
  border: 1px solid #d9dee7;
  border-radius: 6px;
  padding: 6px 8px;
}
</style>
```

- [ ] **Step 3: Create Settings skeleton**

Create `web/src/views/SettingsView.vue`:

```vue
<template>
  <section class="page-section">
    <p class="page-kicker">Configuration</p>
    <h1 class="page-title">Settings</h1>
    <div class="settings-grid">
      <section class="panel">
        <h2>Storage</h2>
        <p>Storage quota and usage controls are configured here.</p>
      </section>
      <section class="panel">
        <h2>Admin</h2>
        <p>Admin profile and API keys will be managed here.</p>
      </section>
    </div>
  </section>
</template>

<style scoped>
.page-kicker {
  color: #667085;
  margin: 0 0 4px;
}

.page-title {
  font-size: 24px;
  margin: 0 0 18px;
}

.settings-grid {
  display: grid;
  gap: 16px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.panel {
  background: #fff;
  border: 1px solid #d9dee7;
  border-radius: 8px;
  padding: 14px;
}
</style>
```

- [ ] **Step 4: Replace home and settings placeholder routes**

In `web/src/router/index.ts`, add imports:

```ts
import HomeView from '@/views/HomeView.vue'
import SettingsView from '@/views/SettingsView.vue'
```

Replace:

```ts
{ path: '', name: 'home', component: placeholderView('Home') },
```

with:

```ts
{ path: '', name: 'home', component: HomeView },
```

Replace:

```ts
{ path: 'settings', name: 'settings', component: placeholderView('Settings') }
```

with:

```ts
{ path: 'settings', name: 'settings', component: SettingsView }
```

- [ ] **Step 5: Run tests**

Run:

```bash
npm run web:test -- --run src/views/HomeView.test.ts
```

Expected: PASS.

- [ ] **Step 6: Commit**

Run:

```bash
git add web/src/views/HomeView.vue web/src/views/HomeView.test.ts web/src/views/SettingsView.vue web/src/router/index.ts
git commit -m "feat: add dashboard and settings shell"
```

## Task 7: Docs And Final Verification

**Files:**

- Modify: `README.md`
- Modify: `docs/smoke-test-guide.md`

- [ ] **Step 1: Update docs**

Add frontend commands:

```bash
npm install --prefix web
npm run web:dev
npm run web:test
npm run web:build
```

Document development URLs:

```text
Backend: http://127.0.0.1:9900
Frontend: http://127.0.0.1:5173
```

Document that Vite proxies `/api` to the backend.

- [ ] **Step 2: Run full frontend verification**

Run:

```bash
npm run web:typecheck
npm run web:test
npm run web:build
```

Expected: all commands pass.

- [ ] **Step 3: Run backend smoke verification**

Run:

```bash
go test ./internal/api ./internal/config ./internal/repository
```

Expected: PASS.

- [ ] **Step 4: Commit**

Run:

```bash
git add README.md docs/smoke-test-guide.md
git commit -m "docs: add web admin shell workflow"
```

## Plan Self-Review

Spec coverage for Phase 1B:

- Vue 3 admin console foundation: Task 1.
- Setup-aware first-run route: Task 3 and Task 4.
- Login page: Task 5.
- Operations-console shell with left navigation: Task 4.
- Home dashboard skeleton: Task 6.
- Storage Usage on Home: Task 6.
- Top Resource Types on Home: Task 6.
- Settings shell: Task 6.
- Tests and frontend commands: Task 1 through Task 7.

Deferred to later phase plans:

- Telegram API setup wizard: Phase 1C.
- Telegram account login: Phase 1C.
- Channel management and Sync Profiles: Phase 1D.
- Global Search implementation: Phase 1E.
- Telegram Resource Library implementation: Phase 1E.
- Task SSE and runtime reliability: Phase 1F.
- Docker frontend integration: Phase 1G.

Verification commands:

```bash
npm run web:typecheck
npm run web:test
npm run web:build
go test ./internal/api ./internal/config ./internal/repository
```
