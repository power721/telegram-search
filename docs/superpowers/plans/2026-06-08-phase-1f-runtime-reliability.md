# Phase 1F Runtime Reliability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make long-running work observable and recoverable with persistent tasks, SSE events, FloodWait/reconnect states, retry/cancel/pause controls, realtime listener recovery, gap recovery, and restart restoration.

**Architecture:** Introduce a persistent task service as the owner of background execution state. Existing metadata sync, history sync, Web Access Detection, remote search, listener recovery, backup, and gap recovery become task types with a shared lifecycle. SSE publishes task, account, listener, and activity updates to the admin console.

**Tech Stack:** Go 1.25, Gin SSE, SQLite, context cancellation, existing Telegram client boundary, Vue 3, TypeScript, Pinia, Naive UI, Vitest.

---

## Prerequisite

Complete Phase 1E first:

[Phase 1E Index Search Resources Plan](/home/harold/workspace/telegram-search/docs/superpowers/plans/2026-06-08-phase-1e-index-search-resources.md)

## Scope

In scope:

- Persistent `sync_tasks` repository and service.
- Task state machine: queued, running, succeeded, failed, canceling, canceled, paused, flood_wait, reconnecting.
- Retry, cancel, pause, resume actions.
- SSE endpoint `/api/events`.
- Task list/detail APIs.
- FloodWait handling with `next_run_at`.
- Account reconnect state and listener restart behavior.
- Gap recovery tasks using sync cursors and idempotent message write path.
- Restore unfinished retryable tasks on service startup.
- Tasks UI with progress, failure reason, retry, cancel, pause/resume, and recent activity.

Out of scope:

- Docker and release packaging. These are Phase 1G.
- New search/resource features beyond surfacing task state for work added in Phase 1E.

## Task State Machine

Allowed transitions:

```text
queued -> running -> succeeded
queued -> running -> failed
failed -> queued
running -> canceling -> canceled
running -> paused -> running
running -> flood_wait -> queued
running -> reconnecting -> running
```

Restart behavior:

- `running`, `canceling`, `paused`, `flood_wait`, and `reconnecting` tasks reload from SQLite.
- Retryable unfinished tasks become `queued` when their `next_run_at` is empty or in the past.
- Canceled and succeeded tasks never restart.

## File Structure

- Modify `internal/model/model.go`: task, task status, event, activity models.
- Modify `internal/db/migrations.go`: `sync_tasks` and recent activity schema.
- Create `internal/task/repository.go`.
- Create `internal/task/service.go`.
- Create `internal/task/runner.go`.
- Create `internal/task/events.go`.
- Modify `internal/history/service.go`: task-aware progress, cancel, pause, FloodWait handling.
- Modify `internal/channel/service.go`: metadata sync task adapter.
- Modify `internal/channel/web_access.go`: task adapter.
- Modify `internal/search/remote.go`: task adapter.
- Modify `internal/update/event.go`: add gap and reconnect event metadata.
- Modify `internal/update/gotd_listener.go`: listener reconnect behavior.
- Modify `internal/update/processor.go`: gap recovery handoff.
- Modify `internal/update/service.go`: task-aware listener runtime.
- Modify `internal/api/router.go`: tasks and events routes.
- Modify `internal/api/handlers.go`: tasks/events handlers.
- Add backend tests under `internal/task`, `internal/history`, `internal/update`, `internal/api`.
- Modify `web/src/api/types.ts`: task/event types.
- Create `web/src/stores/tasks.ts`.
- Create `web/src/stores/events.ts`.
- Create `web/src/views/TasksView.vue`.
- Create `web/src/components/tasks/TaskTable.vue`.
- Create `web/src/components/tasks/TaskDetailDrawer.vue`.
- Modify `web/src/views/HomeView.vue`: recent activity and task errors.
- Add frontend tests.

## Task 1: Persistent Task Repository

**Files:**

- Modify: `internal/model/model.go`
- Modify: `internal/db/migrations.go`
- Create: `internal/task/repository.go`
- Test: `internal/task/repository_test.go`

- [ ] **Step 1: Write repository tests**

Verify create, update status, append progress, find by ID, list by status, and restart query behavior.

Run:

```bash
go test ./internal/task -run 'TestTaskRepository' -v
```

Expected: FAIL because task package does not exist.

- [ ] **Step 2: Add model constants**

Add statuses:

```go
queued, running, succeeded, failed, canceling, canceled, paused, flood_wait, reconnecting
```

Add types:

```go
metadata_sync, channel_analysis, web_access_detection, history_sync, listener_recovery, remote_search, backup, gap_recovery
```

- [ ] **Step 3: Add schema and repository**

`sync_tasks` fields match the product spec:

```text
id, type, status, progress, total, message, error_code, error_message,
retry_count, next_run_at, payload_json, started_at, finished_at, created_at, updated_at
```

- [ ] **Step 4: Verify and commit**

Run:

```bash
go test ./internal/task ./internal/db
```

Expected: PASS.

Commit:

```bash
git add internal/model/model.go internal/db/migrations.go internal/task/repository.go internal/task/repository_test.go
git commit -m "feat: add persistent task repository"
```

