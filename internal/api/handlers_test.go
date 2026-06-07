package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"tg-provider/internal/channel"
	"tg-provider/internal/db"
	"tg-provider/internal/history"
	"tg-provider/internal/link"
	"tg-provider/internal/messagefilter"
	"tg-provider/internal/model"
	"tg-provider/internal/repository"
	"tg-provider/internal/retry"
	"tg-provider/internal/scheduler"
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

func TestSearchRequiresQuery(t *testing.T) {
	router := NewRouter(testDeps(t))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/search", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Error.Code != "bad_request" || body.Error.Message == "" {
		t.Fatalf("error response = %s, want standard bad_request envelope", w.Body.String())
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

func TestDeleteAccountStopsRuntimeAndRemovesSession(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	accountID, err := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	if err != nil {
		t.Fatalf("save account: %v", err)
	}
	sessionPath := deps.Sessions.PathForAccount(accountID)
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	if err := os.WriteFile(sessionPath, []byte("session"), 0o600); err != nil {
		t.Fatalf("write session: %v", err)
	}
	runtime := &recordingAccountRuntime{}
	deps.AccountRuntime = runtime
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/accounts/"+strconv.FormatInt(accountID, 10), nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if got := runtime.stoppedIDs(); !sameInt64s(got, []int64{accountID}) {
		t.Fatalf("stopped ids = %v, want [%d]", got, accountID)
	}
	if _, err := os.Stat(sessionPath); !os.IsNotExist(err) {
		t.Fatalf("session stat error = %v, want not exist", err)
	}
	if _, err := deps.Accounts.FindByID(ctx, accountID); err == nil {
		t.Fatal("FindByID succeeded after delete, want missing account")
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

func TestLatestAPIOmitsAccountFields(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	accountID, _ := deps.Accounts.Save(ctx, model.Account{
		Phone:     "+19999999999",
		FirstName: "PrivateFirst",
		Username:  "privateuser",
		Status:    model.AccountStatusOnline,
	})
	channelID, _ := deps.Channels.Save(ctx, model.Channel{
		AccountID:         accountID,
		TelegramChannelID: 1,
		Title:             "Public Channel",
		Username:          "publicchannel",
		Type:              model.ChannelTypeChannel,
	})
	_, _ = deps.Messages.SaveBatch(ctx, []model.Message{{
		AccountID:         accountID,
		ChannelID:         channelID,
		TelegramMessageID: 1,
		Text:              "latest public message",
		RawJSON:           "{}",
		Date:              time.Now().UTC(),
	}})
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/messages/latest?limit=10", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", w.Code, w.Body.String())
	}

	var body struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(body.Items) != 1 {
		t.Fatalf("items len = %d, want 1: %s", len(body.Items), w.Body.String())
	}
	item := body.Items[0]
	for _, field := range []string{"account_id", "account_phone", "account_username", "account_first_name"} {
		if _, ok := item[field]; ok {
			t.Fatalf("latest item includes %q: %s", field, w.Body.String())
		}
	}
	if item["channel_title"] != "Public Channel" || item["channel_username"] != "publicchannel" {
		t.Fatalf("latest item missing channel context: %+v", item)
	}
	if bytes.Contains(w.Body.Bytes(), []byte("+19999999999")) ||
		bytes.Contains(w.Body.Bytes(), []byte("PrivateFirst")) ||
		bytes.Contains(w.Body.Bytes(), []byte("privateuser")) {
		t.Fatalf("latest response leaked account data: %s", w.Body.String())
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

func TestMergedLinksAPI(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	accountID, _ := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Username: "main", Status: model.AccountStatusOnline})
	channelID, _ := deps.Channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 1, Title: "VIP", Type: model.ChannelTypeChannel})
	oldDate := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	newDate := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	stored, _ := deps.Messages.SaveBatch(ctx, []model.Message{
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1, Text: "庆余年 old", RawJSON: "{}", Date: oldDate},
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 2, Text: "庆余年 new", RawJSON: "{}", Date: newDate},
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 3, Text: "庆余年 aliyun", RawJSON: "{}", Date: newDate.Add(-time.Minute)},
	})
	_, _ = deps.Links.SaveBatch(ctx, stored[0].ID, []model.Link{{Type: "quark", URL: "https://pan.quark.cn/s/same", Note: "庆余年 旧"}})
	_, _ = deps.Links.SaveBatch(ctx, stored[1].ID, []model.Link{{Type: "quark", URL: "https://pan.quark.cn/s/same", Note: "庆余年 最新合集"}})
	_, _ = deps.Links.SaveBatch(ctx, stored[2].ID, []model.Link{{Type: "aliyun", URL: "https://www.alipan.com/s/abc123", Note: "庆余年 S02"}})
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/links/merged?q=庆余年&limit=10", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	var body model.MergedLinksResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Total != 2 {
		t.Fatalf("total = %d, want 2: %s", body.Total, w.Body.String())
	}
	if len(body.MergedByType["quark"]) != 1 || body.MergedByType["quark"][0].Note != "庆余年 最新合集" {
		t.Fatalf("quark merged links = %+v, want newest deduped note", body.MergedByType["quark"])
	}
	if len(body.MergedByType["aliyun"]) != 1 || body.MergedByType["aliyun"][0].Note != "庆余年 S02" {
		t.Fatalf("aliyun merged links = %+v, want aliyun note", body.MergedByType["aliyun"])
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
		"/api/search?q=x&before_date=2026-02-05T12:00:00Z",
		"/api/search?q=x&before_id=10",
		"/api/messages/latest?before_date=2026-02-05T12:00:00Z",
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		router.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d body=%s, want 400", path, w.Code, w.Body.String())
		}
	}
}

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

