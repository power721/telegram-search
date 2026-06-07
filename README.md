# tg-provider

Local Telegram resource provider for AList-TVBox.

## Quickstart

Create `config.yaml` locally for development, or `/data/tg-provider/config.yaml` in the container:

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
  path: /data/tg-provider
```

Build and run:

```bash
go build ./...
go run ./cmd/tg-provider -config config.yaml
```

Core APIs:

```text
POST   /api/login/send-code
POST   /api/login/sign-in
POST   /api/login/password
GET    /api/accounts
DELETE /api/accounts/{id}
POST   /api/accounts/{id}/channels/sync
GET    /api/channels
GET    /api/channels/{id}
POST   /api/channels/sync
POST   /api/channels/web-access/check
POST   /api/channels/{id}/sync
GET    /api/watch-rules
POST   /api/watch-rules
GET    /api/watch-rules/{id}
PUT    /api/watch-rules/{id}
DELETE /api/watch-rules/{id}
GET    /api/search?q=keyword
GET    /api/messages/latest
GET    /api/links
GET    /api/status
POST   /api/maintenance/sqlite
POST   /api/maintenance/backup
```

## First login and search flow

1. Start the provider:

```bash
go run ./cmd/tg-provider -config config.yaml
```

2. Send a Telegram code:

```bash
curl -s -X POST http://127.0.0.1:6000/api/login/send-code \
  -H 'content-type: application/json' \
  -d '{"phone":"+123456789"}'
```

3. Sign in with the received code:

```bash
curl -s -X POST http://127.0.0.1:6000/api/login/sign-in \
  -H 'content-type: application/json' \
  -d '{"phone":"+123456789","code":"12345"}'
```

If 2FA is required, call `/api/login/password` with the same phone and the account password.

4. Sync the account channel list:

```bash
curl -s -X POST http://127.0.0.1:6000/api/accounts/1/channels/sync
```

5. Sync one channel:

```bash
curl -s -X POST http://127.0.0.1:6000/api/channels/1/sync
```

6. Search messages and links:

```bash
curl -s 'http://127.0.0.1:6000/api/search?q=keyword&limit=20'
curl -s 'http://127.0.0.1:6000/api/links?limit=20'
```

The manual sync APIs return a queued job response when the runtime retry queue is enabled.

## Operational docs

- Detailed API documentation: `docs/api.md`
- API response contract: `docs/api-response-contract.md`
- Smoke test guide: `docs/smoke-test-guide.md`
- Production deployment checklist: `docs/production-deployment-checklist.md`

## Known limitations

- Automated tests use fake Telegram clients. Real Telegram login and history sync require valid API credentials and network access.
- The service binds to `127.0.0.1:6000` by default and should not be exposed publicly.
- The retry queue is in-memory. Queued job status does not survive process restart.
- Million-message performance benchmarks are opt-in:

```bash
TG_PROVIDER_MILLION_BENCH=1 go test ./internal/repository -bench BenchmarkMessageRepositorySearchMillion -run '^$'
```
