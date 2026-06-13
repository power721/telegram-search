# Resource Read Model Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a denormalized SQLite `resource_index` read model so admin `/api/resources` and public `/api/search` return common first-page resource results quickly while preserving public API response shape.

**Architecture:** Keep normalized Telegram tables as source of truth. Add `resource_index` plus `resource_index_fts`, a repository that can rebuild and query indexed resources, then route `resource.Service.List` and public external search through that repository. Update history/update write paths to refresh affected message resources after commits.

**Tech Stack:** Go, SQLite, FTS5, standard `testing`, existing repository/resource/api packages.

---

## File Structure

- Modify `internal/db/migrations.go`: add migration 14 for `resource_index`, `resource_index_fts`, triggers, and indexes.
- Modify `internal/db/db_test.go`: assert read model tables and indexes exist.
- Create `internal/model/resource_index.go`: add `ResourceIndexItem`, `ResourceIndexQuery`, `ResourceIndexListResult`, and `ResourceIndexStats`.
- Create `internal/repository/resource_index.go`: add `ResourceIndexRepository` with rebuild, refresh, delete, query, count, grouped, and stats methods.
- Create `internal/repository/resource_index_test.go`: repository-level tests for rebuild, dedupe, FTS, filters, deletes, and query plans.
- Modify `internal/resource/service.go`: accept optional `ResourceIndexRepository`, query read model when available, keep legacy fallback.
- Modify `internal/resource/service_test.go`: add parity tests between legacy and indexed resource listing.
- Modify `cmd/tg-search/main.go`: instantiate `ResourceIndexRepository` and pass it to `resource.NewService`.
- Modify `internal/api/handlers_test.go`: add/adjust `/api/resources` and public `/api/search` contract tests against the index.
- Modify `internal/api/external_search.go`: collapse multi-filter public search into a single resource index query when index exists.
- Modify `internal/history/service.go`: refresh indexed resources for stored history message IDs after successful transaction.
- Modify `internal/update/processor.go`: refresh or remove indexed resources after real-time message writes/deletes.
- Modify affected tests in `internal/history` and `internal/update` to include the new repository where required.

---

## Task 1: Schema And Model Types

**Files:**
- Modify: `internal/db/migrations.go`
- Modify: `internal/db/db_test.go`
- Create: `internal/model/resource_index.go`

- [ ] **Step 1: Write failing migration test**

Update `TestMigrationsAreIdempotentAndCreateFTS` in `internal/db/db_test.go` to assert the new tables:

```go
assertTableExists(t, conn, "resource_index")
assertTableExists(t, conn, "resource_index_fts")
```

Update `TestPerformanceIndexesExist` to include:

```go
"idx_resource_index_category_datetime",
"idx_resource_index_type_datetime",
"idx_resource_index_datetime",
"idx_resource_index_channel_datetime",
"idx_resource_index_account_datetime",
"idx_resource_index_score_datetime",
"idx_resource_index_kind_datetime",
"idx_resource_index_source_message",
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/db
```

Expected: FAIL because `resource_index` and its indexes do not exist.

- [ ] **Step 3: Add migration 14**

Append this migration to `internal/db/migrations.go` after version 13:

```go
{
    version: 14,
    name:    "resource_index_read_model",
    sql: `
