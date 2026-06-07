# Phase 5a Search Performance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Improve search/read performance foundations with search date filters, strict API validation, cursor pagination, batched link loading, controlled SQLite maintenance, and a local benchmark seed path.

**Architecture:** Keep the public HTTP API compatible while adding optional filters. Push search filtering and pagination into repository params, keep API parsing centralized in `internal/api/handlers.go`, and expose SQLite maintenance through an explicit repository method and local API endpoint. Avoid runtime scheduler/retry/FloodWait work in this phase.

**Tech Stack:** Go, Gin, SQLite/FTS5, existing repository/search/api packages, Go tests and benchmarks.

---

## File Structure

- Modify `internal/repository/types.go`: add date and cursor fields to `SearchParams` and cursor fields to `LatestParams`.
- Modify `internal/repository/message.go`: apply search date filters, cursor filters, and replace N+1 link loading with batch loading.
- Modify `internal/db/migrations.go`: add a new performance index migration.
- Modify `internal/db/db_test.go`: assert new performance indexes exist.
- Modify `internal/search/service.go`: pass date and cursor params through from service to repository.
- Modify `internal/search/service_test.go`: add search date range and cursor pagination coverage.
- Modify `internal/api/handlers.go`: add strict query parsing, search date parsing, cursor parsing, and maintenance handler.
- Modify `internal/api/router.go`: add `Maintenance` dependency and `POST /api/maintenance/sqlite`.
- Modify `internal/api/handlers_test.go`: add invalid query, search date range, cursor, and maintenance endpoint tests.
- Modify `cmd/tg-provider/main.go`: wire the maintenance repository into API dependencies.
- Create `internal/repository/maintenance.go`: controlled SQLite maintenance operations.
- Create `internal/repository/maintenance_test.go`: repository-level maintenance test.
- Create `internal/repository/search_benchmark_test.go`: local benchmark seed helper and benchmark.

## Task 1: Search Date Range Filters

**Files:**
- Modify: `internal/search/service_test.go`
- Modify: `internal/api/handlers_test.go`
- Modify: `internal/repository/types.go`
- Modify: `internal/repository/message.go`
- Modify: `internal/search/service.go`
- Modify: `internal/api/handlers.go`

- [ ] **Step 1: Write failing search service date range test**

Add this test to `internal/search/service_test.go` after `TestServiceLinksFiltersByMessageDateRange`:

```go
func TestServiceSearchFiltersByMessageDateRange(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "telegram.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}

	accounts := repository.NewAccountRepository(conn)
	channels := repository.NewChannelRepository(conn)
	messages := repository.NewMessageRepository(conn)
	links := repository.NewLinkRepository(conn)
	accountID, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 1, Title: "VIP", Type: model.ChannelTypeChannel})
	january := time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC)
	february := time.Date(2026, 2, 5, 12, 0, 0, 0, time.UTC)
	if _, err := messages.SaveBatch(ctx, []model.Message{
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1, Text: "shared keyword january", RawJSON: "{}", Date: january},
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 2, Text: "shared keyword february", RawJSON: "{}", Date: february},
	}); err != nil {
		t.Fatalf("save messages: %v", err)
	}

	service := NewService(messages, links)
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	results, err := service.Search(ctx, Params{Query: "shared", DateFrom: &from, DateTo: &to, Limit: 10})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 1 || results[0].Date.Month() != time.January {
		t.Fatalf("date filtered search results = %+v, want January only", results)
	}
}
```

- [ ] **Step 2: Write failing API search date range test**

Add this test to `internal/api/handlers_test.go` after `TestLinksAPIFiltersByDateRangeAndRejectsInvalidDate`:

```go
func TestSearchAPIFiltersByDateRange(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	accountID, _ := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Username: "main", Status: model.AccountStatusOnline})
	channelID, _ := deps.Channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 1, Title: "VIP", Type: model.ChannelTypeChannel})
	january := time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC)
	february := time.Date(2026, 2, 5, 12, 0, 0, 0, time.UTC)
	_, _ = deps.Messages.SaveBatch(ctx, []model.Message{
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1, Text: "shared january", RawJSON: "{}", Date: january},
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 2, Text: "shared february", RawJSON: "{}", Date: february},
	})
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/search?q=shared&date_from=2026-01-01&date_to=2026-01-31", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("january")) || bytes.Contains(w.Body.Bytes(), []byte("february")) {
		t.Fatalf("date range response = %s, want january only", w.Body.String())
	}
}
```

- [ ] **Step 3: Run tests to verify failure**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/search ./internal/api
```

Expected: FAIL because `search.Params` has no `DateFrom`/`DateTo` fields and `/api/search` ignores date filters.

- [ ] **Step 4: Add date fields to repository and service params**

Modify `internal/repository/types.go`:

```go
type SearchParams struct {
	Query     string
	AccountID int64
	ChannelID int64
	LinkType  string
	DateFrom  *time.Time
	DateTo    *time.Time
	Limit     int
	Offset    int
}
```

Modify `internal/search/service.go`:

```go
type Params struct {
	Query     string
	AccountID int64
	ChannelID int64
	LinkType  string
	DateFrom  *time.Time
	DateTo    *time.Time
	Limit     int
	Offset    int
}
```

Pass the fields in `Service.Search`:

```go
return s.messages.Search(ctx, repository.SearchParams{
	Query:     query,
	AccountID: params.AccountID,
	ChannelID: params.ChannelID,
	LinkType:  params.LinkType,
	DateFrom:  params.DateFrom,
	DateTo:    params.DateTo,
	Limit:     params.Limit,
	Offset:    params.Offset,
})
```

- [ ] **Step 5: Add repository date filters**

In `internal/repository/message.go`, add after the `LinkType` filter:

```go
if params.DateFrom != nil {
	where = append(where, `m.date >= ?`)
	args = append(args, *params.DateFrom)
}
if params.DateTo != nil {
	where = append(where, `m.date < ?`)
	args = append(args, *params.DateTo)
}
```

- [ ] **Step 6: Parse search date range in API**

In `internal/api/handlers.go`, update `search`:

```go
func (h handlers) search(c *gin.Context) {
	dateFrom, dateTo, ok := parseDateRange(c)
	if !ok {
		return
	}
	items, err := h.deps.Search.Search(c.Request.Context(), searchsvc.Params{
		Query:     c.Query("q"),
		AccountID: queryInt(c, "account_id"),
		ChannelID: queryInt(c, "channel_id"),
		LinkType:  c.Query("link_type"),
		DateFrom:  dateFrom,
		DateTo:    dateTo,
		Limit:     queryIntValue(c, "limit"),
		Offset:    queryIntValue(c, "offset"),
	})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, searchsvc.ErrEmptyQuery) {
			status = http.StatusBadRequest
		}
		errorJSON(c, status, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}
```

- [ ] **Step 7: Run tests to verify pass**

Run:

```bash
gofmt -w internal/repository/types.go internal/repository/message.go internal/search/service.go internal/search/service_test.go internal/api/handlers.go internal/api/handlers_test.go
GOCACHE=/tmp/go-build-cache go test ./internal/search ./internal/api ./internal/repository
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/repository/types.go internal/repository/message.go internal/search/service.go internal/search/service_test.go internal/api/handlers.go internal/api/handlers_test.go
git commit -m "feat: add search date range filters"
```

## Task 2: Strict Query Parameter Validation

**Files:**
- Modify: `internal/api/handlers_test.go`
- Modify: `internal/api/handlers.go`

- [ ] **Step 1: Write failing API invalid query tests**

Add this test to `internal/api/handlers_test.go`:

```go
func TestReadAPIsRejectInvalidQueryParameters(t *testing.T) {
	router := NewRouter(testDeps(t))
	for _, path := range []string{
		"/api/search?q=x&limit=abc",
		"/api/search?q=x&limit=-1",
		"/api/search?q=x&offset=-1",
		"/api/search?q=x&account_id=abc",
		"/api/search?q=x&account_id=0",
		"/api/search?q=x&channel_id=abc",
		"/api/messages/latest?limit=-1",
		"/api/messages/latest?account_id=abc",
		"/api/links?offset=-1",
		"/api/links?channel_id=abc",
		"/api/channels?account_id=abc",
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		router.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d body=%s, want 400", path, w.Code, w.Body.String())
		}
	}
}
```

- [ ] **Step 2: Run API tests to verify failure**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/api
```

