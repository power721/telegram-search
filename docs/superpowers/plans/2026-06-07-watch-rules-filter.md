# Watch Rules Filter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add database-backed channel watch rules and apply link/include/exclude filtering to realtime updates and manual history sync.

**Architecture:** Add a `telegram_watch_rules` table, repository, API handlers, and a shared `internal/messagefilter` service. Realtime update processing uses only enabled rules; manual history sync applies a rule when present and ignores `enabled`.

**Tech Stack:** Go, SQLite, Gin, gotd, existing repository/service patterns, `go test`.

---

## File Structure

- Modify `internal/model/model.go`: add `WatchRule`.
- Modify `internal/db/migrations.go`: add migration version 5 for `telegram_watch_rules`.
- Create `internal/repository/watch_rule.go`: CRUD and channel lookup repository.
- Create `internal/repository/watch_rule_test.go`: repository behavior tests.
- Create `internal/messagefilter/filter.go`: shared rule application and keyword/link checks.
- Create `internal/messagefilter/filter_test.go`: filter behavior tests.
- Modify `internal/update/processor.go`: use filter for new/edit events; delete unmatched edits when needed.
- Modify `internal/update/processor_test.go`: realtime filtering tests.
- Modify `internal/history/service.go`: apply filter before storing history messages.
- Modify `internal/history/service_test.go`: history filtering tests.
- Modify `internal/api/router.go`: add watch rule routes and dependency.
- Modify `internal/api/handlers.go`: add CRUD handlers and JSON validation.
- Modify `internal/api/handlers_test.go`: API endpoint tests and dependency setup.
- Modify `cmd/tg-provider/main.go`: wire watch rule repository and filter into update/history/API dependencies.
- Modify `docs/api.md` and `README.md`: document watch rule API briefly.

## Task 1: Watch Rule Storage

**Files:**
- Modify: `internal/model/model.go`
- Modify: `internal/db/migrations.go`
- Create: `internal/repository/watch_rule.go`
- Create: `internal/repository/watch_rule_test.go`

- [ ] **Step 1: Write failing repository tests**

Create `internal/repository/watch_rule_test.go`:

```go
package repository

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"tg-provider/internal/db"
	"tg-provider/internal/model"
)

func TestWatchRuleRepositoryCRUDAndChannelLookup(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "telegram.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}

	accounts := NewAccountRepository(conn)
	channels := NewChannelRepository(conn)
	rules := NewWatchRuleRepository(conn)
	accountID, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 200, Title: "VIP", Type: model.ChannelTypeChannel})

	id, err := rules.Create(ctx, model.WatchRule{
		ChannelID: channelID,
		Enabled:   true,
		Includes:  []string{" 庆余年 ", "", "1080p"},
		Excludes:  []string{"预告"},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	got, err := rules.FindByChannelID(ctx, channelID)
	if err != nil {
		t.Fatalf("FindByChannelID returned error: %v", err)
	}
	if got.ID != id || got.ChannelID != channelID || !got.Enabled {
		t.Fatalf("rule identity = %+v, want id=%d channel=%d enabled", got, id, channelID)
	}
	if !sameStrings(got.Includes, []string{"庆余年", "1080p"}) || !sameStrings(got.Excludes, []string{"预告"}) {
		t.Fatalf("terms = includes=%q excludes=%q", got.Includes, got.Excludes)
	}

	got.Enabled = false
	got.Includes = []string{"三体"}
	got.Excludes = []string{"花絮", " trailer "}
	if err := rules.Update(ctx, got); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	updated, err := rules.FindByID(ctx, id)
	if err != nil {
		t.Fatalf("FindByID returned error: %v", err)
	}
	if updated.Enabled || !sameStrings(updated.Includes, []string{"三体"}) || !sameStrings(updated.Excludes, []string{"花絮", "trailer"}) {
		t.Fatalf("updated rule = %+v", updated)
	}

	all, err := rules.FindAll(ctx)
	if err != nil {
		t.Fatalf("FindAll returned error: %v", err)
	}
	if len(all) != 1 || all[0].ID != id {
		t.Fatalf("FindAll = %+v, want one rule id %d", all, id)
	}

	if _, err := rules.Create(ctx, model.WatchRule{ChannelID: channelID, Enabled: true}); !errors.Is(err, ErrDuplicateWatchRule) {
		t.Fatalf("duplicate Create err = %v, want ErrDuplicateWatchRule", err)
	}
	if err := rules.Delete(ctx, id); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := rules.FindByID(ctx, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("FindByID after delete err = %v, want ErrNotFound", err)
	}
}

func TestWatchRuleRepositoryCascadesWithChannelDelete(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "telegram.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	accounts := NewAccountRepository(conn)
	channels := NewChannelRepository(conn)
	rules := NewWatchRuleRepository(conn)
	accountID, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 200, Title: "VIP", Type: model.ChannelTypeChannel})
	_, _ = rules.Create(ctx, model.WatchRule{ChannelID: channelID, Enabled: true})

	if _, err := conn.ExecContext(ctx, `DELETE FROM telegram_channels WHERE id = ?`, channelID); err != nil {
		t.Fatalf("delete channel: %v", err)
	}
	all, err := rules.FindAll(ctx)
	if err != nil {
		t.Fatalf("FindAll returned error: %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("rules after channel delete = %+v, want empty", all)
	}
}

func sameStrings(got []string, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
```

- [ ] **Step 2: Run repository test to verify it fails**

Run:

```bash
go test ./internal/repository -run 'TestWatchRuleRepository' -count=1
```

Expected: FAIL because `NewWatchRuleRepository`, `model.WatchRule`, `ErrDuplicateWatchRule`, and `ErrNotFound` do not exist.

- [ ] **Step 3: Add model and migration**

Add to `internal/model/model.go`:

```go
type WatchRule struct {
	ID        int64     `json:"id"`
	ChannelID int64     `json:"channel_id"`
	Enabled   bool      `json:"enabled"`
	Includes  []string  `json:"includes"`
	Excludes  []string  `json:"excludes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
