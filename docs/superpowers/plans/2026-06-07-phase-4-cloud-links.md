# Phase 4 Cloud Links Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add provider-specific cloud drive link parsing, parser corpus coverage, provider-aware search coverage, and `/api/links` date range filters.

**Architecture:** Keep `link.NewExtractor().Extract(text)` as the public API and refactor internals around small regex-backed parsers. Existing history sync and realtime update processing already call the extractor, so integration work is mostly stronger assertions that provider-specific `telegram_links.type` values flow through. Extend links query params with parsed date bounds that filter by message date.

**Tech Stack:** Go, regexp, SQLite, gin, existing `internal/link`, `internal/repository`, `internal/search`, `internal/history`, `internal/update`, and `internal/api` packages.

---

## File Structure

- Modify `internal/link/extractor_test.go`: replace basic tests with provider corpus, real message corpus, password, dedupe, and fallback tests.
- Modify `internal/link/extractor.go`: add internal parser abstraction, provider regex parsers, protocol parsers, generic fallback parser, normalized dedupe, and expanded password extraction.
- Modify `internal/history/service_test.go`: assert history sync stores provider-specific link types.
- Modify `internal/update/processor_test.go`: assert update new/edit stores and replaces provider-specific link types.
- Modify `internal/search/service_test.go`: assert `LinkType` filters provider-specific links.
- Modify `internal/repository/types.go`: add link date bounds to `LinkSearchParams`.
- Modify `internal/repository/link.go`: apply message-date filters to link search.
- Modify `internal/search/service.go`: expose link date bounds from search params.
- Modify `internal/api/handlers.go`: parse `date_from` and `date_to` for `/api/links`, returning 400 for invalid dates.
- Modify `internal/api/handlers_test.go`: assert combined `/api/links` filters and invalid date handling.

## Task 1: Parser Corpus Red Tests

**Files:**
- Modify: `internal/link/extractor_test.go`

- [ ] **Step 1: Replace extractor tests with provider corpus**

Replace `internal/link/extractor_test.go` with:

```go
package link

import "testing"

func TestExtractProviderCorpus(t *testing.T) {
	extractor := NewExtractor()
	cases := []struct {
		name     string
		text     string
		wantType string
		wantURL  string
		wantPass string
	}{
		{"115", "https://115.com/s/abc-123?password=a1B2", "115", "https://115.com/s/abc-123?password=a1B2", "a1B2"},
		{"115cdn", "https://115cdn.com/s/share_1", "115", "https://115cdn.com/s/share_1", ""},
		{"anxia", "https://anxia.com/s/share-2 密码: z9", "115", "https://anxia.com/s/share-2", "z9"},
		{"xunlei", "https://pan.xunlei.com/s/VOuNMeKrMwroW9HmY21cZWfPA1?pwd=kewd#", "xunlei", "https://pan.xunlei.com/s/VOuNMeKrMwroW9HmY21cZWfPA1?pwd=kewd", "kewd"},
		{"baidu share", "https://pan.baidu.com/s/1Zc_e4792cuvucfI-ZZts0Q?pwd=ruub", "baidu", "https://pan.baidu.com/s/1Zc_e4792cuvucfI-ZZts0Q?pwd=ruub", "ruub"},
		{"baidu init", "https://pan.baidu.com/share/init?surl=abc-123&pwd=7788", "baidu", "https://pan.baidu.com/share/init?surl=abc-123&pwd=7788", "7788"},
		{"pikpak", "https://mypikpak.com/s/Vabc123?pwd=p9", "pikpak", "https://mypikpak.com/s/Vabc123?pwd=p9", "p9"},
		{"tianyi web", "https://cloud.189.cn/web/share?code=AbCd", "tianyi", "https://cloud.189.cn/web/share?code=AbCd", ""},
		{"tianyi t code", "https://cloud.189.cn/t/AbCd（访问码：7x9q）", "tianyi", "https://cloud.189.cn/t/AbCd", "7x9q"},
		{"tianyi h5", "https://h5.cloud.189.cn/share.html#/t/AbCd", "tianyi", "https://h5.cloud.189.cn/share.html#/t/AbCd", ""},
		{"mobile caiyun m", "https://caiyun.139.com/m/i?abc123", "mobile", "https://caiyun.139.com/m/i?abc123", ""},
		{"mobile yun shareweb", "https://yun.139.com/shareweb/#/w/i/abc123", "mobile", "https://yun.139.com/shareweb/#/w/i/abc123", ""},
		{"mobile caiyun w", "https://caiyun.139.com/w/i/abc123", "mobile", "https://caiyun.139.com/w/i/abc123", ""},
		{"quark", "https://pan.quark.cn/s/8a16ab9c06b9", "quark", "https://pan.quark.cn/s/8a16ab9c06b9", ""},
		{"uc password", "https://drive.uc.cn/s/d5eaad53?password=xy9z", "uc", "https://drive.uc.cn/s/d5eaad53?password=xy9z", "xy9z"},
		{"uc public", "https://drive.uc.cn/s/d5eaad53da684?public=1", "uc", "https://drive.uc.cn/s/d5eaad53da684?public=1", ""},
		{"uc fast", "https://fast.uc.cn/s/abc123", "uc", "https://fast.uc.cn/s/abc123", ""},
		{"aliyun folder", "https://www.aliyundrive.com/s/abc123/folder/folder456?password=qwer", "aliyun", "https://www.aliyundrive.com/s/abc123/folder/folder456?password=qwer", "qwer"},
		{"alipan", "https://www.alipan.com/s/MHf34XusdVK", "aliyun", "https://www.alipan.com/s/MHf34XusdVK", ""},
		{"123 inline", "https://123pan.com/s/abc123提取码:9a8b", "123", "https://123pan.com/s/abc123", "9a8b"},
		{"123 html", "https://www.123pan.com/s/abc123.html?提取码:9a8b", "123", "https://www.123pan.com/s/abc123.html", "9a8b"},
		{"guangya", "https://www.guangyapan.com/s/ABC_123", "guangya", "https://www.guangyapan.com/s/ABC_123", ""},
		{"magnet", "magnet:?xt=urn:btih:abcdef", "magnet", "magnet:?xt=urn:btih:abcdef", ""},
		{"ed2k", "ed2k://|file|movie.mkv|123|HASH|/", "ed2k", "ed2k://|file|movie.mkv|123|HASH|/", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			links := extractor.Extract(tc.text)
			if len(links) != 1 {
				t.Fatalf("len = %d, want 1: %+v", len(links), links)
			}
			if links[0].Type != tc.wantType || links[0].URL != tc.wantURL || links[0].Password != tc.wantPass {
				t.Fatalf("link = %+v, want type=%s url=%s password=%s", links[0], tc.wantType, tc.wantURL, tc.wantPass)
			}
		})
	}
}

func TestExtractRealMessageCorpus(t *testing.T) {
	text := `海报
名称：2026年6月6日 短剧更新目录12

链接：
🔗 夸克网盘：https://pan.quark.cn/s/8a16ab9c06b9
🔗 百度网盘：https://pan.baidu.com/s/1Zc_e4792cuvucfI-ZZts0Q?pwd=ruub
🔑 提取码：ruub
🔗 UC 网盘：https://drive.uc.cn/s/d5eaad53da684?public=1
🔗 迅雷云盘：https://pan.xunlei.com/s/VOuNMeKrMwroW9HmY21cZWfPA1?pwd=kewd#
🔑 提取码：kewd
🔗 阿里云盘：https://www.alipan.com/s/MHf34XusdVK

