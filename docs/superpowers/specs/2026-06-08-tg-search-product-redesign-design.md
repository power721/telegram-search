# tg-search Product Redesign Design

Date: 2026-06-08

## Summary

`tg-search` is a self-hosted personal Telegram search engine. It indexes only content the logged-in Telegram account can access: private channels, private groups, Saved Messages, joined public channels, and joined public groups.

The product is not a public Telegram search engine, not a PanSou replacement, not a file drive, and not an AList/TVBox/T4 provider. Phase 1 delivers a complete standalone product with a Vue admin console, setup flow, Telegram login, channel control, local indexing, Global Search, Telegram Resource Library, task observability, storage quota controls, and Docker-friendly deployment.

## Product Positioning

The core value is a private local index for a user's own Telegram content.

Default behavior is conservative:

- First login syncs only account-visible channel, group, and Saved Messages metadata.
- No channel history is synced by default.
- Only channels explicitly selected by the user can run history sync.
- Only channels explicitly selected by the user can run realtime listening.
- Search defaults to the local SQLite/FTS5 index.
- Unsynced channels may be searched remotely only through explicit user action.

Remote search is a narrow exception to the default local-index rule. It is allowed only for user-accessible unsynced channels, groups, or Saved Messages. Results are marked `remote`, displayed temporarily, and are not written to SQLite or FTS. Persisting those results requires running a normal channel history sync.

## Phase 1 Scope

Phase 1 must deliver a complete usable loop:

- First-run Setup Wizard.
- Admin account and optional API Key.
- Telegram API configuration.
- Telegram account login with phone, code, and 2FA password support.
- Metadata sync for channels, groups, and Saved Messages.
- Channel analysis and Telegram Web Access Detection.
- Explicit channel selection for history sync and realtime listening.
- SQLite storage and SQLite FTS5 search.
- History sync, realtime listener, idempotent writes, and link extraction.
- Global Search across messages, links, files, and channels.
- Telegram Resource Library as a primary product surface.
- Sync Profiles for safer history sync selection.
- Storage quota and storage usage reporting.
- Manual remote search for unsynced channels, display-only.
- Vue 3 admin console with home, search, channels, resources, accounts, tasks, and settings.
- Task status, progress, retry, cancel, FloodWait state, reconnect state, and recent activity.
- Dockerfile, Docker Compose, GitHub Release packaging, logs, backup, and production deployment docs.

Phase 1 must not implement:

- Public Telegram web scraping as a primary search path.
- PanSou replacement behavior.
- AList/TVBox/T4/provider compatibility.
- File drive behavior, download proxy, image proxy, video proxy, or file proxy.
- Redis, ElasticSearch, vector databases, or media recognition as runtime dependencies.

## Naming And Storage

The product should be unified under `tg-search`:

- Product name: `tg-search`.
- Binary name: `tg-search`.
- Go module and README wording should move away from `tg-provider`.
- Docker image and Compose service should use `tg-search`.
- Default data directory: `/data/tg-search`.

Default data layout:

```text
/data/tg-search
├── config.yaml
├── tg-search.db
├── sessions
├── logs
├── uploads
├── backup
├── index
└── thumbnails
```

Recommended storage quota defaults:

```yaml
storage:
  max_db_size: 10GB
  max_media_cache: 20GB
```

The backend should parse human-readable size strings and expose effective byte values through Settings and Status APIs. Phase 1 quota behavior should be conservative: report usage, show warnings, and block new Deep or Full sync tasks when configured limits would be exceeded. It should not attempt complex automatic cleanup in the first release.

## Information Architecture

The admin console has seven primary areas.

### Home

Home is an operational dashboard and search entry point. It shows a global search box, account/channel/message/resource statistics, sync state, listener state, storage usage, top resource types, recent activity, and important errors.

Storage Usage shows:

- DB size.
- Index size.
- Media cache size when enabled.
- Total storage used.
- Quota warning state.

Top Resource Types shows the most useful resource groups:

- Cloud drive links.
- Magnet.
- ED2K.
- HTTP links.
- Files.

### Search

