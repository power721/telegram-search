# Phase 1E Index Search Resources Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the local index loop: message metadata/content split, sync cursors, history sync by Sync Profile, FTS5 search, Global Search, link/file extraction, Resources, and executable remote Telegram search for unsynced channels.

**Architecture:** Local search is the primary path. History sync writes metadata to `telegram_messages`, large body content to `telegram_message_contents`, extracted resources to link/file tables, and FTS rows from message content. Remote search reads Telegram directly only for explicit unsynced sources and keeps results out of persistent local index tables.

**Tech Stack:** Go 1.25, SQLite FTS5 via `modernc.org/sqlite`, Gin, existing Telegram client boundary, Vue 3, TypeScript, Pinia, Naive UI, Vitest.

---

## Prerequisite

Complete Phase 1D first:

[Phase 1D Channel Control Plan](/home/harold/workspace/telegram-search/docs/superpowers/plans/2026-06-08-phase-1d-channel-control.md)

## Scope

In scope:

- Split message metadata and content tables.
- Add `telegram_sync_cursors`.
- Update history sync to obey Sync Profile limits.
- Make writes idempotent by `(channel_id, telegram_message_id)`.
- Extract links and file metadata from persisted local messages.
- Add SQLite FTS5 index over `telegram_message_contents.text`.
- Add Global Search APIs grouped by Messages, Links, Files, and Channels.
- Add Resources APIs and UI as Telegram Resource Library.
- Execute remote Telegram search for unsynced allowed channels with temporary results.
- Add Search and Resources frontend views.
- Update Home top resource types using local indexed data.

Out of scope:

- Persistent task lifecycle, SSE, pause/resume/retry/cancel, and restart recovery. These are Phase 1F.
- Docker packaging and backup operations. These are Phase 1G.
- Media download/proxy and file-drive behavior. These are not Phase 1 product features.

## Data Rules

`telegram_messages` stores:

```text
account_id, channel_id, telegram_message_id, sender_id, date, edit_date, deleted, message_type, media_summary
```

`telegram_message_contents` stores:

```text
message_id, text, raw_json
```

FTS indexes only persisted local message content. Remote results use `source="remote"` and must not create rows in:

- `telegram_messages`
- `telegram_message_contents`
- `telegram_links`
- FTS tables

Sync cursors live in `telegram_sync_cursors`, not `telegram_channels.last_sync_message_id`.

## File Structure

- Modify `internal/db/migrations.go`: message/content/cursor/FTS/file/resource schema.
- Modify `internal/model/model.go`: message metadata/content, cursor, search, resource, remote result types.
- Modify `internal/repository/message.go`: split metadata/content writes and FTS maintenance.
- Create `internal/repository/sync_cursor.go`.
- Modify `internal/repository/link.go`: source snippet and resource fields.
- Create `internal/repository/file.go`: file metadata.
- Modify `internal/history/service.go`: Sync Profile limits, cursors, idempotent writes.
- Modify `internal/telegram/client.go`: `SearchMessages` and media/file metadata in message type.
- Modify `internal/search/service.go`: Global Search groups and scoped endpoints.
- Create `internal/resource/service.go`.
- Modify `internal/api/router.go`: search/resources routes.
- Modify `internal/api/handlers.go`: search/resources handlers.
- Add tests under `internal/repository`, `internal/history`, `internal/search`, `internal/resource`, `internal/api`.
- Modify `web/src/api/types.ts`: search/resource types.
- Create `web/src/stores/search.ts`.
- Create `web/src/stores/resources.ts`.
- Create `web/src/views/SearchView.vue`.
- Create `web/src/views/ResourcesView.vue`.
- Create `web/src/components/search/SearchFilters.vue`.
- Create `web/src/components/search/SearchResults.vue`.
- Create `web/src/components/resources/ResourceFilters.vue`.
- Create `web/src/components/resources/ResourceTable.vue`.
- Modify `web/src/views/HomeView.vue`: top resource types.
- Add frontend tests.

## Task 1: Message Contents And Sync Cursors Schema

**Files:**

- Modify: `internal/db/migrations.go`
- Modify: `internal/model/model.go`
- Modify: `internal/repository/message.go`
- Create: `internal/repository/sync_cursor.go`
- Test: `internal/repository/message_test.go`
- Test: `internal/repository/sync_cursor_test.go`