func TestMaintenanceBackupAPI(t *testing.T) {
	router := NewRouter(testDeps(t))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/maintenance/backup", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Path == "" {
		t.Fatalf("path is empty in response %s", w.Body.String())
	}
	if _, err := os.Stat(body.Path); err != nil {
		t.Fatalf("backup path stat: %v", err)
	}
}

func TestBatchSyncAPIValidatesChannelIDs(t *testing.T) {
	router := NewRouter(testDeps(t))
	for _, body := range []string{
		`{}`,
		`{"channel_ids":[]}`,
		`{"channel_ids":[0]}`,
		`{"channel_ids":[-1]}`,
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/channels/sync", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d body=%s, want 400", body, w.Code, w.Body.String())
		}
	}
}

func TestChannelWebAccessCheckAPIUpdatesChannelList(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	accountID, _ := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := deps.Channels.Save(ctx, model.Channel{
		AccountID:         accountID,
		TelegramChannelID: 30,
		Title:             "Public Channel",
		Username:          "public_channel",
		Type:              model.ChannelTypeChannel,
	})
	checker := &apiWebAccessChecker{results: map[string]bool{"public_channel": true}}
	deps.ChannelWebAccess = channel.NewWebAccessService(deps.Channels, checker)
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/channels/web-access/check", bytes.NewBufferString(`{"channel_ids":[`+strconv.FormatInt(channelID, 10)+`]}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	var body struct {
		Items []channel.WebAccessResult `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(body.Items) != 1 || body.Items[0].ChannelID != channelID || !body.Items[0].WebAccess || body.Items[0].CheckedAt.IsZero() {
		t.Fatalf("response = %+v, want checked channel", body)
	}
	if !sameStringSlices(checker.calls, []string{"public_channel"}) {
		t.Fatalf("checker calls = %v, want public_channel", checker.calls)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/channels", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	var list struct {
		Items []model.Channel `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("invalid channel list JSON: %v", err)
	}
	if len(list.Items) != 1 || list.Items[0].WebAccess == nil || *list.Items[0].WebAccess != true || list.Items[0].WebAccessCheckedAt == nil {
		t.Fatalf("channel list = %+v, want web_access true", list)
	}
}

func TestChannelWebAccessCheckAPIValidatesChannelIDs(t *testing.T) {
	deps := testDeps(t)
	deps.ChannelWebAccess = channel.NewWebAccessService(deps.Channels, &apiWebAccessChecker{})
	router := NewRouter(deps)
	for _, body := range []string{
		`{}`,
		`{"channel_ids":[]}`,
		`{"channel_ids":[0]}`,
		`{"channel_ids":[-1]}`,
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/channels/web-access/check", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d body=%s, want 400", body, w.Code, w.Body.String())
		}
	}
}

