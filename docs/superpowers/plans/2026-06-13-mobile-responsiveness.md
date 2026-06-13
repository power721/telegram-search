# Mobile Responsiveness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the TG Search frontend fully usable on 320px–768px screens while preserving the desktop experience.

**Architecture:** File-by-file responsive fixes following the card-layout pattern already in ResourceTable.vue. Each table-heavy view gets a `@media (max-width: 760px)` rule that hides the desktop table and shows a stacked card layout. Drawers get computed responsive widths. Touch targets are enlarged globally.

**Tech Stack:** Vue 3, TypeScript, scoped CSS, Naive UI, UnoCSS

---

### Task 1: Global touch target fixes in base.css

**Files:**
- Modify: `web/src/styles/base.css`

- [ ] **Step 1: Add mobile checkbox touch target rules**

Add at the end of `base.css` (after the existing `@media` blocks, around line 574):

```css
@media (max-width: 760px) {
  input[type="checkbox"] {
    height: 20px;
    width: 20px;
  }
}
```

- [ ] **Step 2: Run frontend type check and tests**

Run: `npm run web:typecheck && npm run web:test`
Expected: All pass, no regressions.

- [ ] **Step 3: Commit**

```bash
git add web/src/styles/base.css
git commit -m "feat: enlarge checkbox touch targets on mobile"
```

---

### Task 2: Responsive drawer widths

**Files:**
- Modify: `web/src/components/channels/ChannelControlDrawer.vue`
- Modify: `web/src/components/tasks/TaskDetailDrawer.vue`

- [ ] **Step 1: Add responsive width to ChannelControlDrawer**

In `ChannelControlDrawer.vue`, add a computed property after line 24 (`const title = ...`):

```typescript
const drawerWidth = computed(() => Math.min(420, window.innerWidth * 0.9))
```

Change line 49 from:

```html
<n-drawer :show="show" width="420" @update:show="emit('update:show', $event)">
```

To:

```html
<n-drawer :show="show" :width="drawerWidth" @update:show="emit('update:show', $event)">
```

- [ ] **Step 2: Add responsive width to TaskDetailDrawer**

In `TaskDetailDrawer.vue`, add a computed import and property. Replace line 2:

```typescript
import type { Task } from '@/api/types'
```

With:

```typescript
import { computed } from 'vue'
import type { Task } from '@/api/types'
```

Add after line 7 (after `defineProps`):

```typescript
const drawerWidth = computed(() => Math.min(520, window.innerWidth * 0.9))
```

Change line 54 from:

```html
<n-drawer :show="show" width="520" @update:show="emit('update:show', $event)">
```

To:

```html
<n-drawer :show="show" :width="drawerWidth" @update:show="emit('update:show', $event)">
```

- [ ] **Step 3: Run frontend type check and tests**

Run: `npm run web:typecheck && npm run web:test`
Expected: All pass.

- [ ] **Step 4: Commit**

```bash
git add web/src/components/channels/ChannelControlDrawer.vue web/src/components/tasks/TaskDetailDrawer.vue
git commit -m "feat: responsive drawer widths on mobile"
```

---

### Task 3: Navigation density on small screens

**Files:**
- Modify: `web/src/layouts/AppLayout.vue`

- [ ] **Step 1: Add 760px and 480px nav breakpoints**

In `AppLayout.vue`, add inside the existing `@media (max-width: 860px)` block (after line 298, before the closing `}`):

```css
  @media (max-width: 760px) {
    .nav-item {
      min-width: 64px;
    }
  }

  @media (max-width: 480px) {
    .nav-item small {
      display: none;
    }

    .toolbar-kicker {
      display: none;
    }
  }
```

- [ ] **Step 2: Run frontend type check and tests**

Run: `npm run web:typecheck && npm run web:test`
Expected: All pass.

- [ ] **Step 3: Commit**

```bash
git add web/src/layouts/AppLayout.vue
git commit -m "feat: reduce nav density on small screens"
```

---

### Task 4: Channels table card layout

**Files:**
- Modify: `web/src/views/ChannelsView.vue`

- [ ] **Step 1: Add mobile card markup in template**