Search is a Global Search experience, not only a message search page. It defaults to local indexed results and splits results into:

- Messages.
- Links.
- Files.
- Channels.

This should feel closer to GitHub, Slack, or Discord search: one query, multiple result scopes, with tabs or grouped sections. It supports keyword search, channel filter, account filter, time range, message type, link type, and file type. Results distinguish `local` and `remote`.

Remote search is only available through an explicit action for unsynced user-accessible channels. Remote results are temporary and do not enter the resource library or local index.

### Channels

Channels manages Telegram channels, groups, and Saved Messages. It displays title, username, type, member count, description, sync state, listener state, Telegram Web access state, last sync time, and progress.

Supported actions:

- Sync metadata.
- Analyze channel.
- Check Telegram Web access.
- Start history sync.
- Choose Sync Profile.
- Enable or disable realtime listening.
- Run remote search for unsynced channels.
- View task progress and failures.

### Resources

Resources is a core product surface, not a secondary utility. The most valuable daily workflow is often finding resources rather than reading raw messages.

The Telegram Resource Library aggregates links and file metadata extracted from indexed messages. It supports link type filtering, resource category filtering, deduplication, source message navigation, time sorting, relevance sorting, and grouped views.

Phase 1 resource categories should be derived from practical metadata rather than complex media recognition:

- Link type.
- File type and extension.
- Keyword or tag hints from message text.
- Source channel.
- Optional manual labels later.

Product-facing categories can include movies, cloud drive resources, e-books, courses, software, comics, generic files, and generic links, but Phase 1 should treat them as lightweight classification targets rather than a full recognition system.

Only persisted local-index content appears in Resources. Remote search results do not appear here unless the user later syncs the source channel.

### Accounts

Accounts covers Telegram account login, relogin, deletion, session status, and runtime state. Multi-account is supported in Phase 1, but the UI should remain simple and status-driven.

### Tasks

Tasks shows long-running and background work:

- Metadata sync.
- Channel analysis.
- Telegram Web Access Detection.
- History sync.
- Realtime listener recovery.
- Remote search.
- Backup.
- Gap recovery.
- Retry jobs.

Each task exposes status, progress, failure reason, retry count, next run time, retry action, cancel action, and relevant payload summary.

### Settings

Settings covers admin profile, API Keys, Telegram API, storage paths, storage quota, backup policy, logging, rate limits, default Sync Profile, and search defaults.

Phase 1 settings should not expose ElasticSearch, Redis, vector search, or proxy media configuration.

## Setup Wizard

First startup enters Setup Wizard and blocks normal admin use until complete.

### Step 1: Admin Account

Collect username and password. Store the password with bcrypt. Do not expose the admin console before this step succeeds.

### Step 2: API Key

Allow creating an API Key or skipping. API Keys are for future scripts, CLI, or external clients. They are not required for browser admin use.

### Step 3: Telegram API

Collect App ID and App Hash. Allow skipping if the build includes a default configuration, but make it clear the values can be changed later in Settings.

### Step 4: Telegram Login

Support phone number, code, and 2FA password. Sensitive values must never be logged or returned in API responses.

After login, start metadata sync for channels, groups, and Saved Messages. This step must not sync message history.

### Step 5: Channel Analysis

Show title, username, type, member count, description, basic statistics, and Telegram Web access status. Analysis must remain lightweight and must not force full history sync.

### Step 6: Listen Rules

Configure includes, excludes, message types, and link types. The default rule is to index all text and links for channels the user later selects.

### Step 7: Select Channels

For each channel, group, or Saved Messages entry, let the user choose:

- History sync enabled or disabled.
- Sync Profile.
- Realtime listening enabled or disabled.
- Remote search allowed for unsynced use.

Defaults:

- Metadata only.
- History sync disabled.
- Realtime listening disabled.
- Sync Profile `Normal` when history sync is enabled.

Sync Profiles:

- `Quick`: latest 100 messages.
- `Normal`: latest 1000 messages.
- `Deep`: latest 10000 messages.
- `Full`: all available history.

The UI should show profile names first and message counts as explanatory copy. `Full` requires an explicit confirmation that explains expected runtime, FloodWait risk, and storage impact.

