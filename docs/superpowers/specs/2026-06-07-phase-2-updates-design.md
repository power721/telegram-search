# Phase 2 Updates Design

## Goal

Implement `TASKS.md` Phase 2 (`038-045`): realtime update service skeleton, sequential update processing, gotd update listener integration, new/edit/delete message handling, queue flush on shutdown, and tests that verify repository state without real Telegram network access.

## Scope

Included:

- `UpdateService` lifecycle that starts and stops per account.
- Internal update queue with sequential processing and shutdown flush.
- Message processor for new, edited, and deleted messages.
- Link extraction and replacement for new and edited messages.
- gotd listener adapter that converts Telegram update events into internal update events.
- Recovery loop that marks account status `RECONNECTING` during listener failures and retries without crashing.
- Tests for processor and queue behavior using SQLite and fake listeners.

Excluded:

- Phase 3 multi-account lifecycle manager and health scheduler.
- Full persistent retry queue.
- FloodWait global strategy beyond non-crashing listener error handling.
- Backfilling missing channel metadata from updates not already known locally.

## Architecture

Phase 2 adds `internal/update` as the coordination layer. `Service` owns per-account listeners, a queue, and a `Processor`. `Processor` persists events through existing repositories and transaction helpers. `Listener` is an interface; gotd is one implementation, tests use fakes.

The API surface remains unchanged. Startup wires the update service after repositories and before the API server. Shutdown stops the update service before closing SQLite.

## Data Flow

gotd Updates Engine receives Telegram updates, converts each supported update to an internal `Event`, and pushes it to the queue. The queue has one worker so message writes are serialized. New and edited messages are upserted into `telegram_messages`, their links are recalculated, and FTS triggers keep search current. Delete events mark messages as deleted, which removes them from FTS via the existing trigger.

## Error Handling

Processor errors are logged and do not crash the process. Listener errors move the account to `RECONNECTING`, wait briefly, then retry until the service is stopped. Unsupported update shapes are ignored. If an update references a channel unknown to SQLite, it is logged and skipped.

## Testing

Tests cover new/edit/delete processor behavior, FTS search visibility, link recalculation, sequential queue processing, stop-and-flush behavior, and listener recovery with a fake listener. gotd integration is compile-covered and manually validated with real credentials.
