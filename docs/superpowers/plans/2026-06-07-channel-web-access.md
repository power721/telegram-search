# Channel Web Access Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add batch web-access checks for existing local Telegram channels, persist the result, and return `web_access` in channel APIs.

**Architecture:** Extend `telegram_channels` with nullable access state, keep repository reads as the single source of channel JSON, add a small `internal/channel` web checker service with injectable HTTP checker, and expose a batch handler through Gin.

**Tech Stack:** Go, Gin, SQLite migrations, `net/http`, existing repository/service test style.

---

## File Structure

- Modify `internal/model/model.go`: add nullable web access fields to `model.Channel`.
- Modify `internal/db/migrations.go`: add migration version 6 for channel web access columns.
- Modify `internal/db/db_test.go`: assert new columns exist.
- Modify `internal/repository/channel.go`: scan web fields and update persisted web access state.
- Modify `internal/repository/repository_test.go`: cover channel web access persistence.
- Create `internal/channel/web_access.go`: service, result type, injectable checker, default HTTP checker.
- Create `internal/channel/web_access_test.go`: cover service behavior with fake checker.
- Modify `internal/api/router.go`: add dependency and route.
- Modify `internal/api/handlers.go`: add request validation and handler.
- Modify `internal/api/handlers_test.go`: cover successful batch, validation, and missing-channel behavior.
- Modify `cmd/tg-provider/main.go`: wire the default service.
- Modify `docs/api.md`: document response field and endpoint.

## Tasks

### Task 1: Schema, Model, Repository

- [ ] Write tests in `internal/db/db_test.go` and `internal/repository/repository_test.go` that expect `web_access`, `web_access_checked_at`, and `UpdateWebAccess`.
- [ ] Run `go test ./internal/db ./internal/repository` and verify it fails because fields/methods are missing.
- [ ] Add `WebAccess *bool` and `WebAccessCheckedAt *time.Time` to `model.Channel`.
- [ ] Add migration version 6 with the two `ALTER TABLE` statements.
- [ ] Update channel SELECT lists and scan code to include nullable bool/time values.
- [ ] Add `UpdateWebAccess(ctx, channelID int64, access bool, checkedAt time.Time) error`.
- [ ] Run `go test ./internal/db ./internal/repository` and verify it passes.
- [ ] Commit schema/repository changes.

### Task 2: Channel Web Access Service

- [ ] Write `internal/channel/web_access_test.go` for public username success, no-username false without checker call, duplicate IDs deduplication, and missing ID all-or-nothing behavior.
- [ ] Run `go test ./internal/channel` and verify it fails because the service is missing.
- [ ] Create `internal/channel/web_access.go` with `WebAccessChecker`, `WebAccessResult`, `WebAccessService`, `CheckMany`, and `HTTPWebAccessChecker`.
- [ ] Ensure `CheckMany` validates all requested channels before checking and returns an error without partial updates if any channel is missing.
- [ ] Run `go test ./internal/channel` and verify it passes.
- [ ] Commit channel service changes.

### Task 3: API Wiring

- [ ] Write API tests for `POST /api/channels/web-access/check`: success updates channel list JSON, bad bodies return 400, missing channel returns error without partial updates.
- [ ] Run `go test ./internal/api` and verify it fails because dependency/route/handler are missing.
- [ ] Add `ChannelWebAccess *channel.WebAccessService` to `api.Dependencies`.
- [ ] Add `POST /channels/web-access/check` route.
- [ ] Implement handler validation matching `/api/channels/sync` style and return `{"items": results}`.
- [ ] Wire fake service in tests and default service in `cmd/tg-provider/main.go`.
- [ ] Run `go test ./internal/api` and verify it passes.
- [ ] Commit API changes.

### Task 4: Docs and Full Verification

- [ ] Update `docs/api.md` with `web_access`, `web_access_checked_at`, and the new endpoint.
- [ ] Run `gofmt` on modified Go files.
- [ ] Run `go test ./...`.
- [ ] Commit documentation and final cleanup.
