# Production Deployment Checklist

## Configuration

- `server.host` is bound to a private interface unless a reverse proxy adds authentication and TLS.
- `server.port` does not conflict with other local services.
- `storage.path` points to persistent storage mounted at `/data/tg-search`.
- `storage.max_db_size` is set for the host disk size.
- `storage.max_media_cache` is set for the host disk size.
- Telegram API credentials are configured in `config.yaml`.

## Storage

- `/data/tg-search` exists and is writable.
- `/data/tg-search/sessions` exists and is not world-readable.
- `/data/tg-search/logs` exists and has enough disk space.
- `/data/tg-search/backup` is included in operational backup retention.
- `/data/tg-search/index` and `/data/tg-search/thumbnails` exist.

## Runtime

- `tg-search` starts with `go run ./cmd/tg-search -config config.yaml` or the packaged binary.
- `GET /api/status` returns `{"service":"ok"}`.
- `GET /api/storage/usage` reports DB, index, media cache, total bytes, and quota flags.
- `GET /api/tasks` returns a JSON task list.
- `GET /api/events` responds with `Content-Type: text/event-stream`.
- Restarting the process restores unfinished tasks without changing `succeeded` or `canceled` tasks.
- Future `flood_wait` tasks keep their `next_run_at`; expired unfinished tasks return to `queued`.
- Logs are reviewed for startup errors.

## Telegram Runtime

- At least one channel has `listen_enabled=true` before expecting realtime listener startup.
- Disconnect tests mark affected accounts `RECONNECTING`.
- FloodWait tests mark affected accounts `FLOOD_WAIT` and create task `next_run_at` values.
- Successful reconnect returns affected accounts to `ONLINE`.
- Realtime update gaps enqueue `gap_recovery` tasks.

## Setup

- `GET /api/setup/status` returns setup state.
- `POST /api/setup/admin` creates the first admin.
- `POST /api/setup/api-key` is used only when a script/client key is needed.
- `POST /api/setup/complete` is called after first-run setup.

## Security

- Protect `/data/tg-search/config.yaml` and `/data/tg-search/sessions` permissions.
- Do not expose the service directly to the public internet.
- Rotate optional API keys when scripts or clients are retired.
- Confirm `password_hash`, API key hashes, Telegram login codes, and Telegram session contents do not appear in API responses.
