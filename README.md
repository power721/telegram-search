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
GET    /api/channels
GET    /api/channels/{id}
POST   /api/channels/{id}/sync
GET    /api/search?q=keyword
GET    /api/messages/latest
GET    /api/links
GET    /api/status
```

Phase 1 limitations:

- Realtime Telegram Updates Engine is not implemented yet.
- Provider-specific cloud drive parsing is not implemented yet; Phase 1 stores generic URLs, magnet links, and ED2K links.
- Automated tests use fake Telegram clients. Real Telegram login and history sync require valid API credentials and network access.
- The service binds to `127.0.0.1:6000` by default and should not be exposed publicly.
