# Setup Channel Onboarding Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Telegram setup fast and usable for large channel lists.

**Architecture:** Queue slow Telegram channel metadata work after login, queue web-access checks after metadata sync, save selected channel controls in one backend call, and treat setup listen rules as global defaults. The frontend setup channel page renders fewer rows, removes per-channel sync profile controls, and displays channel access/state badges.

**Tech Stack:** Go 1.25, Gin, SQLite repositories, existing scheduler queue, Vue 3, Pinia, Naive UI, Vitest.

---

## File Structure

- Modify `internal/api/router.go`: register batch channel control.
- Modify `internal/api/handlers.go`: async metadata login response, batch channel control handler, global listen-rule decoding helper.
- Modify `internal/channel/service.go`: optionally trigger web-access checks after metadata sync.
- Modify `internal/messagefilter/filter.go`: fall back to global setup listen rules when no channel-specific rule exists.
- Modify `internal/repository/channel.go`: add batch control update.
- Modify `web/src/api/types.ts`: add batch payload/response types if needed.
- Modify `web/src/stores/channels.ts`: add `updateControls`, remove setup need for `createWatchRule`.
- Modify `web/src/views/SetupChannelSelectionView.vue`: status-aware, wider, lower-render-cost channel table.
- Modify corresponding Go and Vitest tests before implementation.

## Tasks

- [ ] Backend: write failing tests for async metadata sync queueing and batch channel control.
- [ ] Backend: implement batch channel control route and repository update.
- [ ] Backend: write failing tests for global listen-rule fallback in `messagefilter`.
- [ ] Backend: implement global listen-rule fallback from settings.
- [ ] Backend: write failing tests for metadata sync triggering web-access detection.
- [ ] Backend: implement web-access trigger after metadata sync.
- [ ] Frontend: write failing store/view tests for batch save, no watch-rule fanout, status labels, and reduced rendering.
- [ ] Frontend: implement setup channel selection UI/store changes.
- [ ] Verify: run targeted tests, then `go test ./...`, `npm --prefix web run test`, and typecheck/build if needed.
