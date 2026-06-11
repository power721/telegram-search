# Resource Quality And Trends Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add backend hot resource scoring and a trending API for the `v0.14` resource quality slice.

**Architecture:** Keep scoring in `internal/resource` for this slice and reuse existing link/file repositories. Add a batch aggregate query for link resources by URL, compute score explanations on resource items, and expose `sort=hot` plus `/api/trending`.

**Tech Stack:** Go 1.24, Gin, SQLite, standard `testing`, existing repository/service/API patterns.

---

## File Structure

* Modify `internal/resource/service.go`: add query date filters, score fields, scoring helpers, hot sorting, and trending-friendly list behavior.
* Modify `internal/repository/types.go`: keep existing params and use current date range fields already present on link/file params.
* Modify `internal/repository/link.go`: add `ResourceStatsByURL` batch aggregate query for deduped link resources.
* Modify `internal/api/handlers.go`: add `trending` handler and helper for `today|week|month`.
* Modify `internal/api/router.go`: route `GET /api/trending` under admin session resource access.
* Modify `internal/resource/service_test.go`: add service-level tests for scoring and hot sorting.
* Modify `internal/api/handlers_test.go`: add API tests for `sort=hot`, trending date windows, and invalid range handling.

## Task 1: Resource Score Model And Hot Sorting

**Files:**

* Modify: `internal/resource/service.go`
* Test: `internal/resource/service_test.go`

- [ ] **Step 1: Write the failing service test**

Add a test named `TestResourceLibraryHotSortUsesScoreExplanations` to `internal/resource/service_test.go`. The test creates an older repeated high-value link and a newer single low-value link, then calls:

```go
result, err := service.List(ctx, Query{Sort: "hot", Limit: 10})
```

Assert:

```go
if result.Items[0].URL != "https://pan.quark.cn/s/hot" {
	t.Fatalf("first hot resource = %+v, want repeated quark resource", result.Items[0])
}
if result.Items[0].Score <= result.Items[1].Score {
	t.Fatalf("scores = %d <= %d, want hot resource first", result.Items[0].Score, result.Items[1].Score)
}
if result.Items[0].ScoreExplain.SourceChannelCount != 2 || result.Items[0].ScoreExplain.MessageCount != 2 || result.Items[0].ScoreExplain.ProviderCount != 1 {
	t.Fatalf("score explain = %+v, want source/message/provider counts", result.Items[0].ScoreExplain)
}
```

- [ ] **Step 2: Run the focused test and verify RED**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/resource -run TestResourceLibraryHotSortUsesScoreExplanations -count=1
```

Expected: compile failure for missing `Score`, `ScoreExplain`, or hot sorting behavior.

- [ ] **Step 3: Implement score fields and hot sorting**

In `internal/resource/service.go`, add:

```go
type ScoreExplain struct {
	SourceChannelCount int `json:"source_channel_count"`
	MessageCount       int `json:"message_count"`
	ProviderCount      int `json:"provider_count"`
	RecencyScore       int `json:"recency_score"`
	TypeScore          int `json:"type_score"`
	MetadataScore      int `json:"metadata_score"`
}
```

Add fields to `Item`:

```go
Score        int          `json:"score"`
ScoreExplain ScoreExplain `json:"score_explain"`
```

Add helpers:

```go
func isHotSort(sort string) bool { return sort == "hot" }
func (s *Service) attachScores(ctx context.Context, items []Item, now time.Time) error
func sortItemsByHot(items []Item)
func itemScore(item Item, stats resourceScoreStats, now time.Time) (int, ScoreExplain)
```

Use `attachScores` before sorting. Use `sortItemsByHot` when `query.Sort == "hot"`.

- [ ] **Step 4: Run the focused test and verify GREEN**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/resource -run TestResourceLibraryHotSortUsesScoreExplanations -count=1
```

Expected: PASS.

## Task 2: Link Aggregate Stats

**Files:**

* Modify: `internal/repository/link.go`
* Test: `internal/resource/service_test.go`

- [ ] **Step 1: Write the failing aggregate-backed test**

Use the same test from Task 1 to require aggregate stats from duplicate URLs across two channels. Ensure the fixture saves the same URL in two non-deleted messages from two channels:

```go
if _, err := links.SaveBatch(ctx, hotMsg.ID, []model.Link{{Type: "quark", Category: "cloud_drive", URL: "https://pan.quark.cn/s/hot", MediaTitle: "Hot Pack", MediaQuality: "4K"}}); err != nil {
	t.Fatalf("save hot link: %v", err)
}
```

- [ ] **Step 2: Run the focused test and verify RED**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/resource -run TestResourceLibraryHotSortUsesScoreExplanations -count=1
```

Expected: FAIL because aggregate counts stay at `1` until the repository query is implemented.

- [ ] **Step 3: Add repository aggregate query**

In `internal/repository/link.go`, add:

```go
type LinkResourceStats struct {
	URL                string
	SourceChannelCount int
	MessageCount       int
	ProviderCount      int
}

func (r *LinkRepository) ResourceStatsByURL(ctx context.Context, urls []string) (map[string]LinkResourceStats, error)
```

Build `IN (?, ?, ...)` SQL bind markers from non-empty unique URLs and run:

```sql
SELECT l.url,
       count(DISTINCT m.channel_id),
       count(DISTINCT m.id),
       count(DISTINCT COALESCE(NULLIF(l.type, ''), 'url'))
FROM telegram_links l
JOIN telegram_messages m ON m.id = l.message_id
WHERE m.deleted = 0 AND l.url IN (...)
GROUP BY l.url
```

In `resource.Service.attachScores`, call this method for current link items and default file stats to one source, one message, and one provider.

- [ ] **Step 4: Run the focused test and verify GREEN**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/resource -run TestResourceLibraryHotSortUsesScoreExplanations -count=1
```

