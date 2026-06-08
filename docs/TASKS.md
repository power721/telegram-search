# TASKS.md
# TG Provider for AList-TVBox

Version: v1.0

This document breaks the TG Provider project into small, independently reviewable development tasks. Each task should map to one pull request where possible.

## Development Rules for Codex

- One task equals one focused PR.
- Do not implement future-phase features early unless required by the current task.
- Keep the service runnable after every merged task.
- Add tests for repositories, services, parsers, and API handlers where practical.
- Prefer simple, explicit code over over-engineered abstractions.
- Do not expose the service publicly; it must listen on localhost by default.
- Do not let Telegram FloodWait or network errors crash the service.
- Spring Boot / AList-TVBox must call HTTP APIs only and must not access SQLite directly.

---

# Phase 0: Project Bootstrap

## Task 001: Initialize Go module

Create the initial Go project structure for `tg-provider`.

Deliverables:

- `go.mod`
- `cmd/tg-provider/main.go`
- `internal/` package layout
- Basic build command

Acceptance Criteria:

- `go build ./...` succeeds.
- The binary starts and exits cleanly.

## Task 002: Add base project directories

Create the internal package layout.

Suggested structure:

```text
internal/api
internal/config
internal/db
internal/logger
internal/account
internal/session
internal/channel
internal/history
internal/update
internal/search
internal/link
internal/scheduler
internal/telegram
internal/model
internal/repository
```

Acceptance Criteria:

- Packages compile.
- No circular package dependency exists.

## Task 003: Add YAML configuration loader

Implement config loading from `/data/tg-provider/config.yaml` with fallback to local `config.yaml`.

Config sections:

- `telegram.api_id`
- `telegram.api_hash`
- `server.host`
- `server.port`
- `sync.workers`
- `sync.history_batch_size`
- `storage.path`

Acceptance Criteria:

- Missing required Telegram fields return a clear error.
- Default server is `127.0.0.1:9900`.
- Default storage path is `/data/tg-provider`.

## Task 004: Add runtime directory initialization

Ensure required directories exist at startup.

Directories:

- `/data/tg-provider`
- `/data/tg-provider/sessions`
- `/data/tg-provider/logs`
- `/data/tg-provider/backup`

Acceptance Criteria:

- Directories are created if missing.
- Startup fails with clear error if directories cannot be created.

## Task 005: Add structured logging

Add zap logging with lumberjack rotation.

Log files:

- `app.log`
- `sync.log`
- `telegram.log`
- `error.log`

Acceptance Criteria:

- Logs are written under `/data/tg-provider/logs`.
- Error logs are separated.
- Log rotation is configured.

## Task 006: Add graceful shutdown skeleton

Handle SIGINT and SIGTERM.

Acceptance Criteria:

- Service shuts down cleanly.
- Shutdown path calls registered cleanup functions.
- Logs contain startup and shutdown events.

---

# Phase 1: Login, Session, SQLite, History Sync

## Task 007: Add SQLite connection manager

Implement SQLite open/close logic.

Acceptance Criteria:

- Opens `/data/tg-provider/telegram.db`.
- Enables WAL mode.
- Applies required PRAGMA settings.
- Closes cleanly on shutdown.

## Task 008: Add database migration framework

Create a lightweight migration runner.

Acceptance Criteria:

- Tracks applied migrations.
- Runs migrations in order.
- Safe to run repeatedly.

## Task 009: Create core database tables

Add migrations for:

- `telegram_accounts`
- `telegram_channels`
- `telegram_messages`
- `telegram_links`

Acceptance Criteria:

- Tables match the PRD and architecture design.
- Primary keys are defined.
- Required timestamp fields are present.

## Task 010: Add SQLite indexes

Add indexes for message and link query performance.

Indexes:

- `telegram_messages(channel_id, date)`
- `telegram_messages(telegram_message_id)`
- `telegram_links(type)`
- `telegram_links(message_id)`

Acceptance Criteria:

- Migration creates indexes.
- Re-running migrations is safe.

## Task 011: Add FTS5 virtual table

Create `telegram_messages_fts`.

Acceptance Criteria:

- FTS5 table indexes message text.
- Migration fails clearly if SQLite does not support FTS5.

