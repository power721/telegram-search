# Resource Quality And Trends Design

## Goal

Implement the backend-first `v0.14` slice for resource quality and trends: hot sorting, score fields, score explanations, and a trending API.

## Scope

This slice adds:

* `sort=hot` for `/api/resources`.
* Score fields on resource API items:
  * `score`
  * `score_explain.source_channel_count`
  * `score_explain.message_count`
  * `score_explain.provider_count`
  * `score_explain.recency_score`
  * `score_explain.type_score`
  * `score_explain.metadata_score`
* `GET /api/trending?range=today|week|month`.

This slice does not add:

* Frontend UI.
* A persistent `resource_scores` table.
* Background score recomputation jobs.
* AI ranking or cross-instance trending.

## Architecture

The existing resource library builds `resource.Item` values at query time from `telegram_links` and `telegram_files`. The first `v0.14` slice keeps that shape: it computes scores in `internal/resource` after resources are loaded and before pagination is returned.

Score calculation stays deterministic and local. It uses data already present in resource items plus lightweight aggregate queries for deduped link resources. Link resources use their URL as the resource key; file resources use their `file:<id>` identity. This avoids schema churn while stabilizing the API contract.

`/api/trending` reuses `resource.Service.List` with a date window and `Sort: "hot"`. Date filtering is added to `resource.Query` and passed through the existing repository search parameters.

## Scoring

Total score is the sum of:

```text
source_channel_count * 10
+ message_count * 3
+ provider_count * 6
+ recency_score
+ type_score
+ metadata_score
```

Components:

* `source_channel_count`: number of distinct source channels for the same resource.
* `message_count`: number of non-deleted messages containing the resource.
* `provider_count`: number of distinct providers for the resource. For URL-deduped links this is normally `1`; for files this is `1`.
* `recency_score`: date-based score from the resource publish time.
  * today: `30`
  * last 7 days: `20`
  * last 30 days: `10`
  * older: `0`
* `type_score`: existing resource category/provider weighting reused from resource quality sorting.
* `metadata_score`: existing metadata completeness score.

The implementation computes link aggregate stats by URL in batch for the current result set. If aggregate lookup fails, the request fails rather than silently returning misleading scores.

## API Behavior

`GET /api/resources?sort=hot` returns the normal resource response shape with `items`, `total`, and `grouped`. Each item includes `score` and `score_explain`.

For `sort=hot`, the service computes the matching grouped total first and loads the matching resource set before ranking and paginating. This keeps hot ordering correct when an older resource has a higher score than the newest resources. A future persistent `resource_scores` table can reduce this request-time work.

`GET /api/trending?range=week&limit=20` returns the same `resource.ListResult` shape. Valid ranges:

* `today`: resources with message date from the current day in UTC.
* `week`: resources with message date from the last 7 days.
* `month`: resources with message date from the last 30 days.

Invalid ranges return `400`.

## Error Handling

Repository aggregate query errors return `500` through the existing API error envelope.

Invalid trending range returns `400` with a clear error message.

Trending uses the same admin session requirement as `/api/resources`.

## Testing

Backend tests cover:

* Resource service hot sorting prefers higher score over newer low-value resources.
* Resource service attaches nonzero score fields and explanation fields.
* `/api/resources?sort=hot` returns score fields.
* `/api/trending?range=week` filters by date window and sorts by score.
* `/api/trending?range=invalid` returns `400`.

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/resource ./internal/api
GOCACHE=/tmp/go-build-cache go test ./...
```

## Compatibility

Existing callers that ignore unknown JSON fields remain compatible. Default `/api/resources` behavior keeps date sorting when there is no keyword and quality sorting when a keyword is present. Hot sorting is opt-in through `sort=hot` and through `/api/trending`.
