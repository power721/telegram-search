# tg-search

Self-hosted personal Telegram search foundation.

`tg-search` stores data locally under `/data/tg-search`, exposes a local REST API, and includes a Vue admin shell for first-run setup, login, storage usage, Telegram onboarding, channel control, task observability, Global Search, and the Telegram Resource Library.

## Quickstart

Create `config.yaml` locally for development, or `/data/tg-search/config.yaml` in production:

```yaml
telegram:
  api_id: 123456
  api_hash: your_api_hash
server:
  host: 127.0.0.1
  port: 6000
sync:
  workers: 5
  history_batch_size: 100
storage:
  path: /data/tg-search
  max_db_size: 10GB
  max_media_cache: 20GB
```

Build and run:

```bash
go build ./...
go run ./cmd/tg-search -config config.yaml
```

Run with Docker Compose:

```bash
mkdir -p data
cp config.yaml data/config.yaml
docker compose up -d
docker compose logs -f tg-search
```

The container stores database, sessions, logs, backups, index files, and thumbnails under `/data/tg-search`, mounted as `./data` in the example Compose file.

Start the web admin shell:

```bash
npm install --prefix web
npm run web:dev
```

Development URLs:

```text
Backend:  http://127.0.0.1:6000
Frontend: http://127.0.0.1:5173
```

The Vite development server proxies `/api` requests to the backend at `http://127.0.0.1:6000`.

## First-Run Setup

```text
Admin Account -> API Key -> Telegram API -> Telegram Login -> Home
```

Telegram Login starts a metadata-only channel sync after the account is online. It collects account and channel metadata such as title, username, member count, description, avatar state, sync state, and listen state. It does not fetch message history during onboarding.

Channel Control adds Sync Profiles (`Quick`, `Normal`, `Deep`, `Full`), per-channel history/listen/remote-search toggles, Telegram Web Access Detection for `https://t.me/s/{username}`, listen rule filters, and display-only remote search task records. Web Access Detection is not a search-engine indexing signal.

## Runtime Reliability

Long-running work is tracked in persistent `sync_tasks` rows. The admin Tasks page shows task type, status, progress, retry count, FloodWait scheduling, error messages, and retry/cancel/pause/resume actions.

Task states:

```text
queued -> running -> succeeded
queued -> running -> failed
failed -> queued
running -> canceling -> canceled
running -> paused -> running
running -> flood_wait -> queued
running -> reconnecting -> running
```

On startup, unfinished retryable tasks are restored from SQLite. Future `flood_wait` tasks keep their `next_run_at`; past or unscheduled unfinished tasks return to `queued`; `succeeded` and `canceled` tasks stay unchanged. `/api/events` streams `task.updated`, `account.updated`, `listener.updated`, and `activity.created` events with Server-Sent Events.

Realtime listeners start only when an account has at least one channel with `listen_enabled=true`. Disconnects mark the account `RECONNECTING`; FloodWait marks `FLOOD_WAIT`; successful reconnect returns the account to `ONLINE`. Detected realtime message gaps enqueue `gap_recovery` tasks.

## Local Index

Message metadata and content are stored separately:

```text
telegram_messages          -> account/channel/message ids, sender, dates, type, delete state
telegram_message_contents  -> text and raw_json
telegram_sync_cursors      -> per-account/channel sync state
```

FTS5 indexes persisted local message content only. Remote Telegram search results are display-only, marked `source="remote"`, and are not written into local message, content, link, file, or FTS tables.

## Foundation APIs

```text
GET    /api/setup/status
POST   /api/setup/admin
POST   /api/setup/api-key
POST   /api/setup/telegram-api
POST   /api/setup/complete
POST   /api/auth/login
POST   /api/auth/logout
GET    /api/auth/me
GET    /api/settings/telegram-api
PUT    /api/settings/telegram-api
POST   /api/telegram/login/send-code
POST   /api/telegram/login/sign-in
POST   /api/telegram/login/password
GET    /api/accounts
DELETE /api/accounts/:id
POST   /api/accounts/:id/channels/sync-metadata
GET    /api/channels
PATCH  /api/channels/:id/control
POST   /api/channels/:id/analyze
POST   /api/channels/web-access/check
GET    /api/watch-rules
POST   /api/watch-rules
PUT    /api/watch-rules/:id
DELETE /api/watch-rules/:id
POST   /api/search/remote
GET    /api/search/remote/:task_id
GET    /api/search/global
GET    /api/search/messages
GET    /api/search/links
GET    /api/search/files
GET    /api/search/channels
GET    /api/resources
GET    /api/resources/grouped
GET    /api/resources/:id
GET    /api/storage/usage
GET    /api/status
GET    /api/tasks
GET    /api/tasks/:id
POST   /api/tasks/:id/retry
POST   /api/tasks/:id/cancel
POST   /api/tasks/:id/pause
POST   /api/tasks/:id/resume
GET    /api/events
```

Global Search returns grouped Messages, Links, Files, and Channels. Resources returns the Telegram Resource Library with `cloud_drive`, `magnet`, `ed2k`, `http`, and `files` groups.

## Storage Usage Response

```json
{
  "db_bytes": 3200000000,
  "index_bytes": 1100000000,
  "media_cache_bytes": 0,
  "total_bytes": 4300000000,
  "max_db_bytes": 10000000000,
  "max_media_bytes": 20000000000,
  "db_over_quota": false,
  "media_over_quota": false
}
```

## Development

```bash
GOCACHE=/tmp/go-build-cache go test ./...
npm run web:typecheck
npm run web:test
npm run web:build
```

Operational docs:

- Detailed API documentation: `docs/api.md`
- API response contract: `docs/api-response-contract.md`
- Smoke test guide: `docs/smoke-test-guide.md`
- Production deployment checklist: `docs/production-deployment-checklist.md`