## Task 012: Add FTS5 triggers

Keep FTS table synchronized with message changes.

Acceptance Criteria:

- Insert updates FTS.
- Update updates FTS.
- Delete or soft-delete updates FTS.

## Task 013: Implement account model and repository

Add account model and repository methods.

Methods:

- `Save`
- `Update`
- `Delete`
- `FindByID`
- `FindAll`

Acceptance Criteria:

- Repository tests pass.
- Account status can be persisted and updated.

## Task 014: Implement channel model and repository

Add channel model and repository methods.

Methods:

- `Save`
- `UpdateCursor`
- `FindAll`
- `FindByAccountID`

Acceptance Criteria:

- Channel metadata can be saved.
- Last message cursor can be updated.

## Task 015: Implement message model and repository

Add message model and repository methods.

Methods:

- `SaveBatch`
- `Search`
- `Latest`
- `MarkDeleted`

Acceptance Criteria:

- Batch insert works transactionally.
- Duplicate Telegram message IDs are handled safely.

## Task 016: Implement link model and repository

Add link model and repository methods.

Methods:

- `Save`
- `SaveBatch`
- `Search`
- `FindByMessageID`

Acceptance Criteria:

- Links are associated with messages.
- Search by link type works.

## Task 017: Add Gin server bootstrap

Start HTTP API server using Gin.

Acceptance Criteria:

- Server listens on configured host and port.
- Default bind address is `127.0.0.1:9900`.
- `/api/status` returns a basic response.

## Task 018: Add health/status API

Implement `GET /api/status`.

Response includes:

- accounts count
- channels count
- messages count
- links count
- service status

Acceptance Criteria:

- API returns JSON.
- Counts are read from SQLite.

## Task 019: Add gotd client factory

Create a Telegram client factory based on gotd.

Acceptance Criteria:

- Uses configured `api_id` and `api_hash`.
- Session storage path is account-specific.
- Telegram logs go to `telegram.log`.

## Task 020: Implement SessionManager

Persist and load Telegram sessions from `/data/tg-provider/sessions`.

Acceptance Criteria:

- Session survives service restart.
- Session file path is deterministic per account.

## Task 021: Implement login send-code API

Implement `POST /api/login/send-code`.

Request:

```json
{"phone":"+123456789"}
```

Acceptance Criteria:

- Sends Telegram login code.
- Creates or updates account as `LOGIN_REQUIRED`.
- Does not log sensitive code hash values.

## Task 022: Implement login sign-in API

Implement `POST /api/login/sign-in`.

Request:

```json
{"phone":"+123456789","code":"12345"}
```

Acceptance Criteria:

- Completes login when no 2FA password is required.
- Stores account profile.
- Persists session.

## Task 023: Implement 2FA password API

Implement `POST /api/login/password`.

Acceptance Criteria:

- Handles Telegram password-required flow.
- Updates account status after success.
- Returns clear error for invalid password.

## Task 024: Implement account list API

Implement `GET /api/accounts`.

Acceptance Criteria:

- Returns all accounts.
- Does not expose session file contents.

## Task 025: Implement account delete API

Implement `DELETE /api/accounts/{id}`.

Acceptance Criteria:

- Stops account client if running.
- Removes account record.
- Removes or disables related session according to implementation decision.

## Task 026: Implement ChannelSyncService skeleton

Create service for syncing dialogs/channels/groups/saved messages.

Acceptance Criteria:

- Service can be invoked for one account.
- Logs sync start and end.

## Task 027: Fetch Telegram channel list

Use gotd to fetch user channels and supergroups.

Acceptance Criteria:

- Saves channel metadata.
- Captures `telegram_channel_id`, `access_hash`, `title`, `username`, and `type`.

## Task 028: Add Saved Messages as virtual channel

Add Saved Messages support.

Acceptance Criteria:

- Saved Messages appears as a searchable source.
- Type is `saved_messages`.

## Task 029: Implement channels API

Implement:

- `GET /api/channels`
- `GET /api/channels/{id}`

Acceptance Criteria:

- Supports account filter.
- Returns sync cursor fields.

## Task 030: Implement manual channel sync API

Implement `POST /api/channels/{id}/sync`.

