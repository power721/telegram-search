# PanSou Gap Closure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add PanSou-inspired link notes and merged-by-type link results for logged-in Telegram data.

**Architecture:** Keep gotd login and local SQLite indexing as the source of truth. Add optional link metadata at extraction/storage time and a read-side merged API that groups and ranks links without changing existing endpoints.

**Tech Stack:** Go, Gin, SQLite, FTS5, existing repository/service/API layers.

---

## File Structure

- Modify `internal/model/model.go`: add `Link.Note`, `MergedLink`, and `MergedLinksResponse`.
- Modify `internal/db/migrations.go`: add migration `007_link_note`.
- Modify `internal/db/db_test.go`: assert `telegram_links.note` exists.
- Modify `internal/link/extractor.go`: infer per-link notes from nearby message text.
- Modify `internal/link/extractor_test.go`: cover note attribution and provider-label false positives.
- Modify `internal/repository/types.go`: add merged link query params.
- Modify `internal/repository/link.go`: persist/load `note`; add merged query.
- Add/modify repository tests in `internal/repository/repository_test.go`.
- Modify `internal/search/service.go`: expose merged links through service layer.
- Modify `internal/api/router.go`: add `GET /api/links/merged`.
- Modify `internal/api/handlers.go`: implement merged handler using existing query parsing.
- Modify `internal/api/handlers_test.go`: add merged API tests.
- Modify `docs/api.md`, `docs/api-response-contract.md`, and `docs/TASKS.md`: document the new phase and API.

## Tasks

### Task 1: Persist Link Notes

- [ ] Write failing migration/model/repository tests for `note`.
- [ ] Add `note` to `model.Link`.
- [ ] Add migration version 7 with `ALTER TABLE telegram_links ADD COLUMN note TEXT`.
- [ ] Update link insert/select scans to include `note`.
- [ ] Run targeted tests.

### Task 2: Infer Link Notes

- [ ] Write failing extractor tests for title-before-link and provider-label-only cases.
- [ ] Add note inference after URL extraction and deduplication.
- [ ] Keep note empty when no clear title exists.
- [ ] Run `go test ./internal/link`.

### Task 3: Add Merged Repository And Service

- [ ] Write failing repository test for grouped results, deduplication, and newest context.
- [ ] Add `MergedLinkSearchParams`.
- [ ] Add `LinkRepository.SearchMerged`.
- [ ] Add `search.Service.MergedLinks`.
- [ ] Run repository/search tests.

### Task 4: Add Merged API

- [ ] Write failing API test for `GET /api/links/merged`.
- [ ] Add route and handler.
- [ ] Support existing filters plus `q`.
- [ ] Run `go test ./internal/api`.

### Task 5: Document And Verify

- [ ] Update API docs and task list.
- [ ] Run `go test ./...`.
- [ ] Review git diff for unrelated changes.

