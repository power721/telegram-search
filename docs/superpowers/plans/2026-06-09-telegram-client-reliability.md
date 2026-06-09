# Telegram Client Reliability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add runtime reliability settings to all gotd client constructors without moving Telegram API credentials into config files.

**Architecture:** `internal/config` parses runtime Telegram settings with safe defaults. `internal/telegram` owns a shared gotd options builder used by regular clients and QR login. `internal/update` receives the same runtime config and uses the shared builder for realtime listeners.

**Tech Stack:** Go, gotd/td, gotd/contrib floodwait and ratelimit middleware, existing YAML config loader, Go standard tests.

---

### Task 1: Config Schema

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

- [ ] Write failing tests for default Telegram runtime config, YAML override, and credential-field rejection.
- [ ] Run `GOCACHE=/tmp/go-build-cache go test ./internal/config`.
- [ ] Add `TelegramConfig` and nested `TelegramRateLimitConfig` with defaults.
- [ ] Update default, apply-defaults, and validation logic.
- [ ] Re-run `GOCACHE=/tmp/go-build-cache go test ./internal/config`.

### Task 2: Shared gotd Options Builder

**Files:**
- Create: `internal/telegram/options.go`
- Create: `internal/telegram/options_test.go`
- Modify: `internal/telegram/gotd_client.go`
- Modify: `internal/telegram/qr_login.go`

- [ ] Write failing tests for FloodWait default middleware, disabled/enabled rate limit middleware count, dial timeout, and invalid proxy.
- [ ] Run `GOCACHE=/tmp/go-build-cache go test ./internal/telegram`.
- [ ] Implement `RuntimeConfig`, `DefaultRuntimeConfig`, `BuildOptions`, and option input struct.
- [ ] Replace duplicate options construction in `GotdClient.withClient` and `GotdClient.StartQRLogin`.
- [ ] Re-run `GOCACHE=/tmp/go-build-cache go test ./internal/telegram`.

### Task 3: Listener Wiring

**Files:**
- Modify: `internal/update/gotd_listener.go`
- Test: `internal/update/gotd_listener_test.go`
- Modify: `cmd/tg-search/main.go`

- [ ] Write a failing listener constructor test showing runtime config is stored and defaulted.
- [ ] Run `GOCACHE=/tmp/go-build-cache go test ./internal/update`.
- [ ] Add variadic runtime config support to `NewGotdListener`.
- [ ] Use `telegram.BuildOptions` in `GotdListener.Run`.
- [ ] Pass `cfg.Telegram` from `cmd/tg-search/main.go`.
- [ ] Re-run `GOCACHE=/tmp/go-build-cache go test ./internal/update`.

### Task 4: Full Verification and Commit

**Files:**
- All modified files.

- [ ] Run `gofmt` on modified Go files.
- [ ] Run `GOCACHE=/tmp/go-build-cache go test ./...`.
- [ ] Commit with `feat: add telegram client reliability config`.

