# Resource Read Model Design

## Goal

Make admin `/api/resources` and public `/api/search` return resource results quickly and predictably, targeting sub-100ms responses for common first-page requests on the current SQLite-backed deployment.

Public `/api/search` remains a link-focused external API. Its response contract stays compatible with the current `{code, message, data: {total, merged_by_type}}` shape by default.

## Current Problem

Both endpoints ultimately call `resource.Service.List`, which builds resource results at request time from `telegram_links`, `telegram_files`, `telegram_messages`, `telegram_message_contents`, and `telegram_channels`.

The expensive parts are:

- deduplicating links by URL with window functions;
- joining message/channel/content tables for every list/search request;
- counting totals with separate scans;
- filtering link/file text with `LIKE`;
- running multiple resource list queries in public `/api/search` for cloud type groups;
- attaching resource score statistics with extra URL aggregation queries.

This design moves those computations to write time and rebuild time.

## Proposed Architecture

Add a denormalized SQLite read model table named `resource_index`. Each row is one resource item that can be returned directly by `/api/resources` or converted directly into public `/api/search` output.

Add `resource_index_fts` as an FTS5 table for keyword lookup across title, note, source snippet, URL, provider, and media metadata fields.

The existing normalized Telegram tables remain the source of truth. `resource_index` is a derived table maintained by repository/service code after messages, links, or files change.

## Data Model

`resource_index` stores the resource fields currently assembled in `resource.Item`:

- identity: integer `id`, `resource_id`, `kind`, `source_key`;
- link fields: `url`, `type`, `category`, `password`, `note`, `source_snippet`;
- file fields: `telegram_file_id`, `file_name`, `extension`, `mime_type`, `size_bytes`;
- media metadata: `media_title`, `media_year`, `media_season`, `media_episode`, `media_quality`, `media_size`, `media_tmdb_id`, `media_category`, `media_tags`, `media_summary`;
- source fields: `datetime`, `account_id`, `channel_id`, `telegram_channel_id`, `channel_title`, `channel_username`, `telegram_message_id`, `message_type`;
- ranking fields: `source_channel_count`, `message_count`, `provider_count`, `score`;
- lifecycle fields: `updated_at`.

Suggested keys:

- `id` is the SQLite integer primary key used by FTS.
- `resource_id` is unique and exposed to API clients, matching current IDs like `link:<url>` and `file:<id>`.
- Link resources use one row per deduped URL.
- File resources use one row per file resource that should be visible in the resource library.

Suggested indexes:

- `(category, datetime DESC, resource_id DESC)`;
- `(type, datetime DESC, resource_id DESC)`;
- `(datetime DESC, resource_id DESC)`;
- `(channel_id, datetime DESC, resource_id DESC)`;
- `(account_id, datetime DESC, resource_id DESC)`;
- `(score DESC, datetime DESC, resource_id DESC)`;
- `(kind, datetime DESC, resource_id DESC)`;
- unique `(source_key)` if needed for internal update bookkeeping.

`resource_index_fts` uses `content='resource_index'` and `content_rowid='id'`. It indexes only searchable text, not every API field.

## Query Behavior

### `/api/resources`

The admin resource list reads from `resource_index`.

No keyword:

- apply structured filters directly on indexed columns;
- order by `datetime DESC, resource_id DESC` by default;
- order by `score DESC, datetime DESC, resource_id DESC` for hot sort;
- return `LIMIT/OFFSET` rows directly.

With keyword:

- query `resource_index_fts MATCH ?`;
- join matching rowids back to `resource_index`;
- apply structured filters;
- use current quality scoring semantics where needed, but compute only for the page candidate set instead of the whole normalized corpus.

Grouped/total counts:

- count from `resource_index`;
- keep `resource_group_counts` only as an optional global cache;
- for filtered queries, count directly from `resource_index` or the FTS subset.

### Public `/api/search`

Public `/api/search` also reads from `resource_index`.

Its default behavior remains:

- authenticate with `Authorization`;
- parse `kw`, `q`, `keyword`, `cloud_types`, `limit`, `offset`, `res`, `include_image`, and media metadata flags;
- return `externalAPIResponse{Code: 0, Message: "success", Data: externalSearchResponse{...}}`;
- default `res` is `merged_by_type`.