After the closing `</table>` tag (line 620), before the closing `</div>` of `.table-panel` (line 621), add a mobile cards container:

```html
    <div class="mobile-cards">
      <div v-if="channels.loading && channels.items.length === 0" class="mobile-loading">
        <div class="loading-stack" aria-label="正在加载频道">
          <span class="skeleton-line" />
          <span class="skeleton-line" />
          <span class="skeleton-line short" />
        </div>
      </div>
      <div v-for="channel in filteredChannels" :key="channel.id" class="mobile-card">
        <div class="mobile-card-header">
          <img
            v-if="channel.avatar_state === 'available'"
            class="channel-avatar"
            :src="`/api/channels/${channel.id}/avatar`"
            alt=""
            loading="lazy"
            @error="($event.target as HTMLImageElement).style.display = 'none'"
          />
          <span v-else class="channel-avatar-placeholder">
            {{ channel.title.charAt(0).toUpperCase() }}
          </span>
          <div class="mobile-card-title">
            <a v-if="channelDeepLink(channel)" class="channel-title-link" :href="channelDeepLink(channel)">{{ channel.title }}</a>
            <span v-else>{{ channel.title }}</span>
            <span class="mobile-card-sub">{{ username(channel) }} · {{ channelTypeLabel(channel.type) }} · {{ channel.member_count }} 成员</span>
          </div>
        </div>
        <div class="mobile-card-badges">
          <span class="status-pill" :class="syncStateClass(channel.sync_state)">
            {{ syncStateLabel(channel.sync_state) }}
          </span>
          <span class="status-pill" :class="listenStateClass(channel.listen_state)">
            {{ listenStateLabel(channel.listen_state) }}
          </span>
          <WebAccessBadge :value="channel.web_access" :error="channel.web_access_error" />
        </div>
        <div class="mobile-card-meta">
          <span>{{ channel.indexed_message_count }} 已索引</span>
        </div>
        <div class="mobile-card-actions">
          <n-button size="small" :loading="syncingChannelIds.has(channel.id)" @click="syncHistory(channel)">同步</n-button>
          <n-button size="small" :disabled="!canCheckWebAccess(channel)" :loading="checkingWebAccessChannelIds.has(channel.id)" @click="checkWebAccess(channel)">检测</n-button>
          <n-button size="small" :type="isListeningEnabled(channel) ? '' : 'primary'" :loading="listeningChannelIds.has(channel.id)" @click="toggleListening(channel)">
            {{ isListeningEnabled(channel) ? '取消监听' : '开启监听' }}
          </n-button>
          <n-button size="small" :loading="ruleLoading && ruleTarget?.id === channel.id" @click="openChannelRules(channel)">规则</n-button>
          <n-button size="small" type="error" :loading="clearingChannelIds.has(channel.id)" @click="confirmClearChannel(channel)">清空</n-button>
        </div>
      </div>
      <div v-if="!channels.loading && filteredChannels.length === 0" class="empty-state">
        <strong>暂无频道</strong>
        <span>调整筛选条件，或刷新 Telegram 元数据。</span>
      </div>
    </div>
```

- [ ] **Step 2: Add mobile card CSS**

Add at the end of the `<style scoped>` block (after line 844, before `</style>`):

```css
.mobile-cards {
  display: none;
}

.mobile-card {
  border: 1px solid var(--app-border);
  border-radius: var(--app-radius);
  display: none;
  flex-direction: column;
  gap: 8px;
  padding: 12px;
}

.mobile-card-header {
  align-items: center;
  display: flex;
  gap: 10px;
}

.mobile-card-title {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.mobile-card-sub {
  color: var(--app-text-muted);
  font-size: 12px;
}

.mobile-card-badges {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.mobile-card-meta {
  color: var(--app-text-muted);
  font-size: 12px;
}

.mobile-card-actions {
  border-top: 1px solid var(--app-border);
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding-top: 8px;
}

@media (max-width: 760px) {
  .table-panel table {
    display: none;
  }

  .mobile-cards {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .mobile-card {
    display: flex;
  }
}
```

- [ ] **Step 3: Run frontend type check and tests**

Run: `npm run web:typecheck && npm run web:test`
Expected: All pass.

- [ ] **Step 4: Commit**