```

Append migration version 5 in `internal/db/migrations.go`:

```go
{
	version: 5,
	name:    "watch_rules",
	sql: `
CREATE TABLE IF NOT EXISTS telegram_watch_rules (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  channel_id INTEGER NOT NULL UNIQUE,
  enabled INTEGER NOT NULL DEFAULT 1,
  includes_json TEXT NOT NULL DEFAULT '[]',
  excludes_json TEXT NOT NULL DEFAULT '[]',
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  FOREIGN KEY(channel_id) REFERENCES telegram_channels(id) ON DELETE CASCADE
);
`,
},
```

- [ ] **Step 4: Add repository implementation**

Create `internal/repository/watch_rule.go`:

```go
package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mattn/go-sqlite3"

	"tg-provider/internal/model"
)

var ErrNotFound = sql.ErrNoRows
var ErrDuplicateWatchRule = errors.New("watch rule already exists for channel")

type WatchRuleRepository struct {
	db *sql.DB
}

func NewWatchRuleRepository(db *sql.DB) *WatchRuleRepository {
	return &WatchRuleRepository{db: db}
}

func (r *WatchRuleRepository) Create(ctx context.Context, rule model.WatchRule) (int64, error) {
	rule.Includes = normalizeTerms(rule.Includes)
	rule.Excludes = normalizeTerms(rule.Excludes)
	includes, err := json.Marshal(rule.Includes)
	if err != nil {
		return 0, fmt.Errorf("marshal includes: %w", err)
	}
	excludes, err := json.Marshal(rule.Excludes)
	if err != nil {
		return 0, fmt.Errorf("marshal excludes: %w", err)
	}
	now := time.Now().UTC()
	enabled := boolInt(rule.Enabled)
	var id int64
	err = r.db.QueryRowContext(ctx, `
INSERT INTO telegram_watch_rules (channel_id, enabled, includes_json, excludes_json, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING id`,
		rule.ChannelID, enabled, string(includes), string(excludes), now, now,
	).Scan(&id)
	if err != nil {
		if isSQLiteUniqueConstraint(err) {
			return 0, ErrDuplicateWatchRule
		}
		return 0, fmt.Errorf("create watch rule: %w", err)
	}
	return id, nil
}

func (r *WatchRuleRepository) Update(ctx context.Context, rule model.WatchRule) error {
	rule.Includes = normalizeTerms(rule.Includes)
	rule.Excludes = normalizeTerms(rule.Excludes)
	includes, err := json.Marshal(rule.Includes)
	if err != nil {
		return fmt.Errorf("marshal includes: %w", err)
	}
	excludes, err := json.Marshal(rule.Excludes)
	if err != nil {
		return fmt.Errorf("marshal excludes: %w", err)
	}
	res, err := r.db.ExecContext(ctx, `
UPDATE telegram_watch_rules
SET channel_id = ?, enabled = ?, includes_json = ?, excludes_json = ?, updated_at = ?
WHERE id = ?`,
		rule.ChannelID, boolInt(rule.Enabled), string(includes), string(excludes), time.Now().UTC(), rule.ID,
	)
	if err != nil {
		if isSQLiteUniqueConstraint(err) {
			return ErrDuplicateWatchRule
		}
		return fmt.Errorf("update watch rule: %w", err)
	}
	return requireRows(res, "watch rule not found")
}

func (r *WatchRuleRepository) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM telegram_watch_rules WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete watch rule: %w", err)
	}
	return requireRows(res, "watch rule not found")
}

func (r *WatchRuleRepository) FindByID(ctx context.Context, id int64) (model.WatchRule, error) {
	return scanWatchRule(r.db.QueryRowContext(ctx, `
SELECT id, channel_id, enabled, includes_json, excludes_json, created_at, updated_at
FROM telegram_watch_rules WHERE id = ?`, id))
}

func (r *WatchRuleRepository) FindByChannelID(ctx context.Context, channelID int64) (model.WatchRule, error) {
	return scanWatchRule(r.db.QueryRowContext(ctx, `
SELECT id, channel_id, enabled, includes_json, excludes_json, created_at, updated_at
FROM telegram_watch_rules WHERE channel_id = ?`, channelID))
}

func (r *WatchRuleRepository) FindAll(ctx context.Context) ([]model.WatchRule, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, channel_id, enabled, includes_json, excludes_json, created_at, updated_at
FROM telegram_watch_rules
ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("find watch rules: %w", err)
	}
	defer rows.Close()
	var out []model.WatchRule
	for rows.Next() {
		rule, err := scanWatchRule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rule)
	}
	return out, rows.Err()
}

func scanWatchRule(row interface{ Scan(...any) error }) (model.WatchRule, error) {
	var rule model.WatchRule
	var enabled int
	var includesRaw string
	var excludesRaw string
	if err := row.Scan(&rule.ID, &rule.ChannelID, &enabled, &includesRaw, &excludesRaw, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
		return model.WatchRule{}, err
	}
	rule.Enabled = enabled != 0
	if err := json.Unmarshal([]byte(includesRaw), &rule.Includes); err != nil {
		return model.WatchRule{}, fmt.Errorf("unmarshal includes: %w", err)
	}
	if err := json.Unmarshal([]byte(excludesRaw), &rule.Excludes); err != nil {
		return model.WatchRule{}, fmt.Errorf("unmarshal excludes: %w", err)
	}
	return rule, nil
}

