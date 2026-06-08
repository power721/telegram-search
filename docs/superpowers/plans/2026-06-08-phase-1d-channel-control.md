# Phase 1D Channel Control Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add channel management controls: channel table, channel analysis, Telegram Web Access Detection, Sync Profile selection, listen rules, storage-quota gate checks, and explicit remote-search entry controls.

**Architecture:** Keep channel control separate from message indexing. This phase creates durable per-channel configuration and lightweight analysis, but history sync and local search indexing are Phase 1E. Telegram Web Access Detection checks only `https://t.me/s/{username}` for message wrapper elements and must not be treated as a public search-index signal.

**Tech Stack:** Go 1.25, Gin, SQLite, net/http, `golang.org/x/net/html`, Vue 3, TypeScript, Pinia, Naive UI, Vitest.

---

## Prerequisite

Complete Phase 1C first:

[Phase 1C Telegram Onboarding Plan](/home/harold/workspace/telegram-search/docs/superpowers/plans/2026-06-08-phase-1c-telegram-onboarding.md)

## Scope

In scope:

- Channel list/table with filters and bulk selection.
- Sync Profile enum and mapping: `Quick=100`, `Normal=1000`, `Deep=10000`, `Full=all`.
- Per-channel history sync enabled flag, sync profile, realtime listening enabled flag, remote search allowed flag.
- Lightweight channel analysis fields derived from metadata and existing indexed counts when available.
- Telegram Web Access Detection with fields `web_access`, `web_access_checked_at`, `web_access_error`.
- Listen rules for includes, excludes, message types, and link types.
- Storage quota gate for Deep and Full sync requests.
- Remote-search entry controls that validate unsynced, user-accessible channels and create a display-only request record for Phase 1E execution.

Out of scope:

- Fetching and storing message history. This is Phase 1E.
- SQLite FTS and Global Search. This is Phase 1E.
- Persistent background task runtime and SSE. This is Phase 1F.
- Docker packaging. This is Phase 1G.

## Domain Rules

Sync Profiles:

```text
Quick  -> 100 messages
Normal -> 1000 messages
Deep   -> 10000 messages
Full   -> no explicit message limit
```

Defaults:

```text
history_sync_enabled = false
sync_profile = Normal
listen_enabled = false
remote_search_allowed = true for unsynced user-accessible sources
```

Storage quota rule:

- Allow Quick and Normal without quota gate failure.
- Before accepting Deep or Full, read storage usage.
- If DB usage is at or above `storage.max_db_size`, return HTTP `409` with code `storage_quota_exceeded`.

Telegram Web Access Detection:

- If `username` exists, request `https://t.me/s/{username}`.
- If HTML contains at least one `tgme_widget_message_wrap`, set `web_access=true`.
- If no element exists, set `web_access=false`.
- If no username exists, set `web_access=false` and `web_access_error=""`.
- On HTTP, DNS, timeout, or parse failure, set `web_access=false` and store `web_access_error`.

## File Structure

- Modify `internal/model/model.go`: channel control fields, sync profile constants, analysis response, remote search request record.
- Modify `internal/db/migrations.go`: fresh schema fields and `remote_search_tasks` baseline record.
- Create `internal/channel/profile.go`: Sync Profile parser and limit mapping.
- Modify `internal/channel/service.go`: channel control updates and lightweight analysis.
- Modify `internal/channel/web_access.go`: exact Telegram Web Access Detection behavior and error persistence.
- Modify `internal/repository/channel.go`: filters, control updates, web access error persistence.
- Modify `internal/repository/watch_rule.go`: message/link type filters.
- Create `internal/repository/remote_search.go`: display-only remote search request metadata.
- Modify `internal/api/router.go`: channel control routes.
- Modify `internal/api/handlers.go`: channel control handlers.
- Add/modify backend tests under `internal/channel`, `internal/repository`, and `internal/api`.
- Modify `web/src/api/types.ts`: channel control types.
- Create `web/src/stores/channels.ts`.
- Create `web/src/views/ChannelsView.vue`.
- Create `web/src/components/channels/SyncProfileSelect.vue`.
- Create `web/src/components/channels/WebAccessBadge.vue`.
- Create `web/src/components/channels/ChannelControlDrawer.vue`.
- Add frontend tests beside changed stores/components/views.
- Modify `docs/api.md`.