Acceptance Criteria:

- Starts history sync for selected channel.
- Returns immediately with task status or accepted response.

## Task 031: Implement HistorySyncService batch fetch

Fetch historical messages by batch.

Acceptance Criteria:

- Batch size uses config value.
- Stops when no more messages exist.
- Does not duplicate existing messages.

## Task 032: Implement history sync cursor

Use `last_message_id` and `last_sync_time` as sync cursor.

Acceptance Criteria:

- Sync can resume after restart.
- Cursor updates after successful batch commit.

## Task 033: Implement transactional message storage

Persist fetched messages and extracted links in one transaction.

Acceptance Criteria:

- Message batch and links commit together.
- Failed batch rolls back cleanly.

## Task 034: Add basic message search API

Implement `GET /api/search?q=keyword`.

Acceptance Criteria:

- Uses SQLite FTS5.
- Returns message text, account, channel, date, and links.

## Task 035: Add latest messages API

Implement `GET /api/messages/latest`.

Acceptance Criteria:

- Returns latest messages ordered by date descending.
- Supports limit parameter.

## Task 036: Add links API skeleton

Implement `GET /api/links` with basic pagination.

Acceptance Criteria:

- Returns extracted links.
- Supports `type` filter.

## Task 037: Add Phase 1 integration test

Create an integration test for DB, repositories, and search.

Acceptance Criteria:

- Inserts sample account/channel/messages/links.
- Search returns expected messages.
- Latest messages API returns expected order.

---

# Phase 2: Updates Engine Realtime Listener

## Task 038: Add UpdateService skeleton

Create UpdateService and UpdateListener packages.

Acceptance Criteria:

- Service can start and stop per account.
- No-op listener compiles and runs.

## Task 039: Integrate gotd Updates Engine

Start Telegram Updates Engine after session restore.

Acceptance Criteria:

- Updates listener starts for an online account.
- Listener stops cleanly on shutdown.

## Task 040: Handle new message updates

Process new message events.

Acceptance Criteria:

- New Telegram message is persisted.
- FTS index is updated.
- Links are extracted and stored.

## Task 041: Handle edited message updates

Process message edit events.

Acceptance Criteria:

- Existing message text is updated.
- `edit_date` is stored.
- FTS entry is updated.
- Links are recalculated.

## Task 042: Handle deleted message updates

Process delete events.

Acceptance Criteria:

- Message is marked `deleted=1`.
- Deleted message is removed from search results or clearly filtered.

## Task 043: Add update processor queue

Add an internal queue between UpdateListener and database writes.

Acceptance Criteria:

- Updates are processed sequentially or with safe workers.
- Queue flushes on shutdown.

## Task 044: Add update recovery logic

Ensure listener can recover from temporary Telegram/network errors.

Acceptance Criteria:

- Account moves to `RECONNECTING` on disconnect.
- Listener restarts automatically.
- Errors are logged without process crash.

## Task 045: Add Phase 2 tests

Test message processor behavior without real Telegram dependency.

Acceptance Criteria:

- New/edit/delete update handlers are covered.
- Repository state is correct after each event.

---

# Phase 3: Multi-account Management

## Task 046: Implement AccountManager lifecycle

Implement account lifecycle manager.

Methods:

- `Start`
- `Stop`
- `Restart`
- `List`

Acceptance Criteria:

- Loads all accounts from SQLite.
- Starts online accounts.
- Stops all accounts on shutdown.

## Task 047: Implement account state machine

Support states:

- `NEW`
- `LOGIN_REQUIRED`
- `SYNCING`
- `ONLINE`
- `RECONNECTING`
- `FLOOD_WAIT`
- `DISCONNECTED`

Acceptance Criteria:

- State transitions are explicit.
- Invalid transitions are logged.

## Task 048: Start multiple Telegram clients

Run one gotd client per account.

Acceptance Criteria:

- Multiple accounts can be online simultaneously.
- Session files do not conflict.

## Task 049: Add account health check job

Scheduler checks account health every minute.

Acceptance Criteria:

- Offline accounts are detected.
- Reconnect is attempted.

## Task 050: Add account-level channel sync

Allow channel sync per account.

Acceptance Criteria:

- Channel sync does not mix accounts.
- APIs support account filters.

## Task 051: Add account-aware search

Extend search to support account filtering.

Acceptance Criteria:

- `GET /api/search?q=x&account_id=1` works.
- Results include account display name.

## Task 052: Add account-aware status response

Enhance `/api/status` with account state summary.

Acceptance Criteria:

- Shows count by state.
- Shows total channels/messages/links.

## Task 053: Add Phase 3 tests

Test multi-account repository and service isolation.

Acceptance Criteria:

- Data from different accounts remains isolated.
- Search filter returns only selected account data.

---

# Phase 4: Cloud Drive Link Parsing

## Task 054: Create LinkExtractor interface

Add parser abstraction.

Interface idea:

```go
type Extractor interface {
    Extract(text string) []Link
}
```

Acceptance Criteria:

- Extractor is independent from database.
- Unit tests can run without Telegram.

## Task 055: Parse 115 links

Implement 115 cloud link parser.

Acceptance Criteria:

- Detects 115 share URLs.
- Extracts password/code when present.

## Task 056: Parse 123 cloud links

Implement 123 cloud parser.

Acceptance Criteria:

- Detects 123 share URLs.
- Extracts password/code when present.

## Task 057: Parse Aliyun Drive links

Implement Aliyun Drive parser.

Acceptance Criteria:

- Detects Aliyun Drive share URLs.
- Extracts password/code when present.

## Task 058: Parse Quark links

Implement Quark cloud parser.

Acceptance Criteria:

- Detects Quark share URLs.
- Extracts password/code when present.

## Task 059: Parse UC cloud links

Implement UC cloud parser.

Acceptance Criteria:

- Detects UC share URLs.
- Extracts password/code when present.

## Task 060: Parse Baidu Netdisk links

Implement Baidu Netdisk parser.

Acceptance Criteria:

- Detects Baidu links.
- Extracts extraction code when present.

## Task 061: Parse Tianyi cloud links

Implement Tianyi cloud parser.

Acceptance Criteria:

- Detects Tianyi cloud URLs.
- Extracts password/code when present.

## Task 062: Parse China Mobile cloud links

Implement China Mobile cloud parser.

Acceptance Criteria:

- Detects mobile cloud URLs.
- Extracts password/code when present.

## Task 063: Parse PikPak links

Implement PikPak parser.

Acceptance Criteria:

- Detects PikPak share URLs.
- Extracts password/code when present.

## Task 064: Parse Xunlei links

Implement Xunlei parser.

Acceptance Criteria:

- Detects Xunlei share URLs.
- Extracts password/code when present.

## Task 065: Parse magnet and ED2K links

Implement magnet and ED2K parser.

Acceptance Criteria:

- Detects `magnet:?` links.
- Detects `ed2k://` links.

## Task 066: Add link parser test corpus

Create realistic parser test samples.

Acceptance Criteria:

- Each supported provider has positive tests.
- Common false positives are covered.

## Task 067: Integrate LinkExtractor into history sync

Extract links during historical message sync.

Acceptance Criteria:

- Links are saved with message batch.
- Duplicate links are handled safely.

## Task 068: Integrate LinkExtractor into update listener

Extract links for new and edited messages.

Acceptance Criteria:

- New message creates links.
- Edited message refreshes links.

## Task 069: Add cloud-drive filter to search

Support `link_type` filter in search API.

Acceptance Criteria:

- `GET /api/search?q=x&link_type=aliyun` works.
- Results only include matching link type.

## Task 070: Enhance links API

Improve `GET /api/links`.

Filters:

- type
- account_id
- channel_id
- keyword
- date range

Acceptance Criteria:

- Pagination works.
- Filters can be combined.

---

# Phase 5: FTS5 and Performance Optimization

## Task 071: Add search filters

Enhance search API filters.

Filters:

- channel_id
- account_id
- date range
- link_type
- limit
- offset or cursor

Acceptance Criteria:

- Filters are covered by tests.
- Invalid filters return clear 400 errors.

## Task 072: Optimize FTS query plan

Review and optimize FTS search SQL.

Acceptance Criteria:

- Uses FTS table efficiently.
- Joins messages/channels/accounts/links without full scans where possible.