func TestChannelWebAccessCheckAPIRejectsMissingWithoutPartialUpdates(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	accountID, _ := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := deps.Channels.Save(ctx, model.Channel{
		AccountID:         accountID,
		TelegramChannelID: 31,
		Title:             "Existing",
		Username:          "existing_channel",
		Type:              model.ChannelTypeChannel,
	})
	checker := &apiWebAccessChecker{results: map[string]bool{"existing_channel": true}}
	deps.ChannelWebAccess = channel.NewWebAccessService(deps.Channels, checker)
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/channels/web-access/check", bytes.NewBufferString(`{"channel_ids":[`+strconv.FormatInt(channelID, 10)+`,999999]}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s, want 404", w.Code, w.Body.String())
	}
	if len(checker.calls) != 0 {
		t.Fatalf("checker calls = %v, want none", checker.calls)
	}
	stored, err := deps.Channels.FindByID(ctx, channelID)
	if err != nil {
		t.Fatalf("find channel: %v", err)
	}
	if stored.WebAccess != nil || stored.WebAccessCheckedAt != nil {
		t.Fatalf("stored web access = %v checked_at=%v, want unchanged nil values", stored.WebAccess, stored.WebAccessCheckedAt)
	}
}

func TestBatchSyncAPIReturnsAsyncJob(t *testing.T) {
	ctx := context.Background()
	deps, conn := testDepsWithDB(t)
	deps.SyncQueue = scheduler.NewRetryQueue(scheduler.RetryQueueOptions{
		Policy: retry.Policy{BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, MaxTries: 1, Sleep: func(context.Context, time.Duration) error { return nil }},
	})
	accountID, _ := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := deps.Channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 10, AccessHash: 20, Title: "VIP", Type: model.ChannelTypeChannel})
	deps.History = history.NewService(history.Options{
		DB: conn, Accounts: deps.Accounts, Channels: deps.Channels, Messages: deps.Messages, Links: deps.Links,
		Telegram:  &apiHistoryClient{date: time.Now().UTC()},
		Sessions:  session.NewManager(filepath.Join(t.TempDir(), "sessions")),
		Extractor: link.NewExtractor(), HistoryBatchSize: 10, Workers: 2,
		RetryPolicy: retry.Policy{BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, MaxTries: 1, Sleep: func(context.Context, time.Duration) error { return nil }},
	})
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/channels/sync", bytes.NewBufferString(`{"channel_ids":[`+strconv.FormatInt(channelID, 10)+`]}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d body=%s, want 202", w.Code, w.Body.String())
	}
	var body struct {
		JobID  string `json:"job_id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.JobID == "" || body.Status != "queued" {
		t.Fatalf("response = %+v, want queued job id", body)
	}
	done, err := deps.SyncQueue.Wait(ctx, body.JobID)
	if err != nil {
		t.Fatalf("wait job: %v", err)
	}
	if done.Status != scheduler.RetryJobSucceeded {
		t.Fatalf("job status = %q error=%s, want succeeded", done.Status, done.Error)
	}
}

func TestAccountChannelSyncAPIReturnsAsyncJob(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	deps.SyncQueue = scheduler.NewRetryQueue(scheduler.RetryQueueOptions{
		Policy: retry.Policy{BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, MaxTries: 1, Sleep: func(context.Context, time.Duration) error { return nil }},
	})
	accountID, _ := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelClient := &apiChannelClient{
		items: []telegram.Channel{{TelegramChannelID: 11, AccessHash: 22, Title: "Account Channel", Type: model.ChannelTypeChannel}},
	}
	deps.ChannelSync = channel.NewService(deps.Channels, channelClient, deps.Sessions)
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/accounts/"+strconv.FormatInt(accountID, 10)+"/channels/sync", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d body=%s, want 202", w.Code, w.Body.String())
	}
	var body struct {
		JobID  string `json:"job_id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.JobID == "" || body.Status != "queued" {
		t.Fatalf("response = %+v, want queued job id", body)
	}
	done, err := deps.SyncQueue.Wait(ctx, body.JobID)
	if err != nil {
		t.Fatalf("wait job: %v", err)
	}
	if done.Status != scheduler.RetryJobSucceeded {
		t.Fatalf("job status = %q error=%s, want succeeded", done.Status, done.Error)
	}
	items, err := deps.Channels.FindByAccountID(ctx, accountID)
	if err != nil {
		t.Fatalf("find channels: %v", err)
	}
	if len(items) != 1 || items[0].Title != "Account Channel" {
		t.Fatalf("channels = %+v, want synced account channel", items)
	}
}