🏷 标签：#短剧 #最新短剧 #合集
📢 频道：https://t.me/+Djia5z2lVsI5ODRl
👥 群组：@Quark_Share_Group (https://t.me/Quark_Share_Group)
🤖 投稿：@QuarkRobot (https://t.me/QuarkRobot)`

	links := NewExtractor().Extract(text)
	byType := map[string][]string{}
	for _, item := range links {
		byType[item.Type] = append(byType[item.Type], item.URL)
	}
	want := map[string]string{
		"quark":  "https://pan.quark.cn/s/8a16ab9c06b9",
		"baidu":  "https://pan.baidu.com/s/1Zc_e4792cuvucfI-ZZts0Q?pwd=ruub",
		"uc":     "https://drive.uc.cn/s/d5eaad53da684?public=1",
		"xunlei": "https://pan.xunlei.com/s/VOuNMeKrMwroW9HmY21cZWfPA1?pwd=kewd",
		"aliyun": "https://www.alipan.com/s/MHf34XusdVK",
	}
	for typ, url := range want {
		if !contains(byType[typ], url) {
			t.Fatalf("missing %s %s in links %+v", typ, url, links)
		}
	}
	for _, typ := range []string{"quark", "baidu", "uc", "xunlei", "aliyun"} {
		if len(byType[typ]) != 1 {
			t.Fatalf("type %s count = %d, want 1: %+v", typ, len(byType[typ]), links)
		}
	}
	if len(byType["url"]) != 3 {
		t.Fatalf("fallback url count = %d, want 3 telegram links: %+v", len(byType["url"]), byType["url"])
	}
}

func TestExtractDeduplicatesProviderAndFallback(t *testing.T) {
	links := NewExtractor().Extract("https://pan.quark.cn/s/abc123 https://pan.quark.cn/s/abc123")
	if len(links) != 1 {
		t.Fatalf("len = %d, want 1: %+v", len(links), links)
	}
	if links[0].Type != "quark" {
		t.Fatalf("type = %q, want quark", links[0].Type)
	}
}

func TestExtractFallbackURLAndFalsePositive(t *testing.T) {
	links := NewExtractor().Extract("官网 https://example.com/a 不是网盘 pan.baidu.com/s/no-scheme")
	if len(links) != 1 {
		t.Fatalf("len = %d, want 1: %+v", len(links), links)
	}
	if links[0].Type != "url" || links[0].URL != "https://example.com/a" {
		t.Fatalf("link = %+v, want fallback url", links[0])
	}
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/link
```

Expected: FAIL because current extractor returns provider links as `url`, does not support all provider patterns, and does not trim trailing `#`.

## Task 2: Provider Parser Implementation

**Files:**
- Modify: `internal/link/extractor.go`

- [ ] **Step 1: Implement parser abstraction and provider patterns**

Replace `internal/link/extractor.go` with an implementation that defines:

```go
type Parser interface {
	Extract(text string) []Candidate
}

type Candidate struct {
	Type       string
	URL        string
	Password   string
	MatchStart int
	MatchEnd   int
}

type Extractor struct {
	parsers         []Parser
	passwordPattern *regexp.Regexp
}
```

`NewExtractor` must initialize parsers in this order:

```go
func NewExtractor() *Extractor {
	return &Extractor{
		parsers: []Parser{
			providerParser("115", `(?i)(https://(?:115|115cdn|anxia)\.com/s/[\w-]+(?:\?password=([\w-]+))?)`, 1, 2),
			providerParser("xunlei", `(?i)(https://pan\.xunlei\.com/s/[\w-]+(?:\?pwd=([\w-]+))?)`, 1, 2),
			providerParser("baidu", `(?i)(https://pan\.baidu\.com/s/[\w-]+(?:\?pwd=([\w-]+))?)`, 1, 2),
			providerParser("baidu", `(?i)(https://pan\.baidu\.com/(?:share|wap)/init\?surl=[\w-]+(?:&pwd=([\w-]+))?)`, 1, 2),
			providerParser("pikpak", `(?i)(https://mypikpak\.com/s/[\w-]+(?:\?pwd=([\w-]+))?)`, 1, 2),
			providerParser("tianyi", `(?i)(https://cloud\.189\.cn/web/share\?code=[\w-]+)`, 1, 0),
			providerParser("tianyi", `(?i)(https://cloud\.189\.cn/t/[\w-]+)(?:（访问码：(\w+)）)?`, 1, 2),
			providerParser("tianyi", `(?i)(https://h5\.cloud\.189\.cn/share\.html#/t/[\w-]+)`, 1, 0),
			providerParser("mobile", `(?i)(https://caiyun\.139\.com/m/i\?[\w-]+)`, 1, 0),
			providerParser("mobile", `(?i)(https://yun\.139\.com/shareweb/#/w/i/[\w-]+)`, 1, 0),
			providerParser("mobile", `(?i)(https://caiyun\.139\.com/w/i/[\w-]+)`, 1, 0),
			providerParser("quark", `(?i)(https://pan\.quark\.cn/s/[\w-]+)`, 1, 0),
			providerParser("uc", `(?i)(https://(?:drive|fast)\.uc\.cn/s/[\w-]+(?:\?[^\s"'<>，。；、]*)?)`, 1, 0),
			providerParser("aliyun", `(?i)(https://www\.(?:alipan|aliyundrive)\.com/s/[\w-]+(?:/folder/[\w-]+)?(?:\?password=([\w-]+))?)`, 1, 2),
			providerParser("123", `(?i)(https://(?:www\.)?123[A-Za-z0-9]{3}\.com/s/[\w-]+(?:\.html)?)(?:\??提取码[:：](\w+))?`, 1, 2),
			providerParser("guangya", `(?i)(https://(?:www\.)?guangyapan\.com/s/[A-Za-z0-9_-]+)`, 1, 0),
			providerParser("magnet", `(?i)(magnet:\?[^\s"'<>，。；、]+)`, 1, 0),
			providerParser("ed2k", `(?i)(ed2k://[^\s"'<>，。；、]+)`, 1, 0),
			providerParser("url", `(?i)(https?://[^\s"'<>，。；、]+)`, 1, 0),
		},
		passwordPattern: regexp.MustCompile(`(?i)(?:密码|提取码|验证码|访问码|分享密码|密钥|pwd|password|code|share_pwd|pass_code|#)[=:：\s]*([A-Za-z0-9]{1,4})`),
	}
}
```

The implementation must:

- Use `FindAllStringSubmatchIndex`.
- Use the URL capture group as the stored URL.
- Use the password capture group when present.
- Fall back to extracting query password from `pwd`, `password`, `code`, `share_pwd`, or `pass_code`.
- Fill missing passwords from nearby text after the URL.
- Trim trailing punctuation with `strings.TrimRight(url, ".,;:!?)]}）】》\"'，#")`.
- Deduplicate by normalized URL and skip generic fallback URLs that have an existing provider URL prefix.

- [ ] **Step 2: Run link tests**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/link
```

