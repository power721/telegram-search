# Settings System Info Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a read-only system information panel to the Settings page.

**Architecture:** The backend exposes a small settings API that gathers runtime system metadata from Go standard library calls and Linux procfs. The frontend loads this endpoint alongside existing Settings data and renders a compact panel.

**Tech Stack:** Go, Gin, Vue 3, TypeScript, Vitest.

---

### Task 1: Backend API

**Files:**
- Modify: `internal/model/model.go`
- Modify: `internal/api/version.go`
- Modify: `internal/api/router.go`
- Test: `internal/api/handlers_test.go`

- [x] Add a failing API test for `GET /api/settings/system-info` that expects non-empty `name`, `architecture`, `go_version`, and positive `cpu_count`.
- [x] Run `GOCACHE=/tmp/go-build-cache go test ./internal/api -run TestSystemInfoSettings -count=1` and confirm it fails because the route is missing.
- [x] Add `SystemInfoResponse`, implement `loadSystemInfo`, add `getSystemInfoSettings`, and register the route.
- [x] Run `GOCACHE=/tmp/go-build-cache go test ./internal/api -run TestSystemInfoSettings -count=1` and confirm it passes.

### Task 2: Frontend Settings Panel

**Files:**
- Modify: `web/src/api/types.ts`
- Modify: `web/src/views/SettingsView.vue`
- Test: `web/src/views/SettingsView.test.ts`

- [x] Add a failing SettingsView test that mocks `/api/settings/system-info` and expects the system details to render.
- [x] Run `npm --prefix web run test -- SettingsView.test.ts` and confirm it fails because the endpoint is not requested or rendered.
- [x] Add the `SystemInfoResponse` TypeScript type, load system info on mount, and render a `系统` panel.
- [x] Run `npm --prefix web run test -- SettingsView.test.ts` and confirm it passes.

### Task 3: Verification And Commit

**Files:**
- All changed files

- [x] Run `GOCACHE=/tmp/go-build-cache go test ./...`.
- [x] Run `npm run web:typecheck`.
- [x] Run `npm run web:test`.
- [ ] Commit the complete task branch with `feat: show system info in settings`.
