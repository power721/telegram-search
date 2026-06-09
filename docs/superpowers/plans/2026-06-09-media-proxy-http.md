# Media Proxy HTTP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add standard HTTP metadata and HEAD support to file-id based media proxy endpoints.

**Architecture:** `internal/api/media_proxy.go` will carry indexed file metadata through media request resolution and centralize common header writing helpers. Tests in `internal/api/media_proxy_test.go` will seed file-id media rows and verify headers and method behavior.

**Tech Stack:** Go, Gin router, httptest, existing repository test helpers.

---

### Task 1: Media Context and Video Headers

**Files:**
- Modify: `internal/api/media_proxy_test.go`
- Modify: `internal/api/media_proxy.go`

- [ ] Write failing tests for `GET /v/:fileid` standard headers and `HEAD /v/:fileid` avoiding stream calls.
- [ ] Run `GOCACHE=/tmp/go-build-cache go test ./internal/api -run 'TestServeTelegramVideo'`.
- [ ] Add a `mediaRequest` struct carrying session, channel, message ID, and file metadata.
- [ ] Add helper functions for ETag, Last-Modified, Content-Disposition, and Cache-Control.
- [ ] Add `HEAD /v/:fileid` route.
- [ ] Update video handler to set headers and skip streaming on HEAD.
- [ ] Re-run `GOCACHE=/tmp/go-build-cache go test ./internal/api -run 'TestServeTelegramVideo'`.

### Task 2: Image Headers

**Files:**
- Modify: `internal/api/media_proxy_test.go`
- Modify: `internal/api/media_proxy.go`

- [ ] Write failing tests for image ETag, Last-Modified, and Content-Disposition.
- [ ] Run `GOCACHE=/tmp/go-build-cache go test ./internal/api -run 'TestServeTelegramImage'`.
- [ ] Add `HEAD /i/:fileid` route.
- [ ] Update image handler to set common headers and skip body writes on HEAD.
- [ ] Re-run `GOCACHE=/tmp/go-build-cache go test ./internal/api -run 'TestServeTelegramImage'`.

### Task 3: Verification and Commit

**Files:**
- Modify: `internal/api/media_proxy.go`
- Modify: `internal/api/media_proxy_test.go`
- Modify: `internal/api/router.go`

- [ ] Run `gofmt` on modified Go files.
- [ ] Run `GOCACHE=/tmp/go-build-cache go test ./internal/api`.
- [ ] Run `GOCACHE=/tmp/go-build-cache go test ./...`.
- [ ] Commit with `feat: improve media proxy http headers`.

