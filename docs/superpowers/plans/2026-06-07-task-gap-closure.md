# Task Gap Closure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the remaining `docs/TASKS.md` gaps except tasks 084, 085, 086, 087, and 090.

**Architecture:** Keep the provider local and lightweight. Add focused runtime helpers for account cleanup, async sync jobs, retry queue, backup, redaction, response contracts, and lifecycle orchestration without changing the public API more than necessary. Use repository/service tests for behavior and docs for operational tasks.

**Tech Stack:** Go, Gin, SQLite, existing repository/service packages, Go tests, Markdown docs.

---

### Task 1: Runtime Lifecycle And Account Cleanup

**Files:**
- Modify: `internal/session/session.go`
- Modify: `internal/account/manager.go`
- Modify: `internal/update/service.go`
- Modify: `internal/api/router.go`
- Modify: `internal/api/handlers.go`
- Test: `internal/session/session_test.go`
- Test: `internal/account/manager_test.go`
- Test: `internal/api/handlers_test.go`

- [ ] Add tests for deleting an account to stop runtime hooks and remove the deterministic session file.
- [ ] Add per-account stop support in the account/update lifecycle.
- [ ] Wire API account delete through the lifecycle and session cleanup.
- [ ] Verify targeted tests and `go test ./...`.

### Task 2: Async Sync, Account Channel Sync, Retry Queue

**Files:**
- Create: `internal/scheduler/retry_queue.go`
- Test: `internal/scheduler/retry_queue_test.go`
- Modify: `internal/history/service.go`
- Test: `internal/history/service_test.go`
- Modify: `internal/api/router.go`
- Modify: `internal/api/handlers.go`
- Test: `internal/api/handlers_test.go`

- [ ] Add tests for async channel sync immediate responses and account-level sync.
- [ ] Add a small in-memory retry queue with temporary retry and permanent failure recording.
- [ ] Use retry queue for async sync jobs.
- [ ] Verify targeted tests and `go test ./...`.

### Task 3: Performance, Benchmark, Memory, Backup, Redaction

**Files:**
- Create: `internal/backup/backup.go`
- Test: `internal/backup/backup_test.go`
- Create: `internal/redact/redact.go`
- Test: `internal/redact/redact_test.go`
- Modify: `internal/repository/search_benchmark_test.go`
- Modify: `internal/repository/message.go`
- Test: `internal/repository/repository_test.go`

- [ ] Add redaction helper tests and implementation.
- [ ] Add SQLite backup tests and implementation.
- [ ] Extend benchmark seed helpers to support 100k+ data, links, channels, million-message benchmark opt-in, and memory-bounded search checks.
- [ ] Keep normal tests fast.
- [ ] Verify targeted tests and `go test ./...`.

### Task 4: Response Contracts, Error Standard, Docs

**Files:**
- Modify: `internal/api/handlers.go`
- Test: `internal/api/handlers_test.go`
- Create: `docs/api-response-contract.md`
- Create: `docs/smoke-test-guide.md`
- Create: `docs/production-deployment-checklist.md`
- Modify: `README.md`

- [ ] Add tests for standard error response shape.
- [ ] Update API errors to a consistent JSON envelope without leaking secrets.
- [ ] Document response model fields and null/empty conventions.
- [ ] Add smoke test guide, deployment checklist, and README first-login/search flow.
- [ ] Verify docs are present and `go test ./...` passes.