Expected: PASS.

- [ ] **Step 3: Commit parser implementation**

```bash
git add internal/link/extractor.go internal/link/extractor_test.go
git commit -m "feat: parse cloud drive link types"
```

## Task 3: History, Update, And Search Integration

**Files:**
- Modify: `internal/history/service_test.go`
- Modify: `internal/update/processor_test.go`
- Modify: `internal/search/service_test.go`
- Modify: `internal/api/handlers_test.go`

- [ ] **Step 1: Strengthen history sync test**

In `internal/history/service_test.go`, change the fake batch message containing `https://example.com/a` to:

```go
{TelegramMessageID: 10, SenderID: 1, Text: "庆余年 https://www.alipan.com/s/abc123 提取码: abcd", RawJSON: "{}", Date: now.Add(-time.Minute)}
```

After the existing count assertions, add:

```go
linkResults, err := links.Search(ctx, repository.LinkSearchParams{Type: "aliyun", Limit: 10})
if err != nil {
	t.Fatalf("search aliyun links: %v", err)
}
if len(linkResults) != 1 || linkResults[0].Type != "aliyun" {
	t.Fatalf("aliyun links = %+v, want 1 aliyun link", linkResults)
}
```

- [ ] **Step 2: Strengthen update processor test**

In `internal/update/processor_test.go`, change the new event text to:

```go
Text: "庆余年 https://pan.quark.cn/s/old123 提取码: abcd",
```

Update the assertion after new-event search:

```go
if results[0].Links[0].Type != "quark" || results[0].Links[0].URL != "https://pan.quark.cn/s/old123" || results[0].Links[0].Password != "abcd" {
	t.Fatalf("new event link = %+v", results[0].Links[0])
}
```

Change the edit event text to:

```go
Text: "三体 https://pan.baidu.com/s/new123?pwd=wxyz",
```

Update the edited link assertion:

```go
if len(results[0].Links) != 1 || results[0].Links[0].Type != "baidu" || results[0].Links[0].URL != "https://pan.baidu.com/s/new123?pwd=wxyz" || results[0].Links[0].Password != "wxyz" {
	t.Fatalf("edited links = %+v", results[0].Links)
}
```

