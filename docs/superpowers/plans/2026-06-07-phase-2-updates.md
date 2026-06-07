# Phase 2 Updates Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Phase 2 realtime update listener support with a tested processor and queue.

**Architecture:** Add `internal/update` with internal events, processor, queue-backed service, listener interface, and gotd listener adapter. Keep gotd-specific code behind interfaces so processor and lifecycle tests run without Telegram network access.

**Tech Stack:** Go, gotd/td, SQLite FTS5, existing repository/link/session/logger packages.

---

## File Structure

- `internal/update/event.go`: internal event types for new/edit/delete message updates.
- `internal/update/processor.go`: transactional new/edit/delete processing.
- `internal/update/processor_test.go`: SQLite-backed processor tests.
- `internal/update/service.go`: per-account service lifecycle, queue worker, flush-on-stop, recovery loop.
- `internal/update/service_test.go`: queue and recovery tests using fake listeners.
- `internal/update/gotd_listener.go`: gotd update handler adapter.
- `internal/repository/channel.go`: add lookup by account and Telegram channel id.
- `internal/repository/message.go`: expose transactional single-message save and deletion by tx if needed.
- `internal/repository/account.go`: add status update helper for reconnect recovery.
- `internal/telegram/client.go`: add listener factory interface methods or DTO helpers only if needed.
- `cmd/tg-provider/main.go`: construct and start update service before API server, stop it during shutdown.

## Tasks

### Task 1: Processor Red Test

- [ ] Create `internal/update/processor_test.go`.
- [ ] Test new message event stores message, extracts link, and makes FTS search return it.
- [ ] Test edit event updates text, edit date, and replaces old links.
- [ ] Test delete event marks message deleted and removes it from search.
- [ ] Run `go test ./internal/update`; expected failure: package or symbols missing.

### Task 2: Processor Implementation

- [ ] Create `internal/update/event.go` and `internal/update/processor.go`.
- [ ] Add repository helpers for channel lookup and account status where required.
- [ ] Implement processor with one transaction per event.
- [ ] Run `go test ./internal/update`; expected pass.

### Task 3: Queue and Lifecycle

- [ ] Add service tests for sequential processing and stop flush.
- [ ] Implement `Service.StartAccount`, `Service.Stop`, `Enqueue`, and worker loop.
- [ ] Run `go test ./internal/update`; expected pass.

### Task 4: Recovery Loop

- [ ] Add service test where fake listener fails once, account status becomes `RECONNECTING`, and listener restarts.
- [ ] Implement listener retry loop with context cancellation.
- [ ] Run `go test ./internal/update`; expected pass.

### Task 5: gotd Listener Adapter

- [ ] Implement `internal/update/gotd_listener.go` to convert gotd new/edit/delete updates to internal events.
- [ ] Compile with `go test ./internal/update ./internal/telegram`.
- [ ] Keep unsupported update types ignored.

### Task 6: Main Wiring

- [ ] Wire `UpdateService` into `cmd/tg-provider/main.go`.
- [ ] Load online accounts and call `StartAccount` during startup.
- [ ] Stop update service before database close during shutdown.
- [ ] Run `go test ./...` and `go build ./...`.

## Self-Review

Spec coverage: tasks cover `038-045`.

Placeholder scan: no implementation step depends on unspecified future work.

Type consistency: `Event`, `Processor`, `Listener`, and `Service` are introduced before main wiring uses them.