## Task 2: Task Service And State Transitions

**Files:**

- Create: `internal/task/service.go`
- Create: `internal/task/runner.go`
- Test: `internal/task/service_test.go`

- [ ] **Step 1: Write state transition tests**

Verify allowed transitions succeed and invalid transitions return `ErrInvalidTransition`.

Run:

```bash
go test ./internal/task -run 'TestTaskStateTransitions' -v
```

Expected: FAIL until service exists.

- [ ] **Step 2: Implement task service**

Methods:

```go
Enqueue(ctx context.Context, taskType string, payload any) (model.Task, error)
Start(ctx context.Context, id int64) error
Succeed(ctx context.Context, id int64, message string) error
Fail(ctx context.Context, id int64, code string, message string) error
Retry(ctx context.Context, id int64) error
Cancel(ctx context.Context, id int64) error
Pause(ctx context.Context, id int64) error
Resume(ctx context.Context, id int64) error
SetFloodWait(ctx context.Context, id int64, nextRunAt time.Time, message string) error
```

- [ ] **Step 3: Implement runner cancellation**

Use `context.WithCancel` per running task. `Cancel` marks `canceling`, cancels the context, then workers mark `canceled`.

- [ ] **Step 4: Verify and commit**

Run:

```bash
go test ./internal/task
```

Expected: PASS.

Commit:

```bash
git add internal/task/service.go internal/task/runner.go internal/task/service_test.go
git commit -m "feat: add task state service"
```

## Task 3: Convert Long-Running Work To Tasks

**Files:**

- Modify: `internal/history/service.go`
- Modify: `internal/channel/service.go`
- Modify: `internal/channel/web_access.go`
- Modify: `internal/search/remote.go`
- Test: `internal/history/service_test.go`
- Test: `internal/channel/service_test.go`
- Test: `internal/search/remote_test.go`

- [ ] **Step 1: Write task integration tests**

Verify history sync progress updates:

```text
progress increases after each batch
total equals Sync Profile limit for Quick/Normal/Deep
Full uses total=0
FloodWait moves task to flood_wait with next_run_at
cancel stops future batches and preserves written rows
```

Run:

```bash
go test ./internal/history -run 'TestHistorySyncTask' -v
```

Expected: FAIL until history service accepts task progress hooks.

- [ ] **Step 2: Add task hooks**

Add a small interface:

```go
type ProgressSink interface {
	Progress(ctx context.Context, progress int64, total int64, message string) error
	Status(ctx context.Context) (string, error)
}
```

Use it in history sync, metadata sync, web access detection, and remote search execution.

- [ ] **Step 3: Handle FloodWait and cancellation**

On Telegram FloodWait:

- Mark task `flood_wait`.
- Set `next_run_at`.
- Set account status `FLOOD_WAIT`.

On cancel:

- Stop fetching additional batches.
- Leave already written local data intact.

- [ ] **Step 4: Verify and commit**

Run:

```bash
go test ./internal/history ./internal/channel ./internal/search ./internal/task
```

Expected: PASS.

Commit:

```bash
git add internal/history/service.go internal/history/service_test.go internal/channel/service.go internal/channel/service_test.go internal/channel/web_access.go internal/search/remote.go internal/search/remote_test.go
git commit -m "feat: run sync operations as tasks"
```

## Task 4: SSE Events And Task APIs

**Files:**

- Create: `internal/task/events.go`
- Modify: `internal/api/router.go`
- Modify: `internal/api/handlers.go`
- Test: `internal/api/handlers_test.go`

- [ ] **Step 1: Write API tests**

Verify:

- `GET /api/tasks` returns list envelope.
- `GET /api/tasks/:id` returns detail.
- `POST /api/tasks/:id/retry` requeues failed task.
- `POST /api/tasks/:id/cancel` cancels running task.
- `GET /api/events` streams `text/event-stream`.

Run:

```bash
go test ./internal/api -run 'TestTaskAPI|TestEventsAPI' -v
```

Expected: FAIL until routes exist.

- [ ] **Step 2: Register routes**

```go
api.GET("/tasks", h.tasks)
api.GET("/tasks/:id", h.task)
api.POST("/tasks/:id/retry", h.retryTask)
api.POST("/tasks/:id/cancel", h.cancelTask)
api.POST("/tasks/:id/pause", h.pauseTask)
api.POST("/tasks/:id/resume", h.resumeTask)
api.GET("/events", h.events)
```

- [ ] **Step 3: Implement event broker**

Events:

```text
task.updated
account.updated
listener.updated
activity.created
```

Use bounded subscriber channels and drop stale events for slow clients.

- [ ] **Step 4: Verify and commit**

Run:

```bash
go test ./internal/api ./internal/task
```

Expected: PASS.

Commit:

```bash
git add internal/task/events.go internal/api/router.go internal/api/handlers.go internal/api/handlers_test.go
git commit -m "feat: add task api and events stream"
```

## Task 5: Realtime Listener And Gap Recovery

**Files:**