- [ ] **Step 1: Write message split tests**

Verify saving a message creates one row in `telegram_messages`, one row in `telegram_message_contents`, and an FTS row containing the message text.

Run:

```bash
go test ./internal/repository -run 'TestMessageContentSplit' -v
```

Expected: FAIL because content split is not implemented.

- [ ] **Step 2: Update schema**

Define:

```sql
CREATE TABLE telegram_messages (... metadata columns ...);
CREATE TABLE telegram_message_contents (
  message_id INTEGER PRIMARY KEY REFERENCES telegram_messages(id) ON DELETE CASCADE,
  text TEXT NOT NULL DEFAULT '',
  raw_json TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);
CREATE VIRTUAL TABLE telegram_message_fts USING fts5(text, content='telegram_message_contents', content_rowid='message_id');
```

Keep uniqueness:

```sql
UNIQUE(channel_id, telegram_message_id)
```

- [ ] **Step 3: Write cursor tests**

Verify unique cursor by `(account_id, channel_id, cursor_type)` and round-trip fields:

```go
CursorType: "history"
LastMessageID: 123
PTS: 10
QTS: 20
Date: now
```

Run:

```bash
go test ./internal/repository -run 'TestSyncCursorRepository' -v
```

Expected: FAIL until repository exists.

- [ ] **Step 4: Implement cursor repository**

Create methods:

```go
Save(ctx context.Context, cursor model.SyncCursor) error
Find(ctx context.Context, accountID int64, channelID int64, cursorType string) (model.SyncCursor, error)
```

- [ ] **Step 5: Verify and commit**

Run:

```bash
go test ./internal/db ./internal/repository
```

Expected: PASS.

Commit:

```bash
git add internal/db/migrations.go internal/model/model.go internal/repository/message.go internal/repository/message_test.go internal/repository/sync_cursor.go internal/repository/sync_cursor_test.go
git commit -m "feat: split message contents and add sync cursors"
```

## Task 2: History Sync With Sync Profiles

**Files:**

- Modify: `internal/history/service.go`
- Modify: `internal/telegram/client.go`
- Test: `internal/history/service_test.go`

- [ ] **Step 1: Write profile sync tests**

Verify:

- Quick fetches at most 100 messages.
- Normal fetches at most 1000 messages.
- Deep fetches at most 10000 messages.
- Full fetches until Telegram returns an empty batch.
- Cursor updates `telegram_sync_cursors` instead of `telegram_channels.last_sync_message_id`.

Run:

```bash
go test ./internal/history -run 'TestSyncChannelUsesSyncProfile' -v
```

Expected: FAIL because history sync still uses old cursor behavior.

- [ ] **Step 2: Update history sync request**

Add:

```go
func (s *Service) SyncChannel(ctx context.Context, channelID int64, profile string) (SyncResult, error)
```

Keep API wrappers passing the stored channel profile when no explicit profile is provided.

- [ ] **Step 3: Write split message output**

For each Telegram message:

- Save metadata to `telegram_messages`.
- Save text/raw JSON to `telegram_message_contents`.
- Update FTS after content write.
- Extract links and file metadata after content write.
- Save cursor after each successful batch.

- [ ] **Step 4: Verify and commit**

Run:

```bash
go test ./internal/history ./internal/repository
```

Expected: PASS.

Commit:

```bash
git add internal/history/service.go internal/history/service_test.go internal/telegram/client.go internal/repository/message.go
git commit -m "feat: sync channel history by profile"
```

## Task 3: Link And File Extraction

**Files:**

- Modify: `internal/model/model.go`
- Modify: `internal/repository/link.go`
- Create: `internal/repository/file.go`
- Modify: `internal/link/extractor.go`
- Test: `internal/repository/link_test.go`
- Test: `internal/repository/file_test.go`
- Test: `internal/link/extractor_test.go`

- [ ] **Step 1: Write extraction tests**

Verify link types:

```text
cloud_drive, magnet, ed2k, http
```

Verify file metadata fields:

```text
message_id, file_name, extension, mime_type, size_bytes
```

Run:

```bash
go test ./internal/link ./internal/repository -run 'Test.*Resource.*|Test.*File.*' -v
```

Expected: FAIL until repository and extraction fields exist.

- [ ] **Step 2: Add file schema and repository**