## Task 073: Add pagination strategy

Implement efficient pagination.

Acceptance Criteria:

- Supports stable ordering.
- Avoids slow deep offset where practical.

## Task 074: Add bulk insert optimization

Optimize history sync write path.

Acceptance Criteria:

- Uses transactions.
- Uses prepared statements where useful.
- Batch write performance is measured.

## Task 075: Add SQLite maintenance options

Add optional maintenance operations.

Possible operations:

- `ANALYZE`
- `VACUUM`
- FTS optimize

Acceptance Criteria:

- Operations are not run dangerously on every request.
- Can be triggered by scheduler or internal command.

## Task 076: Add performance benchmark seed tool

Create a local tool to seed test data.

Acceptance Criteria:

- Can generate 100k+ sample messages.
- Can generate links and channels.

## Task 077: Add 1 million message benchmark

Create benchmark scenario for 1 million messages.

Acceptance Criteria:

- Search target is under 200ms on expected deployment hardware.
- Results are documented.

## Task 078: Add memory usage checks

Measure memory during sync and search.

Acceptance Criteria:

- History sync does not load all messages into memory.
- Search response is bounded by limit.

## Task 079: Add sync worker pool

Implement configurable history sync workers.

Acceptance Criteria:

- Worker count uses config.
- Same channel is not synced concurrently by multiple workers.

## Task 080: Add retry queue

Implement retry queue for failed sync/update jobs.

Acceptance Criteria:

- Temporary failures are retried.
- Permanent failures are logged with reason.

## Task 081: Implement FloodWait strategy

Handle Telegram FloodWait globally.

Acceptance Criteria:

- Detects wait seconds.
- Sleeps or schedules retry safely.
- Uses exponential backoff up to 30 minutes.
- Service does not crash.

## Task 082: Add cleanup scheduler

Implement cleanup job skeleton.

Acceptance Criteria:

- Old temporary data can be cleaned safely.
- Logs cleanup activity.

---

# Phase 5C: PanSou Gap Closure

This phase borrows PanSou's result-processing ideas while keeping `tg-provider` focused on logged-in Telegram data. It must not add `https://t.me/s/...` scraping as a primary search path.

## Task PGC-001: Document PanSou access boundary

Record how PanSou searches public Telegram Web pages and why that does not cover private or joined-only Telegram channels/groups.

Acceptance Criteria:

- Documents PanSou's `https://t.me/s/{channel}?q={keyword}` approach.
- States that this project uses logged-in gotd sync instead.
- Lists borrowed and rejected ideas.

## Task PGC-002: Persist per-link note

Add an optional `note` field to extracted links.

Acceptance Criteria:

- `telegram_links.note` exists.
- Existing links remain valid with an empty note.
- Search and link APIs include `note` when present.

## Task PGC-003: Infer note from message text

Infer a link note from nearby resource titles.

Acceptance Criteria:

- Title-before-link messages assign the title to the link note.
- `链接：` and provider-label lines are skipped when looking upward for a title.
- Provider labels alone do not become notes.

## Task PGC-004: Add merged links repository and service

Add read-side merged link results grouped by provider type.

Acceptance Criteria:

- Same URL is deduplicated.
- Latest message context wins.
- Filters support type, account, channel, keyword, and date range.

## Task PGC-005: Add merged links API

Implement `GET /api/links/merged`.

Acceptance Criteria:

- Returns `total` and `merged_by_type`.
- Supports `q`, `type`, account/channel/date, limit, and offset filters.
- Invalid filters use the standard API error envelope.

---

# Phase 6: AList-TVBox Integration

## Task 083: Finalize API response models

Stabilize response models for AList-TVBox.

Models:

- `SearchResult`
- `StatusResponse`
- `LinkResult`
- `ChannelResult`

Acceptance Criteria:

- JSON fields are stable and documented.
- Null and empty values are consistent.

## Task 084: Add OpenAPI document

Create OpenAPI spec for core APIs.

Acceptance Criteria:

- Documents login, accounts, channels, search, latest messages, links, and status.
- Can be used by Spring Boot client generation or manual SDK work.

## Task 085: Add Spring Boot SDK contract examples

Add example request/response docs for AList-TVBox integration.