### Step 8: Finish

Create tasks for selected history sync and listener startup, then enter Home. Unselected channels remain metadata-only and can later be synced or searched remotely.

## Backend Architecture

The backend is organized into six layers.

### API Layer

Gin serves REST APIs for the Vue admin console. SSE provides task progress, account state, listener state, and recent activity updates. Phase 1 can keep hand-written handlers, but API documentation should follow OpenAPI 3.1.

### Application Services

Services should follow clear boundaries:

- Setup service.
- Auth service.
- Account service.
- Channel service.
- History sync service.
- Update listener service.
- Remote search service.
- Search service.
- Resource service.
- Task service.
- Settings service.
- Backup service.

### Task Runtime

All long-running work runs as tasks: metadata sync, channel analysis, history sync, remote search, Telegram Web Access Detection, backup, restore, and gap recovery. Tasks persist status and progress so the UI can explain what the system is doing.

### Telegram Runtime

Use gotd/td through a per-account client pool. Centralize session storage, FloodWait handling, reconnect, rate limits, and gap recovery. Do not use `t.me/s` public HTML scraping as the main product search path.

### Repository And Index

SQLite is the Phase 1 database. SQLite FTS5 is the Phase 1 search index. Writes are idempotent, and FTS changes follow message insert, update, delete, and soft-delete behavior.

Message metadata and message content should be separated. `telegram_messages` stores identifiers, channel/account ownership, dates, type, deletion state, and media summary. `telegram_message_contents` stores large text and raw JSON. This keeps metadata queries, cursor updates, backups, and index maintenance lighter as message volume grows.

Sync cursors should be stored in `telegram_sync_cursors`, not on `telegram_channels`. This gives history sync, incremental sync, gap recovery, and multi-account runtime state a dedicated place to evolve without overloading channel metadata.

### Local Storage

The backend owns `/data/tg-search` and creates required directories at startup. Startup fails with clear errors if directories or the database cannot be initialized.

## Search Paths

### Local Indexed Search

Local search queries SQLite and FTS5. Results may join message, channel, account, link, media, and file metadata. Local search supports pagination, highlighting, time filtering, channel filtering, account filtering, message type filtering, and link type filtering.

### Remote Search

Remote search is a manual read-through Telegram query for unsynced channels. It is not a replacement for local search.

Rules:

- User must choose a specific user-accessible channel, group, or Saved Messages scope.
- The source must be unsynced. Synced channels use local indexed search.
- Results use `source: "remote"`.
- Results do not include local database IDs.
- Results are not inserted into `telegram_messages`, `telegram_links`, or FTS.
- Results can be kept in short-lived memory or temporary storage tied to a remote search task.
- A service restart may expire remote results.
- FloodWait, permission errors, and timeout errors must be visible in task state.

## Telegram Web Access Detection

Telegram Web Access Detection checks only whether a channel with a public username can be viewed through Telegram's Web preview page:

```text
https://t.me/s/{username}
```

It is not a discoverability score and must not be named `Discoverability`.

It does not mean:

- Google indexed the channel.
- Bing indexed the channel.
- PanSou collected the channel.
- The channel is publicly searchable.
- The complete channel history is available through Web preview.

### Detection Logic

After channel metadata sync, the system may asynchronously run Web Access Detection.

Rules:

- If the channel has a `username`, request `https://t.me/s/{username}`.
- Parse the response HTML for `tgme_widget_message_wrap`.
- If at least one message element exists, store `web_access=true`.
- If no message element exists, store `web_access=false`.
- If the channel has no `username`, store `web_access=false`.
- Store HTTP, timeout, DNS, parse, and other probe failures in `web_access_error`.

### Scope

Applicable:

- Channels with a `username`.
- Joined public channels.
- Channels that may be reachable through `t.me/s`.

Not applicable:

- Private channels.
- Private groups.
- Channels without `username`.
- Saved Messages.

The database stays simple:

- `web_access BOOLEAN`
- `web_access_checked_at DATETIME`
- `web_access_error TEXT`