The current multiple-pass `externalResourceItems` loop should become one query against `resource_index`:

- convert `cloud_types` to category/type filters;
- use `IN` predicates for categories and providers;
- query and page once;
- build `merged_by_type` from the returned page.

This removes duplicate per-category scans and avoids fetching `offset+limit` per category.

## Write-Time Maintenance

Add a `ResourceIndexRepository` and service methods:

- `RefreshMessage(ctx, messageID int64) error`;
- `RefreshMessages(ctx, messageIDs []int64) error`;
- `DeleteMessage(ctx, messageID int64) error`;
- `Rebuild(ctx) error`;
- `Stats(ctx) (indexedRows int, updatedAt time.Time, err error)`.

Message/link/file writes call refresh methods after successful transaction commit. If a transaction already writes messages plus links/files together, index refresh should happen after commit to avoid partial reads.

Link dedupe rules:

- for each URL, choose the newest non-deleted source message by `message.date DESC, link.id DESC`;
- aggregate `source_channel_count`, `message_count`, and `provider_count` from all non-deleted rows for that URL;
- recompute score using the same scoring function currently used by `resource.Service`.

File rules:

- exclude image files by default, matching current resource library behavior;
- include video/audio/document/archive/software/file categories;
- keep file rows independent from link URL dedupe.

Deletes:

- deleting a resource by URL removes source links as today, then refreshes/removes the affected `resource_index` URL row;
- deleting file resources removes the file and its index row;
- soft-deleting messages removes their resources from the index or selects the next newest link source for the same URL.

## Migration And Backfill

Add a migration that creates `resource_index`, `resource_index_fts`, triggers for FTS maintenance if using external content, and all performance indexes.

Backfill should be explicit application code rather than a giant migration SQL block:

1. `db.Migrate` creates empty read model tables.
2. startup detects empty or stale resource index and schedules/executes a rebuild.
3. a maintenance API or command can run `RebuildResourceIndex`.
4. rebuild runs in batches and swaps data safely:
   - write to `resource_index_build`;
   - populate FTS for build table;
   - replace live table inside a short transaction, or delete/insert live rows in chunks if table swap is too invasive.

For the first implementation, an in-place batch rebuild is acceptable if tests prove readers always see either old rows or a consistent partial state is marked as rebuilding and not used.

## Error Handling

If `resource_index` is missing or empty after migration, the app should not silently return empty search results forever.

Behavior:

- `Rebuild(ctx)` errors are logged and surfaced through maintenance/status endpoints;
- `/api/resources` and `/api/search` may fall back to the old normalized query path during the first deployment only if the index is unavailable;
- once the index exists and has rows, query errors should return normal 500 responses rather than falling back silently.

## Testing

Coverage should include:

- migration creates tables, FTS table, and indexes;
- rebuild produces the same visible resource set as current `resource.Service.List` for representative links/files;
- URL dedupe chooses newest source and preserves aggregate stats;
- image files remain excluded by default;
- `/api/resources` returns same JSON fields and totals as before;
- public `/api/search` preserves existing response contract for `merged_by_type`, `results`, and `all`;
- cloud type filtering runs as a single resource index query;
- keyword search uses FTS and matches title, note, URL, and media tags;
- deletes and message soft deletes update the index;
- benchmarks or query-plan tests verify common first-page reads do not use normalized-table window dedupe.

## Non-Goals

- Do not replace SQLite.
- Do not change public `/api/search` response shape.
- Do not remove normalized Telegram source tables.
- Do not redesign frontend resource UI in this phase.
- Do not implement distributed indexing or async external search infrastructure.

## Rollout

1. Create the read model schema and repository behind tests.
2. Add rebuild logic and compare read model output against current resource service.
3. Switch `/api/resources` to read from the index.
4. Switch public `/api/search` to read from the same index and preserve response contract.
5. Add maintenance/status visibility for index rebuild state.
6. Keep the previous query path available only as a temporary fallback during rollout, then remove it after confidence is established.