```bash
git add web/src/views/ChannelsView.vue
git commit -m "feat: add mobile card layout for channels table"
```

---

### Task 5: Accounts table card layout

**Files:**
- Modify: `web/src/views/AccountsView.vue`

- [ ] **Step 1: Add mobile card markup in template**

After the closing `</table>` tag (line 380), before the closing `</div>` of `.table-panel` (line 381), add:

```html
    <div class="mobile-cards">
      <div v-if="telegram.loading && telegram.accounts.length === 0" class="mobile-loading">
        <div class="loading-stack" aria-label="正在加载账号">
          <span class="skeleton-line" />
          <span class="skeleton-line short" />
        </div>
      </div>
      <div v-for="account in pagedAccounts" :key="account.id" class="mobile-card">
        <div class="mobile-card-header">
          <img
            v-if="account.photo_id"
            :src="`/api/accounts/${account.id}/avatar`"
            alt="头像"
            class="account-avatar"
          />
          <div v-else class="account-avatar-placeholder">
            {{ (account.first_name || account.username || '?')[0].toUpperCase() }}
          </div>
          <div class="mobile-card-title">
            <span class="mobile-card-name">{{ displayName(account.first_name, account.last_name, account.username) }}</span>
            <span class="mobile-card-sub">{{ account.phone }}</span>
          </div>
          <span class="status-pill" :class="statusClass(account.status)">{{ statusLabel(account.status) }}</span>
        </div>
        <div class="mobile-card-meta">
          <span>最后在线：{{ formatDate(account.last_online_at) }}</span>
          <span v-if="account.last_error" class="mobile-card-error">错误：{{ account.last_error }}</span>
        </div>
        <p v-if="needsLogin(account)" class="status-help">
          点击登录后发送验证码，并在 Telegram 官方消息中查看验证码。
        </p>
        <div class="mobile-card-actions">
          <n-button v-if="needsLogin(account)" size="small" type="primary" @click="openTelegramLogin(account)">登录</n-button>
          <n-button v-else size="small" :loading="telegram.loading" @click="logoutAccount(account)">登出</n-button>
          <n-button
            v-if="!needsLogin(account)"
            size="small"
            :loading="syncingAccountIds.has(account.id)"
            @click="syncAccountChannels(account)"
          >同步频道</n-button>
          <n-button size="small" type="error" ghost :loading="telegram.loading" @click="confirmDeleteAccount(account)">删除</n-button>
        </div>
      </div>
      <div v-if="!telegram.loading && telegram.accounts.length === 0" class="empty-state">
        <strong>暂无账号</strong>
        <span>添加 Telegram 账号后即可同步频道元数据。</span>
      </div>
    </div>
```

- [ ] **Step 2: Add mobile card CSS**

Add at the end of the `<style scoped>` block (after line 621, before `</style>`):

```css
.mobile-cards {
  display: none;
}

.mobile-card {
  border: 1px solid var(--app-border);
  border-radius: var(--app-radius);
  display: none;
  flex-direction: column;
  gap: 8px;
  padding: 12px;
}

.mobile-card-header {
  align-items: center;
  display: flex;
  gap: 10px;
}

.mobile-card-title {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.mobile-card-name {
  font-weight: 600;
}

.mobile-card-sub {
  color: var(--app-text-muted);
  font-size: 12px;
}

.mobile-card-meta {
  color: var(--app-text-muted);
  display: flex;
  flex-direction: column;
  font-size: 12px;
  gap: 2px;
}

.mobile-card-error {
  color: var(--app-danger);
}

.mobile-card-actions {
  border-top: 1px solid var(--app-border);
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding-top: 8px;
}

@media (max-width: 760px) {
  .table-panel table {
    display: none;
  }

  .mobile-cards {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .mobile-card {
    display: flex;
  }
}
```

- [ ] **Step 3: Run frontend type check and tests**

Run: `npm run web:typecheck && npm run web:test`
Expected: All pass.

- [ ] **Step 4: Commit**

```bash
git add web/src/views/AccountsView.vue
git commit -m "feat: add mobile card layout for accounts table"
```

---

### Task 6: Tasks table card layout

**Files:**
- Modify: `web/src/components/tasks/TaskTable.vue`