type apiHistoryClient struct {
	telegram.NopClient
	date time.Time
}

type apiChannelClient struct {
	telegram.NopClient
	items []telegram.Channel
}

func (c *apiChannelClient) ListChannels(ctx context.Context, account telegram.AccountSession) ([]telegram.Channel, error) {
	return c.items, nil
}

func (f *apiHistoryClient) FetchHistory(ctx context.Context, account telegram.AccountSession, channel telegram.ChannelRef, offsetID int64, limit int) ([]telegram.Message, error) {
	if offsetID > 0 {
		return nil, nil
	}
	return []telegram.Message{{TelegramMessageID: 1, SenderID: 1, Text: "api sync", RawJSON: "{}", Date: f.date}}, nil
}

func testDeps(t *testing.T) Dependencies {
	t.Helper()
	deps, _ := testDepsWithDB(t)
	return deps
}

func testDepsWithDB(t *testing.T) (Dependencies, *sql.DB) {
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
	watchRules := repository.NewWatchRuleRepository(conn)
	maintenance := repository.NewMaintenanceRepository(conn)
	status := repository.NewStatusRepository(conn)
	sessions := session.NewManager(filepath.Join(t.TempDir(), "sessions"))
	client := telegram.NopClient{}
	watchFilter := messagefilter.New(watchRules)
	searchService := search.NewService(messages, links)
	historyService := history.NewService(history.Options{
		DB: conn, Accounts: accounts, Channels: channels, Messages: messages, Links: links,
		Telegram: client, Sessions: sessions, Extractor: link.NewExtractor(), Filter: watchFilter, HistoryBatchSize: 100,
	})
	channelService := channel.NewService(channels, client, sessions)
	channelWebAccessService := channel.NewWebAccessService(channels, nil)
	return Dependencies{
		Accounts: accounts, Channels: channels, Messages: messages, Links: links, WatchRules: watchRules, Maintenance: maintenance, Status: status,
		BackupDB: conn, BackupDir: filepath.Join(t.TempDir(), "backup"),
		Search: searchService, History: historyService, ChannelSync: channelService, ChannelWebAccess: channelWebAccessService,
		Telegram: client, Sessions: sessions, CodeStore: telegram.NewCodeStore(),
	}, conn
}

type fakeTelegram struct {
	telegram.NopClient
}

func (fakeTelegram) SendCode(ctx context.Context, phone string, sessionPath string) (telegram.SentCode, error) {
	return telegram.SentCode{PhoneCodeHash: "hash"}, nil
}

type apiWebAccessChecker struct {
	results map[string]bool
	calls   []string
}

func (c *apiWebAccessChecker) Check(ctx context.Context, username string) (bool, error) {
	c.calls = append(c.calls, username)
	return c.results[username], nil
}

type recordingAccountRuntime struct {
	mu      sync.Mutex
	stopped []int64
}

func (r *recordingAccountRuntime) StopAccount(ctx context.Context, accountID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopped = append(r.stopped, accountID)
	return nil
}

func (r *recordingAccountRuntime) stoppedIDs() []int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]int64, len(r.stopped))
	copy(out, r.stopped)
	return out
}

func sameInt64s(got []int64, want []int64) bool {
	if len(got) != len(want) {
		return false
	}
	seen := map[int64]int{}
	for _, id := range got {
		seen[id]++
	}
	for _, id := range want {
		seen[id]--
	}
	for _, count := range seen {
		if count != 0 {
			return false
		}
	}
	return true
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