Expected: FAIL because invalid integer query values are silently treated as zero.

- [ ] **Step 3: Add strict query helpers**

In `internal/api/handlers.go`, replace the old `queryInt`/`queryIntValue` helpers with:

```go
func queryPositiveInt64(c *gin.Context, key string) (int64, bool) {
	value := c.Query(key)
	if value == "" {
		return 0, true
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n <= 0 {
		errorText(c, http.StatusBadRequest, key+" must be a positive integer")
		return 0, false
	}
	return n, true
}

func queryNonNegativeInt(c *gin.Context, key string) (int, bool) {
	value := c.Query(key)
	if value == "" {
		return 0, true
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n < 0 {
		errorText(c, http.StatusBadRequest, key+" must be a non-negative integer")
		return 0, false
	}
	if int64(int(n)) != n {
		errorText(c, http.StatusBadRequest, key+" is too large")
		return 0, false
	}
	return int(n), true
}

func readFilters(c *gin.Context) (accountID int64, channelID int64, limit int, offset int, ok bool) {
	accountID, ok = queryPositiveInt64(c, "account_id")
	if !ok {
		return 0, 0, 0, 0, false
	}
	channelID, ok = queryPositiveInt64(c, "channel_id")
	if !ok {
		return 0, 0, 0, 0, false
	}
	limit, ok = queryNonNegativeInt(c, "limit")
	if !ok {
		return 0, 0, 0, 0, false
	}
	offset, ok = queryNonNegativeInt(c, "offset")
	if !ok {
		return 0, 0, 0, 0, false
	}
	return accountID, channelID, limit, offset, true
}
```

- [ ] **Step 4: Use helpers in read handlers**

Update `search`, `latest`, `links`, and `channels`.

For `search`:

```go
accountID, channelID, limit, offset, ok := readFilters(c)
if !ok {
	return
}
```

Use the parsed values in `searchsvc.Params`.

For `latest`, use `readFilters(c)` and ignore `offset`.

For `links`, use `readFilters(c)` and pass all four values.

For `channels`, parse only `account_id`:

```go
accountID, ok := queryPositiveInt64(c, "account_id")
if !ok {
	return
}
```

- [ ] **Step 5: Run API tests**

Run:

```bash
gofmt -w internal/api/handlers.go internal/api/handlers_test.go
GOCACHE=/tmp/go-build-cache go test ./internal/api
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/api/handlers.go internal/api/handlers_test.go
git commit -m "feat: validate read query parameters"
```

## Task 3: Cursor Pagination and Performance Indexes

**Files:**
- Modify: `internal/search/service_test.go`
- Modify: `internal/api/handlers_test.go`
- Modify: `internal/repository/types.go`
- Modify: `internal/repository/message.go`
- Modify: `internal/search/service.go`
- Modify: `internal/api/handlers.go`
- Modify: `internal/db/migrations.go`
- Modify: `internal/db/db_test.go`

- [ ] **Step 1: Write failing service cursor test**

Add this test to `internal/search/service_test.go`:

```go
func TestServiceSearchCursorReturnsOlderRows(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "telegram.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}

	accounts := repository.NewAccountRepository(conn)
	channels := repository.NewChannelRepository(conn)
	messages := repository.NewMessageRepository(conn)
	links := repository.NewLinkRepository(conn)
	accountID, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 1, Title: "VIP", Type: model.ChannelTypeChannel})
	newer := time.Date(2026, 2, 5, 12, 0, 0, 0, time.UTC)
	older := time.Date(2026, 2, 4, 12, 0, 0, 0, time.UTC)
	stored, err := messages.SaveBatch(ctx, []model.Message{
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1, Text: "shared newer", RawJSON: "{}", Date: newer},
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 2, Text: "shared older", RawJSON: "{}", Date: older},
	})
	if err != nil {
		t.Fatalf("save messages: %v", err)
	}

	service := NewService(messages, links)
	results, err := service.Search(ctx, Params{Query: "shared", BeforeDate: &newer, BeforeID: stored[0].ID, Limit: 10})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 1 || results[0].Text != "shared older" {
		t.Fatalf("cursor search results = %+v, want older row only", results)
	}
}
```

- [ ] **Step 2: Write failing API cursor validation test**

Add these cases to `TestReadAPIsRejectInvalidQueryParameters`:

```go
"/api/search?q=x&before_date=2026-02-05T12:00:00Z",
"/api/search?q=x&before_id=10",
"/api/messages/latest?before_date=2026-02-05T12:00:00Z",
```

Add this API cursor test:

```go
func TestSearchAPICursorReturnsOlderRows(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	accountID, _ := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Username: "main", Status: model.AccountStatusOnline})
	channelID, _ := deps.Channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 1, Title: "VIP", Type: model.ChannelTypeChannel})
	newer := time.Date(2026, 2, 5, 12, 0, 0, 0, time.UTC)
	older := time.Date(2026, 2, 4, 12, 0, 0, 0, time.UTC)
	stored, _ := deps.Messages.SaveBatch(ctx, []model.Message{
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1, Text: "shared newer", RawJSON: "{}", Date: newer},
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 2, Text: "shared older", RawJSON: "{}", Date: older},
	})
	router := NewRouter(deps)

	path := "/api/search?q=shared&before_date=" + url.QueryEscape(newer.Format(time.RFC3339)) + "&before_id=" + strconv.FormatInt(stored[0].ID, 10)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("shared older")) || bytes.Contains(w.Body.Bytes(), []byte("shared newer")) {
		t.Fatalf("cursor response = %s, want older only", w.Body.String())
	}
}
```

Add `net/url` to `internal/api/handlers_test.go` imports.

- [ ] **Step 3: Write failing performance index test**

Add this test to `internal/db/db_test.go`:

```go
func TestPerformanceIndexesExist(t *testing.T) {
	ctx := context.Background()
	conn, err := Open(filepath.Join(t.TempDir(), "telegram.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer conn.Close()
	if err := Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	for _, name := range []string{
		"idx_telegram_messages_account_date_id",
		"idx_telegram_messages_channel_date_id",
		"idx_telegram_links_type_message_id",
	} {
		assertIndexExists(t, conn, name)
	}
}

func assertIndexExists(t *testing.T, conn *sql.DB, name string) {
	t.Helper()
	var count int
	err := conn.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name = ?`, name).Scan(&count)
	if err != nil {
		t.Fatalf("check index %s: %v", name, err)
	}
	if count != 1 {
		t.Fatalf("index %s count = %d, want 1", name, count)
	}
}
```

- [ ] **Step 4: Run tests to verify failure**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/search ./internal/api ./internal/db
```

Expected: FAIL because cursor fields and indexes do not exist.

- [ ] **Step 5: Add cursor params**

Modify `internal/repository/types.go`:

```go
type SearchParams struct {
	Query      string
	AccountID  int64
	ChannelID  int64
	LinkType   string
	DateFrom   *time.Time
	DateTo     *time.Time
	BeforeDate *time.Time
	BeforeID   int64
	Limit      int
	Offset     int
}

type LatestParams struct {
	AccountID  int64
	ChannelID  int64
	BeforeDate *time.Time
	BeforeID   int64
	Limit      int
}
```

Modify matching structs in `internal/search/service.go` and pass fields through.

- [ ] **Step 6: Add cursor filters to repository queries**

In `MessageRepository.Search`, after date filters:

```go
if params.BeforeDate != nil && params.BeforeID > 0 {
	where = append(where, `(m.date < ? OR (m.date = ? AND m.id < ?))`)
	args = append(args, *params.BeforeDate, *params.BeforeDate, params.BeforeID)
}
```