CREATE TABLE IF NOT EXISTS resource_index (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  resource_id TEXT NOT NULL UNIQUE,
  kind TEXT NOT NULL,
  source_key TEXT NOT NULL UNIQUE,
  source_message_id INTEGER NOT NULL DEFAULT 0,
  url TEXT NOT NULL DEFAULT '',
  type TEXT NOT NULL DEFAULT '',
  category TEXT NOT NULL,
  password TEXT NOT NULL DEFAULT '',
  note TEXT NOT NULL DEFAULT '',
  title TEXT NOT NULL DEFAULT '',
  source_snippet TEXT NOT NULL DEFAULT '',
  telegram_file_id INTEGER NOT NULL DEFAULT 0,
  file_name TEXT NOT NULL DEFAULT '',
  extension TEXT NOT NULL DEFAULT '',
  mime_type TEXT NOT NULL DEFAULT '',
  size_bytes INTEGER NOT NULL DEFAULT 0,
  media_title TEXT NOT NULL DEFAULT '',
  media_year TEXT NOT NULL DEFAULT '',
  media_season TEXT NOT NULL DEFAULT '',
  media_episode TEXT NOT NULL DEFAULT '',
  media_quality TEXT NOT NULL DEFAULT '',
  media_size TEXT NOT NULL DEFAULT '',
  media_tmdb_id TEXT NOT NULL DEFAULT '',
  media_category TEXT NOT NULL DEFAULT '',
  media_tags TEXT NOT NULL DEFAULT '',
  media_summary TEXT NOT NULL DEFAULT '',
  datetime DATETIME NOT NULL,
  account_id INTEGER NOT NULL,
  channel_id INTEGER NOT NULL,
  telegram_channel_id INTEGER NOT NULL,
  channel_title TEXT NOT NULL DEFAULT '',
  channel_username TEXT NOT NULL DEFAULT '',
  telegram_message_id INTEGER NOT NULL,
  message_type TEXT NOT NULL DEFAULT '',
  source_channel_count INTEGER NOT NULL DEFAULT 1,
  message_count INTEGER NOT NULL DEFAULT 1,
  provider_count INTEGER NOT NULL DEFAULT 1,
  score INTEGER NOT NULL DEFAULT 0,
  updated_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_resource_index_category_datetime ON resource_index(category, datetime DESC, resource_id DESC);
CREATE INDEX IF NOT EXISTS idx_resource_index_type_datetime ON resource_index(type, datetime DESC, resource_id DESC);
CREATE INDEX IF NOT EXISTS idx_resource_index_datetime ON resource_index(datetime DESC, resource_id DESC);
CREATE INDEX IF NOT EXISTS idx_resource_index_channel_datetime ON resource_index(channel_id, datetime DESC, resource_id DESC);
CREATE INDEX IF NOT EXISTS idx_resource_index_account_datetime ON resource_index(account_id, datetime DESC, resource_id DESC);
CREATE INDEX IF NOT EXISTS idx_resource_index_score_datetime ON resource_index(score DESC, datetime DESC, resource_id DESC);
CREATE INDEX IF NOT EXISTS idx_resource_index_kind_datetime ON resource_index(kind, datetime DESC, resource_id DESC);
CREATE INDEX IF NOT EXISTS idx_resource_index_source_message ON resource_index(source_message_id);

CREATE VIRTUAL TABLE IF NOT EXISTS resource_index_fts
USING fts5(title, note, source_snippet, url, type, category, media_title, media_tags, file_name, content='resource_index', content_rowid='id');

CREATE TRIGGER IF NOT EXISTS resource_index_ai AFTER INSERT ON resource_index BEGIN
  INSERT INTO resource_index_fts(rowid, title, note, source_snippet, url, type, category, media_title, media_tags, file_name)
  VALUES (new.id, new.title, new.note, new.source_snippet, new.url, new.type, new.category, new.media_title, new.media_tags, new.file_name);
END;

CREATE TRIGGER IF NOT EXISTS resource_index_ad AFTER DELETE ON resource_index BEGIN
  INSERT INTO resource_index_fts(resource_index_fts, rowid, title, note, source_snippet, url, type, category, media_title, media_tags, file_name)
  VALUES ('delete', old.id, old.title, old.note, old.source_snippet, old.url, old.type, old.category, old.media_title, old.media_tags, old.file_name);
END;

CREATE TRIGGER IF NOT EXISTS resource_index_au AFTER UPDATE ON resource_index BEGIN
  INSERT INTO resource_index_fts(resource_index_fts, rowid, title, note, source_snippet, url, type, category, media_title, media_tags, file_name)
  VALUES ('delete', old.id, old.title, old.note, old.source_snippet, old.url, old.type, old.category, old.media_title, old.media_tags, old.file_name);
  INSERT INTO resource_index_fts(rowid, title, note, source_snippet, url, type, category, media_title, media_tags, file_name)
  VALUES (new.id, new.title, new.note, new.source_snippet, new.url, new.type, new.category, new.media_title, new.media_tags, new.file_name);
END;
`,
},
```

Run:

```bash
gofmt -w internal/db/migrations.go internal/db/db_test.go
```

- [ ] **Step 4: Add model types**

Create `internal/model/resource_index.go`:

```go
package model

import "time"

type ResourceIndexItem struct {
	ID                 int64
	ResourceID         string
	Kind               string
	SourceKey          string
	SourceMessageID    int64
	URL                string
	Type               string
	Category           string
	Password           string
	Note               string
	Title              string
	SourceSnippet      string
	TelegramFileID     int64
	FileName           string
	Extension          string
	MimeType           string
	SizeBytes          int64
	MediaTitle         string
	MediaYear          string
	MediaSeason        string
	MediaEpisode       string
	MediaQuality       string
	MediaSize          string
	MediaTMDBID        string
	MediaCategory      string
	MediaTags          string
	MediaSummary       string
	Datetime           time.Time
	AccountID          int64
	ChannelID          int64
	TelegramChannelID  int64
	ChannelTitle       string
	ChannelUsername    string
	TelegramMessageID  int64
	MessageType        string
	SourceChannelCount int
	MessageCount       int
	ProviderCount      int
	Score              int
	UpdatedAt          time.Time
}

type ResourceIndexQuery struct {
	Keyword    string
	Type       string
	Types      []string
	Category   string
	Categories []string
	AccountID  int64
	ChannelID  int64
	Extension  string
	Sort       string
	DateFrom   *time.Time
	DateTo     *time.Time
	Limit      int
	Offset     int
	MaxLimit   int
}

type ResourceIndexListResult struct {
	Items   []ResourceIndexItem
	Total   int
	Grouped map[string]int
}

type ResourceIndexStats struct {
	IndexedRows int
	UpdatedAt   time.Time
}
```

- [ ] **Step 5: Run test to verify it passes**

Run:

```bash
go test ./internal/db ./internal/model
```

Expected: PASS.

- [ ] **Step 6: Commit**

Run:

```bash
git add internal/db/migrations.go internal/db/db_test.go internal/model/resource_index.go
git commit -m "feat: add resource index schema"
```

---

## Task 2: Resource Index Repository Rebuild And Query

**Files:**
- Create: `internal/repository/resource_index.go`
- Create: `internal/repository/resource_index_test.go`

- [ ] **Step 1: Write failing repository test for rebuild parity**

Create `internal/repository/resource_index_test.go` with this first test:

```go
package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"tg-search/internal/db"
	"tg-search/internal/model"
)

func TestResourceIndexRebuildDeduplicatesLinksAndExcludesImages(t *testing.T) {
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
	files := NewFileRepository(conn)
	index := NewResourceIndexRepository(conn)

	accountID, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 100, Title: "VIP", Username: "vip", Type: model.ChannelTypeChannel})
	oldDate := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	newDate := time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC)
	stored, err := messages.SaveBatch(ctx, []model.Message{
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1, Text: "old ubuntu", RawJSON: "{}", Date: oldDate},
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 2, Text: "new ubuntu", RawJSON: "{}", Date: newDate},
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 3, Text: "image only", RawJSON: "{}", Date: newDate.Add(time.Minute)},
	})
	if err != nil {
		t.Fatalf("save messages: %v", err)
	}
	for _, msg := range stored[:2] {
		if _, err := links.SaveBatch(ctx, msg.ID, []model.Link{{
			Type: "quark", Category: "cloud_drive", URL: "https://pan.quark.cn/s/same", Note: "Ubuntu 24.04", MediaTitle: "Ubuntu",
		}}); err != nil {
			t.Fatalf("save link: %v", err)
		}
	}
	if _, err := files.SaveBatch(ctx, stored[2].ID, []model.File{{TelegramFileID: 33, FileName: "poster.jpg", Extension: ".jpg", MimeType: "image/jpeg", Category: "image"}}); err != nil {
		t.Fatalf("save image: %v", err)
	}

	if err := index.Rebuild(ctx); err != nil {
		t.Fatalf("Rebuild returned error: %v", err)
	}
	result, err := index.List(ctx, model.ResourceIndexQuery{Limit: 10})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if result.Total != 1 || len(result.Items) != 1 {
		t.Fatalf("result total=%d items=%+v, want one deduped link", result.Total, result.Items)
	}
	item := result.Items[0]
	if item.URL != "https://pan.quark.cn/s/same" || item.SourceMessageID != stored[1].ID {
		t.Fatalf("indexed item = %+v, want newest source message", item)
	}
	if item.MessageCount != 2 || item.SourceChannelCount != 1 || item.ProviderCount != 1 {
		t.Fatalf("stats = channels:%d messages:%d providers:%d, want 1/2/1", item.SourceChannelCount, item.MessageCount, item.ProviderCount)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/repository -run TestResourceIndexRebuildDeduplicatesLinksAndExcludesImages -v
```

Expected: FAIL because `NewResourceIndexRepository` is undefined.

- [ ] **Step 3: Implement repository skeleton and rebuild**

Create `internal/repository/resource_index.go` with:

```go
package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"tg-search/internal/model"
)

type ResourceIndexRepository struct {
	db *sql.DB
}

func NewResourceIndexRepository(db *sql.DB) *ResourceIndexRepository {
	return &ResourceIndexRepository{db: db}
}

func (r *ResourceIndexRepository) Rebuild(ctx context.Context) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM resource_index`); err != nil {
		return fmt.Errorf("clear resource index: %w", err)
	}
	if err := r.rebuildLinks(ctx); err != nil {
		return err
	}
	if err := r.rebuildFiles(ctx); err != nil {
		return err
	}
	return nil
}