- [ ] **Step 3: Add search service link_type test**

In `internal/search/service_test.go`, save two messages, one with an `aliyun` link and one with a `quark` link, then assert:

```go
results, err := service.Search(ctx, Params{Query: "庆余年", LinkType: "aliyun", Limit: 10})
if err != nil {
	t.Fatalf("Search returned error: %v", err)
}
if len(results) != 1 || results[0].Links[0].Type != "aliyun" {
	t.Fatalf("filtered search results = %+v, want only aliyun", results)
}
```

Keep the existing unfiltered search/latest/links assertions.

- [ ] **Step 4: Add API search link_type assertion**

In `internal/api/handlers_test.go`, extend `TestReadAPIsFilterByAccount` or add a focused test that stores an `aliyun` link and a `quark` link, calls:

```text
/api/search?q=shared&link_type=aliyun
```

and asserts the response contains the aliyun URL but not the quark URL.

- [ ] **Step 5: Run integration target tests**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/history ./internal/update ./internal/search ./internal/api
```

Expected: PASS.

- [ ] **Step 6: Commit integration coverage**

```bash
git add internal/history/service_test.go internal/update/processor_test.go internal/search/service_test.go internal/api/handlers_test.go
git commit -m "test: cover cloud link integration"
```

## Task 4: Links API Date Range

**Files:**
- Modify: `internal/repository/types.go`
- Modify: `internal/repository/link.go`
- Modify: `internal/search/service.go`
- Modify: `internal/search/service_test.go`
- Modify: `internal/api/handlers.go`
- Modify: `internal/api/handlers_test.go`

- [ ] **Step 1: Write failing repository/search date range tests**

In `internal/search/service_test.go`, add two linked messages with dates `2026-01-05` and `2026-02-05`. Then assert:

```go
from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
to := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
linkResults, err := service.Links(ctx, LinkParams{Type: "aliyun", DateFrom: &from, DateTo: &to, Limit: 10})
if err != nil {
	t.Fatalf("Links date range returned error: %v", err)
}
if len(linkResults) != 1 || linkResults[0].MessageDate.Month() != time.January {
	t.Fatalf("date filtered links = %+v, want only January aliyun link", linkResults)
}
```

This should fail because `LinkParams` has no date fields.

- [ ] **Step 2: Write failing API date tests**

In `internal/api/handlers_test.go`, add:

```go
func TestLinksAPIFiltersByDateRangeAndRejectsInvalidDate(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	accountID, _ := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Username: "main", Status: model.AccountStatusOnline})
	channelID, _ := deps.Channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 1, Title: "VIP", Type: model.ChannelTypeChannel})
	january := time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC)
	february := time.Date(2026, 2, 5, 12, 0, 0, 0, time.UTC)
	stored, _ := deps.Messages.SaveBatch(ctx, []model.Message{
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1, Text: "jan", RawJSON: "{}", Date: january},
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 2, Text: "feb", RawJSON: "{}", Date: february},
	})
	_, _ = deps.Links.SaveBatch(ctx, stored[0].ID, []model.Link{{Type: "aliyun", URL: "https://www.alipan.com/s/jan"}})
	_, _ = deps.Links.SaveBatch(ctx, stored[1].ID, []model.Link{{Type: "aliyun", URL: "https://www.alipan.com/s/feb"}})
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/links?type=aliyun&date_from=2026-01-01&date_to=2026-01-31", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("jan")) || bytes.Contains(w.Body.Bytes(), []byte("feb")) {
		t.Fatalf("date range response = %s, want jan only", w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/links?date_from=not-a-date", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid date status = %d body=%s, want 400", w.Code, w.Body.String())
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/search ./internal/api
```

Expected: FAIL because date fields and API parsing are missing.

- [ ] **Step 4: Add date fields and repository filters**

Modify `internal/repository/types.go`:

```go
type LinkSearchParams struct {
	Type      string
	AccountID int64
	ChannelID int64
	Keyword   string
	DateFrom  *time.Time
	DateTo    *time.Time
	Limit     int
	Offset    int
}
```

Add `time` to the imports in `internal/repository/types.go`.

Modify `internal/repository/link.go` in `Search`:

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

- [ ] **Step 5: Pass date fields through search service**

Modify `internal/search/service.go`:

```go
type LinkParams struct {
	Type      string
	AccountID int64
	ChannelID int64
	Keyword   string
	DateFrom  *time.Time
	DateTo    *time.Time
	Limit     int
	Offset    int
}
```

Add `time` to the imports and pass `DateFrom`/`DateTo` into `repository.LinkSearchParams`.

- [ ] **Step 6: Parse API date range**

Modify `internal/api/handlers.go` links handler:

```go
dateFrom, dateTo, ok := parseDateRange(c)
if !ok {
	return
}
items, err := h.deps.Search.Links(c.Request.Context(), searchsvc.LinkParams{
	Type:      c.Query("type"),
	AccountID: queryInt(c, "account_id"),
	ChannelID: queryInt(c, "channel_id"),
	Keyword:   c.Query("keyword"),
	DateFrom:  dateFrom,
	DateTo:    dateTo,
	Limit:     queryIntValue(c, "limit"),
	Offset:    queryIntValue(c, "offset"),
})
```

Add helpers in `internal/api/handlers.go`:

```go
func parseDateRange(c *gin.Context) (*time.Time, *time.Time, bool) {
	from, ok := parseDateQuery(c, "date_from", false)
	if !ok {
		return nil, nil, false
	}
	to, ok := parseDateQuery(c, "date_to", true)
	if !ok {
		return nil, nil, false
	}
	return from, to, true
}