The UI may display channels without a username, private groups, and Saved Messages as "Not applicable", but the database should not introduce a separate enum for this in Phase 1.

Search ranking may use this as a local-index value signal:

- `web_access=false` first.
- `web_access=true` later.
- unknown last.

Rationale: if a channel cannot be accessed through Telegram Web preview, its content depends more on the user's account permissions, so the local index is more valuable.

### Telegram Web Preview Probe

A helper such as `searchWeb(username, keyword)` can exist only as a Telegram Web Preview or Web Access probe. It is not the main search capability and must not bypass the product rule that normal search uses the local index, with explicit remote Telegram search reserved for unsynced user-accessible sources.

## Data Model

Phase 1 core tables:

### users

Admin users.

Fields:

- `id`
- `username`
- `password_hash`
- `role`
- `last_login_at`
- `created_at`
- `updated_at`

### api_keys

Optional API Keys.

Fields:

- `id`
- `name`
- `key_hash`
- `prefix`
- `enabled`
- `last_used_at`
- `created_at`
- `updated_at`

### telegram_accounts

Telegram account and session state.

Fields:

- `id`
- `phone`
- `telegram_user_id`
- `first_name`
- `last_name`
- `username`
- `status`
- `session_path`
- `last_online_at`
- `last_error`
- `created_at`
- `updated_at`

### telegram_channels

Channels, groups, and Saved Messages metadata.

Fields:

- `id`
- `account_id`
- `telegram_channel_id`
- `access_hash`
- `title`
- `username`
- `type`
- `member_count`
- `description`
- `avatar_state`
- `sync_state`
- `listen_state`
- `web_access`
- `web_access_checked_at`
- `web_access_error`
- `last_sync_at`
- `created_at`
- `updated_at`

### telegram_messages

Persisted indexed messages.

Fields:

- `id`
- `account_id`
- `channel_id`
- `telegram_message_id`
- `sender_id`
- `date`
- `edit_date`
- `deleted`
- `message_type`
- `media_summary`
- `created_at`
- `updated_at`

### telegram_message_contents

Large message body content separated from message metadata.

Fields:

- `message_id`
- `text`
- `raw_json`
- `created_at`
- `updated_at`

SQLite FTS5 should index `telegram_message_contents.text` and join back to `telegram_messages` for channel, account, date, deletion state, and message type filters.

### telegram_sync_cursors

Per-account and per-channel sync cursor state.

Fields:

- `id`
- `account_id`
- `channel_id`
- `cursor_type`
- `last_message_id`
- `pts`
- `qts`
- `date`
- `created_at`
- `updated_at`

`cursor_type` allows separate cursors for history sync, listener update state, and gap recovery. Unique constraints should prevent duplicate cursor rows for the same account, channel, and cursor type.

### telegram_links

Links extracted from persisted messages.

Fields:

- `id`
- `message_id`
- `type`
- `url`
- `password`
- `note`
- `source_snippet`
- `created_at`

### telegram_media And telegram_files

Phase 1 stores only metadata needed for filtering and future growth. It does not proxy or download content.

### sync_tasks

Persistent task records for background work.

Fields:

- `id`
- `type`
- `status`
- `progress`
- `total`
- `message`
- `error_code`
- `error_message`
- `retry_count`
- `next_run_at`
- `payload_json`
- `started_at`
- `finished_at`
- `created_at`
- `updated_at`

### remote_search_tasks

Remote search task metadata. Long-term local index tables must not store remote result content.

Fields:

- `id`
- `account_id`
- `channel_id`
- `query`
- `status`
- `progress`
- `total`
- `error_code`
- `error_message`
- `expires_at`
- `created_at`
- `updated_at`

### watch_rules

Realtime listener rules.

Fields:

- `id`
- `channel_id`
- `enabled`
- `includes_json`
- `excludes_json`
- `message_types_json`
- `link_types_json`
- `created_at`
- `updated_at`

### settings

Instance configuration stored in the database where runtime edits are needed.

Fields:

- `key`
- `value_json`
- `updated_at`

## API Contract

All list responses use:

```json
{
  "items": [],
  "total": 0
}
```

All errors use:

```json
{
  "error": {
    "code": "bad_request",
    "message": "message"
  }
}
```

Sensitive values such as `api_hash`, login codes, 2FA passwords, session content, and API Key secrets must never appear in responses or logs.

Suggested Phase 1 API groups:

### Setup

- `GET /api/setup/status`
- `POST /api/setup/admin`
- `POST /api/setup/api-key`
- `POST /api/setup/telegram-api`
- `POST /api/setup/complete`

### Auth

- `POST /api/auth/login`
- `POST /api/auth/logout`
- `GET /api/auth/me`

### Telegram Login

- `POST /api/telegram/login/send-code`
- `POST /api/telegram/login/sign-in`
- `POST /api/telegram/login/password`

### Accounts

- `GET /api/accounts`
- `POST /api/accounts`
- `DELETE /api/accounts/:id`
- `POST /api/accounts/:id/relogin`

### Channels

- `GET /api/channels`
- `GET /api/channels/:id`
- `POST /api/channels/sync-metadata`
- `POST /api/channels/:id/sync-history`
- `POST /api/channels/:id/listen`
- `DELETE /api/channels/:id/listen`
- `POST /api/channels/:id/analyze`
- `POST /api/channels/web-access/check`

### Search

- `GET /api/search/global`
- `GET /api/search/messages`
- `GET /api/search/links`
- `GET /api/search/files`
- `GET /api/search/channels`
- `POST /api/search/remote`
- `GET /api/search/remote/:task_id`

`GET /api/search/global` returns grouped sections for messages, links, files, and channels. The specific endpoints power tabs, pagination, and deep filtering for each scope.

Local results include `source: "local"` and local IDs. Remote results include `source: "remote"` and Telegram location fields, but no local message or link ID.

### Resources

- `GET /api/resources`
- `GET /api/resources/:id`
- `GET /api/resources/grouped`

### Tasks

- `GET /api/tasks`
- `GET /api/tasks/:id`
- `POST /api/tasks/:id/retry`
- `POST /api/tasks/:id/cancel`
- `GET /api/events`

### Settings

- `GET /api/settings`
- `PUT /api/settings`
- `GET /api/storage/usage`
- `POST /api/backup`
- `GET /api/logs`

## Sync, Listener, And Task State

### Sync Rules

- Metadata sync runs after login and does not sync history.
- Channel analysis remains lightweight and must not force full history sync.
- History sync requires explicit channel selection.
- Realtime listening requires explicit channel selection.
- Default Sync Profile is `Normal`.
- Sync Profiles map to internal limits: `Quick=100`, `Normal=1000`, `Deep=10000`, `Full=all available history`.
- `Full` sync requires explicit confirmation and should run only as a resumable task.
- Deep and Full sync requests must check storage quota before starting.
- History sync writes message metadata, writes message contents, extracts links, updates FTS, and updates `telegram_sync_cursors`.

### Task State Machine

```text
queued -> running -> succeeded
queued -> running -> failed
failed -> queued
running -> canceling -> canceled
running -> paused -> running
running -> flood_wait -> queued
running -> reconnecting -> running
```

Rules:

- FloodWait moves a task to `flood_wait`; the task resumes after `next_run_at`.
- Network disconnect moves account or task state to `reconnecting`; work resumes after reconnect.
- History sync uses cursors for resume.
- Message writes are idempotent by `(channel_id, telegram_message_id)`.
- Link writes are idempotent by `(message_id, type, url)`.
- Message deletes soft-delete local messages and remove them from FTS.
- Task cancelation stops future batches and does not roll back already written data.
- Service restart restores unfinished history sync, listener configuration, and retryable tasks.
- Remote search tasks can expire after restart or timeout; remote results do not need recovery.

### Gap Recovery

The listener tracks account and channel update state. When it detects a gap, it creates a gap recovery task. Gap recovery fetches only the missing range and uses the same idempotent write path as history sync.

## Frontend Direction

Use an Operations Console style:

- Stable left navigation.
- Dense, scan-friendly tables.
- Compact dashboard cards.
- Prominent search entry points.
- Clear task progress and error surfaces.
- Restrained visual style suitable for a NAS/local operations tool.