Create `telegram_files` with idempotency by `(message_id, file_name, size_bytes)`.

- [ ] **Step 3: Update link repository**

Add:

```text
source_snippet
category
```

The category is lightweight and derived from link type, file extension, and keyword hints.

- [ ] **Step 4: Verify and commit**

Run:

```bash
go test ./internal/link ./internal/repository
```

Expected: PASS.

Commit:

```bash
git add internal/model/model.go internal/db/migrations.go internal/repository/link.go internal/repository/link_test.go internal/repository/file.go internal/repository/file_test.go internal/link/extractor.go internal/link/extractor_test.go
git commit -m "feat: extract resource links and files"
```

## Task 4: Global Search API

**Files:**

- Modify: `internal/search/service.go`
- Modify: `internal/api/router.go`
- Modify: `internal/api/handlers.go`
- Test: `internal/search/service_test.go`
- Test: `internal/api/handlers_test.go`

- [ ] **Step 1: Write search service tests**

Seed messages, links, files, and channels. Verify `GlobalSearch("ubuntu")` returns grouped sections:

```json
{
  "messages": { "items": [], "total": 1 },
  "links": { "items": [], "total": 1 },
  "files": { "items": [], "total": 1 },
  "channels": { "items": [], "total": 1 }
}
```

Run:

```bash
go test ./internal/search -run 'TestGlobalSearchGroupsResults' -v
```

Expected: FAIL until Global Search exists.

- [ ] **Step 2: Implement scoped search methods**

Methods:

```go
Global(ctx, query SearchQuery) (GlobalSearchResult, error)
Messages(ctx, query SearchQuery) (ListResult[MessageSearchResult], error)
Links(ctx, query SearchQuery) (ListResult[LinkSearchResult], error)
Files(ctx, query SearchQuery) (ListResult[FileSearchResult], error)
Channels(ctx, query SearchQuery) (ListResult[ChannelSearchResult], error)
```

Filters:

- `account_id`
- `channel_id`
- `from`
- `to`
- `message_type`
- `link_type`
- `file_type`

- [ ] **Step 3: Add API routes**

Register:

```go
api.GET("/search/global", h.searchGlobal)
api.GET("/search/messages", h.searchMessages)
api.GET("/search/links", h.searchLinks)
api.GET("/search/files", h.searchFiles)
api.GET("/search/channels", h.searchChannels)
```

- [ ] **Step 4: Verify and commit**

Run:

```bash
go test ./internal/search ./internal/api
```

Expected: PASS.

Commit:

```bash
git add internal/search/service.go internal/search/service_test.go internal/api/router.go internal/api/handlers.go internal/api/handlers_test.go
git commit -m "feat: add global search api"
```

## Task 5: Remote Search Execution

**Files:**

- Modify: `internal/telegram/client.go`
- Create: `internal/search/remote.go`
- Modify: `internal/repository/remote_search.go`
- Modify: `internal/api/handlers.go`
- Test: `internal/search/remote_test.go`
- Test: `internal/api/handlers_test.go`

- [ ] **Step 1: Write remote search tests**

Verify remote search:

- Requires unsynced channel.
- Uses Telegram client search for that channel.
- Returns `source="remote"`.
- Does not increase local message, content, link, or FTS row counts.

Run:

```bash
go test ./internal/search -run 'TestRemoteSearchDoesNotPersistResults' -v
```

Expected: FAIL until remote search service exists.

- [ ] **Step 2: Extend Telegram client**

Add:

```go
SearchMessages(ctx context.Context, session AccountSession, channel ChannelRef, query string, limit int) ([]Message, error)
```

- [ ] **Step 3: Implement remote search service**

Use `remote_search_tasks` for task metadata and keep result content in short-lived memory keyed by task ID. Expire results at `expires_at`.

- [ ] **Step 4: Add result endpoint**

Register:

```go
api.GET("/search/remote/:task_id", h.getRemoteSearchTask)
```

Response items include Telegram location fields but no local message IDs.

- [ ] **Step 5: Verify and commit**

Run:

```bash
go test ./internal/search ./internal/api ./internal/repository
```

Expected: PASS.

Commit:

```bash
git add internal/telegram/client.go internal/search/remote.go internal/search/remote_test.go internal/repository/remote_search.go internal/api/handlers.go internal/api/handlers_test.go
git commit -m "feat: execute remote telegram search"
```

