package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"tg-provider/internal/channel"
	"tg-provider/internal/db"
	"tg-provider/internal/history"
	"tg-provider/internal/link"
	"tg-provider/internal/model"
	"tg-provider/internal/repository"
	"tg-provider/internal/search"
	"tg-provider/internal/session"
	"tg-provider/internal/telegram"
)

func TestCoreReadAPIs(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	accountID, _ := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Username: "main", Status: model.AccountStatusOnline})
	channelID, _ := deps.Channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 1, Title: "VIP", Type: model.ChannelTypeChannel})
	stored, err := deps.Messages.SaveBatch(ctx, []model.Message{
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1, Text: "庆余年 https://example.com/a", RawJSON: "{}", Date: time.Now().UTC()},
	})
	if err != nil {
		t.Fatalf("save messages: %v", err)
	}
	_, _ = deps.Links.SaveBatch(ctx, stored[0].ID, []model.Link{{Type: "url", URL: "https://example.com/a"}})
	router := NewRouter(deps)

	for _, tc := range []struct {
		path string
		key  string
	}{
		{"/api/status", "accounts"},
		{"/api/search?q=庆余年", "items"},
		{"/api/messages/latest?limit=10", "items"},
		{"/api/links?type=url", "items"},
		{"/api/accounts", "items"},
		{"/api/channels", "items"},
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s status = %d body=%s", tc.path, w.Code, w.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s invalid JSON: %v", tc.path, err)
		}
		if _, ok := body[tc.key]; !ok {
			t.Fatalf("%s response missing key %q: %s", tc.path, tc.key, w.Body.String())
		}
	}
}

func TestSearchRequiresQuery(t *testing.T) {
	router := NewRouter(testDeps(t))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/search", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestSendCodeCreatesLoginRequiredAccount(t *testing.T) {
	deps := testDeps(t)
	deps.Telegram = &fakeTelegram{}
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/login/send-code", bytes.NewBufferString(`{"phone":"+10000000000"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	account, err := deps.Accounts.FindByPhone(context.Background(), "+10000000000")
	if err != nil {
		t.Fatalf("find account: %v", err)
	}
	if account.Status != model.AccountStatusLoginRequired {
		t.Fatalf("status = %q, want LOGIN_REQUIRED", account.Status)
	}
}