## Task 1: Sync Profile And Channel Control Schema

**Files:**

- Modify: `internal/model/model.go`
- Modify: `internal/db/migrations.go`
- Create: `internal/channel/profile.go`
- Test: `internal/channel/profile_test.go`
- Test: `internal/repository/channel_test.go`

- [ ] **Step 1: Write Sync Profile tests**

Create tests verifying:

```go
ProfileLimit("Quick") == 100
ProfileLimit("Normal") == 1000
ProfileLimit("Deep") == 10000
ProfileLimit("Full") == 0
ParseProfile("raw-1000") returns an error
```

Run:

```bash
go test ./internal/channel -run 'TestSyncProfile' -v
```

Expected: FAIL because profile helpers do not exist.

- [ ] **Step 2: Implement profile helpers**

Create `internal/channel/profile.go` with constants:

```go
const (
	SyncProfileQuick = "Quick"
	SyncProfileNormal = "Normal"
	SyncProfileDeep = "Deep"
	SyncProfileFull = "Full"
)
```

`ProfileLimit("Full")` returns `0`, meaning no message limit.

- [ ] **Step 3: Write channel control persistence tests**

Verify a channel can save and load:

```go
HistorySyncEnabled: true
SyncProfile: "Deep"
ListenEnabled: false
RemoteSearchAllowed: true
```

Run:

```bash
go test ./internal/repository -run 'TestChannelControlFields' -v
```

Expected: FAIL because fields and columns are missing.

- [ ] **Step 4: Add schema and repository fields**

Add columns:

```sql
history_sync_enabled BOOLEAN NOT NULL DEFAULT 0,
sync_profile TEXT NOT NULL DEFAULT 'Normal',
listen_enabled BOOLEAN NOT NULL DEFAULT 0,
remote_search_allowed BOOLEAN NOT NULL DEFAULT 1
```

Add repository method:

```go
func (r *ChannelRepository) UpdateControl(ctx context.Context, id int64, control model.ChannelControl) error
```

- [ ] **Step 5: Verify and commit**

Run:

```bash
go test ./internal/channel ./internal/repository
```

Expected: PASS.

Commit:

```bash
git add internal/model/model.go internal/db/migrations.go internal/channel/profile.go internal/channel/profile_test.go internal/repository/channel.go internal/repository/channel_test.go
git commit -m "feat: add channel sync profiles"
```

## Task 2: Channel Control API

**Files:**

- Modify: `internal/api/router.go`
- Modify: `internal/api/handlers.go`
- Test: `internal/api/handlers_test.go`

- [ ] **Step 1: Write API tests**

Add tests for:

- `PATCH /api/channels/:id/control` updates profile and toggles.
- Invalid profile returns `400`.
- Deep profile returns `409 storage_quota_exceeded` when DB usage exceeds configured max.

Run:

```bash
go test ./internal/api -run 'TestChannelControlAPI' -v
```

Expected: FAIL because route does not exist.

- [ ] **Step 2: Implement route**

Register:

```go
api.PATCH("/channels/:id/control", h.updateChannelControl)
```

Request:

```json
{
  "history_sync_enabled": true,
  "sync_profile": "Normal",
  "listen_enabled": false,
  "remote_search_allowed": true
}
```

Validate profile through `channel.ParseProfile`.

- [ ] **Step 3: Verify and commit**

Run:

```bash
go test ./internal/api ./internal/channel ./internal/repository
```

Expected: PASS.

Commit:

```bash
git add internal/api/router.go internal/api/handlers.go internal/api/handlers_test.go
git commit -m "feat: add channel control api"
```

## Task 3: Telegram Web Access Detection

**Files:**

- Modify: `internal/channel/web_access.go`
- Modify: `internal/repository/channel.go`
- Test: `internal/channel/web_access_test.go`
- Test: `internal/api/handlers_test.go`

- [ ] **Step 1: Write detector tests**

Use an `httptest.Server` and a checker base URL override to verify:

- HTML containing `<div class="tgme_widget_message_wrap">` stores `web_access=true`.
- HTML without that class stores `web_access=false`.
- Empty username stores `web_access=false` and no error.
- HTTP 500 stores `web_access=false` and non-empty `web_access_error`.

Run:

```bash
go test ./internal/channel -run 'TestWebAccessDetection' -v
```

Expected: FAIL until checker persists errors.

- [ ] **Step 2: Implement exact detection behavior**

Rename UI/API wording to `Telegram Web Access Detection`. Keep database fields:

```go
WebAccess *bool
WebAccessCheckedAt *time.Time
WebAccessError string
```

Persist errors with:

```go
func (r *ChannelRepository) UpdateWebAccessResult(ctx context.Context, channelID int64, access bool, checkedAt time.Time, errorText string) error
```

- [ ] **Step 3: Keep old web-search status names out**

Run:

```bash
rg -n 'WEB_SEARCHABLE|PRIVATE_ONLY|PARTIALLY_SEARCHABLE' internal web docs/api.md README.md
```

Expected: no matches.

- [ ] **Step 4: Verify and commit**

Run:

```bash
go test ./internal/channel ./internal/api ./internal/repository
```

Expected: PASS.

Commit:

```bash
git add internal/channel/web_access.go internal/channel/web_access_test.go internal/repository/channel.go internal/api/handlers.go internal/api/handlers_test.go
git commit -m "feat: add telegram web access detection"
```

## Task 4: Listen Rules And Lightweight Analysis

**Files:**

- Modify: `internal/model/model.go`
- Modify: `internal/repository/watch_rule.go`
- Modify: `internal/channel/service.go`
- Modify: `internal/api/router.go`
- Modify: `internal/api/handlers.go`
- Test: `internal/repository/watch_rule_test.go`
- Test: `internal/channel/service_test.go`
- Test: `internal/api/handlers_test.go`

- [ ] **Step 1: Write watch rule tests**

Verify create/update/load supports:

```json
{
  "includes": ["电影", "课程"],
  "excludes": ["广告"],
  "message_types": ["text", "file"],
  "link_types": ["cloud_drive", "magnet", "ed2k", "http"]
}
```

Run:

```bash
go test ./internal/repository -run 'TestWatchRuleMessageAndLinkTypes' -v
```

Expected: FAIL until fields are added.

- [ ] **Step 2: Implement rule fields**

Add `message_types_json` and `link_types_json` columns. Expose JSON fields as `message_types` and `link_types`.

- [ ] **Step 3: Add analysis endpoint tests**

Test `POST /api/channels/:id/analyze` returns channel metadata, current control state, watch rule summary, and available indexed counts as zero when no message index exists.

Run:

```bash
go test ./internal/api -run 'TestChannelAnalyze' -v
```

Expected: FAIL until endpoint exists.

- [ ] **Step 4: Implement lightweight analysis**

Register:

```go
api.POST("/channels/:id/analyze", h.analyzeChannel)
```

The response must not fetch history. It may read current repository counts only.

- [ ] **Step 5: Verify and commit**

Run:

```bash
go test ./internal/repository ./internal/channel ./internal/api
```

Expected: PASS.

Commit:

```bash
git add internal/model/model.go internal/db/migrations.go internal/repository/watch_rule.go internal/repository/watch_rule_test.go internal/channel/service.go internal/channel/service_test.go internal/api/router.go internal/api/handlers.go internal/api/handlers_test.go
git commit -m "feat: add channel analysis and listen rules"
```

## Task 5: Remote Search Entry Records

**Files:**

- Modify: `internal/model/model.go`
- Modify: `internal/db/migrations.go`
- Create: `internal/repository/remote_search.go`
- Test: `internal/repository/remote_search_test.go`
- Modify: `internal/api/router.go`
- Modify: `internal/api/handlers.go`
- Test: `internal/api/handlers_test.go`

- [ ] **Step 1: Write repository tests**

Verify a remote search request stores:

```go
AccountID: accountID
ChannelID: channelID
Query: "ubuntu iso"
Status: "queued"
ExpiresAt: now.Add(30 * time.Minute)
```

Run:

```bash
go test ./internal/repository -run 'TestRemoteSearchTaskRepository' -v
```

Expected: FAIL until repository exists.