## Task 6: Resources API

**Files:**

- Create: `internal/resource/service.go`
- Modify: `internal/api/router.go`
- Modify: `internal/api/handlers.go`
- Test: `internal/resource/service_test.go`
- Test: `internal/api/handlers_test.go`

- [ ] **Step 1: Write resource service tests**

Seed duplicate links and files. Verify Resources returns deduplicated rows with source message navigation fields.

Run:

```bash
go test ./internal/resource -run 'TestResourceLibraryDeduplicatesLinks' -v
```

Expected: FAIL until service exists.

- [ ] **Step 2: Implement resource service**

Filters:

- `type`
- `category`
- `channel_id`
- `account_id`
- `extension`
- `sort=relevance|time`

Grouped endpoint returns counts for:

```text
cloud_drive, magnet, ed2k, http, files
```

- [ ] **Step 3: Add API routes**

Register:

```go
api.GET("/resources", h.resources)
api.GET("/resources/:id", h.resource)
api.GET("/resources/grouped", h.resourcesGrouped)
```

- [ ] **Step 4: Verify and commit**

Run:

```bash
go test ./internal/resource ./internal/api
```

Expected: PASS.

Commit:

```bash
git add internal/resource/service.go internal/resource/service_test.go internal/api/router.go internal/api/handlers.go internal/api/handlers_test.go
git commit -m "feat: add telegram resource library api"
```

## Task 7: Search And Resources UI

**Files:**

- Modify: `web/src/api/types.ts`
- Create: `web/src/stores/search.ts`
- Create: `web/src/stores/resources.ts`
- Create: `web/src/components/search/SearchFilters.vue`
- Create: `web/src/components/search/SearchResults.vue`
- Create: `web/src/components/resources/ResourceFilters.vue`
- Create: `web/src/components/resources/ResourceTable.vue`
- Create: `web/src/views/SearchView.vue`
- Create: `web/src/views/ResourcesView.vue`
- Modify: `web/src/views/HomeView.vue`
- Test: `web/src/stores/search.test.ts`
- Test: `web/src/stores/resources.test.ts`
- Test: `web/src/views/SearchView.test.ts`
- Test: `web/src/views/ResourcesView.test.ts`

- [ ] **Step 1: Write frontend tests**

Verify:

- Search page has tabs or grouped sections for Messages, Links, Files, Channels.
- Results show `local` or `remote` source.
- Resources page filters by cloud drive, magnet, ED2K, HTTP, and files.
- Home shows Top Resource Types.

Run:

```bash
npm run web:test -- search resources Home
```

Expected: FAIL until stores and views exist.

- [ ] **Step 2: Implement stores and views**

Search store calls:

```text
GET /api/search/global
GET /api/search/messages
GET /api/search/links
GET /api/search/files
GET /api/search/channels
POST /api/search/remote
GET /api/search/remote/:task_id
```

Resources store calls:

```text
GET /api/resources
GET /api/resources/grouped
GET /api/resources/:id
```

- [ ] **Step 3: Verify and commit**

Run:

```bash
npm run web:typecheck
npm run web:test
```

Expected: PASS.

Commit:

```bash
git add web/src/api/types.ts web/src/stores/search.ts web/src/stores/resources.ts web/src/components/search web/src/components/resources web/src/views/SearchView.vue web/src/views/ResourcesView.vue web/src/views/HomeView.vue web/src/**/*.test.ts
git commit -m "feat: add search and resources ui"
```

## Task 8: Documentation And Final Verification

**Files:**

- Modify: `docs/api.md`
- Modify: `README.md`

- [ ] **Step 1: Update documentation**

Document message/content split, sync cursors, Global Search scopes, Resource Library filters, and remote search display-only behavior.

- [ ] **Step 2: Run verification**

Run:

```bash
go test ./...
npm run web:typecheck
npm run web:test
```

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add docs/api.md README.md
git commit -m "docs: document search and resources"
```

## Self-Review Checklist

- [ ] `telegram_messages` no longer stores large text or raw JSON.
- [ ] History sync updates `telegram_sync_cursors`.
- [ ] FTS indexes local persisted content only.
- [ ] Global Search includes Messages, Links, Files, and Channels.
- [ ] Resources excludes remote-only results.
- [ ] Remote search does not persist local index rows.