Expected: PASS.

## Task 3: Correct Hot Pagination And Date Filters

**Files:**

* Modify: `internal/resource/service.go`
* Test: `internal/resource/service_test.go`

- [ ] **Step 1: Write the failing pagination/date test**

Add `TestResourceLibraryHotSortConsidersResourcesOutsideNewestPage`. Create 60 newer weak links and one older repeated high-score link. Call:

```go
result, err := service.List(ctx, Query{Sort: "hot", Limit: 10})
```

Assert the older repeated URL is first.

Also call with:

```go
from := now.Add(-24 * time.Hour)
result, err := service.List(ctx, Query{Sort: "hot", DateFrom: &from, Limit: 10})
```

Assert the older hot URL is excluded.

- [ ] **Step 2: Run the focused test and verify RED**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/resource -run 'TestResourceLibraryHotSortConsidersResourcesOutsideNewestPage|TestResourceLibraryHotSortUsesScoreExplanations' -count=1
```

Expected: FAIL because `Query` lacks date fields or hot sorting only sees the initial newest window.

- [ ] **Step 3: Implement date filters and hot fetch behavior**

Add fields to `resource.Query`:

```go
DateFrom *time.Time
DateTo   *time.Time
```

Pass them to `repository.LinkSearchParams` and `repository.FileSearchParams` in list, grouped, and count calls.

When `isHotSort(query.Sort)` is true:

```go
grouped, err := s.groupedForQuery(ctx, query)
total := groupedTotal(grouped)
fetchLimit := total
```

Return early with empty items when `total == 0`. Otherwise fetch all matching resources, score, hot-sort, and apply offset/limit in memory.

- [ ] **Step 4: Run resource tests and verify GREEN**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/resource -count=1
```

Expected: PASS.

## Task 4: Resources API Hot Sort And Trending Endpoint

**Files:**

* Modify: `internal/api/handlers.go`
* Modify: `internal/api/router.go`
* Test: `internal/api/handlers_test.go`

- [ ] **Step 1: Write failing API tests**

Add tests:

* `TestResourcesAPIHotSortReturnsScoreFields`
* `TestTrendingResourcesAPIUsesRangeAndHotSort`
* `TestTrendingResourcesAPIRejectsInvalidRange`

Assertions:

```go
req := httptest.NewRequest(http.MethodGet, "/api/resources?sort=hot", nil)
```

Response item has `score > 0` and `score_explain.message_count > 0`.

```go
req := httptest.NewRequest(http.MethodGet, "/api/trending?range=week&limit=10", nil)
```

Response excludes a resource older than seven days and sorts the remaining resources by `score`.

```go
req := httptest.NewRequest(http.MethodGet, "/api/trending?range=year", nil)
```

Status is `400`.

- [ ] **Step 2: Run API tests and verify RED**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/api -run 'TestResourcesAPIHotSortReturnsScoreFields|TestTrendingResourcesAPIUsesRangeAndHotSort|TestTrendingResourcesAPIRejectsInvalidRange' -count=1
```

Expected: FAIL because `/api/trending` is not routed or score fields are missing.

- [ ] **Step 3: Implement API handler and route**

Add `trendingResources` handler in `internal/api/handlers.go`:

```go
func (h handlers) trendingResources(c *gin.Context)
```

Parse `range` with helper:

```go
func trendingDateRange(value string, now time.Time) (*time.Time, *time.Time, bool)
```

Supported values:

* empty or `today`: start of current UTC day.
* `week`: `now.AddDate(0, 0, -7)`.
* `month`: `now.AddDate(0, 0, -30)`.

Build `resource.Query{Sort: "hot", DateFrom: from, DateTo: to, Limit: limit, Offset: offset}` and return the normal list result after media attachment.

In `internal/api/router.go`, add:

```go
resourceAccess.GET("/trending", h.trendingResources)
```

- [ ] **Step 4: Run API tests and verify GREEN**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/api -run 'TestResourcesAPIHotSortReturnsScoreFields|TestTrendingResourcesAPIUsesRangeAndHotSort|TestTrendingResourcesAPIRejectsInvalidRange' -count=1
```

Expected: PASS.

## Task 5: Full Verification And Commit

**Files:**

* All modified files from previous tasks.

- [ ] **Step 1: Run backend verification**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/resource ./internal/api
```

Expected: PASS.

- [ ] **Step 2: Run full repository verification**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./...
```

Expected: PASS.

- [ ] **Step 3: Check diff and whitespace**

Run:

```bash
git diff --check
git status --short
```

Expected: no whitespace errors; only intended files modified.

- [ ] **Step 4: Commit implementation**

Run:

```bash
git add internal/resource/service.go internal/resource/service_test.go internal/repository/link.go internal/api/handlers.go internal/api/router.go internal/api/handlers_test.go docs/superpowers/specs/2026-06-11-resource-quality-trends-design.md docs/superpowers/plans/2026-06-11-resource-quality-trends.md
git commit -m "feat: add resource hot sorting and trends API"
```

Expected: commit succeeds on `feat/resource-quality-trends`.

## Self-Review

Spec coverage:

* `sort=hot`: Task 1 and Task 3.
* Score fields and explanations: Task 1 and Task 4.
* Link aggregate stats: Task 2.
* Trending API: Task 4.
* Date window filtering: Task 3 and Task 4.
* Tests and verification: Task 5.

Incomplete-section scan:

* No incomplete sections remain.

Type consistency:

* `ScoreExplain` uses JSON field names from the design.
* `resource.Query.DateFrom` and `DateTo` map to existing repository date filter params.
* `/api/trending` returns `resource.ListResult`, matching `/api/resources`.