- [ ] **Step 2: Implement remote search metadata table**

Create `remote_search_tasks` with fields from the product spec. This phase stores task metadata only; result execution is handled by Phase 1E search work.

- [ ] **Step 3: Add API validation tests**

Test `POST /api/search/remote`:

- Rejects empty query with `400`.
- Rejects synced channels with `409` and code `remote_search_requires_unsynced_channel`.
- Rejects `remote_search_allowed=false` with `409`.
- Creates a queued task for unsynced allowed channels.

Run:

```bash
go test ./internal/api -run 'TestRemoteSearchEntry' -v
```

Expected: FAIL until route exists.

- [ ] **Step 4: Implement route**

Register:

```go
api.POST("/search/remote", h.createRemoteSearchTask)
```

Response:

```json
{
  "id": 1,
  "status": "queued",
  "source": "remote",
  "expires_at": "2026-06-08T10:30:00Z"
}
```

- [ ] **Step 5: Verify and commit**

Run:

```bash
go test ./internal/repository ./internal/api
```

Expected: PASS.

Commit:

```bash
git add internal/model/model.go internal/db/migrations.go internal/repository/remote_search.go internal/repository/remote_search_test.go internal/api/router.go internal/api/handlers.go internal/api/handlers_test.go
git commit -m "feat: add remote search entry records"
```

## Task 6: Channels UI

**Files:**

- Modify: `web/src/api/types.ts`
- Create: `web/src/stores/channels.ts`
- Create: `web/src/components/channels/SyncProfileSelect.vue`
- Create: `web/src/components/channels/WebAccessBadge.vue`
- Create: `web/src/components/channels/ChannelControlDrawer.vue`
- Create: `web/src/views/ChannelsView.vue`
- Test: `web/src/stores/channels.test.ts`
- Test: `web/src/views/ChannelsView.test.ts`

- [ ] **Step 1: Write frontend tests**

Test that the Channels page renders title, username, type, sync state, listen state, web access state, and Sync Profile labels `Quick`, `Normal`, `Deep`, `Full`.

Run:

```bash
npm run web:test -- channels
```

Expected: FAIL until store and view exist.

- [ ] **Step 2: Implement channel store**

Actions:

```ts
loadChannels(accountId?: number)
updateControl(channelId: number, payload: ChannelControlPayload)
checkWebAccess(channelIds: number[])
analyzeChannel(channelId: number)
createRemoteSearch(channelId: number, query: string)
```

- [ ] **Step 3: Implement Channels view**

Use a dense table with row actions:

- Analyze
- Check Web Access
- Edit Controls
- Remote Search

Full sync profile selection requires a confirmation modal before saving `sync_profile="Full"`.

- [ ] **Step 4: Verify and commit**

Run:

```bash
npm run web:typecheck
npm run web:test
```

Expected: PASS.

Commit:

```bash
git add web/src/api/types.ts web/src/stores/channels.ts web/src/components/channels web/src/views/ChannelsView.vue web/src/**/*.test.ts
git commit -m "feat: add channel control ui"
```

## Task 7: Documentation And Final Verification

**Files:**

- Modify: `docs/api.md`
- Modify: `README.md`

- [ ] **Step 1: Update documentation**

Document Sync Profiles, channel control payloads, Telegram Web Access Detection semantics, listen rule fields, and remote-search entry constraints.

- [ ] **Step 2: Run verification**

Run:

```bash
go test ./...
npm run web:typecheck
npm run web:test
rg -n 'WEB_SEARCHABLE|PRIVATE_ONLY|PARTIALLY_SEARCHABLE' internal web docs/api.md README.md
```

Expected:

- Tests pass.
- Typecheck passes.
- Search command finds no obsolete naming in active product spec or this plan.

- [ ] **Step 3: Commit**

```bash
git add docs/api.md README.md
git commit -m "docs: document channel control"
```

## Self-Review Checklist

- [ ] Sync Profile names are user-facing; raw numbers are implementation details.
- [ ] Telegram Web Access Detection is not described as search engine indexing.
- [ ] Deep and Full control changes check storage quota.
- [ ] Channel analysis does not fetch message history.
- [ ] Remote search records are display-only task metadata and do not write local index rows.