- [ ] **Step 1: Add mobile card markup in template**

After the closing `</table>` tag (line 225), before the closing `</div>` of `.table-panel` (line 226), add:

```html
    <div class="mobile-cards">
      <div v-if="loading" class="mobile-loading">
        <div class="loading-stack" aria-label="正在加载任务">
          <span class="skeleton-line" />
          <span class="skeleton-line" />
          <span class="skeleton-line short" />
        </div>
      </div>
      <div v-for="task in tasks" :key="task.id" class="mobile-card">
        <div class="mobile-card-header">
          <label class="mobile-card-check">
            <input
              :aria-label="`选择任务 ${task.id}`"
              :checked="isSelected(task, selectedIds)"
              type="checkbox"
              @change="emit('toggleSelect', task, ($event.target as HTMLInputElement).checked)"
            />
          </label>
          <div class="mobile-card-title">
            <span>#{{ task.id }} · {{ taskTypeLabel(task.type) }}</span>
          </div>
          <span class="status-pill" :class="statusClass(task.status)">{{ statusLabel(task.status) }}</span>
        </div>
        <div v-if="task.total > 0" class="mobile-card-progress">
          <div class="progress-bar">
            <div class="progress-fill" :style="{ width: `${task.total > 0 ? (task.progress / task.total * 100) : 0}%` }"></div>
          </div>
          <span class="progress-text">{{ progressLabel(task) }}</span>
        </div>
        <div class="mobile-card-meta">
          <span>创建：{{ formatDate(task.created_at) }}</span>
          <span>重试：{{ task.retry_count }} · 下次运行：{{ formatDate(task.next_run_at) }}</span>
        </div>
        <div v-if="task.error_message || task.message" class="mobile-card-message">
          {{ task.error_message || task.message }}
        </div>
        <div class="mobile-card-actions">
          <n-button size="small" @click="emit('select', task)">详情</n-button>
          <n-button v-if="canRetry(task)" size="small" @click="emit('retry', task)">重试</n-button>
          <n-button v-if="canCancel(task)" size="small" @click="emit('cancel', task)">取消</n-button>
          <n-button v-if="canPause(task)" size="small" @click="emit('pause', task)">暂停</n-button>
          <n-button v-if="canResume(task)" size="small" @click="emit('resume', task)">恢复</n-button>
          <n-button v-if="canDelete(task)" size="small" type="error" ghost @click="emit('delete', task)">删除</n-button>
        </div>
      </div>
      <div v-if="!loading && tasks.length === 0" class="empty-state">
        <strong>暂无任务</strong>
        <span>同步、检测、清理等后台任务会显示在这里。</span>
      </div>
    </div>
```

- [ ] **Step 2: Add mobile card CSS**

Add at the end of the `<style scoped>` block (after line 262, before `</style>`):

```css
.mobile-cards {
  display: none;
}

.mobile-card {
  border: 1px solid var(--app-border);
  border-radius: var(--app-radius);
  display: none;
  flex-direction: column;
  gap: 8px;
  padding: 12px;
}

.mobile-card-header {
  align-items: center;
  display: flex;
  gap: 8px;
}

.mobile-card-check {
  align-items: center;
  display: flex;
}

.mobile-card-title {
  flex: 1;
  font-weight: 600;
  min-width: 0;
}

.mobile-card-progress {
  align-items: center;
  display: flex;
  gap: 8px;
}

.progress-bar {
  background: var(--app-border);
  border-radius: 3px;
  flex: 1;
  height: 6px;
  overflow: hidden;
}

.progress-fill {
  background: var(--app-accent);
  border-radius: 3px;
  height: 100%;
  transition: width 0.2s;
}

.progress-text {
  color: var(--app-text-muted);
  font-size: 12px;
  white-space: nowrap;
}

.mobile-card-meta {
  color: var(--app-text-muted);
  display: flex;
  flex-direction: column;
  font-size: 12px;
  gap: 2px;
}

.mobile-card-message {
  color: var(--app-text-muted);
  font-size: 12px;
  overflow-wrap: anywhere;
}

.mobile-card-actions {
  border-top: 1px solid var(--app-border);
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding-top: 8px;
}

@media (max-width: 760px) {
  table {
    display: none;
  }

  .mobile-cards {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .mobile-card {
    display: flex;
  }
}
```

