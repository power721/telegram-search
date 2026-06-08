# Smoke Test Guide

This guide verifies the packaged `tg-search` service, embedded admin console, setup flow, Telegram sync path, search/resource surfaces, task operations, backup, and restart behavior.

## Prerequisites

- A local `config.yaml` or `/data/tg-search/config.yaml`.
- Valid Telegram API credentials in config.
- Writable storage path.
- `curl` for script checks.
- `sqlite3` for local backup scripts.

## Start

Run from source:

```bash
go run ./cmd/tg-search -config config.yaml
```

Or run the packaged service:

```bash
mkdir -p data
cp config.yaml data/config.yaml
docker compose up -d
```

URLs:

```text
Service: http://127.0.0.1:6000
```

## Automated Checks

Run:

```bash
BASE_URL=http://127.0.0.1:6000 scripts/smoke.sh
```

The script checks:

- `GET /api/health`
- `GET /api/ready`
- `GET /api/setup/status`
- `GET /`

## Manual Checks

Start from empty data:

```bash
rm -rf data
mkdir -p data
cp config.yaml data/config.yaml
docker compose up -d
```

Confirm runtime health:

```bash
curl -s http://127.0.0.1:6000/api/health
curl -s http://127.0.0.1:6000/api/ready
curl -s http://127.0.0.1:6000/api/storage/usage
```

Open `http://127.0.0.1:6000` and verify the embedded admin console appears.

Create admin:

```bash
curl -s -X POST http://127.0.0.1:6000/api/setup/admin \
  -H 'content-type: application/json' \
  -d '{"username":"admin","password":"secret123"}'
```

Login:

```bash
curl -i -X POST http://127.0.0.1:6000/api/auth/login \
  -H 'content-type: application/json' \
  -d '{"username":"admin","password":"secret123"}'
```

Expected:

- Login response sets `tg_search_session` with `HttpOnly`.
- `/api/auth/me` returns the admin user when the cookie is sent.
- `/api/storage/usage` includes `db_bytes`, `index_bytes`, `media_cache_bytes`, and quota flags.
- Runtime directories exist under `/data/tg-search`.

Complete the setup wizard:

- Save Telegram API settings.
- Log in to Telegram with a test account.
- Confirm metadata sync creates accounts and channels.
- Confirm channel control can run `Quick` history sync.
- Confirm Telegram Web Access Detection can be triggered for channels with usernames.

Verify search and resources:

- Run Global Search and confirm Messages, Links, Files, and Channels groups render.
- Open Resources and confirm resource groups such as `cloud_drive`, `magnet`, `ed2k`, `http`, and `files`.
- Confirm a never-synced channel with remote search enabled can create a display-only remote search task.

Verify task operations:

- Open Tasks and confirm queued/running/succeeded rows render.
- Retry a failed task if one exists.
- Pause, resume, or cancel a running task when available.
- Confirm `/api/events` keeps a Server-Sent Events connection open.

Verify backup and restart:

```bash
DATA_DIR=./data scripts/backup.sh
docker compose restart tg-search
BASE_URL=http://127.0.0.1:6000 scripts/smoke.sh
```

For restore drills, stop the service before replacing the database:

```bash
docker compose stop tg-search
DATA_DIR=./data scripts/restore.sh ./data/backup/tg-search-YYYYMMDDTHHMMSSZ.db
docker compose up -d
```