Acceptance Criteria:

- Search example includes keyword, channel, date, and links.
- Status example includes counts.

## Task 086: Add tg-provider Dockerfile

Create Dockerfile for tg-provider binary.

Acceptance Criteria:

- Binary runs in container.
- Data directory is mounted at `/data/tg-provider`.

## Task 087: Add supervisord config

Add supervisord config for running tg-provider inside AList-TVBox container.

Acceptance Criteria:

- Autostart enabled.
- Autorestart enabled.
- stdout/stderr logs go to `/data/tg-provider/logs`.

## Task 088: Ensure localhost-only exposure

Harden server binding and Docker integration.

Acceptance Criteria:

- Default service listens on localhost only.
- Docker does not publish port 9900 publicly by default.

## Task 089: Add Spring Boot provider adapter design

Document how AList-TVBox calls tg-provider.

Acceptance Criteria:

- Spring Boot uses HTTP only.
- No direct SQLite access.
- Timeout and error mapping are defined.

## Task 090: Add AList-TVBox search aggregation contract

Define how Telegram search results merge with existing AList-TVBox search.

Acceptance Criteria:

- Result source is clearly marked as Telegram.
- Link metadata is preserved.

## Task 091: Add end-to-end smoke test guide

Create manual smoke test steps.

Acceptance Criteria:

- Login account.
- Sync channel.
- Search keyword.
- Verify links.
- Verify status.
- Restart service and verify session recovery.

## Task 092: Add production deployment checklist

Create deployment checklist.

Acceptance Criteria:

- Covers config, data directory, logs, backup, startup, health check, and security.

---

# Hardening and Quality Tasks

## Task 093: Add API error response standard

Define consistent error response format.

Acceptance Criteria:

- All APIs return consistent error JSON.
- Validation errors use HTTP 400.
- Internal errors use HTTP 500 without leaking secrets.

## Task 094: Add request validation

Validate request bodies and query parameters.

Acceptance Criteria:

- Invalid phone/code/password requests fail clearly.
- Invalid pagination parameters fail clearly.

## Task 095: Add secret redaction

Ensure sensitive values are never logged.

Sensitive values:

- api_hash
- login code
- password
- session data

Acceptance Criteria:

- Logs redact secrets.
- Tests cover redaction helper.

## Task 096: Add backup command or job

Add basic SQLite backup support.

Acceptance Criteria:

- Backup file is written under `/data/tg-provider/backup`.
- Backup does not corrupt running database.

## Task 097: Add repository transaction helper

Centralize transaction handling.

Acceptance Criteria:

- Services can run multiple repository calls in one transaction.
- Rollback behavior is tested.

## Task 098: Add service startup lifecycle orchestration

Implement startup order.

Order:

1. Load config
2. Open SQLite
3. Run migrations
4. Load accounts
5. Restore sessions
6. Start update engine
7. Start scheduler
8. Start API server

Acceptance Criteria:

- Startup follows defined order.
- Failure in any step stops startup cleanly.

## Task 099: Add service shutdown lifecycle orchestration

Implement shutdown order.

Order:

1. Stop scheduler
2. Flush queues
3. Close Telegram clients
4. Close database

Acceptance Criteria:

- Shutdown follows defined order.
- Shutdown has timeout protection.

## Task 100: Add README quickstart

Create developer quickstart documentation.

Acceptance Criteria:

- Includes config example.
- Includes build/run commands.
- Includes first login and search flow.
- Includes known limitations.

---

# Suggested PR Order

1. Tasks 001-006: project skeleton
2. Tasks 007-018: database, migration, repositories, basic API
3. Tasks 019-025: Telegram login and sessions
4. Tasks 026-037: channel sync, history sync, basic search
5. Tasks 038-045: Updates Engine
6. Tasks 046-053: multi-account lifecycle
7. Tasks 054-070: link extraction
8. Tasks 071-082: performance and reliability
9. Tasks 083-092: AList-TVBox integration
10. Tasks 093-100: production hardening

# Non-goals for Current Scope

Do not implement these in the current 100 tasks:

- File download
- Online playback
- STRM generation
- Web management UI
- Standalone Vue frontend
- Media transcoding
- TMDB scraping
- Poster generation