func (r *ResourceIndexRepository) rebuildLinks(ctx context.Context) error {
	rows, err := r.db.QueryContext(ctx, `
WITH ranked AS (
  SELECT
    l.id AS link_id, l.message_id, l.type, l.url, COALESCE(l.password, '') AS password,
    COALESCE(l.note, '') AS note, COALESCE(l.source_snippet, '') AS source_snippet,
    CASE
      WHEN COALESCE(l.category, '') <> '' THEN l.category
      WHEN l.type = 'magnet' THEN 'magnet'
      WHEN l.type = 'ed2k' THEN 'ed2k'
      WHEN l.type = 'url' THEN 'http'
      ELSE 'cloud_drive'
    END AS category,
    COALESCE(l.media_title, '') AS media_title, COALESCE(l.media_year, '') AS media_year,
    COALESCE(l.media_season, '') AS media_season, COALESCE(l.media_episode, '') AS media_episode,
    COALESCE(l.media_quality, '') AS media_quality, COALESCE(l.media_size, '') AS media_size,
    COALESCE(l.media_tmdb_id, '') AS media_tmdb_id, COALESCE(l.media_category, '') AS media_category,
    COALESCE(l.media_tags, '') AS media_tags,
    m.date, m.message_type, m.media_summary, m.account_id, m.channel_id, c.telegram_channel_id,
    c.title AS channel_title, c.username AS channel_username, m.telegram_message_id,
    row_number() OVER (PARTITION BY l.url ORDER BY m.date DESC, l.id DESC) AS rn
  FROM telegram_links l
  JOIN telegram_messages m ON m.id = l.message_id
  JOIN telegram_channels c ON c.id = m.channel_id
  WHERE m.deleted = 0
),
stats AS (
  SELECT
    l.url,
    count(DISTINCT m.channel_id) AS source_channel_count,
    count(DISTINCT m.id) AS message_count,
    count(DISTINCT COALESCE(NULLIF(l.type, ''), 'url')) AS provider_count
  FROM telegram_links l
  JOIN telegram_messages m ON m.id = l.message_id
  WHERE m.deleted = 0
  GROUP BY l.url
)
SELECT link_id, message_id, type, url, password, note, source_snippet, category,
       media_title, media_year, media_season, media_episode, media_quality, media_size, media_tmdb_id, media_category, media_tags,
       date, message_type, media_summary, account_id, channel_id, telegram_channel_id, channel_title, channel_username, telegram_message_id,
       stats.source_channel_count, stats.message_count, stats.provider_count
FROM ranked
JOIN stats ON stats.url = ranked.url
WHERE rn = 1`)
	if err != nil {
		return fmt.Errorf("query resource index links: %w", err)
	}
	defer rows.Close()
	now := time.Now().UTC()
	for rows.Next() {
		var item model.ResourceIndexItem
		var linkID int64
		if err := rows.Scan(
			&linkID, &item.SourceMessageID, &item.Type, &item.URL, &item.Password, &item.Note, &item.SourceSnippet, &item.Category,
			&item.MediaTitle, &item.MediaYear, &item.MediaSeason, &item.MediaEpisode, &item.MediaQuality, &item.MediaSize, &item.MediaTMDBID, &item.MediaCategory, &item.MediaTags,
			&item.Datetime, &item.MessageType, &item.MediaSummary, &item.AccountID, &item.ChannelID, &item.TelegramChannelID, &item.ChannelTitle, &item.ChannelUsername, &item.TelegramMessageID,
			&item.SourceChannelCount, &item.MessageCount, &item.ProviderCount,
		); err != nil {
			return err
		}
		item.ResourceID = "link:" + item.URL
		item.Kind = "link"
		item.SourceKey = item.ResourceID
		item.Title = firstNonEmptyString(item.MediaTitle, item.Note, item.URL)
		item.Score = resourceIndexScore(item, now)
		item.UpdatedAt = now
		if err := r.upsert(ctx, item); err != nil {
			return err
		}
		_ = linkID
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return nil
}

func (r *ResourceIndexRepository) rebuildFiles(ctx context.Context) error {
	rows, err := r.db.QueryContext(ctx, `
SELECT f.id, f.message_id, f.telegram_file_id, f.file_name, f.extension, f.mime_type, f.size_bytes, f.category,
       m.date, m.message_type, m.media_summary, m.account_id, m.channel_id, c.telegram_channel_id,
       c.title, c.username, m.telegram_message_id
FROM telegram_files f
JOIN telegram_messages m ON m.id = f.message_id
JOIN telegram_channels c ON c.id = m.channel_id
WHERE m.deleted = 0 AND f.category <> 'image'`)
	if err != nil {
		return fmt.Errorf("query resource index files: %w", err)
	}
	defer rows.Close()
	now := time.Now().UTC()
	for rows.Next() {
		var item model.ResourceIndexItem
		var fileID int64
		if err := rows.Scan(
			&fileID, &item.SourceMessageID, &item.TelegramFileID, &item.FileName, &item.Extension, &item.MimeType, &item.SizeBytes, &item.Type,
			&item.Datetime, &item.MessageType, &item.MediaSummary, &item.AccountID, &item.ChannelID, &item.TelegramChannelID,
			&item.ChannelTitle, &item.ChannelUsername, &item.TelegramMessageID,
		); err != nil {
			return err
		}
		item.ResourceID = fmt.Sprintf("file:%d", fileID)
		item.Kind = "file"
		item.SourceKey = item.ResourceID
		item.Category = "files"
		item.Title = item.FileName
		item.SourceChannelCount = 1
		item.MessageCount = 1
		item.ProviderCount = 1
		item.Score = resourceIndexScore(item, now)
		item.UpdatedAt = now
		if err := r.upsert(ctx, item); err != nil {
			return err
		}
	}
	return rows.Err()
}
```

- [ ] **Step 4: Implement upsert, scan, list, count helpers**

Add these methods to `internal/repository/resource_index.go`:

```go
func (r *ResourceIndexRepository) upsert(ctx context.Context, item model.ResourceIndexItem) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO resource_index (
  resource_id, kind, source_key, source_message_id, url, type, category, password, note, title, source_snippet,
  telegram_file_id, file_name, extension, mime_type, size_bytes,
  media_title, media_year, media_season, media_episode, media_quality, media_size, media_tmdb_id, media_category, media_tags, media_summary,
  datetime, account_id, channel_id, telegram_channel_id, channel_title, channel_username, telegram_message_id, message_type,
  source_channel_count, message_count, provider_count, score, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(resource_id) DO UPDATE SET
  kind = excluded.kind,
  source_key = excluded.source_key,
  source_message_id = excluded.source_message_id,
  url = excluded.url,
  type = excluded.type,
  category = excluded.category,
  password = excluded.password,
  note = excluded.note,
  title = excluded.title,
  source_snippet = excluded.source_snippet,
  telegram_file_id = excluded.telegram_file_id,
  file_name = excluded.file_name,
  extension = excluded.extension,
  mime_type = excluded.mime_type,
  size_bytes = excluded.size_bytes,
  media_title = excluded.media_title,
  media_year = excluded.media_year,
  media_season = excluded.media_season,
  media_episode = excluded.media_episode,
  media_quality = excluded.media_quality,
  media_size = excluded.media_size,
  media_tmdb_id = excluded.media_tmdb_id,
  media_category = excluded.media_category,
  media_tags = excluded.media_tags,
  media_summary = excluded.media_summary,
  datetime = excluded.datetime,
  account_id = excluded.account_id,
  channel_id = excluded.channel_id,
  telegram_channel_id = excluded.telegram_channel_id,
  channel_title = excluded.channel_title,
  channel_username = excluded.channel_username,
  telegram_message_id = excluded.telegram_message_id,
  message_type = excluded.message_type,
  source_channel_count = excluded.source_channel_count,
  message_count = excluded.message_count,
  provider_count = excluded.provider_count,
  score = excluded.score,
  updated_at = excluded.updated_at`,
		item.ResourceID, item.Kind, item.SourceKey, item.SourceMessageID, item.URL, item.Type, item.Category, item.Password, item.Note, item.Title, item.SourceSnippet,
		item.TelegramFileID, item.FileName, item.Extension, item.MimeType, item.SizeBytes,
		item.MediaTitle, item.MediaYear, item.MediaSeason, item.MediaEpisode, item.MediaQuality, item.MediaSize, item.MediaTMDBID, item.MediaCategory, item.MediaTags, item.MediaSummary,
		item.Datetime, item.AccountID, item.ChannelID, item.TelegramChannelID, item.ChannelTitle, item.ChannelUsername, item.TelegramMessageID, item.MessageType,
		positiveInt(item.SourceChannelCount), positiveInt(item.MessageCount), positiveInt(item.ProviderCount), item.Score, item.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert resource index %s: %w", item.ResourceID, err)
	}
	return nil
}

func (r *ResourceIndexRepository) List(ctx context.Context, query model.ResourceIndexQuery) (model.ResourceIndexListResult, error) {
	limit := clampLimit(query.Limit, 50)
	if query.MaxLimit > 0 && limit > query.MaxLimit {
		limit = query.MaxLimit
	}
	offset := query.Offset
	if offset < 0 {
		offset = 0
	}
	where, args, joinFTS := resourceIndexWhere(query)
	total, err := r.count(ctx, where, args, joinFTS)
	if err != nil {
		return model.ResourceIndexListResult{}, err
	}
	grouped, err := r.grouped(ctx, where, args, joinFTS)
	if err != nil {
		return model.ResourceIndexListResult{}, err
	}
	items, err := r.list(ctx, where, args, joinFTS, resourceIndexOrder(query.Sort), limit, offset)
	if err != nil {
		return model.ResourceIndexListResult{}, err
	}
	return model.ResourceIndexListResult{Items: items, Total: total, Grouped: grouped}, nil
}
```

Also implement these exact helpers:

```go
func resourceIndexWhere(query model.ResourceIndexQuery) ([]string, []any, bool)
func (r *ResourceIndexRepository) count(ctx context.Context, where []string, args []any, joinFTS bool) (int, error)
func (r *ResourceIndexRepository) grouped(ctx context.Context, where []string, args []any, joinFTS bool) (map[string]int, error)
func (r *ResourceIndexRepository) list(ctx context.Context, where []string, args []any, joinFTS bool, orderBy string, limit int, offset int) ([]model.ResourceIndexItem, error)
func scanResourceIndexItem(row interface{ Scan(...any) error }) (model.ResourceIndexItem, error)
func resourceIndexOrder(sort string) string
func fts5ResourceQuery(query string) string
func firstNonEmptyString(values ...string) string
func positiveInt(value int) int
```

`resourceIndexWhere` must:

```go
func resourceIndexWhere(query model.ResourceIndexQuery) ([]string, []any, bool) {
	where := []string{"1 = 1"}
	args := []any{}
	joinFTS := strings.TrimSpace(query.Keyword) != ""
	if joinFTS {
		where = append(where, "resource_index_fts MATCH ?")
		args = append(args, fts5ResourceQuery(query.Keyword))
	}
	if query.Type != "" {
		where = append(where, "ri.type = ?")
		args = append(args, query.Type)
	}
	if len(query.Types) > 0 {
		markers := strings.TrimRight(strings.Repeat("?,", len(query.Types)), ",")
		where = append(where, "ri.type IN ("+markers+")")
		for _, value := range query.Types {
			args = append(args, value)
		}
	}
	if query.Category != "" {
		where = append(where, "ri.category = ?")
		args = append(args, query.Category)
	}
	if len(query.Categories) > 0 {
		markers := strings.TrimRight(strings.Repeat("?,", len(query.Categories)), ",")
		where = append(where, "ri.category IN ("+markers+")")
		for _, value := range query.Categories {
			args = append(args, value)
		}
	}
	if query.AccountID > 0 {
		where = append(where, "ri.account_id = ?")
		args = append(args, query.AccountID)
	}
	if query.ChannelID > 0 {
		where = append(where, "ri.channel_id = ?")
		args = append(args, query.ChannelID)
	}
	if query.Extension != "" {
		where = append(where, "ri.extension = ?")
		args = append(args, query.Extension)
	}
	if query.DateFrom != nil {
		where = append(where, "ri.datetime >= ?")
		args = append(args, *query.DateFrom)
	}
	if query.DateTo != nil {
		where = append(where, "ri.datetime < ?")
		args = append(args, *query.DateTo)
	}
	return where, args, joinFTS
}
```

Use SQL aliases consistently:

```sql
FROM resource_index ri
JOIN resource_index_fts fts ON fts.rowid = ri.id
```

Only include the FTS join when keyword is non-empty.

- [ ] **Step 5: Add FTS/filter/delete tests**

Add tests in `internal/repository/resource_index_test.go`:

```go
func TestResourceIndexListSearchesFTSAndFiltersProvider(t *testing.T) {
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
	index := NewResourceIndexRepository(conn)
	accountID, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 100, Title: "VIP", Type: model.ChannelTypeChannel})
	stored, err := messages.SaveBatch(ctx, []model.Message{
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1, Text: "ubuntu", RawJSON: "{}", Date: time.Now().UTC()},
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 2, Text: "debian", RawJSON: "{}", Date: time.Now().UTC().Add(-time.Minute)},
	})
	if err != nil {
		t.Fatalf("save messages: %v", err)
	}
	if _, err := links.SaveBatch(ctx, stored[0].ID, []model.Link{{Type: "quark", Category: "cloud_drive", URL: "https://pan.quark.cn/s/ubuntu", Note: "Ubuntu 24.04"}}); err != nil {
		t.Fatalf("save ubuntu link: %v", err)
	}
	if _, err := links.SaveBatch(ctx, stored[1].ID, []model.Link{{Type: "baidu", Category: "cloud_drive", URL: "https://pan.baidu.com/s/debian", Note: "Debian"}}); err != nil {
		t.Fatalf("save debian link: %v", err)
	}
	if err := index.Rebuild(ctx); err != nil {
		t.Fatalf("Rebuild returned error: %v", err)
	}
	result, err := index.List(ctx, model.ResourceIndexQuery{Keyword: "ubuntu", Type: "quark", Limit: 10})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if result.Total != 1 || len(result.Items) != 1 || result.Items[0].Type != "quark" {
		t.Fatalf("result = %+v, want one quark ubuntu result", result)
	}
}

func TestResourceIndexRefreshAfterSoftDeleteSelectsNextNewestLink(t *testing.T) {
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
	index := NewResourceIndexRepository(conn)
	accountID, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 100, Title: "VIP", Type: model.ChannelTypeChannel})
	oldDate := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	newDate := time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC)
	stored, err := messages.SaveBatch(ctx, []model.Message{
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1, Text: "same", RawJSON: "{}", Date: oldDate},
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 2, Text: "same", RawJSON: "{}", Date: newDate},
	})
	if err != nil {
		t.Fatalf("save messages: %v", err)
	}
	for _, msg := range stored {
		if _, err := links.SaveBatch(ctx, msg.ID, []model.Link{{Type: "quark", Category: "cloud_drive", URL: "https://pan.quark.cn/s/same", Note: "same resource"}}); err != nil {
			t.Fatalf("save link: %v", err)
		}
	}
	if err := index.Rebuild(ctx); err != nil {
		t.Fatalf("Rebuild returned error: %v", err)
	}
	if err := messages.MarkDeleted(ctx, channelID, 2); err != nil {
		t.Fatalf("MarkDeleted returned error: %v", err)
	}
	if err := index.RefreshMessage(ctx, stored[1].ID); err != nil {
		t.Fatalf("RefreshMessage returned error: %v", err)
	}
	result, err := index.List(ctx, model.ResourceIndexQuery{Keyword: "same", Limit: 10})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if result.Total != 1 || result.Items[0].SourceMessageID != stored[0].ID {
		t.Fatalf("result = %+v, want old source after new source deleted", result)
	}
}
```

Use real repositories and real data, not mocks.

- [ ] **Step 6: Implement refresh/delete/stats methods**

Add methods:

```go
func (r *ResourceIndexRepository) RefreshMessage(ctx context.Context, messageID int64) error
func (r *ResourceIndexRepository) RefreshMessages(ctx context.Context, messageIDs []int64) error
func (r *ResourceIndexRepository) DeleteMessage(ctx context.Context, messageID int64) error
func (r *ResourceIndexRepository) Stats(ctx context.Context) (model.ResourceIndexStats, error)
```

Minimal correct implementation is allowed for first pass:

```go
func (r *ResourceIndexRepository) RefreshMessage(ctx context.Context, messageID int64) error {
	return r.Rebuild(ctx)
}

func (r *ResourceIndexRepository) RefreshMessages(ctx context.Context, messageIDs []int64) error {
	if len(messageIDs) == 0 {
		return nil
	}
	return r.Rebuild(ctx)
}

func (r *ResourceIndexRepository) DeleteMessage(ctx context.Context, messageID int64) error {
	return r.Rebuild(ctx)
}
```

Do not leave these methods as full `Rebuild` calls when moving past Task 2. Implement the optimized affected-message refresh in this task:

1. collect URLs currently indexed by `source_message_id = ?`;
2. collect URLs from `telegram_links WHERE message_id = ?`;
3. delete file rows with `source_message_id = ?`;
4. refresh each affected URL by selecting newest source and stats;
5. insert visible files for the message if not deleted and not image.

- [ ] **Step 7: Run repository tests**

Run:

```bash
go test ./internal/repository -run ResourceIndex -v
```

Expected: PASS.

- [ ] **Step 8: Commit**

Run:

```bash
git add internal/repository/resource_index.go internal/repository/resource_index_test.go
git commit -m "feat: add resource index repository"
```

---

## Task 3: Resource Service Uses The Index

**Files:**
- Modify: `internal/resource/service.go`
- Modify: `internal/resource/service_test.go`

- [ ] **Step 1: Write failing resource service index test**

Add to `internal/resource/service_test.go`:

```go
func TestResourceServiceUsesIndexWhenAvailable(t *testing.T) {
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
	files := repository.NewFileRepository(conn)
	index := repository.NewResourceIndexRepository(conn)
	accountID, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 1, Title: "VIP", Type: model.ChannelTypeChannel})
	stored, err := messages.SaveBatch(ctx, []model.Message{{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1, Text: "ubuntu", RawJSON: "{}", Date: time.Now().UTC()}})
	if err != nil {
		t.Fatalf("save message: %v", err)
	}
	if _, err := links.SaveBatch(ctx, stored[0].ID, []model.Link{{Type: "quark", Category: "cloud_drive", URL: "https://pan.quark.cn/s/indexed", Note: "Ubuntu"}}); err != nil {
		t.Fatalf("save link: %v", err)
	}
	if err := index.Rebuild(ctx); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	service := NewService(links, files, nil, index)
	result, err := service.List(ctx, Query{Keyword: "ubuntu", Limit: 10})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if result.Total != 1 || len(result.Items) != 1 || result.Items[0].URL != "https://pan.quark.cn/s/indexed" {
		t.Fatalf("result = %+v, want indexed resource", result)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/resource -run TestResourceServiceUsesIndexWhenAvailable -v
```

Expected: FAIL because `NewService` does not accept the index or `List` ignores it.

- [ ] **Step 3: Extend Service and constructor**

Modify `internal/resource/service.go`:

```go
type Service struct {
	links *repository.LinkRepository
	files *repository.FileRepository
	stats *repository.ResourceStatsRepository
	index *repository.ResourceIndexRepository
}

func NewService(links *repository.LinkRepository, files *repository.FileRepository, extras ...any) *Service {
	service := &Service{links: links, files: files}
	for _, extra := range extras {
		switch repo := extra.(type) {
		case *repository.ResourceStatsRepository:
			service.stats = repo
		case *repository.ResourceIndexRepository:
			service.index = repo
		}
	}
	return service
}
```

Run `rg -n "NewService\\(" internal cmd` and update any compile failures caused by the constructor type change. Calls that pass `nil` as an extra argument may stay unchanged because `nil` extras are ignored by the type switch.

- [ ] **Step 4: Add index conversion helpers**

Add helpers in `internal/resource/service.go`:

```go
func (s *Service) indexedList(ctx context.Context, query Query) (ListResult, bool, error) {
	if s.index == nil {
		return ListResult{}, false, nil
	}
	result, err := s.index.List(ctx, model.ResourceIndexQuery{
		Keyword:   query.Keyword,
		Type:      query.Type,
		Category:  query.Category,
		AccountID: query.AccountID,
		ChannelID: query.ChannelID,
		Extension: query.Extension,
		Sort:      query.Sort,
		DateFrom:  query.DateFrom,
		DateTo:    query.DateTo,
		Limit:     query.Limit,
		Offset:    query.Offset,
		MaxLimit:  query.MaxLimit,
	})
	if err != nil {
		return ListResult{}, true, err
	}
	items := make([]Item, 0, len(result.Items))
	for _, indexed := range result.Items {
		items = append(items, itemFromIndex(indexed))
	}
	return ListResult{Items: items, Total: result.Total, Grouped: normalizeGrouped(result.Grouped)}, true, nil
}
```

Import `tg-search/internal/model`.

Implement:

```go
func itemFromIndex(indexed model.ResourceIndexItem) Item
```

Set all fields matching `resource.Item`, including `ScoreExplain` with stored aggregate fields and score components recomputed with `itemScore`.

- [ ] **Step 5: Call index first in List**

At the start of `func (s *Service) List` after limit normalization is okay, call:

```go
if result, ok, err := s.indexedList(ctx, query); ok || err != nil {
	return result, err
}
```

If the index exists but returns an error, do not silently fallback.

- [ ] **Step 6: Update refresh/delete methods**

Modify:

```go
func (s *Service) RefreshGlobalGrouped(ctx context.Context) error
```

If `s.index != nil`, compute grouped counts from `s.index.List(ctx, model.ResourceIndexQuery{Limit: 1, MaxLimit: 1})` and save those counts to `resource_group_counts` when `s.stats != nil`. Keep the existing legacy grouped computation only for the `s.index == nil` path.

Modify `Delete` and `DeleteMany` so after deleting source links/files they call:

```go
if s.index != nil {
	if err := s.index.Rebuild(ctx); err != nil {
		return err
	}
}
```

Then keep `RefreshGlobalGrouped`.

- [ ] **Step 7: Run resource tests**

Run:

```bash
go test ./internal/resource
```

Expected: PASS.

- [ ] **Step 8: Commit**

Run:

```bash
git add internal/resource/service.go internal/resource/service_test.go
git commit -m "feat: query resources from read model"
```

---

## Task 4: Wire Index Into App And Write Paths

**Files:**
- Modify: `cmd/tg-search/main.go`
- Modify: `internal/history/service.go`
- Modify: `internal/update/processor.go`
- Modify: `internal/history/service_test.go`
- Modify: `internal/update/processor_test.go`

- [ ] **Step 1: Write failing history/update tests**

In `internal/update/processor_test.go`, add a test that processes a new message event, then queries `resources.List` and sees the new link without manually calling rebuild.

Use the existing processor test fixture. The assertion shape:

```go
result, err := resources.List(ctx, resource.Query{Keyword: "ubuntu", Limit: 10})
if err != nil {
	t.Fatalf("List returned error: %v", err)
}
if result.Total != 1 || result.Items[0].URL != "https://pan.quark.cn/s/live" {
	t.Fatalf("indexed resources = %+v, want live link", result)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/update -run Resource -v
```

Expected: FAIL because update path does not refresh `resource_index`.

- [ ] **Step 3: Wire repository in main**

Modify `cmd/tg-search/main.go` near existing resource repository setup:

```go
resourceStats := repository.NewResourceStatsRepository(conn)
resourceIndex := repository.NewResourceIndexRepository(conn)
resourceService := resource.NewService(links, files, resourceStats, resourceIndex)
```

Run:

```bash
gofmt -w cmd/tg-search/main.go
```

- [ ] **Step 4: Refresh after history batch commits**

In `internal/history/service.go`, collect stored message IDs inside `storeHistoryBatch`:

```go
storedMessageIDs := []int64{}
```

Append `msg.ID` inside the transaction after `stored` is returned.

After the transaction and before notifications:

```go
if s.resources != nil && len(storedMessageIDs) > 0 {
	if err := s.resources.RefreshMessages(ctx, storedMessageIDs); err != nil {
		return 0, fmt.Errorf("refresh resource index: %w", err)
	}
}
```

Add these methods to `internal/resource/service.go`:

```go
func (s *Service) RefreshMessage(ctx context.Context, messageID int64) error {
	if s.index == nil {
		return s.RefreshGlobalGrouped(ctx)
	}
	if err := s.index.RefreshMessage(ctx, messageID); err != nil {
		return err
	}
	return s.RefreshGlobalGrouped(ctx)
}

func (s *Service) RefreshMessages(ctx context.Context, messageIDs []int64) error {
	if s.index == nil {
		return s.RefreshGlobalGrouped(ctx)
	}
	if err := s.index.RefreshMessages(ctx, messageIDs); err != nil {
		return err
	}
	return s.RefreshGlobalGrouped(ctx)
}

func (s *Service) DeleteMessageResources(ctx context.Context, messageID int64) error {
	if s.index == nil {
		return s.RefreshGlobalGrouped(ctx)
	}
	if err := s.index.DeleteMessage(ctx, messageID); err != nil {
		return err
	}
	return s.RefreshGlobalGrouped(ctx)
}
```

- [ ] **Step 5: Refresh after real-time update commits**

In `internal/update/processor.go`, after the write transaction in `processMessage`, keep the stored message ID outside transaction:

```go
var storedMessageID int64
```

Set `storedMessageID = stored[0].ID` in the transaction.

After the transaction:

```go
if p.resources != nil && storedMessageID > 0 {
	if err := p.resources.RefreshMessage(ctx, storedMessageID); err != nil {
		return err
	}
}
```

In `deleteMessage`, after marking deleted, find the message ID before or during the transaction. If the repository does not expose lookup by channel/telegram ID, add `FindIDByTelegramMessageID(ctx, channelID, telegramMessageID)` to `MessageRepository`.

Then call:

```go
if p.resources != nil && messageID > 0 {
	return p.resources.DeleteMessageResources(ctx, messageID)
}
return p.refreshResourceStats(ctx)
```

- [ ] **Step 6: Update tests fixtures**

Where tests construct `resource.NewService`, pass a `ResourceIndexRepository` only in tests that need indexed behavior. Keep legacy tests passing by omitting it.

For new write-path tests:

```go
resourceIndex := repository.NewResourceIndexRepository(conn)
resources := resource.NewService(links, files, resourceStats, resourceIndex)
```

- [ ] **Step 7: Run targeted tests**

Run:

```bash
go test ./internal/update ./internal/history ./cmd/tg-search
```

Expected: PASS.

- [ ] **Step 8: Commit**

Run:

```bash
git add cmd/tg-search/main.go internal/history/service.go internal/update/processor.go internal/history/service_test.go internal/update/processor_test.go internal/resource/service.go internal/repository/message.go
git commit -m "feat: maintain resource index on writes"
```

---

## Task 5: Public `/api/search` Uses One Indexed Query

**Files:**
- Modify: `internal/api/external_search.go`
- Modify: `internal/api/handlers_test.go`

- [ ] **Step 1: Write failing public search query-shape test**

Add a test near existing external search tests in `internal/api/handlers_test.go`:

```go
func TestExternalSearchUsesSingleIndexedQueryForDefaultCloudTypes(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	index := repository.NewResourceIndexRepository(deps.BackupDB)
	deps.Resources = resource.NewService(deps.Links, deps.Files, repository.NewResourceStatsRepository(deps.BackupDB), index)
	accountID, _ := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Username: "main", Status: model.AccountStatusOnline})
	channelID, _ := deps.Channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 1, Title: "Public", Type: model.ChannelTypeChannel})
	stored, err := deps.Messages.SaveBatch(ctx, []model.Message{
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1, Text: "ubuntu quark", RawJSON: "{}", Date: time.Now().UTC()},
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 2, Text: "ubuntu magnet", RawJSON: "{}", Date: time.Now().UTC().Add(-time.Minute)},
	})
	if err != nil {
		t.Fatalf("save messages: %v", err)
	}
	if _, err := deps.Links.SaveBatch(ctx, stored[0].ID, []model.Link{{Type: "quark", Category: "cloud_drive", URL: "https://pan.quark.cn/s/ubuntu", Note: "Ubuntu Quark"}}); err != nil {
		t.Fatalf("save quark: %v", err)
	}
	if _, err := deps.Links.SaveBatch(ctx, stored[1].ID, []model.Link{{Type: "magnet", Category: "magnet", URL: "magnet:?xt=urn:btih:ubuntu", Note: "Ubuntu Magnet"}}); err != nil {
		t.Fatalf("save magnet: %v", err)
	}
	if err := index.Rebuild(ctx); err != nil {
		t.Fatalf("rebuild index: %v", err)
	}
	router := NewRouter(deps)
	key := createTestAPIKey(t, router)
	req := httptest.NewRequest(http.MethodGet, "/api/search?kw=ubuntu&limit=10", nil)
	req.Header.Set("Authorization", key)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	var body struct {
		Code int `json:"code"`
		Data struct {
			Total        int                            `json:"total"`
			MergedByType map[string][]externalMergedLink `json:"merged_by_type"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Code != 0 || body.Data.Total != 2 || len(body.Data.MergedByType["quark"]) != 1 || len(body.Data.MergedByType["magnet"]) != 1 {
		t.Fatalf("body = %+v raw=%s, want quark and magnet results", body, w.Body.String())
	}
}
```

The repository tests from Task 2 must include a `ResourceIndexRepository.List` call with `Categories: []string{"cloud_drive","magnet","ed2k","video"}` and assert that cloud-drive, magnet, and ed2k resources can be returned by one list query.

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/api -run TestExternalSearchUsesSingleIndexedQueryForDefaultCloudTypes -v
```

Expected: FAIL until `externalResourceItems` is changed.

- [ ] **Step 3: Add external query support to resource service**

Add to `internal/resource/service.go`:

```go
func (s *Service) ListIndexed(ctx context.Context, query model.ResourceIndexQuery) (ListResult, bool, error) {
	if s.index == nil {
		return ListResult{}, false, nil
	}
	result, err := s.index.List(ctx, query)
	if err != nil {
		return ListResult{}, true, err
	}
	items := make([]Item, 0, len(result.Items))
	for _, indexed := range result.Items {
		items = append(items, itemFromIndex(indexed))
	}
	return ListResult{Items: items, Total: result.Total, Grouped: normalizeGrouped(result.Grouped)}, true, nil
}
```

- [ ] **Step 4: Rewrite `externalResourceItems` indexed path**

In `internal/api/external_search.go`, modify `externalResourceItems`:

```go
func (h handlers) externalResourceItems(c *gin.Context, keyword string, cloudTypes []string, limit int, offset int, includeImage bool) ([]resource.Item, int, error) {
	filters := externalResourceFilters(cloudTypes)
	if result, ok, err := h.externalResourceItemsIndexed(c, keyword, filters, limit, offset, includeImage); ok || err != nil {
		if err != nil {
			return nil, 0, err
		}
		return result.Items, result.Total, nil
	}
	// existing legacy loop remains here as fallback
}
```

Add helper:

```go
type externalResourceItemsResult struct {
	Items []resource.Item
	Total int
}

func (h handlers) externalResourceItemsIndexed(c *gin.Context, keyword string, filters []externalResourceFilter, limit int, offset int, includeImage bool) (externalResourceItemsResult, bool, error) {
	categories := []string{}
	types := []string{}
	for _, filter := range filters {
		if filter.category != "" {
			categories = append(categories, filter.category)
		}
		if filter.typ != "" {
			types = append(types, filter.typ)
		}
	}
	result, ok, err := h.deps.Resources.ListIndexed(c.Request.Context(), model.ResourceIndexQuery{
		Keyword:    keyword,
		Categories: categories,
		Types:      types,
		Limit:      limit,
		Offset:     offset,
		MaxLimit:   externalSearchMaxLimit,
		Sort:       "date_desc",
	})
	if err != nil || !ok {
		return externalResourceItemsResult{}, ok, err
	}
	result.Items, err = h.attachMediaToExternalResourceItems(c.Request.Context(), result.Items, true, includeImage)
	if err != nil {
		return externalResourceItemsResult{}, true, err
	}
	return externalResourceItemsResult{Items: result.Items, Total: result.Total}, true, nil
}
```

Import `tg-search/internal/model`.

Make sure type filtering semantics preserve current behavior:

- no cloud types means categories cloud_drive, magnet, ed2k, video;
- provider-specific cloud types filter category `cloud_drive` and type in provider list;
- if both broad cloud_drive and provider-specific values are present, broad cloud_drive wins, matching `hasExternalCloudDriveGroup`.

- [ ] **Step 5: Run public API tests**

Run:

```bash
go test ./internal/api -run 'ExternalSearch|ResourcesAPI' -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

Run:

```bash
git add internal/api/external_search.go internal/api/handlers_test.go internal/resource/service.go
git commit -m "feat: serve public search from resource index"
```

---

## Task 6: Startup Rebuild And Operational Visibility

**Files:**
- Modify: `cmd/tg-search/main.go`
- Modify: `internal/api/handlers.go`
- Modify: `internal/api/router.go`
- Modify: `internal/api/handlers_test.go`

- [ ] **Step 1: Write failing status/maintenance test**

Add an API test:

```go
func TestResourceIndexMaintenanceRebuild(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	index := repository.NewResourceIndexRepository(deps.BackupDB)
	deps.Resources = resource.NewService(deps.Links, deps.Files, repository.NewResourceStatsRepository(deps.BackupDB), index)
	accountID, _ := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Username: "main", Status: model.AccountStatusOnline})
	channelID, _ := deps.Channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 1, Title: "Public", Type: model.ChannelTypeChannel})
	stored, err := deps.Messages.SaveBatch(ctx, []model.Message{{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1, Text: "ubuntu", RawJSON: "{}", Date: time.Now().UTC()}})
	if err != nil {
		t.Fatalf("save message: %v", err)
	}
	if _, err := deps.Links.SaveBatch(ctx, stored[0].ID, []model.Link{{Type: "quark", Category: "cloud_drive", URL: "https://pan.quark.cn/s/ubuntu", Note: "Ubuntu"}}); err != nil {
		t.Fatalf("save link: %v", err)
	}
	router := NewRouter(deps)
	req := httptest.NewRequest(http.MethodPost, "/api/maintenance/resource-index/rebuild", nil)
	withAdminSession(t, deps, req)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	result, err := deps.Resources.List(ctx, resource.Query{Keyword: "ubuntu", Limit: 10})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("total = %d, want rebuilt resource", result.Total)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/api -run TestResourceIndexMaintenanceRebuild -v
