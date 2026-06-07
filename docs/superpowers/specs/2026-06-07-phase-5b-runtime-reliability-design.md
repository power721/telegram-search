# Phase 5b Runtime Reliability Design

## Goal

Add the runtime reliability slice left out of Phase 5a: configurable history sync workers, retry/backoff behavior, FloodWait handling, and a cleanup scheduler skeleton.

This phase covers `docs/TASKS.md` tasks 079-082. It keeps the implementation local and in-memory so the service becomes more resilient without adding a persistent job system or changing the public search contract.

## Current State

Phase 5a already improved the read path with stricter filters, cursor pagination, batch link loading, SQLite maintenance, and a benchmark seed path.

The remaining runtime gaps are:

- `sync.workers` exists in config but history sync only exposes `SyncChannel`.
- Multiple callers can ask to sync the same channel at the same time.
- History sync returns the first Telegram/network error to the caller without retry classification.
- The update listener retries with a fixed interval and does not classify FloodWait.
- There is no scheduler package for cleanup jobs.

## Scope

### History Worker Pool

`internal/history.Service` will support syncing multiple channel IDs with a configurable worker count:

```go
type SyncManyResult struct {
	Queued   int                    `json:"queued"`
	Skipped  int                    `json:"skipped"`
	Results  map[int64]SyncResult   `json:"results"`
	Failures map[int64]string       `json:"failures"`
}
```

`SyncMany(ctx, channelIDs)` will:

- Deduplicate repeated channel IDs in the same request.
- Use `sync.workers` as the maximum number of concurrent workers.
- Preserve `SyncChannel(ctx, channelID)` for the existing single-channel API.
- Avoid syncing the same channel concurrently across overlapping calls.

Overlapping calls use an in-process channel lock map. If a channel is already running, the new request records it as skipped instead of blocking indefinitely.

### Retry and FloodWait

A small retry package will classify errors into:

- `temporary`: retry with exponential backoff.
- `flood_wait`: wait for the reported duration, capped by the configured max backoff.
- `permanent`: stop retrying and return/log the reason.

The package will expose a testable policy:

```go
type Policy struct {
	BaseDelay time.Duration
	MaxDelay  time.Duration
	MaxTries  int
	Sleep     func(context.Context, time.Duration) error
}
```

Defaults:

- Base delay: 1 second.
- Max delay: 30 minutes.
- Max tries: 3 for history sync.

FloodWait detection will be based on a typed local error and common Telegram error text patterns such as `FLOOD_WAIT_60`. The gotd client can continue returning raw errors; the retry classifier will wrap detection at the boundary where errors are handled.

History sync will retry each channel sync operation through this policy. If a FloodWait is detected and the account repository is available, the account state is moved to `FLOOD_WAIT` before waiting.

The update listener will reuse the same classifier for its restart loop. Temporary failures keep the existing `RECONNECTING` state. FloodWait failures move the account to `FLOOD_WAIT`, wait according to the classifier, then retry unless the service is stopping.

### Cleanup Scheduler Skeleton

Add `internal/scheduler` with a simple lifecycle-managed ticker:

```go
type Job interface {
	Name() string
	Run(context.Context) error
}
```

The scheduler will:

- Start one goroutine per registered job.
- Run jobs on a configured interval.
- Log successful and failed job executions.
- Stop cleanly when the service shuts down.

Phase 5b will include a cleanup job skeleton that only logs activity. It will not delete messages, links, accounts, sessions, logs, or database rows.

### API and Wiring

Existing API behavior remains compatible:

- `POST /api/channels/:id/sync` keeps returning the single-channel `SyncResult`.

Add a batch sync endpoint only for the new worker pool:

```text
POST /api/channels/sync
```

Request:

```json
{"channel_ids":[1,2,3]}
```

Response:

```json
{
  "queued": 3,
  "skipped": 0,
  "results": {
    "1": {"messages": 10, "links": 2}
  },
  "failures": {}
}
```

Invalid or empty IDs return `400`. Sync failures for individual channels are returned in the `failures` map while the endpoint itself returns `202` when the request was valid and processed.

`cmd/tg-provider/main.go` will wire:

- `cfg.Sync.Workers` into history service options.
- Retry defaults into history and update services.
- The cleanup scheduler into startup/shutdown.

## Non-Goals

This phase will not implement:

- Persistent `sync_jobs` or `retry_jobs` tables.
- Restart recovery for in-flight sync jobs.
- Automatic periodic history sync for every channel.
- Destructive cleanup or retention policies.
- Public authentication/authorization changes.
- New search result contracts.

## Testing

Tests will cover:

- `SyncMany` deduplicates channel IDs and respects worker limits.
- The same channel is not synced concurrently across overlapping calls.
- Temporary history sync failures are retried and eventually succeed.
- Permanent history sync failures are not retried beyond policy rules.
- FloodWait parsing detects typed and text-form errors and caps wait duration.
- Update listener marks FloodWait accounts and retries without crashing.
- Batch sync API validates IDs and returns per-channel failures.
- Scheduler starts jobs, logs failures, and stops cleanly.

## Success Criteria

- Full test suite passes.
- `sync.workers` controls history batch sync concurrency.
- Overlapping sync requests cannot process the same channel concurrently.
- Temporary errors and FloodWait do not crash history sync or update listener loops.
- Cleanup scheduler skeleton is wired and can stop cleanly.
- No persistent job schema is introduced in this phase.