func parseDateQuery(c *gin.Context, key string, end bool) (*time.Time, bool) {
	value := c.Query(key)
	if value == "" {
		return nil, true
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		if end {
			t = t.Add(time.Nanosecond)
		}
		return &t, true
	}
	if t, err := time.Parse("2006-01-02", value); err == nil {
		if end {
			t = t.AddDate(0, 0, 1)
		}
		return &t, true
	}
	errorText(c, http.StatusBadRequest, key+" must be YYYY-MM-DD or RFC3339")
	return nil, false
}
```

The existing `time` import already exists in `handlers.go`.

- [ ] **Step 7: Run date range tests**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/search ./internal/api ./internal/repository
```

Expected: PASS.

- [ ] **Step 8: Commit date range support**

```bash
git add internal/repository/types.go internal/repository/link.go internal/search/service.go internal/search/service_test.go internal/api/handlers.go internal/api/handlers_test.go
git commit -m "feat: add links date range filters"
```

## Task 5: Final Verification

**Files:**
- Verify all modified files.

- [ ] **Step 1: Run all tests**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./...
```

Expected: PASS for every package.

- [ ] **Step 2: Run build**

Run:

```bash
GOCACHE=/tmp/go-build-cache go build ./...
```

Expected: PASS with exit code 0.

- [ ] **Step 3: Check git status**

Run:

```bash
git status --short --branch
```

Expected: on `phase-4-cloud-links` with no uncommitted changes.

- [ ] **Step 4: Review recent commits**

Run:

```bash
git log --oneline --decorate -8
```

Expected: shows Phase 4 design, plan, and implementation commits on `phase-4-cloud-links`.

## Self-Review

Spec coverage:

- Task 054 is covered by Task 2 parser abstraction.
- Tasks 055-064 are covered by Task 1 corpus and Task 2 provider patterns.
- Task 065 is covered by Task 1 magnet/ED2K corpus.
- Task 066 is covered by Task 1 provider, real-message, fallback, false-positive, and dedupe tests.
- Task 067 is covered by Task 3 history assertions.
- Task 068 is covered by Task 3 update processor assertions.
- Task 069 is covered by Task 3 search/API link_type assertions.
- Task 070 is covered by Task 4 links API date range and combined filters.

Placeholder scan: all required work is defined in the tasks above.

Type consistency: `Candidate`, `Parser`, `LinkSearchParams.DateFrom`, `LinkSearchParams.DateTo`, `search.LinkParams.DateFrom`, and `search.LinkParams.DateTo` are introduced before later tasks use them.