```

Expected: FAIL because route does not exist.

- [ ] **Step 3: Add service methods**

In `internal/resource/service.go` add:

```go
func (s *Service) RebuildIndex(ctx context.Context) error {
	if s.index == nil {
		return nil
	}
	if err := s.index.Rebuild(ctx); err != nil {
		return err
	}
	return s.RefreshGlobalGrouped(ctx)
}

func (s *Service) IndexStats(ctx context.Context) (model.ResourceIndexStats, error) {
	if s.index == nil {
		return model.ResourceIndexStats{}, nil
	}
	return s.index.Stats(ctx)
}
```

- [ ] **Step 4: Add maintenance handlers**

In `internal/api/handlers.go` add:

```go
func (h handlers) rebuildResourceIndex(c *gin.Context) {
	if h.deps.Resources == nil {
		errorText(c, http.StatusServiceUnavailable, "resources are unavailable")
		return
	}
	if err := h.deps.Resources.RebuildIndex(c.Request.Context()); err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	stats, err := h.deps.Resources.IndexStats(c.Request.Context())
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"rebuilt": true, "indexed_rows": stats.IndexedRows, "updated_at": stats.UpdatedAt})
}
```

In `internal/api/router.go` under `adminOnly` maintenance routes:

```go
adminOnly.POST("/maintenance/resource-index/rebuild", h.rebuildResourceIndex)
```

- [ ] **Step 5: Add startup rebuild**

In `cmd/tg-search/main.go`, after creating `resourceService`, call a startup helper:

```go
if stats, err := resourceService.IndexStats(ctx); err == nil && stats.IndexedRows == 0 {
	if err := resourceService.RebuildIndex(ctx); err != nil {
		logger.Warn("resource index rebuild failed", zap.Error(err))
	}
} else if err != nil {
	logger.Warn("resource index stats failed", zap.Error(err))
}
```

Use the existing logger variable and context name in `main.go`.

- [ ] **Step 6: Run tests**

Run:

```bash
go test ./internal/api ./cmd/tg-search
```

Expected: PASS.

- [ ] **Step 7: Commit**

Run:

```bash
git add cmd/tg-search/main.go internal/api/handlers.go internal/api/router.go internal/api/handlers_test.go internal/resource/service.go
git commit -m "feat: add resource index rebuild maintenance"
```

---

## Task 7: Performance Verification And Cleanup

**Files:**
- Modify: `internal/repository/resource_index_test.go`
- Modify: `docs/superpowers/specs/2026-06-13-resource-read-model-design.md` only if implementation intentionally differs.

- [ ] **Step 1: Add query-plan regression test**

Add to `internal/repository/resource_index_test.go`:

```go
func TestResourceIndexListQueryPlanAvoidsTelegramLinksWindowScan(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "telegram.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	rows, err := conn.QueryContext(ctx, `EXPLAIN QUERY PLAN
SELECT ri.id
FROM resource_index ri
WHERE ri.category = ?
ORDER BY ri.datetime DESC, ri.resource_id DESC
LIMIT 50`, "cloud_drive")
	if err != nil {
		t.Fatalf("explain query plan: %v", err)
	}
	defer rows.Close()
	plan := ""
	for rows.Next() {
		var id, parent, notUsed int
		var detail string
		if err := rows.Scan(&id, &parent, &notUsed, &detail); err != nil {
			t.Fatalf("scan plan: %v", err)
		}
		plan += detail + "\n"
	}
	if strings.Contains(plan, "telegram_links") || strings.Contains(plan, "USE TEMP B-TREE") {
		t.Fatalf("resource index plan = %s, want indexed resource_index plan without normalized link scan or temp sort", plan)
	}
}
```

Import `strings`.

- [ ] **Step 2: Run query-plan test**

Run:

```bash
go test ./internal/repository -run TestResourceIndexListQueryPlanAvoidsTelegramLinksWindowScan -v
```

Expected: PASS.

- [ ] **Step 3: Run full backend verification**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 4: Optional local timing check**

If local `data/tg-search.db` is available, start the app or run repository benchmark code against a copy. Do not mutate the production DB directly. Use:

```bash
cp data/tg-search.db /tmp/tg-search-resource-index-check.db
```

Then run the app or a short Go test against `/tmp/tg-search-resource-index-check.db` to rebuild and query the index. Record observed timings in the final implementation notes.

- [ ] **Step 5: Commit**

Run:

```bash
git add internal/repository/resource_index_test.go docs/superpowers/specs/2026-06-13-resource-read-model-design.md
git commit -m "test: verify resource index query plan"
```

---

## Final Verification

- [ ] Run:

```bash
go test ./...
```

Expected: every package passes.

- [ ] If frontend types are touched unexpectedly, run:

```bash
npm run web:typecheck
npm run web:test
```

Expected: both pass. These should not be required for a backend-only implementation.

- [ ] Check git status:

```bash
git status --short
```

Expected: clean working tree.

- [ ] Summarize:

- schema migration version added;
- endpoints switched to read model;
- public `/api/search` response compatibility;
- verification commands and results;
- any operational note about first rebuild.
