# tg-search

Self-hosted personal Telegram search foundation.

`tg-search` stores data locally under `/data/tg-search`, exposes a local REST API, and includes a Vue admin shell for first-run setup, login, storage usage, Telegram onboarding, and account navigation. Later phases add channel control, Global Search, and the Telegram Resource Library.

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
GET    /api/storage/usage
GET    /api/status
```

Search, maintenance, and backup APIs from the existing backend remain available while the product is redesigned in later phases.

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