func normalizeTerms(in []string) []string {
	out := make([]string, 0, len(in))
	for _, term := range in {
		trimmed := strings.TrimSpace(term)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func isSQLiteUniqueConstraint(err error) bool {
	var sqliteErr sqlite3.Error
	return errors.As(err, &sqliteErr) && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique
}
```

- [ ] **Step 5: Run repository tests to verify pass**

Run:

```bash
go test ./internal/repository -run 'TestWatchRuleRepository' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit storage layer**

Run:

```bash
git add internal/model/model.go internal/db/migrations.go internal/repository/watch_rule.go internal/repository/watch_rule_test.go
git commit -m "feat: add watch rule storage"
```

## Task 2: Shared Message Filter

**Files:**
- Create: `internal/messagefilter/filter.go`
- Create: `internal/messagefilter/filter_test.go`

- [ ] **Step 1: Write failing filter tests**

Create `internal/messagefilter/filter_test.go`:

```go
package messagefilter

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"tg-provider/internal/model"
)

func TestFilterAppliesLinksIncludesAndExcludes(t *testing.T) {
	rules := fakeRules{byChannel: map[int64]model.WatchRule{
		1: {ChannelID: 1, Enabled: true, Includes: []string{"庆余年", "S01"}, Excludes: []string{"预告"}},
	}}
	filter := New(rules)

	result, err := filter.Apply(context.Background(), Request{
		ChannelID:      1,
		Text:           "庆余年 S01 https://pan.quark.cn/s/abc",
		RequireEnabled: true,
	})
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if !result.Keep || len(result.Links) != 1 || result.Links[0].Type != "quark" {
		t.Fatalf("result = %+v, want keep with quark link", result)
	}

	rejectedNoLink, _ := filter.Apply(context.Background(), Request{ChannelID: 1, Text: "庆余年 S01", RequireEnabled: true})
	if rejectedNoLink.Keep || rejectedNoLink.Reason != ReasonNoLinks {
		t.Fatalf("no-link result = %+v, want ReasonNoLinks", rejectedNoLink)
	}

	rejectedInclude, _ := filter.Apply(context.Background(), Request{ChannelID: 1, Text: "三体 https://pan.quark.cn/s/abc", RequireEnabled: true})
	if rejectedInclude.Keep || rejectedInclude.Reason != ReasonIncludeMiss {
		t.Fatalf("include result = %+v, want ReasonIncludeMiss", rejectedInclude)
	}

	rejectedExclude, _ := filter.Apply(context.Background(), Request{ChannelID: 1, Text: "庆余年 预告 https://pan.quark.cn/s/abc", RequireEnabled: true})
	if rejectedExclude.Keep || rejectedExclude.Reason != ReasonExcludeHit {
		t.Fatalf("exclude result = %+v, want ReasonExcludeHit", rejectedExclude)
	}
}

func TestFilterHandlesMissingAndDisabledRules(t *testing.T) {
	rules := fakeRules{byChannel: map[int64]model.WatchRule{
		2: {ChannelID: 2, Enabled: false, Includes: []string{"庆余年"}},
	}}
	filter := New(rules)

	missing, err := filter.Apply(context.Background(), Request{ChannelID: 1, Text: "anything", RequireRule: false})
	if err != nil {
		t.Fatalf("missing optional rule returned error: %v", err)
	}
	if !missing.Keep || missing.RuleApplied {
		t.Fatalf("missing optional result = %+v, want keep without rule", missing)
	}

	disabledRealtime, _ := filter.Apply(context.Background(), Request{ChannelID: 2, Text: "庆余年 https://pan.quark.cn/s/abc", RequireEnabled: true})
	if disabledRealtime.Keep || disabledRealtime.Reason != ReasonRuleDisabled {
		t.Fatalf("disabled realtime result = %+v, want ReasonRuleDisabled", disabledRealtime)
	}

	disabledHistory, _ := filter.Apply(context.Background(), Request{ChannelID: 2, Text: "庆余年 https://pan.quark.cn/s/abc", RequireEnabled: false})
	if !disabledHistory.Keep || !disabledHistory.RuleApplied {
		t.Fatalf("disabled history result = %+v, want keep because enabled ignored", disabledHistory)
	}
}

type fakeRules struct {
	byChannel map[int64]model.WatchRule
}

func (f fakeRules) FindByChannelID(ctx context.Context, channelID int64) (model.WatchRule, error) {
	rule, ok := f.byChannel[channelID]
	if !ok {
		return model.WatchRule{}, sql.ErrNoRows
	}
	return rule, nil
}

func TestFilterReturnsRuleLookupErrors(t *testing.T) {
	filter := New(errorRules{})
	_, err := filter.Apply(context.Background(), Request{ChannelID: 1, Text: "x", RequireRule: true})
	if err == nil {
		t.Fatal("Apply returned nil error, want lookup error")
	}
}

type errorRules struct{}

func (errorRules) FindByChannelID(ctx context.Context, channelID int64) (model.WatchRule, error) {
	return model.WatchRule{}, errors.New("database unavailable")
}
```

- [ ] **Step 2: Run filter tests to verify they fail**

Run:

```bash
go test ./internal/messagefilter -count=1
```

Expected: FAIL because the package and types do not exist.

- [ ] **Step 3: Implement filter**

Create `internal/messagefilter/filter.go`:

```go
package messagefilter

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"tg-provider/internal/link"
	"tg-provider/internal/model"
)

type RuleStore interface {
	FindByChannelID(context.Context, int64) (model.WatchRule, error)
}

type Filter struct {
	rules     RuleStore
	extractor *link.Extractor
}

type Request struct {
	ChannelID      int64
	Text           string
	RequireRule    bool
	RequireEnabled bool
}

type Result struct {
	Keep        bool
	RuleApplied bool
	Links       []model.Link
	Reason      Reason
}

type Reason string

const (
	ReasonNone        Reason = ""
	ReasonNoRule      Reason = "no_rule"
	ReasonRuleDisabled Reason = "rule_disabled"
	ReasonNoLinks     Reason = "no_links"
	ReasonIncludeMiss Reason = "include_miss"
	ReasonExcludeHit  Reason = "exclude_hit"
)

func New(rules RuleStore) *Filter {
	return &Filter{rules: rules, extractor: link.NewExtractor()}
}

func (f *Filter) Apply(ctx context.Context, req Request) (Result, error) {
	if f == nil || f.rules == nil {
		return Result{Keep: true, Reason: ReasonNone}, nil
	}
	rule, err := f.rules.FindByChannelID(ctx, req.ChannelID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if req.RequireRule {
				return Result{Keep: false, Reason: ReasonNoRule}, nil
			}
			return Result{Keep: true, Reason: ReasonNoRule}, nil
		}
		return Result{}, err
	}
	if req.RequireEnabled && !rule.Enabled {
		return Result{Keep: false, RuleApplied: true, Reason: ReasonRuleDisabled}, nil
	}
	extractor := f.extractor
	if extractor == nil {
		extractor = link.NewExtractor()
	}
	links := extractor.Extract(req.Text)
	if len(links) == 0 {
		return Result{Keep: false, RuleApplied: true, Reason: ReasonNoLinks}, nil
	}
	text := strings.ToLower(req.Text)
	if len(rule.Includes) > 0 && !containsAny(text, rule.Includes) {
		return Result{Keep: false, RuleApplied: true, Links: links, Reason: ReasonIncludeMiss}, nil
	}
	if containsAny(text, rule.Excludes) {
		return Result{Keep: false, RuleApplied: true, Links: links, Reason: ReasonExcludeHit}, nil
	}
	return Result{Keep: true, RuleApplied: true, Links: links, Reason: ReasonNone}, nil
}

func containsAny(lowerText string, terms []string) bool {
	for _, term := range terms {
		normalized := strings.ToLower(strings.TrimSpace(term))
		if normalized == "" {
			continue
		}
		if strings.Contains(lowerText, normalized) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run filter tests to verify pass**

Run:

```bash
go test ./internal/messagefilter -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit shared filter**

Run:

```bash
git add internal/messagefilter/filter.go internal/messagefilter/filter_test.go
git commit -m "feat: add watch rule message filter"
```

## Task 3: Realtime Update Filtering

**Files:**
- Modify: `internal/update/processor.go`
- Modify: `internal/update/processor_test.go`

- [ ] **Step 1: Write failing realtime tests**

Add to `internal/update/processor_test.go`:

```go
func TestProcessorFiltersRealtimeMessagesByEnabledWatchRule(t *testing.T) {
	ctx := context.Background()
	fixture := newProcessorFixture(t)
	rules := repository.NewWatchRuleRepository(fixture.conn)
	_, err := rules.Create(ctx, model.WatchRule{
		ChannelID: fixture.channelID,
		Enabled:   true,
		Includes:  []string{"庆余年"},
		Excludes:  []string{"预告"},
	})
	if err != nil {
		t.Fatalf("create watch rule: %v", err)
	}
	processor := NewProcessor(ProcessorOptions{
		DB: fixture.conn, Channels: fixture.channels, Messages: fixture.messages, Links: fixture.links,
		Extractor: link.NewExtractor(), Filter: messagefilter.New(rules),
	})
	now := time.Now().UTC()
	for _, event := range []Event{
		{Type: EventNewMessage, AccountID: fixture.accountID, TelegramChannelID: fixture.telegramChannelID, MessageID: 11, Text: "庆余年 https://pan.quark.cn/s/abc", RawJSON: "{}", Date: now},
		{Type: EventNewMessage, AccountID: fixture.accountID, TelegramChannelID: fixture.telegramChannelID, MessageID: 12, Text: "庆余年 无链接", RawJSON: "{}", Date: now},
		{Type: EventNewMessage, AccountID: fixture.accountID, TelegramChannelID: fixture.telegramChannelID, MessageID: 13, Text: "三体 https://pan.quark.cn/s/def", RawJSON: "{}", Date: now},
		{Type: EventNewMessage, AccountID: fixture.accountID, TelegramChannelID: fixture.telegramChannelID, MessageID: 14, Text: "庆余年 预告 https://pan.quark.cn/s/ghi", RawJSON: "{}", Date: now},
	} {
		if err := processor.Process(ctx, event); err != nil {
			t.Fatalf("process event %d: %v", event.MessageID, err)
		}
	}
	results, err := fixture.messages.Search(ctx, repository.SearchParams{Query: "庆余年", Limit: 10})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 || results[0].TelegramMessageID != 11 || len(results[0].Links) != 1 {
		t.Fatalf("results = %+v, want only message 11 with link", results)
	}
}

func TestProcessorSkipsRealtimeMessagesWithoutEnabledWatchRule(t *testing.T) {
	ctx := context.Background()
	fixture := newProcessorFixture(t)
	rules := repository.NewWatchRuleRepository(fixture.conn)
	processor := NewProcessor(ProcessorOptions{
		DB: fixture.conn, Channels: fixture.channels, Messages: fixture.messages, Links: fixture.links,
		Extractor: link.NewExtractor(), Filter: messagefilter.New(rules),
	})
	err := processor.Process(ctx, Event{
		Type: EventNewMessage, AccountID: fixture.accountID, TelegramChannelID: fixture.telegramChannelID,
		MessageID: 20, Text: "庆余年 https://pan.quark.cn/s/abc", RawJSON: "{}", Date: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	latest, err := fixture.messages.Latest(ctx, repository.LatestParams{Limit: 10})
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	if len(latest) != 0 {
		t.Fatalf("latest = %+v, want no stored realtime messages without enabled rule", latest)
	}
}

func TestProcessorDeletesStoredMessageWhenRealtimeEditStopsMatching(t *testing.T) {
	ctx := context.Background()
	fixture := newProcessorFixture(t)
	rules := repository.NewWatchRuleRepository(fixture.conn)
	_, err := rules.Create(ctx, model.WatchRule{ChannelID: fixture.channelID, Enabled: true, Includes: []string{"庆余年"}})
	if err != nil {
		t.Fatalf("create watch rule: %v", err)
	}
	processor := NewProcessor(ProcessorOptions{
		DB: fixture.conn, Channels: fixture.channels, Messages: fixture.messages, Links: fixture.links,
		Extractor: link.NewExtractor(), Filter: messagefilter.New(rules),
	})
	now := time.Now().UTC()
	if err := processor.Process(ctx, Event{Type: EventNewMessage, AccountID: fixture.accountID, TelegramChannelID: fixture.telegramChannelID, MessageID: 30, Text: "庆余年 https://pan.quark.cn/s/abc", RawJSON: "{}", Date: now}); err != nil {
		t.Fatalf("process new: %v", err)
	}
	if err := processor.Process(ctx, Event{Type: EventEditMessage, AccountID: fixture.accountID, TelegramChannelID: fixture.telegramChannelID, MessageID: 30, Text: "三体 https://pan.quark.cn/s/abc", RawJSON: "{}", Date: now}); err != nil {
		t.Fatalf("process edit: %v", err)
	}
	results, err := fixture.messages.Search(ctx, repository.SearchParams{Query: "庆余年", Limit: 10})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("search after non-matching edit = %+v, want empty", results)
	}
}
```

Add imports to `internal/update/processor_test.go`:

```go
"tg-provider/internal/messagefilter"
```

- [ ] **Step 2: Run realtime tests to verify they fail**

Run:

```bash
go test ./internal/update -run 'TestProcessor.*WatchRule|TestProcessorDeletesStoredMessageWhenRealtimeEditStopsMatching' -count=1
```

Expected: FAIL because `ProcessorOptions.Filter` does not exist and realtime messages are not filtered.

- [ ] **Step 3: Implement realtime filtering**

Modify `internal/update/processor.go`:

```go
import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	dbpkg "tg-provider/internal/db"
	"tg-provider/internal/link"
	"tg-provider/internal/messagefilter"
	"tg-provider/internal/model"
	"tg-provider/internal/repository"
)

type ProcessorOptions struct {
	DB        *sql.DB
	Channels  *repository.ChannelRepository
	Messages  *repository.MessageRepository
	Links     *repository.LinkRepository
	Extractor *link.Extractor
	Filter    *messagefilter.Filter
}

type Processor struct {
	db        *sql.DB
	channels  *repository.ChannelRepository
	messages  *repository.MessageRepository
	links     *repository.LinkRepository
	extractor *link.Extractor
	filter    *messagefilter.Filter
}
```

In `NewProcessor`, assign `filter: opts.Filter`.

Replace `storeMessage` with:

```go
func (p *Processor) storeMessage(ctx context.Context, channel model.Channel, event Event) error {
	extracted := p.extractor.Extract(event.Text)
	if p.filter != nil {
		result, err := p.filter.Apply(ctx, messagefilter.Request{
			ChannelID:      channel.ID,
			Text:           event.Text,
			RequireRule:    true,
			RequireEnabled: true,
		})
		if err != nil {
			return err
		}
		if !result.Keep {
			if event.Type == EventEditMessage && result.RuleApplied {
				if err := p.messages.MarkDeleted(ctx, channel.ID, event.MessageID); err != nil && !errors.Is(err, sql.ErrNoRows) {
					return err
				}
			}
			return nil
		}
		extracted = result.Links
	}
	return dbpkg.WithTx(ctx, p.db, func(tx *sql.Tx) error {
		date := event.Date
		if date.IsZero() {
			date = event.EditDateOrNow()
		}
		stored, err := p.messages.SaveBatchTx(ctx, tx, []model.Message{{
			AccountID:         event.AccountID,
			ChannelID:         channel.ID,
			TelegramMessageID: event.MessageID,
			SenderID:          event.SenderID,
			Text:              event.Text,
			RawJSON:           event.RawJSON,
			Date:              date,
			EditDate:          event.EditDate,
		}})
		if err != nil {
			return err
		}
		_, err = p.links.ReplaceForMessageTx(ctx, tx, stored[0].ID, extracted)
		return err
	})
}
```

- [ ] **Step 4: Run realtime tests to verify pass**

Run:

```bash
go test ./internal/update -run 'TestProcessor.*WatchRule|TestProcessorDeletesStoredMessageWhenRealtimeEditStopsMatching|TestProcessorHandlesNewEditAndDeleteEvents' -count=1
```

Expected: PASS. The existing no-filter processor test still passes because `Filter` is nil there.

- [ ] **Step 5: Commit realtime filtering**

Run:

```bash
git add internal/update/processor.go internal/update/processor_test.go
git commit -m "feat: filter realtime updates with watch rules"
```

## Task 4: History Sync Filtering

**Files:**
- Modify: `internal/history/service.go`
- Modify: `internal/history/service_test.go`

- [ ] **Step 1: Write failing history tests**

Add to `internal/history/service_test.go`:

```go
func TestSyncChannelAppliesWatchRuleAndIgnoresEnabled(t *testing.T) {
	ctx := context.Background()
	conn, accounts, channels, messages, links := setupHistoryTestStore(t)
	_, channelID := seedHistoryAccountAndChannel(t, ctx, accounts, channels)
	rules := repository.NewWatchRuleRepository(conn)
	_, err := rules.Create(ctx, model.WatchRule{ChannelID: channelID, Enabled: false, Includes: []string{"庆余年"}, Excludes: []string{"预告"}})
	if err != nil {
		t.Fatalf("create watch rule: %v", err)
	}
	now := time.Now().UTC()
	fake := &fakeTelegramClient{batches: map[int64][]telegram.Message{
		0: {
			{TelegramMessageID: 3, Text: "庆余年 https://pan.quark.cn/s/keep", RawJSON: "{}", Date: now},
			{TelegramMessageID: 2, Text: "庆余年 无链接", RawJSON: "{}", Date: now},
			{TelegramMessageID: 1, Text: "庆余年 预告 https://pan.quark.cn/s/drop", RawJSON: "{}", Date: now},
		},
	}}
	service := NewService(Options{
		DB: conn, Accounts: accounts, Channels: channels, Messages: messages, Links: links,
		Telegram: fake, Sessions: session.NewManager(filepath.Join(t.TempDir(), "sessions")),
		Extractor: link.NewExtractor(), HistoryBatchSize: 10, Filter: messagefilter.New(rules),
	})
	result, err := service.SyncChannel(ctx, channelID)
	if err != nil {
		t.Fatalf("SyncChannel returned error: %v", err)
	}
	if result.Messages != 3 || result.Links != 1 {
		t.Fatalf("result = %+v, want 3 fetched messages and 1 stored link", result)
	}
	results, err := messages.Search(ctx, repository.SearchParams{Query: "庆余年", Limit: 10})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 || results[0].TelegramMessageID != 3 {
		t.Fatalf("results = %+v, want only message 3", results)
	}
}

func TestSyncChannelWithoutWatchRuleKeepsExistingBehavior(t *testing.T) {
	ctx := context.Background()
	conn, accounts, channels, messages, links := setupHistoryTestStore(t)
	_, channelID := seedHistoryAccountAndChannel(t, ctx, accounts, channels)
	rules := repository.NewWatchRuleRepository(conn)
	now := time.Now().UTC()
	fake := &fakeTelegramClient{batches: map[int64][]telegram.Message{
		0: {
			{TelegramMessageID: 2, Text: "plain message without link", RawJSON: "{}", Date: now},
			{TelegramMessageID: 1, Text: "linked https://pan.quark.cn/s/abc", RawJSON: "{}", Date: now},
		},
	}}
	service := NewService(Options{
		DB: conn, Accounts: accounts, Channels: channels, Messages: messages, Links: links,
		Telegram: fake, Sessions: session.NewManager(filepath.Join(t.TempDir(), "sessions")),
		Extractor: link.NewExtractor(), HistoryBatchSize: 10, Filter: messagefilter.New(rules),
	})
	_, err := service.SyncChannel(ctx, channelID)
	if err != nil {
		t.Fatalf("SyncChannel returned error: %v", err)
	}
	latest, err := messages.Latest(ctx, repository.LatestParams{Limit: 10})
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	if len(latest) != 2 {
		t.Fatalf("latest len = %d, want 2 messages when no watch rule exists", len(latest))
	}
}
```

Add import:

```go
"tg-provider/internal/messagefilter"
```

- [ ] **Step 2: Run history tests to verify they fail**

Run:

```bash
go test ./internal/history -run 'TestSyncChannel.*WatchRule|TestSyncChannelWithoutWatchRuleKeepsExistingBehavior' -count=1
```

Expected: FAIL because `history.Options.Filter` does not exist and history sync does not filter.

- [ ] **Step 3: Implement history filtering**

Modify `internal/history/service.go`:

```go
import "tg-provider/internal/messagefilter"

type Options struct {
	DB               *sql.DB
	Accounts         *repository.AccountRepository
	Channels         *repository.ChannelRepository
	Messages         *repository.MessageRepository
	Links            *repository.LinkRepository
	Telegram         telegram.Client
	Sessions         *session.Manager
	Extractor        *link.Extractor
	Filter           *messagefilter.Filter
	HistoryBatchSize int
	Workers          int
	RetryPolicy      retry.Policy
}

type Service struct {
	db               *sql.DB
	accounts         *repository.AccountRepository
	channels         *repository.ChannelRepository
	messages         *repository.MessageRepository
	links            *repository.LinkRepository
	telegram         telegram.Client
	sessions         *session.Manager
	extractor        *link.Extractor
	filter           *messagefilter.Filter
	historyBatchSize int
	workers          int
	retryPolicy      retry.Policy
	mu               sync.Mutex
	runningChannels  map[int64]struct{}
}
```

In `NewService`, assign `filter: opts.Filter`.

Replace `storeBatch` with this implementation so filtered history messages are never inserted:

```go
func (s *Service) storeBatch(ctx context.Context, channelID int64, cursor int64, messages []model.Message) (int, error) {
	filtered := make([]model.Message, 0, len(messages))
	linksByTelegramID := map[int64][]model.Link{}
	for _, msg := range messages {
		extracted := s.extractor.Extract(msg.Text)
		if s.filter != nil {
			result, err := s.filter.Apply(ctx, messagefilter.Request{
				ChannelID:      msg.ChannelID,
				Text:           msg.Text,
				RequireRule:    false,
				RequireEnabled: false,
			})
			if err != nil {
				return 0, fmt.Errorf("filter history message: %w", err)
			}
			if result.RuleApplied {
				if !result.Keep {
					continue
				}
				extracted = result.Links
			}
		}
		filtered = append(filtered, msg)
		linksByTelegramID[msg.TelegramMessageID] = extracted
	}

	var linkCount int
	err := dbpkg.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		if len(filtered) > 0 {
			stored, err := s.messages.SaveBatchTx(ctx, tx, filtered)
			if err != nil {
				return err
			}
			for _, msg := range stored {
				extracted := linksByTelegramID[msg.TelegramMessageID]
				_, err := s.links.ReplaceForMessageTx(ctx, tx, msg.ID, extracted)
				if err != nil {
					return err
				}
				linkCount += len(extracted)
			}
		}
		if cursor > 0 {
			if err := s.channels.UpdateCursorTx(ctx, tx, channelID, cursor, time.Now().UTC()); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("store history batch: %w", err)
	}
	return linkCount, nil
}
```

This keeps `result.Messages` as the count of fetched Telegram messages while `result.Links` counts only persisted links.

Update `syncChannelOnce` is not required; it already passes `channel.ID`, `maxSeen`, and the candidate `modelMessages` into `storeBatch`.

- [ ] **Step 4: Run history tests to verify pass**

Run:

```bash
go test ./internal/history -run 'TestSyncChannelStoresBatchesLinksAndCursor|TestSyncChannel.*WatchRule|TestSyncChannelWithoutWatchRuleKeepsExistingBehavior' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit history filtering**

Run:

```bash
git add internal/history/service.go internal/history/service_test.go
git commit -m "feat: filter history sync with watch rules"
```

## Task 5: Watch Rule API

**Files:**
- Modify: `internal/api/router.go`
- Modify: `internal/api/handlers.go`
- Modify: `internal/api/handlers_test.go`

- [ ] **Step 1: Write failing API tests**

Add to `internal/api/handlers_test.go`:

```go
func TestWatchRuleAPICRUD(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	accountID, _ := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := deps.Channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 1, Title: "VIP", Type: model.ChannelTypeChannel})
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/watch-rules", bytes.NewBufferString(`{"channel_id":`+strconv.FormatInt(channelID, 10)+`,"includes":[" 庆余年 "],"excludes":["预告"]}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s, want 201", w.Code, w.Body.String())
	}
	var created model.WatchRule
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("invalid create JSON: %v", err)
	}
	if created.ID == 0 || created.ChannelID != channelID || !created.Enabled || !sameStringSlices(created.Includes, []string{"庆余年"}) {
		t.Fatalf("created = %+v", created)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/watch-rules/"+strconv.FormatInt(created.ID, 10), bytes.NewBufferString(`{"channel_id":`+strconv.FormatInt(channelID, 10)+`,"enabled":false,"includes":["三体"],"excludes":["花絮"]}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("update status = %d body=%s, want 200", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/watch-rules/"+strconv.FormatInt(created.ID, 10), nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	var got model.WatchRule
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got.Enabled || !sameStringSlices(got.Includes, []string{"三体"}) || !sameStringSlices(got.Excludes, []string{"花絮"}) {
		t.Fatalf("got = %+v", got)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/watch-rules", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte(`"items"`)) {
		t.Fatalf("list status=%d body=%s, want items", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/watch-rules/"+strconv.FormatInt(created.ID, 10), nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("delete status = %d body=%s, want 200", w.Code, w.Body.String())
	}
}

func TestWatchRuleAPIRejectsInvalidRequests(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	accountID, _ := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := deps.Channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 1, Title: "VIP", Type: model.ChannelTypeChannel})
	router := NewRouter(deps)

	for _, body := range []string{
		`{"channel_id":0}`,
		`{"channel_id":999999}`,
		`{"channel_id":` + strconv.FormatInt(channelID, 10) + `,"includes":["ok",5]}`,
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/watch-rules", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d response=%s, want 400", body, w.Code, w.Body.String())
		}
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/watch-rules", bytes.NewBufferString(`{"channel_id":`+strconv.FormatInt(channelID, 10)+`}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("first create status = %d body=%s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/watch-rules", bytes.NewBufferString(`{"channel_id":`+strconv.FormatInt(channelID, 10)+`}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d body=%s, want 409", w.Code, w.Body.String())
	}
}