In `MessageRepository.Latest`, after `ChannelID`:

```go
if params.BeforeDate != nil && params.BeforeID > 0 {
	where = append(where, `(m.date < ? OR (m.date = ? AND m.id < ?))`)
	args = append(args, *params.BeforeDate, *params.BeforeDate, params.BeforeID)
}
```

- [ ] **Step 7: Add API cursor parsing**

In `internal/api/handlers.go`, add:

```go
func parseCursor(c *gin.Context) (*time.Time, int64, bool) {
	beforeDateRaw := c.Query("before_date")
	beforeIDRaw := c.Query("before_id")
	if beforeDateRaw == "" && beforeIDRaw == "" {
		return nil, 0, true
	}
	if beforeDateRaw == "" || beforeIDRaw == "" {
		errorText(c, http.StatusBadRequest, "before_date and before_id must be provided together")
		return nil, 0, false
	}
	beforeDate, ok := parseDateQuery(c, "before_date", false)
	if !ok {
		return nil, 0, false
	}
	beforeID, err := strconv.ParseInt(beforeIDRaw, 10, 64)
	if err != nil || beforeID <= 0 {
		errorText(c, http.StatusBadRequest, "before_id must be a positive integer")
		return nil, 0, false
	}
	return beforeDate, beforeID, true
}
```

Call `parseCursor` in `search` and `latest`, then pass `BeforeDate` and `BeforeID` into service params.

- [ ] **Step 8: Add performance indexes migration**

Add a new migration to `internal/db/migrations.go`:

```go
{
	version: 4,
	name:    "performance_indexes",
	sql: `