- Modify: `internal/update/event.go`
- Modify: `internal/update/gotd_listener.go`
- Modify: `internal/update/processor.go`
- Modify: `internal/update/service.go`
- Modify: `internal/account/manager.go`
- Modify: `internal/account/state.go`
- Test: `internal/update/gotd_listener_test.go`
- Test: `internal/update/processor_test.go`
- Test: `internal/update/service_test.go`
- Test: `internal/account/manager_test.go`
- Test: `internal/account/state_test.go`

- [ ] **Step 1: Write listener recovery tests**

Verify:

- Listener starts only for channels with `listen_enabled=true`.
- Disconnect marks account `RECONNECTING`.
- Successful reconnect returns account to `ONLINE`.
- Detected update gap enqueues `gap_recovery`.

Run:

```bash
go test ./internal/update ./internal/account -run 'Test.*Listener.*|Test.*GapRecovery.*' -v
```

Expected: FAIL until listener uses control state and task service.

- [ ] **Step 2: Implement listener selection**

Load enabled channels from `telegram_channels.listen_enabled=1`. Apply watch rules before writing updates.

- [ ] **Step 3: Implement gap recovery**

Gap recovery reads `telegram_sync_cursors` and fetches the missing range through the same idempotent write path as history sync.

- [ ] **Step 4: Verify and commit**

Run:

```bash
go test ./internal/update ./internal/account ./internal/history
```

Expected: PASS.

Commit:

```bash
git add internal/update internal/account internal/history
git commit -m "feat: add listener recovery and gap recovery"
```

## Task 6: Restart Recovery

**Files:**

- Modify: `cmd/tg-search/main.go`
- Modify: `internal/task/service.go`
- Test: `internal/task/service_test.go`

- [ ] **Step 1: Write restart tests**

Seed tasks in `running`, `flood_wait`, `succeeded`, and `canceled`. Verify restart:

- Requeues retryable unfinished tasks.
- Keeps `succeeded` and `canceled` unchanged.
- Keeps future `flood_wait` scheduled for `next_run_at`.

Run:

```bash
go test ./internal/task -run 'TestRestoreUnfinishedTasks' -v
```

Expected: FAIL until restore behavior exists.

- [ ] **Step 2: Implement restore**

Add:

```go
RestoreUnfinished(ctx context.Context, now time.Time) error
```

Call it during startup after repositories and services are wired.

- [ ] **Step 3: Verify and commit**

Run:

```bash
go test ./internal/task ./...
```

Expected: PASS.

Commit:

```bash
git add cmd/tg-search/main.go internal/task/service.go internal/task/service_test.go
git commit -m "feat: restore unfinished tasks on startup"
```

## Task 7: Tasks UI

**Files:**

- Modify: `web/src/api/types.ts`
- Create: `web/src/stores/tasks.ts`
- Create: `web/src/stores/events.ts`
- Create: `web/src/components/tasks/TaskTable.vue`
- Create: `web/src/components/tasks/TaskDetailDrawer.vue`
- Create: `web/src/views/TasksView.vue`
- Modify: `web/src/views/HomeView.vue`
- Test: `web/src/stores/tasks.test.ts`
- Test: `web/src/stores/events.test.ts`
- Test: `web/src/views/TasksView.test.ts`

- [ ] **Step 1: Write frontend tests**

Verify task rows show status, progress, error message, retry count, next run time, and action buttons based on status.

Run:

```bash
npm run web:test -- tasks events
```

Expected: FAIL until stores and views exist.

- [ ] **Step 2: Implement stores**

Tasks store actions:

```ts
loadTasks()
loadTask(id: number)
retryTask(id: number)
cancelTask(id: number)
pauseTask(id: number)
resumeTask(id: number)
```

Events store opens `/api/events` and updates task/account/activity stores.

- [ ] **Step 3: Implement UI**

Use compact table rows and a detail drawer. Show FloodWait with `next_run_at`, reconnecting state, and recent activity on Home.

- [ ] **Step 4: Verify and commit**

Run:

```bash
npm run web:typecheck
npm run web:test
```

Expected: PASS.

Commit:

```bash
git add web/src/api/types.ts web/src/stores/tasks.ts web/src/stores/events.ts web/src/components/tasks web/src/views/TasksView.vue web/src/views/HomeView.vue web/src/**/*.test.ts
git commit -m "feat: add task observability ui"
```

## Task 8: Documentation And Final Verification

**Files:**

- Modify: `docs/api.md`
- Modify: `README.md`
- Modify: `docs/production-deployment-checklist.md`

- [ ] **Step 1: Document runtime behavior**

Document task states, task APIs, SSE events, FloodWait behavior, restart recovery, listener recovery, and gap recovery.

- [ ] **Step 2: Run verification**

Run:

```bash
go test ./...
npm run web:typecheck
npm run web:test
```

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add docs/api.md README.md docs/production-deployment-checklist.md
git commit -m "docs: document runtime reliability"
```

## Self-Review Checklist

- [ ] All long-running work has a task record.
- [ ] Task cancelation does not roll back written data.
- [ ] FloodWait records `next_run_at`.
- [ ] Restart recovery is deterministic.
- [ ] SSE endpoint emits task, account, listener, and activity changes.
- [ ] Gap recovery uses the same idempotent write path as history sync.