- [ ] **Step 3: Run frontend type check and tests**

Run: `npm run web:typecheck && npm run web:test`
Expected: All pass.

- [ ] **Step 4: Commit**

```bash
git add web/src/components/tasks/TaskTable.vue
git commit -m "feat: add mobile card layout for tasks table"
```

---

### Task 7: Logs table mobile simplification

**Files:**
- Modify: `web/src/views/LogsView.vue`

- [ ] **Step 1: Add mobile CSS to hide secondary columns**

Add at the end of the `<style scoped>` block (after line 345, before `</style>`):

```css
@media (max-width: 760px) {
  .time-col {
    min-width: auto;
  }

  .message-cell {
    max-width: none;
  }

  .raw-line {
    max-width: none;
  }

  th:nth-child(2),
  td:nth-child(2),
  th:nth-child(5),
  td:nth-child(5),
  th:nth-child(6),
  td:nth-child(6) {
    display: none;
  }
}
```

- [ ] **Step 2: Run frontend type check and tests**

Run: `npm run web:typecheck && npm run web:test`
Expected: All pass.

- [ ] **Step 3: Commit**

```bash
git add web/src/views/LogsView.vue
git commit -m "feat: hide secondary log columns on mobile"
```

---

### Task 8: API Help table mobile fix

**Files:**
- Modify: `web/src/views/ApiHelpView.vue`

- [ ] **Step 1: Add mobile breakpoint for table columns**

Add inside the existing `@media (max-width: 720px)` block (after line 647, before the closing `}`):

```css
  .table-panel td {
    min-width: auto;
  }

  .table-panel td:last-child {
    min-width: auto;
  }
```

- [ ] **Step 2: Run frontend type check and tests**

Run: `npm run web:typecheck && npm run web:test`
Expected: All pass.

- [ ] **Step 3: Commit**

```bash
git add web/src/views/ApiHelpView.vue
git commit -m "feat: remove fixed min-widths on API help tables for mobile"
```

---

### Task 9: Settings table and touch target fixes

**Files:**
- Modify: `web/src/views/SettingsView.vue`

- [ ] **Step 1: Add mobile breakpoint for settings tables and checkboxes**

Add at the end of the `<style scoped>` block (after line 1738, before `</style>`):

```css
@media (max-width: 760px) {
  .settings-table {
    min-width: auto;
  }

  .checkbox-row input {
    height: 20px;
    width: 20px;
  }

  .checkbox-row {
    min-height: 44px;
  }
}
```

- [ ] **Step 2: Run frontend type check and tests**

Run: `npm run web:typecheck && npm run web:test`
Expected: All pass.

- [ ] **Step 3: Commit**

```bash
git add web/src/views/SettingsView.vue
git commit -m "feat: responsive settings tables and larger touch targets on mobile"
```

---

### Task 10: ResourceTable checkbox touch targets

**Files:**
- Modify: `web/src/components/resources/ResourceTable.vue`

- [ ] **Step 1: Enlarge checkbox on mobile**

In the existing `@media (max-width: 760px)` block (starts at line 644), add after line 655 (`.select-cell { justify-content: flex-start; }`):

```css
  .select-cell input {
    height: 20px;
    width: 20px;
  }
```

- [ ] **Step 2: Run frontend type check and tests**

Run: `npm run web:typecheck && npm run web:test`
Expected: All pass.

- [ ] **Step 3: Commit**

```bash
git add web/src/components/resources/ResourceTable.vue
git commit -m "feat: enlarge resource table checkboxes on mobile"
```

---

### Task 11: Final verification

- [ ] **Step 1: Run full test suite**

Run: `GOCACHE=/tmp/go-build-cache go test ./... && npm run web:typecheck && npm run web:test`
Expected: All backend and frontend tests pass.

- [ ] **Step 2: Build frontend**

Run: `npm run web:build`
Expected: Clean build with no errors.

- [ ] **Step 3: Commit any remaining changes**

```bash
git add -A
git commit -m "chore: mobile responsiveness complete"
```