func TestStatusIncludesAccountStateSummary(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	_, _ = deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	_, _ = deps.Accounts.Save(ctx, model.Account{Phone: "+10000000001", Status: model.AccountStatusReconnecting})
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		AccountStates map[string]int64 `json:"account_states"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.AccountStates[model.AccountStatusOnline] != 1 || body.AccountStates[model.AccountStatusReconnecting] != 1 {
		t.Fatalf("account_states = %+v, want ONLINE=1 RECONNECTING=1", body.AccountStates)
	}
}

func TestReadAPIsFilterByAccount(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	account1, _ := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Username: "one", Status: model.AccountStatusOnline})
	account2, _ := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000001", Username: "two", Status: model.AccountStatusOnline})
	channel1, _ := deps.Channels.Save(ctx, model.Channel{AccountID: account1, TelegramChannelID: 1, Title: "one-channel", Type: model.ChannelTypeChannel})
	channel2, _ := deps.Channels.Save(ctx, model.Channel{AccountID: account2, TelegramChannelID: 2, Title: "two-channel", Type: model.ChannelTypeChannel})
	now := time.Now().UTC()
	stored1, _ := deps.Messages.SaveBatch(ctx, []model.Message{{AccountID: account1, ChannelID: channel1, TelegramMessageID: 1, Text: "shared title one", RawJSON: "{}", Date: now}})
	stored2, _ := deps.Messages.SaveBatch(ctx, []model.Message{{AccountID: account2, ChannelID: channel2, TelegramMessageID: 2, Text: "shared title two", RawJSON: "{}", Date: now}})
	_, _ = deps.Links.SaveBatch(ctx, stored1[0].ID, []model.Link{{Type: "url", URL: "https://example.com/one"}})
	_, _ = deps.Links.SaveBatch(ctx, stored2[0].ID, []model.Link{{Type: "url", URL: "https://example.com/two"}})
	router := NewRouter(deps)

	for _, path := range []string{
		"/api/search?q=shared&account_id=" + strconv.FormatInt(account1, 10),
		"/api/messages/latest?account_id=" + strconv.FormatInt(account1, 10),
		"/api/links?account_id=" + strconv.FormatInt(account1, 10),
		"/api/channels?account_id=" + strconv.FormatInt(account1, 10),
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s status = %d body=%s", path, w.Code, w.Body.String())
		}
		if !bytes.Contains(w.Body.Bytes(), []byte("one")) {
			t.Fatalf("%s response missing account one data: %s", path, w.Body.String())
		}
		if bytes.Contains(w.Body.Bytes(), []byte("two")) || bytes.Contains(w.Body.Bytes(), []byte("https://example.com/two")) {
			t.Fatalf("%s response leaked account two data: %s", path, w.Body.String())
		}
	}
}

func TestSearchAPIFiltersByLinkType(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	accountID, _ := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Username: "main", Status: model.AccountStatusOnline})
	channelID, _ := deps.Channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 1, Title: "VIP", Type: model.ChannelTypeChannel})
	now := time.Now().UTC()
	stored, _ := deps.Messages.SaveBatch(ctx, []model.Message{
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1, Text: "shared aliyun", RawJSON: "{}", Date: now},
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 2, Text: "shared quark", RawJSON: "{}", Date: now.Add(-time.Minute)},
	})
	_, _ = deps.Links.SaveBatch(ctx, stored[0].ID, []model.Link{{Type: "aliyun", URL: "https://www.alipan.com/s/abc123"}})
	_, _ = deps.Links.SaveBatch(ctx, stored[1].ID, []model.Link{{Type: "quark", URL: "https://pan.quark.cn/s/quark123"}})
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/search?q=shared&link_type=aliyun", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("https://www.alipan.com/s/abc123")) {
		t.Fatalf("response missing aliyun link: %s", w.Body.String())
	}
	if bytes.Contains(w.Body.Bytes(), []byte("https://pan.quark.cn/s/quark123")) {
		t.Fatalf("response included quark link: %s", w.Body.String())
	}
}

func TestLinksAPIFiltersByDateRangeAndRejectsInvalidDate(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	accountID, _ := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Username: "main", Status: model.AccountStatusOnline})
	channelID, _ := deps.Channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 1, Title: "VIP", Type: model.ChannelTypeChannel})
	january := time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC)
	february := time.Date(2026, 2, 5, 12, 0, 0, 0, time.UTC)
	stored, _ := deps.Messages.SaveBatch(ctx, []model.Message{
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1, Text: "jan aliyun", RawJSON: "{}", Date: january},
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 2, Text: "feb aliyun", RawJSON: "{}", Date: february},
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

func testDeps(t *testing.T) Dependencies {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "telegram.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(context.Background(), conn); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	accounts := repository.NewAccountRepository(conn)
	channels := repository.NewChannelRepository(conn)
	messages := repository.NewMessageRepository(conn)
	links := repository.NewLinkRepository(conn)
	status := repository.NewStatusRepository(conn)
	sessions := session.NewManager(filepath.Join(t.TempDir(), "sessions"))
	client := telegram.NopClient{}
	searchService := search.NewService(messages, links)
	historyService := history.NewService(history.Options{
		DB: conn, Accounts: accounts, Channels: channels, Messages: messages, Links: links,
		Telegram: client, Sessions: sessions, Extractor: link.NewExtractor(), HistoryBatchSize: 100,
	})
	channelService := channel.NewService(channels, client, sessions)
	return Dependencies{
		Accounts: accounts, Channels: channels, Messages: messages, Links: links, Status: status,
		Search: searchService, History: historyService, ChannelSync: channelService,
		Telegram: client, Sessions: sessions, CodeStore: telegram.NewCodeStore(),
	}
}

type fakeTelegram struct {
	telegram.NopClient
}

func (fakeTelegram) SendCode(ctx context.Context, phone string, sessionPath string) (telegram.SentCode, error) {
	return telegram.SentCode{PhoneCodeHash: "hash"}, nil
}