CREATE INDEX IF NOT EXISTS idx_telegram_messages_account_date_id ON telegram_messages(account_id, date DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_telegram_messages_channel_date_id ON telegram_messages(channel_id, date DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_telegram_links_type_message_id ON telegram_links(type, message_id);
`,
},
```

- [ ] **Step 9: Run tests**

Run:

```bash
gofmt -w internal/repository/types.go internal/repository/message.go internal/search/service.go internal/search/service_test.go internal/api/handlers.go internal/api/handlers_test.go internal/db/migrations.go internal/db/db_test.go
GOCACHE=/tmp/go-build-cache go test ./internal/search ./internal/api ./internal/db ./internal/repository
```

Expected: PASS.

- [ ] **Step 10: Commit**

```bash
git add internal/repository/types.go internal/repository/message.go internal/search/service.go internal/search/service_test.go internal/api/handlers.go internal/api/handlers_test.go internal/db/migrations.go internal/db/db_test.go
git commit -m "feat: add cursor pagination for message reads"
```

## Task 4: Batch Load Search Result Links

**Files:**
- Modify: `internal/repository/message.go`
- Modify: `internal/repository/repository_test.go`

- [ ] **Step 1: Strengthen repository test for multiple result links**

In `internal/repository/repository_test.go`, add a focused test:

```go
func TestMessageSearchAttachesLinksForMultipleResults(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "telegram.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	accounts := NewAccountRepository(conn)
	channels := NewChannelRepository(conn)
	messages := NewMessageRepository(conn)
	links := NewLinkRepository(conn)
	accountID, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", Username: "main", Status: model.AccountStatusOnline})
	channelID, _ := channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 1, Title: "VIP", Type: model.ChannelTypeChannel})
	now := time.Now().UTC()
	stored, _ := messages.SaveBatch(ctx, []model.Message{
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1, Text: "shared one", RawJSON: "{}", Date: now},
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 2, Text: "shared two", RawJSON: "{}", Date: now.Add(-time.Minute)},
	})
	_, _ = links.SaveBatch(ctx, stored[0].ID, []model.Link{{Type: "aliyun", URL: "https://www.alipan.com/s/one"}})
	_, _ = links.SaveBatch(ctx, stored[1].ID, []model.Link{{Type: "quark", URL: "https://pan.quark.cn/s/two"}})

	results, err := messages.Search(ctx, SearchParams{Query: "shared", Limit: 10})
	if err != nil {
		t.Fatalf("search messages: %v", err)
	}
	if len(results) != 2 || len(results[0].Links) != 1 || len(results[1].Links) != 1 {
		t.Fatalf("search results links = %+v", results)
	}
}
```

- [ ] **Step 2: Run repository test**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/repository
```

Expected: PASS before implementation. This test preserves behavior before refactoring link loading.

- [ ] **Step 3: Replace N+1 link loading with batch loading**

In `internal/repository/message.go`, replace `attachLinks` with:

```go
func attachLinks(ctx context.Context, db *sql.DB, items []model.SearchResult) ([]model.SearchResult, error) {
	if len(items) == 0 {
		return items, nil
	}
	ids := make([]any, 0, len(items))
	placeholders := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
		placeholders = append(placeholders, "?")
	}
	rows, err := db.QueryContext(ctx, `
SELECT id, message_id, type, url, password, created_at
FROM telegram_links
WHERE message_id IN (`+strings.Join(placeholders, ",")+`)
ORDER BY message_id, id`, ids...)
	if err != nil {
		return nil, fmt.Errorf("load links: %w", err)
	}
	defer rows.Close()

	byMessageID := map[int64][]model.Link{}
	for rows.Next() {
		var link model.Link
		if err := rows.Scan(&link.ID, &link.MessageID, &link.Type, &link.URL, &link.Password, &link.CreatedAt); err != nil {
			return nil, err
		}
		byMessageID[link.MessageID] = append(byMessageID[link.MessageID], link)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range items {
		items[i].Links = byMessageID[items[i].ID]
	}
	return items, nil
}
```

Keep `loadLinks` for any direct single-message use.

- [ ] **Step 4: Run tests**

Run:

```bash
gofmt -w internal/repository/message.go internal/repository/repository_test.go
GOCACHE=/tmp/go-build-cache go test ./internal/repository ./internal/search ./internal/api
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/repository/message.go internal/repository/repository_test.go
git commit -m "perf: batch load result links"
```

## Task 5: SQLite Maintenance Endpoint

**Files:**
- Create: `internal/repository/maintenance.go`
- Create: `internal/repository/maintenance_test.go`
- Modify: `internal/api/router.go`
- Modify: `internal/api/handlers.go`
- Modify: `internal/api/handlers_test.go`
- Modify: `cmd/tg-provider/main.go`

- [ ] **Step 1: Write failing maintenance repository test**

Create `internal/repository/maintenance_test.go`:

```go
package repository

import (
	"context"
	"path/filepath"
	"testing"

	"tg-provider/internal/db"
)

func TestMaintenanceRepositoryOptimizeSQLite(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "telegram.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	ops, err := NewMaintenanceRepository(conn).OptimizeSQLite(ctx)
	if err != nil {
		t.Fatalf("OptimizeSQLite returned error: %v", err)
	}
	want := []string{"ANALYZE", "PRAGMA optimize", "telegram_messages_fts optimize"}
	if len(ops) != len(want) {
		t.Fatalf("ops = %+v, want %+v", ops, want)
	}
	for i := range want {
		if ops[i] != want[i] {
			t.Fatalf("ops = %+v, want %+v", ops, want)
		}
	}
}
```

- [ ] **Step 2: Write failing API maintenance test**

Add to `internal/api/handlers_test.go`:

```go
func TestMaintenanceSQLiteAPI(t *testing.T) {
	router := NewRouter(testDeps(t))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/maintenance/sqlite", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("ANALYZE")) || !bytes.Contains(w.Body.Bytes(), []byte("telegram_messages_fts optimize")) {
		t.Fatalf("maintenance response = %s", w.Body.String())
	}
}
```

- [ ] **Step 3: Run tests to verify failure**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/repository ./internal/api
```

Expected: FAIL because `NewMaintenanceRepository`, API dependency, handler, and route do not exist.

- [ ] **Step 4: Implement maintenance repository**

Create `internal/repository/maintenance.go`:

```go
package repository

import (
	"context"
	"database/sql"
	"fmt"
)

type MaintenanceRepository struct {
	db *sql.DB
}

func NewMaintenanceRepository(db *sql.DB) *MaintenanceRepository {
	return &MaintenanceRepository{db: db}
}

func (r *MaintenanceRepository) OptimizeSQLite(ctx context.Context) ([]string, error) {
	ops := []string{}
	if _, err := r.db.ExecContext(ctx, `ANALYZE`); err != nil {
		return nil, fmt.Errorf("analyze sqlite: %w", err)
	}
	ops = append(ops, "ANALYZE")
	if _, err := r.db.ExecContext(ctx, `PRAGMA optimize`); err != nil {
		return nil, fmt.Errorf("pragma optimize: %w", err)
	}
	ops = append(ops, "PRAGMA optimize")
	if _, err := r.db.ExecContext(ctx, `INSERT INTO telegram_messages_fts(telegram_messages_fts) VALUES ('optimize')`); err != nil {
		return nil, fmt.Errorf("optimize fts: %w", err)
	}
	ops = append(ops, "telegram_messages_fts optimize")
	return ops, nil
}
```

- [ ] **Step 5: Wire API dependency and route**

Modify `internal/api/router.go`:

```go
type Dependencies struct {
	Accounts    *repository.AccountRepository
	Channels    *repository.ChannelRepository
	Messages    *repository.MessageRepository
	Links       *repository.LinkRepository
	Maintenance *repository.MaintenanceRepository
	Status      *repository.StatusRepository
	Search      *search.Service
	History     *history.Service
	ChannelSync *channel.Service
	Telegram    telegram.Client
	Sessions    *session.Manager
	CodeStore   *telegram.CodeStore
}
```

Add route:

```go
api.POST("/maintenance/sqlite", h.maintenanceSQLite)
```

- [ ] **Step 6: Add API handler**

Add to `internal/api/handlers.go`:

```go
func (h handlers) maintenanceSQLite(c *gin.Context) {
	if h.deps.Maintenance == nil {
		errorText(c, http.StatusServiceUnavailable, "maintenance repository is unavailable")
		return
	}
	ops, err := h.deps.Maintenance.OptimizeSQLite(c.Request.Context())
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"operations": ops})
}
```

- [ ] **Step 7: Wire tests and main**

In `internal/api/handlers_test.go`, create maintenance dependency in `testDeps`:

```go
maintenance := repository.NewMaintenanceRepository(conn)
```

Return it:

```go
Maintenance: maintenance,
```

In `cmd/tg-provider/main.go`, add:

```go
maintenance := repository.NewMaintenanceRepository(conn)
```

Pass into `api.Dependencies`:

```go
Maintenance: maintenance,
```

- [ ] **Step 8: Run tests**

Run:

```bash
gofmt -w internal/repository/maintenance.go internal/repository/maintenance_test.go internal/api/router.go internal/api/handlers.go internal/api/handlers_test.go cmd/tg-provider/main.go
GOCACHE=/tmp/go-build-cache go test ./internal/repository ./internal/api ./cmd/tg-provider
```

Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/repository/maintenance.go internal/repository/maintenance_test.go internal/api/router.go internal/api/handlers.go internal/api/handlers_test.go cmd/tg-provider/main.go
git commit -m "feat: add sqlite maintenance endpoint"
```

## Task 6: Benchmark Seed Path

**Files:**
- Create: `internal/repository/search_benchmark_test.go`

- [ ] **Step 1: Add benchmark seed test and benchmark**

Create `internal/repository/search_benchmark_test.go`:

```go
package repository

import (
	"context"
	"database/sql"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"tg-provider/internal/db"
	"tg-provider/internal/model"
)

func TestSearchBenchmarkSeedReturnsBoundedResults(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "telegram.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	messages := seedSearchBenchmarkData(t, ctx, conn, 250)
	results, err := messages.Search(ctx, SearchParams{Query: "target", Limit: 5})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 5 {
		t.Fatalf("results len = %d, want bounded limit 5", len(results))
	}
}

func BenchmarkMessageRepositorySearch(b *testing.B) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(b.TempDir(), "telegram.db"))
	if err != nil {
		b.Fatalf("Open returned error: %v", err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		b.Fatalf("Migrate returned error: %v", err)
	}
	messages := seedSearchBenchmarkData(b, ctx, conn, 5000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := messages.Search(ctx, SearchParams{Query: "target", Limit: 20})
		if err != nil {
			b.Fatalf("Search returned error: %v", err)
		}
		if len(results) == 0 {
			b.Fatal("Search returned no results")
		}
	}
}

