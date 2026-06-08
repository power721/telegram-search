# tg-search

Self-hosted personal Telegram search foundation.

`tg-search` stores data locally under `/data/tg-search`, exposes a local REST API, and will grow into a Vue admin console in later phases. Phase 1A provides the backend foundation: config, storage quota settings, setup/admin auth APIs, API key creation, and storage usage reporting.

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

## Foundation APIs

```text
GET    /api/setup/status
POST   /api/setup/admin
POST   /api/setup/api-key
POST   /api/setup/complete
POST   /api/auth/login
POST   /api/auth/logout
GET    /api/auth/me
GET    /api/storage/usage
GET    /api/status
```

Telegram account, channel, search, maintenance, and backup APIs from the existing backend remain available while the product is redesigned in later phases.

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
```

Operational docs:

- Detailed API documentation: `docs/api.md`
- API response contract: `docs/api-response-contract.md`
- Smoke test guide: `docs/smoke-test-guide.md`
- Production deployment checklist: `docs/production-deployment-checklist.md`
