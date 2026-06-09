# Runtime Settings UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the Settings page read live storage limits and manage Telegram API credentials that are used by runtime Telegram operations.

**Architecture:** Keep startup-only values in YAML, including `server.*` and `storage.path`. Store editable Telegram API settings in the existing `settings` table and inject them into Telegram runtime operations through a credentials provider. Read storage limits from `GET /api/storage/usage` for display rather than hard-coded frontend values.

**Tech Stack:** Go, Gin, SQLite-backed settings repository, Vue 3, Pinia, Naive UI, Vitest.

---

### Task 1: Runtime Telegram API Credentials

**Files:**
- Modify: `internal/repository/settings.go`
- Modify: `internal/telegram/gotd_client.go`
- Modify: `internal/update/gotd_listener.go`
- Modify: `cmd/tg-search/main.go`
- Test: `internal/repository/settings_test.go`
- Test: `internal/telegram/client_test.go`

- [ ] Add tests proving DB Telegram API settings can be loaded and used by a provider.
- [ ] Run the focused Go tests and confirm they fail before implementation.
- [ ] Implement a credentials provider that returns DB settings and no longer falls back to YAML for user-editable Telegram credentials.
- [ ] Wire `GotdClient` and `GotdListener` to resolve credentials at call time.
- [ ] Run the focused Go tests and confirm they pass.

### Task 2: Storage Limit Display Source

**Files:**
- Modify: `web/src/views/SettingsView.test.ts`
- Modify: `web/src/views/SettingsView.vue`

- [ ] Add a Vitest case proving Settings renders `max_db_bytes` and `max_media_bytes` from `/api/storage/usage`.
- [ ] Run the focused Vitest case and confirm it fails against the hard-coded values.
- [ ] Load storage usage in Settings and format the returned max bytes.
- [ ] Run the focused Vitest case and confirm it passes.

### Task 3: Telegram API Settings UI

**Files:**
- Modify: `web/src/views/SettingsView.test.ts`
- Modify: `web/src/views/SettingsView.vue`

- [ ] Add a Vitest case proving Settings loads `/api/settings/telegram-api` and saves app ID/hash with `PUT /api/settings/telegram-api`.
- [ ] Run the focused Vitest case and confirm it fails before UI implementation.
- [ ] Add Settings page fields and save action for Telegram API credentials.
- [ ] Run the focused Vitest case and confirm it passes.

### Task 4: Verification

**Files:**
- Verify all changed backend and frontend tests.

- [ ] Run `go test ./...`.
- [ ] Run focused Settings view Vitest tests.
- [ ] Run frontend typecheck if dependencies are available.
