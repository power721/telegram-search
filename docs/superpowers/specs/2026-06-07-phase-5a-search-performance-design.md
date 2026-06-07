# Phase 5a Search Performance Design

## Goal

Improve the read/search path so it is predictable under larger local indexes, with clearer API validation and a foundation for later million-message benchmarks.

This phase covers the performance-focused core of Phase 5: search filters, query shape, pagination behavior, bulk link loading, and controlled SQLite maintenance. It does not cover history sync worker pools, retry queues, FloodWait handling, or cleanup scheduler work; those belong in a later Phase 5b runtime reliability slice.

## Current State

The service already has:

- SQLite with WAL-oriented pragmas.
- FTS5-backed message search.
- Basic indexes on messages, links, and channels.
- `account_id`, `channel_id`, `link_type`, `limit`, and `offset` for `/api/search`.
- `date_from` and `date_to` for `/api/links`.
- Repository and API tests for multi-account isolation and link filtering.

The main gaps are:

- `/api/search` has no date range filter.
- Query integer parameters silently fall back to zero when invalid.
- `Search` and `Latest` load links with one query per result row.
- Pagination is offset-only and does not have a stable cursor boundary.
- There is no controlled way to run `ANALYZE`, `PRAGMA optimize`, or FTS optimize.
- There is no local benchmark seed path for larger search datasets.

## Scope

### Search Filters and Validation

`GET /api/search` will accept:

- `q`
- `account_id`
- `channel_id`
- `link_type`
- `date_from`
- `date_to`
- `limit`
- `offset`

Date parsing will match `/api/links`:

- RFC3339 timestamps are accepted.
- `YYYY-MM-DD` dates are accepted.
- `date_to` with `YYYY-MM-DD` is inclusive by converting it to the next day as an exclusive upper bound.

Integer query parameters will return `400` for invalid values instead of silently using zero. IDs must be positive when present. `limit` and `offset` must be non-negative, with existing repository clamping preserving the maximum response size.

### Repository Query Shape

`repository.SearchParams` and `search.Params` will add `DateFrom` and `DateTo`.

`MessageRepository.Search` will filter on `m.date >= ?` and `m.date < ?` when date bounds are provided. The FTS table remains the driver for keyword search. Ordering remains stable as:

```sql
ORDER BY m.date DESC, m.id DESC
```

Offset pagination stays supported for compatibility. Cursor pagination is added in a minimal form with `before_date` and `before_id` on search/latest service parameters. When present, repositories apply:

```sql
(m.date < ? OR (m.date = ? AND m.id < ?))
```

This allows callers to page through large result sets without deep offsets.

### Bulk Link Loading

`attachLinks` will replace per-message link queries with a batch query:

```sql
SELECT id, message_id, type, url, password, created_at
FROM telegram_links
WHERE message_id IN (...)
ORDER BY message_id, id
```

The function will preserve each result row and attach links by `message_id`. Empty result sets still return without querying links.

### SQLite Maintenance

A small maintenance repository/service will expose a controlled operation:

- `ANALYZE`
- `PRAGMA optimize`
- `INSERT INTO telegram_messages_fts(telegram_messages_fts) VALUES ('optimize')`

The operation is explicit and not run on read requests. API exposure will be local-service oriented:

```text
POST /api/maintenance/sqlite
```

The response will list the operations that ran. This endpoint has no destructive actions and does not run `VACUUM` in this phase, because `VACUUM` can be expensive and should be planned separately.

### Benchmark Seed Support

Add a Go benchmark in repository/search tests that seeds a bounded local dataset in a temp SQLite DB. The benchmark will not try to insert one million rows in normal tests. It will provide a reusable path for local performance checks and establish that result limits bound memory and response size.

## API Behavior

Invalid query examples return 400:

- `/api/search?q=x&limit=abc`
- `/api/search?q=x&limit=-1`
- `/api/search?q=x&offset=-1`
- `/api/search?q=x&account_id=abc`
- `/api/search?q=x&date_from=not-a-date`

Valid date examples:

- `/api/search?q=x&date_from=2026-01-01&date_to=2026-01-31`
- `/api/search?q=x&date_from=2026-01-01T00:00:00Z&date_to=2026-02-01T00:00:00Z`

Cursor pagination examples:

- `/api/search?q=x&before_date=2026-02-05T12:00:00Z&before_id=123`
- `/api/messages/latest?before_date=2026-02-05T12:00:00Z&before_id=123`

Both `before_date` and `before_id` must be present together. If only one is supplied, return 400.

## Testing

Tests will cover:

- Search service date range filters.
- Search API date range filters and invalid date handling.
- Invalid integer parameters return 400.
- Cursor pagination returns only rows older than the boundary.
- Search/latest attach links with one bulk query path and still return correct links.
- SQLite maintenance endpoint runs and returns operation names.
- Repository benchmark seed function can populate larger datasets without changing normal test runtime.

## Non-Goals

This phase will not implement:

- Real 1 million row CI tests.
- `VACUUM` API or scheduler.
- Sync worker pools.
- Retry queue.
- FloodWait handling.
- Public authentication/authorization, because the service remains localhost-only.

## Success Criteria

- Full test suite passes.
- `/api/search` and `/api/links` use consistent date parsing and validation.
- Invalid query parameters return clear 400 responses.
- Search/latest result link loading avoids N+1 repository queries.
- SQLite maintenance can be triggered explicitly.
- Cursor pagination is available for search/latest while offset compatibility remains.