func seedSearchBenchmarkData(tb testing.TB, ctx context.Context, conn *sql.DB, count int) *MessageRepository {
	tb.Helper()
	accounts := NewAccountRepository(conn)
	channels := NewChannelRepository(conn)
	messages := NewMessageRepository(conn)
	accountID, err := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	if err != nil {
		tb.Fatalf("save account: %v", err)
	}
	channelID, err := channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 1, Title: "Bench", Type: model.ChannelTypeChannel})
	if err != nil {
		tb.Fatalf("save channel: %v", err)
	}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	batch := make([]model.Message, 0, 200)
	for i := 0; i < count; i++ {
		text := "ordinary message " + strconv.Itoa(i)
		if i%5 == 0 {
			text = "target resource message " + strconv.Itoa(i)
		}
		batch = append(batch, model.Message{
			AccountID: accountID, ChannelID: channelID, TelegramMessageID: int64(i + 1),
			Text: text, RawJSON: "{}", Date: base.Add(time.Duration(i) * time.Second),
		})
		if len(batch) == cap(batch) {
			if _, err := messages.SaveBatch(ctx, batch); err != nil {
				tb.Fatalf("save batch: %v", err)
			}
			batch = batch[:0]
		}
	}
	if len(batch) > 0 {
		if _, err := messages.SaveBatch(ctx, batch); err != nil {
			tb.Fatalf("save final batch: %v", err)
		}
	}
	return messages
}
```

- [ ] **Step 2: Run test and benchmark compile**

Run:

```bash
gofmt -w internal/repository/search_benchmark_test.go
GOCACHE=/tmp/go-build-cache go test ./internal/repository
GOCACHE=/tmp/go-build-cache go test ./internal/repository -run '^$' -bench BenchmarkMessageRepositorySearch -benchtime=1x
```

Expected: PASS. Benchmark should run one iteration and report timing.

- [ ] **Step 3: Commit**

```bash
git add internal/repository/search_benchmark_test.go
git commit -m "test: add search benchmark seed path"
```

## Task 7: Final Verification

**Files:**
- Verify all modified files and commits.

- [ ] **Step 1: Run full tests**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./...
```

Expected: PASS.

- [ ] **Step 2: Run full build**

Run:

```bash
GOCACHE=/tmp/go-build-cache go build ./...
```

Expected: PASS.

- [ ] **Step 3: Run benchmark smoke check**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/repository -run '^$' -bench BenchmarkMessageRepositorySearch -benchtime=1x
```

Expected: PASS with benchmark timing output.

- [ ] **Step 4: Check git status**

Run:

```bash
git status --short --branch
```

Expected: on `phase-5-search-performance` with no uncommitted changes.

- [ ] **Step 5: Review recent commits**

Run:

```bash
git log --oneline --decorate -10
```

Expected: shows Phase 5a design, plan, and implementation commits.

## Self-Review

Spec coverage:

- Search date range filters are covered by Task 1.
- Invalid query parameters are covered by Task 2.
- Cursor pagination and performance indexes are covered by Task 3.
- Batched link loading is covered by Task 4.
- SQLite maintenance is covered by Task 5.
- Benchmark seed support is covered by Task 6.
- Full verification is covered by Task 7.

Non-goals are preserved:

- No worker pool.
- No retry queue.
- No FloodWait handling.
- No `VACUUM` endpoint.
- No million-row CI test.
