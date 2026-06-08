# Smoke Test Guide

This guide verifies the `tg-search` backend foundation and Phase 1B web admin shell.

## Prerequisites

- A local `config.yaml` or `/data/tg-search/config.yaml`.
- Valid Telegram API credentials in config.
- Writable storage path.

## Start

```bash
go run ./cmd/tg-search -config config.yaml
```

In another shell, start the frontend:

```bash
npm install --prefix web
npm run web:dev
```

Development URLs:

```text
Backend:  http://127.0.0.1:6000
Frontend: http://127.0.0.1:5173
```

The Vite development server proxies `/api` to `http://127.0.0.1:6000`.

## Checks

```bash
curl -s http://127.0.0.1:6000/api/status
curl -s http://127.0.0.1:6000/api/setup/status
curl -s http://127.0.0.1:6000/api/storage/usage
```

Open `http://127.0.0.1:5173` and verify the first-run setup or login route appears. After login, the Home dashboard should render Storage Usage and Top Resource Types.

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
