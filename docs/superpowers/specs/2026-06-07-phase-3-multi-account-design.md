# Phase 3 Multi-Account Design

## Goal

Implement the Phase 3 multi-account management MVP from `docs/TASKS.md` tasks 046-053. The provider should manage multiple Telegram accounts at runtime, keep account state explicit, start realtime listeners for every online account, support account-aware search/status responses, and prove account isolation with tests.

## Scope

This phase includes:

- Account lifecycle manager with `Start`, `Stop`, `Restart`, and `List`.
- Explicit account state transition validation for:
  - `NEW`
  - `LOGIN_REQUIRED`
  - `SYNCING`
  - `ONLINE`
  - `RECONNECTING`
  - `FLOOD_WAIT`
  - `DISCONNECTED`
- Multiple online account startup using one independent session path per account.
- Health check loop that periodically restarts accounts in reconnectable states.
- Account-aware channel sync and search test coverage.
- `/api/status` account state summary plus existing aggregate counts.

This phase does not include:

- Cloud drive parser expansion. That belongs to Phase 4.
- A persistent gotd client pool shared across history, channel sync, and updates.
- Public network exposure, external health endpoints, or cross-process coordination.

## Current Code Context

Phase 1 already provides SQLite migrations, repositories, login, channel/history sync, FTS search, latest messages, links, and status counts.

Phase 2 already provides `internal/update.Service`, `internal/update.Processor`, and `internal/update.GotdListener`. `cmd/tg-provider/main.go` currently starts the update service directly for accounts with `ONLINE` status.

Some Phase 3 behavior already exists:

- `GET /api/search`, `GET /api/messages/latest`, `GET /api/links`, and `GET /api/channels` accept `account_id`.
- Search results include account phone, username, and first name.
- Session paths are already based on account id through `session.Manager`.

The missing pieces are centralized lifecycle management, explicit state validation, health checks, richer status summaries, and stronger multi-account isolation tests.

## Architecture

Add `internal/account` as the runtime owner for account lifecycle. It will depend on repositories, the update service, optional channel sync, and a logger. Existing packages keep their responsibilities:

- `internal/update` remains responsible for listener queueing and event processing.
- `internal/channel` remains responsible for listing/saving Telegram channels for one account.
- `internal/history` remains responsible for history sync for one channel.
- `internal/search` and repositories remain responsible for query filtering.

`cmd/tg-provider/main.go` will construct `account.Manager`, start it after repositories/services are ready, and stop it during graceful shutdown before closing the database.

## Account Manager

`account.Manager` will expose:

```go
type Manager struct { ... }

func NewManager(opts ManagerOptions) *Manager
func (m *Manager) Start(ctx context.Context) error
func (m *Manager) Stop(ctx context.Context) error
func (m *Manager) Restart(ctx context.Context, accountID int64) error
func (m *Manager) List(ctx context.Context) ([]model.Account, error)
```

Runtime behavior:

- `Start` loads all accounts from SQLite.
- Accounts in `ONLINE` or `RECONNECTING` start an update listener.
- Accounts in `NEW`, `LOGIN_REQUIRED`, `SYNCING`, `FLOOD_WAIT`, or `DISCONNECTED` are not started automatically.
- `Stop` stops the health loop and update service.
- `Restart` marks the account `RECONNECTING`, then starts its listener if the service is active.
- `List` returns repository data without runtime-only fields.

The manager tracks account ids that it has already started to avoid duplicate listener goroutines during repeated `Start` or health check passes.

## State Machine

Add a small state machine in `internal/account/state.go`:

```go
func CanTransition(from string, to string) bool
func KnownStatus(status string) bool
```

Allowed transitions:

- `NEW -> LOGIN_REQUIRED`
- `LOGIN_REQUIRED -> ONLINE`
- `LOGIN_REQUIRED -> DISCONNECTED`
- `ONLINE -> SYNCING`
- `ONLINE -> RECONNECTING`
- `ONLINE -> FLOOD_WAIT`
- `ONLINE -> DISCONNECTED`
- `SYNCING -> ONLINE`
- `SYNCING -> FLOOD_WAIT`
- `SYNCING -> DISCONNECTED`
- `RECONNECTING -> ONLINE`
- `RECONNECTING -> FLOOD_WAIT`
- `RECONNECTING -> DISCONNECTED`
- `FLOOD_WAIT -> RECONNECTING`
- `FLOOD_WAIT -> DISCONNECTED`
- `DISCONNECTED -> RECONNECTING`
- same-status transitions are allowed as idempotent updates.

Invalid transitions are logged and rejected by the manager. Existing repository `UpdateStatus` stays simple because lower-level storage should not own lifecycle policy.

## Health Check

The manager will run a periodic health loop. Defaults:

- Interval: 1 minute in production.
- Tests can configure a shorter interval.

On each tick:

- Load accounts from SQLite.
- For `RECONNECTING`, call `StartAccount` if not already active.
- For `DISCONNECTED`, transition to `RECONNECTING` and call `StartAccount`.
- Leave `FLOOD_WAIT` untouched in this MVP because it needs flood wait expiry data that the current schema does not store.
- Log errors without crashing the process.

This provides automatic recovery for temporary disconnects while keeping FloodWait handling conservative.

## API Changes

`GET /api/status` will keep the existing top-level fields:

```json
{
  "service": "ok",
  "accounts": 2,
  "channels": 10,
  "messages": 1000,
  "links": 20
}
```

It will add:

```json
{
  "account_states": {
    "ONLINE": 1,
    "RECONNECTING": 1
  }
}
```

No existing response field is removed.

Search/channel APIs keep existing query parameters. Tests will explicitly verify:

- `GET /api/search?q=x&account_id=1` returns only account 1 data.
- Results include account display fields.
- Channel list filtering does not mix account channels.

## Startup and Shutdown Flow

Startup:

1. Load config and open SQLite.
2. Run migrations.
3. Construct repositories, sessions, Telegram client, update processor, update service, channel/history/search services.
4. Construct account manager with update service and repositories.
5. Start account manager.
6. Start HTTP server.

Shutdown:

1. Stop HTTP server.
2. Stop account manager.
3. Account manager stops update service.
4. Close SQLite through existing defer.

## Error Handling

- Listener failures are still handled by `update.Service`, which marks accounts `RECONNECTING` and retries.
- Manager health check errors are logged and do not stop the process.
- Invalid state transitions return an error in manager APIs and are logged.
- Duplicate account starts are ignored as idempotent success.
- Stop is idempotent.

## Testing

Add focused tests:

- State machine allows valid transitions and rejects invalid transitions.
- Manager start loads multiple accounts and starts only online/reconnecting accounts.
- Manager restart updates status and starts the requested account.
- Manager health check transitions `DISCONNECTED` to `RECONNECTING` and starts it.
- Status repository returns account state counts.
- API status includes `account_states`.
- Search, latest, links, and channel filtering keep account data isolated.

All tests avoid real Telegram network access by using fake update listeners and existing fake/nop clients.

## Acceptance Mapping

- Task 046: `account.Manager` implements lifecycle methods and shutdown ownership.
- Task 047: `internal/account/state.go` provides explicit state validation.
- Task 048: Manager starts one update listener per online account and uses existing per-account session paths.
- Task 049: Health check loop restarts reconnectable accounts.
- Task 050: Existing account-aware channel sync is retained and covered by tests.
- Task 051: Existing account-aware search is retained and covered by API/repository tests.
- Task 052: `/api/status` includes account state summary and existing totals.
- Task 053: Tests cover multi-account isolation and lifecycle behavior.