func sameStringSlices(got []string, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
```

- [ ] **Step 2: Run API tests to verify they fail**

Run:

```bash
go test ./internal/api -run 'TestWatchRuleAPI' -count=1
```

Expected: FAIL because routes and dependencies do not exist.

- [ ] **Step 3: Add API dependency and routes**

Modify `internal/api/router.go`:

```go
type Dependencies struct {
	Accounts       *repository.AccountRepository
	Channels       *repository.ChannelRepository
	Messages       *repository.MessageRepository
	Links          *repository.LinkRepository
	WatchRules     *repository.WatchRuleRepository
	Maintenance    *repository.MaintenanceRepository
	Status         *repository.StatusRepository
	BackupDB       *sql.DB
	BackupDir      string
	SyncQueue      *scheduler.RetryQueue
	Search         *search.Service
	History        *history.Service
	ChannelSync    *channel.Service
	AccountRuntime AccountRuntime
	Telegram       telegram.Client
	Sessions       *session.Manager
	CodeStore      *telegram.CodeStore
}
```

Add routes:

```go
api.GET("/watch-rules", h.watchRules)
api.POST("/watch-rules", h.createWatchRule)
api.GET("/watch-rules/:id", h.watchRule)
api.PUT("/watch-rules/:id", h.updateWatchRule)
api.DELETE("/watch-rules/:id", h.deleteWatchRule)
```

- [ ] **Step 4: Add handlers**

Add to `internal/api/handlers.go`:

```go
type watchRulePayload struct {
	ChannelID int64           `json:"channel_id"`
	Enabled   *bool           `json:"enabled"`
	Includes  json.RawMessage `json:"includes"`
	Excludes  json.RawMessage `json:"excludes"`
}

func (h handlers) watchRules(c *gin.Context) {
	items, err := h.deps.WatchRules.FindAll(c.Request.Context())
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h handlers) watchRule(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	item, err := h.deps.WatchRules.FindByID(c.Request.Context(), id)
	if err != nil {
		errorJSON(c, http.StatusNotFound, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h handlers) createWatchRule(c *gin.Context) {
	rule, ok := h.readWatchRuleRequest(c, true)
	if !ok {
		return
	}
	id, err := h.deps.WatchRules.Create(c.Request.Context(), rule)
	if err != nil {
		if errors.Is(err, repository.ErrDuplicateWatchRule) {
			errorText(c, http.StatusConflict, "watch rule already exists for channel")
			return
		}
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	item, err := h.deps.WatchRules.FindByID(c.Request.Context(), id)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h handlers) updateWatchRule(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	rule, ok := h.readWatchRuleRequest(c, false)
	if !ok {
		return
	}
	rule.ID = id
	if err := h.deps.WatchRules.Update(c.Request.Context(), rule); err != nil {
		if errors.Is(err, repository.ErrDuplicateWatchRule) {
			errorText(c, http.StatusConflict, "watch rule already exists for channel")
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			errorJSON(c, http.StatusNotFound, err)
			return
		}
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	item, err := h.deps.WatchRules.FindByID(c.Request.Context(), id)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h handlers) deleteWatchRule(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.deps.WatchRules.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			errorJSON(c, http.StatusNotFound, err)
			return
		}
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (h handlers) readWatchRuleRequest(c *gin.Context, create bool) (model.WatchRule, bool) {
	var payload watchRulePayload
	if !bindJSON(c, &payload) {
		return model.WatchRule{}, false
	}
	if payload.ChannelID <= 0 {
		errorText(c, http.StatusBadRequest, "channel_id must be a positive integer")
		return model.WatchRule{}, false
	}
	if _, err := h.deps.Channels.FindByID(c.Request.Context(), payload.ChannelID); err != nil {
		errorText(c, http.StatusBadRequest, "channel_id must reference an existing channel")
		return model.WatchRule{}, false
	}
	enabled := true
	if payload.Enabled != nil {
		enabled = *payload.Enabled
	} else if !create {
		errorText(c, http.StatusBadRequest, "enabled is required")
		return model.WatchRule{}, false
	}
	includes, ok := decodeStringArray(c, payload.Includes, "includes")
	if !ok {
		return model.WatchRule{}, false
	}
	excludes, ok := decodeStringArray(c, payload.Excludes, "excludes")
	if !ok {
		return model.WatchRule{}, false
	}
	return model.WatchRule{ChannelID: payload.ChannelID, Enabled: enabled, Includes: includes, Excludes: excludes}, true
}

func decodeStringArray(c *gin.Context, raw json.RawMessage, field string) ([]string, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, true
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		errorText(c, http.StatusBadRequest, field+" must be an array of strings")
		return nil, false
	}
	return out, true
}
```

Add these imports to `internal/api/handlers.go`:

```go
"encoding/json"
"tg-provider/internal/repository"
```

Update requests require callers to send `enabled`; create requests default `enabled` to `true`.

- [ ] **Step 5: Wire test dependencies**

In `testDepsWithDB`, add:

```go
watchRules := repository.NewWatchRuleRepository(conn)
```

Return it in `Dependencies`:

```go
WatchRules: watchRules,
```

- [ ] **Step 6: Run API tests to verify pass**

Run:

```bash
go test ./internal/api -run 'TestWatchRuleAPI' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit API**

Run:

```bash
git add internal/api/router.go internal/api/handlers.go internal/api/handlers_test.go
git commit -m "feat: add watch rule api"
```

## Task 6: Runtime Wiring and Documentation

**Files:**
- Modify: `cmd/tg-provider/main.go`
- Modify: `internal/api/handlers_test.go`
- Modify: `README.md`
- Modify: `docs/api.md`

- [ ] **Step 1: Write failing compile by wiring references**

Modify `cmd/tg-provider/main.go` to create the watch rule repository and filter:

```go
watchRules := repository.NewWatchRuleRepository(conn)
watchFilter := messagefilter.New(watchRules)
```

Add import:

```go
"tg-provider/internal/messagefilter"
```

Pass `watchFilter` into `updatepkg.NewProcessor`:

```go
Filter: watchFilter,
```

Pass `watchFilter` into `history.NewService`:

```go
Filter: watchFilter,
```

Pass `watchRules` into `api.Dependencies`:

```go
WatchRules: watchRules,
```

- [ ] **Step 2: Run compile to verify missing import/wiring issues**

Run:

```bash
go test ./cmd/tg-provider ./internal/api ./internal/history ./internal/update -count=1
```

Expected: FAIL until every dependency struct and import is consistent.

- [ ] **Step 3: Wire API test dependencies**

In `internal/api/handlers_test.go`, ensure test history service gets the same filter:

```go
watchRules := repository.NewWatchRuleRepository(conn)
watchFilter := messagefilter.New(watchRules)
historyService := history.NewService(history.Options{
	DB: conn, Accounts: accounts, Channels: channels, Messages: messages, Links: links,
	Telegram: client, Sessions: sessions, Extractor: link.NewExtractor(), Filter: watchFilter, HistoryBatchSize: 100,
})
```

Add import:

```go
"tg-provider/internal/messagefilter"
```

- [ ] **Step 4: Document API**

Add to `README.md` Core APIs list:

```text
GET    /api/watch-rules
POST   /api/watch-rules
GET    /api/watch-rules/{id}
PUT    /api/watch-rules/{id}
DELETE /api/watch-rules/{id}
```

Add to `docs/api.md` after the channel API section:

```markdown
## 监听规则 API

监听规则绑定本地频道 ID。实时监听只使用 `enabled=true` 的规则；手动历史同步只要频道存在规则就应用 `includes`、`excludes` 和“必须包含链接”的过滤，并忽略 `enabled`。

### GET `/api/watch-rules`

返回所有监听规则。

### POST `/api/watch-rules`

创建监听规则。

```bash
curl -s -X POST http://127.0.0.1:6000/api/watch-rules \
  -H 'content-type: application/json' \
  -d '{"channel_id":1,"enabled":true,"includes":["庆余年"],"excludes":["预告"]}'
```

### GET `/api/watch-rules/{id}`

返回单条监听规则。

### PUT `/api/watch-rules/{id}`

更新监听规则。

### DELETE `/api/watch-rules/{id}`

删除监听规则。
```

- [ ] **Step 5: Run package tests**

Run:

```bash
go test ./internal/repository ./internal/messagefilter ./internal/update ./internal/history ./internal/api ./cmd/tg-provider -count=1
```

Expected: PASS.

- [ ] **Step 6: Run full test suite**

Run:

```bash
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit wiring and docs**

Run:

```bash
git add cmd/tg-provider/main.go internal/api/handlers_test.go README.md docs/api.md
git commit -m "docs: document watch rule filtering"
```

## Final Verification

- [ ] Run:

```bash
go test ./... -count=1
```

Expected: PASS.

- [ ] Inspect git history:

```bash
git log --oneline -5
```

Expected: includes commits for storage, filter, realtime, history, API, and docs/wiring.

- [ ] Inspect final diff against base:

```bash
git diff HEAD~6..HEAD --stat
```

Expected: only watch-rule storage, filtering, API, wiring, tests, and docs changed.