Avoid:

- Marketing-style landing pages.
- Hero sections.
- Sparse card-heavy media library layouts as the main UI.
- Decorative gradients, oversized typography, or one-note color palettes.

Page behavior:

- Home emphasizes status, quick search, storage usage, and top resource types.
- Search emphasizes Global Search result grouping, filters, highlights, and local/remote source labels.
- Channels emphasizes bulk management and explicit sync/listen actions.
- Resources emphasizes Telegram Resource Library workflows: link type, file type, lightweight categories, source message, deduplication, and filtering.
- Accounts emphasizes session and runtime health.
- Tasks emphasizes progress, failures, retry, cancel, FloodWait, reconnect, and recent activity.
- Settings emphasizes safe defaults, storage quota, Sync Profile defaults, and local deployment.

## Delivery Strategy

Implement Phase 1 in focused stages:

1. **Foundation**
   Rename product, binary, docs, data directory, Docker naming, config, setup status, admin auth, and base server.

2. **Admin Shell**
   Create Vue 3 + TypeScript + Vite + Naive UI frontend under `web/`. Add login, layout, home shell, and settings basics.

3. **Telegram Onboarding**
   Add Telegram API configuration, phone login, code login, 2FA, account state, and metadata sync.

4. **Channel Control**
   Add channel table, channel analysis, Telegram Web access status, Sync Profile selection, listener selection, storage quota checks, and remote search entry points.

5. **Index And Search**
   Add SQLite/FTS5 search surfaces, `telegram_message_contents`, `telegram_sync_cursors`, history sync UI, Global Search, Telegram Resource Library, and link extraction UI.

6. **Runtime Reliability**
   Add persistent tasks, SSE, FloodWait handling, reconnect state, gap recovery, pause/resume/retry/cancel, and restart recovery.

7. **Packaging**
   Add Dockerfile, Compose, release artifacts, production checklist, backup, logs, and smoke tests.

## Testing Strategy

Backend:

- Repository tests for migrations, constraints, message contents, sync cursors, search, links, settings, users, tasks, storage usage, and remote search task metadata.
- Service tests for setup, auth, account state, channel sync, Sync Profile mapping, storage quota checks, history sync, remote search, resources, and task state transitions.
- API handler tests for response contracts and sensitive value redaction.
- Fake Telegram client tests for login, metadata sync, history sync, listener updates, FloodWait, reconnect, and gap recovery.
- FTS5 tests for content insert, update, delete, soft-delete, filtering, highlighting, and pagination.

Frontend:

- Component tests for Setup Wizard, login, Sync Profile selection, channel selection, Global Search filters, Resource Library filters, storage usage, task rows, and settings forms.
- Store tests for auth state, setup state, task events, Global Search state, resource state, storage usage state, and channel selection.
- End-to-end smoke tests for first setup, login mock, metadata sync mock, channel sync mock, local Global Search, Resource Library filtering, remote search mock, storage quota warning, and backup.

Operational:

- Docker Compose starts with an empty data directory.
- Setup Wizard completes from a clean state.
- Service restarts preserve admin setup, Telegram sessions, sync tasks, listener settings, and local index.
- Storage usage reporting matches the SQLite database, index directory, and media cache directory.
- Logs redact sensitive values.
- Backup and restore procedures are documented and smoke-tested.

## Open Decisions

None for this design. The following decisions were confirmed during brainstorming:

- The product is a standalone `tg-search`, not an AList/TVBox provider.
- Phase 1 includes a complete Vue admin console.
- Project naming should be unified to `tg-search`.
- Unsynced channels may run explicit remote Telegram search.
- Remote search results are display-only and are not inserted into the local index.
- Channel Web checks are named Telegram Web Access Detection and use `web_access`, not Discoverability.
- History sync uses Sync Profiles instead of raw numeric choices.
- Storage quota and storage usage are Phase 1 requirements.
- Search is Global Search across messages, links, files, and channels.
- Resources is a primary Telegram Resource Library surface.
- Message metadata, message contents, and sync cursors use separate tables.
- The frontend direction is Operations Console.
