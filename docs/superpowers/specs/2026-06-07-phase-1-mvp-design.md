# Phase 1 MVP Design

## Goal

Build the tg-provider Phase 1 MVP described by `TASKS.md`: a local Go service that can store Telegram account/channel/message data in SQLite, synchronize historical messages through a Telegram client abstraction, and expose localhost HTTP APIs for login, channel sync, search, latest messages, links, and status.

## Scope

Included tasks: `001` through `037`.

Out of scope: realtime Updates Engine, full multi-account lifecycle orchestration, provider-specific cloud-drive parsing, performance benchmark tooling, Docker/supervisord integration, Spring Boot SDK work, and production hardening tasks after Phase 1.

## Architecture

The service is organized into narrow internal packages:

- `internal/config`: YAML loading, defaults, validation, runtime paths.
- `internal/db`: SQLite opening, PRAGMAs, migrations, transaction helper.
- `internal/model`: account, channel, message, link, and API-facing model types.
- `internal/repository`: SQLite repositories for accounts, channels, messages, links, and status counts.
- `internal/link`: Phase 1 generic URL extraction.
- `internal/telegram`: client interface plus a gotd-backed implementation for login, channel listing, and history fetching.
- `internal/session`: deterministic per-account session file paths.
- `internal/channel` and `internal/history`: channel metadata sync and history sync services.
- `internal/api`: Gin handlers and route registration.
- `cmd/tg-provider`: startup/shutdown orchestration.

The API and sync services depend on interfaces instead of gotd concrete types where practical. Tests use fake Telegram clients and real temporary SQLite databases.

## Data Flow

Startup loads config, creates runtime directories, opens SQLite, runs migrations, constructs repositories/services, and starts the Gin server on `127.0.0.1:9900` by default.

Login APIs create or update account records and delegate send-code/sign-in/password operations to the Telegram gateway. Session files are stored under the configured storage path's `sessions` directory.

Channel sync fetches dialogs for an account, upserts channels, and adds Saved Messages as a virtual channel. Manual channel history sync fetches batches using the configured history batch size, stores messages and extracted links transactionally, and advances the channel cursor after each successful batch.

Search queries use SQLite FTS5 against non-deleted messages, join account/channel/link metadata, and return bounded JSON results. Latest messages return date-descending rows. Status reads counts from SQLite.

## Error Handling

Config validation fails fast when `telegram.api_id` or `telegram.api_hash` is missing. Runtime directory, SQLite, migration, and startup errors return clear errors and stop startup. API validation errors return HTTP 400 JSON. Internal failures return HTTP 500 JSON without leaking secrets. Telegram network and FloodWait errors are returned to the caller and logged; the service process must not panic.

## Testing

Tests cover config defaults/validation, migration idempotency, repositories, FTS search, link extraction, API handlers, and history sync with a fake Telegram client. Real Telegram login and channel fetch are compile-covered but manually validated because they require credentials and network access.
