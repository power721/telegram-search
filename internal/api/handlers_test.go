package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"tg-search/internal/adminauth"
	"tg-search/internal/apikey"
	"tg-search/internal/channel"
	"tg-search/internal/config"
	"tg-search/internal/db"
	"tg-search/internal/history"
	"tg-search/internal/link"
	"tg-search/internal/messagefilter"
	"tg-search/internal/model"
	"tg-search/internal/repository"
	"tg-search/internal/resource"
	"tg-search/internal/retry"
	"tg-search/internal/scheduler"
	"tg-search/internal/search"
	"tg-search/internal/session"
	"tg-search/internal/storage"
	taskpkg "tg-search/internal/task"
	"tg-search/internal/telegram"
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
		withAPIKey(t, deps, req)
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

func TestGlobalSearchAPI(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	files := repository.NewFileRepository(deps.BackupDB)
	accountID, _ := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Username: "main", Status: model.AccountStatusOnline})
	channelID, _ := deps.Channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 1, Title: "Ubuntu Channel", Username: "ubuntu_resources", Type: model.ChannelTypeChannel})
	stored, err := deps.Messages.SaveBatch(ctx, []model.Message{{
		AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1,
		Text: "ubuntu release mirror", RawJSON: "{}", Date: time.Now().UTC(),
	}})
	if err != nil {
		t.Fatalf("save messages: %v", err)
	}
	if _, err := deps.Links.SaveBatch(ctx, stored[0].ID, []model.Link{{Type: "url", Category: "http", URL: "https://example.com/ubuntu", Note: "ubuntu download"}}); err != nil {
		t.Fatalf("save links: %v", err)
	}
	if _, err := files.SaveBatch(ctx, stored[0].ID, []model.File{{FileName: "ubuntu.iso", Extension: ".iso", MimeType: "application/x-iso9660-image", SizeBytes: 5000}}); err != nil {
		t.Fatalf("save files: %v", err)
	}
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/search/global?q=ubuntu", nil)
	withAPIKey(t, deps, req)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	var body model.GlobalSearchResult
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Messages.Total != 1 || body.Links.Total != 1 || body.Files.Total != 1 || body.Channels.Total != 1 {
		t.Fatalf("global search body = %+v, want one item per group", body)
	}
}

func TestResourcesAPI(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	files := repository.NewFileRepository(deps.BackupDB)
	accountID, _ := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Username: "main", Status: model.AccountStatusOnline})
	channelID, _ := deps.Channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 1, Title: "VIP", Type: model.ChannelTypeChannel})
	stored, err := deps.Messages.SaveBatch(ctx, []model.Message{{
		AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1,
		Text: "ubuntu resources", RawJSON: "{}", Date: time.Now().UTC(),
	}})
	if err != nil {
		t.Fatalf("save messages: %v", err)
	}
	if _, err := deps.Links.SaveBatch(ctx, stored[0].ID, []model.Link{{Type: "url", Category: "http", URL: "https://example.com/ubuntu", Note: "ubuntu"}}); err != nil {
		t.Fatalf("save links: %v", err)
	}
	if _, err := files.SaveBatch(ctx, stored[0].ID, []model.File{{FileName: "ubuntu.iso", Extension: ".iso", SizeBytes: 5000, Category: "software"}}); err != nil {
		t.Fatalf("save files: %v", err)
	}
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/resources?q=ubuntu", nil)
	withAPIKey(t, deps, req)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	var body resource.ListResult
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Total != 2 || body.Grouped["http"] != 1 || body.Grouped["files"] != 1 {
		t.Fatalf("resources body = %+v, want link and file", body)
	}
}

func TestResourcesGroupedReturnsGlobalCountsOutsideListWindow(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	accountID, _ := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Username: "main", Status: model.AccountStatusOnline})
	channelID, _ := deps.Channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 1, Title: "VIP", Type: model.ChannelTypeChannel})
	now := time.Now().UTC()

	messages := make([]model.Message, 0, 202)
	for i := 0; i < 201; i++ {
		messages = append(messages, model.Message{
			AccountID: accountID, ChannelID: channelID, TelegramMessageID: int64(i + 1),
			Text: "http resource", RawJSON: "{}", Date: now.Add(time.Duration(i) * time.Minute),
		})
	}
	messages = append(messages, model.Message{
		AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1000,
		Text: "cloud resource", RawJSON: "{}", Date: now.Add(-time.Hour),
	})
	stored, err := deps.Messages.SaveBatch(ctx, messages)
	if err != nil {
		t.Fatalf("save messages: %v", err)
	}
	for i := 0; i < 201; i++ {
		if _, err := deps.Links.SaveBatch(ctx, stored[i].ID, []model.Link{{
			Type: "url", Category: "http", URL: "https://example.com/" + strconv.Itoa(i),
		}}); err != nil {
			t.Fatalf("save http link %d: %v", i, err)
		}
	}
	if _, err := deps.Links.SaveBatch(ctx, stored[201].ID, []model.Link{{
		Type: "aliyun", Category: "cloud_drive", URL: "https://www.alipan.com/s/older",
	}}); err != nil {
		t.Fatalf("save cloud link: %v", err)
	}

	router := NewRouter(deps)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/resources/grouped", nil)
	withAPIKey(t, deps, req)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	var body struct {
		Grouped map[string]int `json:"grouped"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Grouped["http"] != 201 || body.Grouped["cloud_drive"] != 1 {
		t.Fatalf("grouped = %+v, want http=201 cloud_drive=1", body.Grouped)
	}
}

func TestLinksGroupedReturnsCountsByLinkType(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	accountID, _ := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Username: "main", Status: model.AccountStatusOnline})
	channelID, _ := deps.Channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 1, Title: "VIP", Type: model.ChannelTypeChannel})
	stored, err := deps.Messages.SaveBatch(ctx, []model.Message{
		{
			AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1,
			Text: "cloud resources", RawJSON: "{}", Date: time.Now().UTC(),
		},
		{
			AccountID: accountID, ChannelID: channelID, TelegramMessageID: 2,
			Text: "magnet resources", RawJSON: "{}", Date: time.Now().UTC(),
		},
	})
	if err != nil {
		t.Fatalf("save messages: %v", err)
	}
	if _, err := deps.Links.SaveBatch(ctx, stored[0].ID, []model.Link{
		{Type: "aliyun", Category: "cloud_drive", URL: "https://www.alipan.com/s/a"},
		{Type: "quark", Category: "cloud_drive", URL: "https://pan.quark.cn/s/b"},
	}); err != nil {
		t.Fatalf("save cloud links: %v", err)
	}
	if _, err := deps.Links.SaveBatch(ctx, stored[1].ID, []model.Link{
		{Type: "magnet", Category: "magnet", URL: "magnet:?xt=urn:btih:abc"},
	}); err != nil {
		t.Fatalf("save magnet links: %v", err)
	}

	router := NewRouter(deps)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/links/grouped", nil)
	withAPIKey(t, deps, req)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	var body struct {
		Grouped map[string]int `json:"grouped"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Grouped["aliyun"] != 1 || body.Grouped["quark"] != 1 || body.Grouped["magnet"] != 1 {
		t.Fatalf("grouped = %+v, want counts by original link type", body.Grouped)
	}
}

func TestResourcesAPIUsesCompleteGroupedCountsWithoutKeyword(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	accountID, _ := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Username: "main", Status: model.AccountStatusOnline})
	channelID, _ := deps.Channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 1, Title: "VIP", Type: model.ChannelTypeChannel})
	now := time.Now().UTC()

	messages := make([]model.Message, 0, 202)
	for i := 0; i < 201; i++ {
		messages = append(messages, model.Message{
			AccountID: accountID, ChannelID: channelID, TelegramMessageID: int64(i + 1),
			Text: "http resource", RawJSON: "{}", Date: now.Add(time.Duration(i) * time.Minute),
		})
	}
	messages = append(messages, model.Message{
		AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1000,
		Text: "cloud resource", RawJSON: "{}", Date: now.Add(-time.Hour),
	})
	stored, err := deps.Messages.SaveBatch(ctx, messages)
	if err != nil {
		t.Fatalf("save messages: %v", err)
	}
	for i := 0; i < 201; i++ {
		if _, err := deps.Links.SaveBatch(ctx, stored[i].ID, []model.Link{{
			Type: "url", Category: "http", URL: "https://example.com/" + strconv.Itoa(i),
		}}); err != nil {
			t.Fatalf("save http link %d: %v", i, err)
		}
	}
	if _, err := deps.Links.SaveBatch(ctx, stored[201].ID, []model.Link{{
		Type: "aliyun", Category: "cloud_drive", URL: "https://www.alipan.com/s/older",
	}}); err != nil {
		t.Fatalf("save cloud link: %v", err)
	}

	router := NewRouter(deps)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/resources?limit=50&offset=100", nil)
	withAPIKey(t, deps, req)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	var body resource.ListResult
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(body.Items) != 50 {
		t.Fatalf("items len = %d, want 50", len(body.Items))
	}
	if body.Total != 202 {
		t.Fatalf("total = %d, want complete resource count", body.Total)
	}
	if body.Grouped["http"] != 201 || body.Grouped["cloud_drive"] != 1 {
		t.Fatalf("grouped = %+v, want complete http=201 cloud_drive=1", body.Grouped)
	}
}

func TestResourcesAPIUsesCompleteGroupedCountsWithKeyword(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	accountID, _ := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Username: "main", Status: model.AccountStatusOnline})
	channelID, _ := deps.Channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 1, Title: "VIP", Type: model.ChannelTypeChannel})
	now := time.Now().UTC()

	messages := make([]model.Message, 0, 202)
	for i := 0; i < 201; i++ {
		messages = append(messages, model.Message{
			AccountID: accountID, ChannelID: channelID, TelegramMessageID: int64(i + 1),
			Text: "ubuntu http resource", RawJSON: "{}", Date: now.Add(time.Duration(i) * time.Minute),
		})
	}
	messages = append(messages, model.Message{
		AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1000,
		Text: "ubuntu cloud resource", RawJSON: "{}", Date: now.Add(-time.Hour),
	})
	stored, err := deps.Messages.SaveBatch(ctx, messages)
	if err != nil {
		t.Fatalf("save messages: %v", err)
	}
	for i := 0; i < 201; i++ {
		if _, err := deps.Links.SaveBatch(ctx, stored[i].ID, []model.Link{{
			Type: "url", Category: "http", URL: "https://example.com/" + strconv.Itoa(i),
		}}); err != nil {
			t.Fatalf("save http link %d: %v", i, err)
		}
	}
	if _, err := deps.Links.SaveBatch(ctx, stored[201].ID, []model.Link{{
		Type: "aliyun", Category: "cloud_drive", URL: "https://www.alipan.com/s/older",
	}}); err != nil {
		t.Fatalf("save cloud link: %v", err)
	}

	router := NewRouter(deps)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/resources?q=ubuntu&limit=50", nil)
	withAPIKey(t, deps, req)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	var body resource.ListResult
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(body.Items) != 50 {
		t.Fatalf("items len = %d, want 50", len(body.Items))
	}
	if body.Total != 202 {
		t.Fatalf("total = %d, want complete resource count", body.Total)
	}
	if body.Grouped["http"] != 201 || body.Grouped["cloud_drive"] != 1 {
		t.Fatalf("grouped = %+v, want complete http=201 cloud_drive=1", body.Grouped)
	}
}

func TestSetupAndAuthAPIs(t *testing.T) {
	deps := testDeps(t)
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("setup status code = %d body=%s", w.Code, w.Body.String())
	}
	var status model.SetupStatus
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode setup status: %v", err)
	}
	if status.AdminConfigured {
		t.Fatalf("admin_configured = true, want false")
	}

	depsWithRuntimeTelegram := testDeps(t)
	depsWithRuntimeTelegram.RuntimeConfig.Telegram.APIID = config.DefaultTelegramAPIID
	depsWithRuntimeTelegram.RuntimeConfig.Telegram.APIHash = config.DefaultTelegramAPIHash
	routerWithRuntimeTelegram := NewRouter(depsWithRuntimeTelegram)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
	routerWithRuntimeTelegram.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("setup status with runtime telegram code = %d body=%s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode setup status with runtime telegram: %v", err)
	}
	if !status.TelegramConfigured {
		t.Fatalf("telegram_configured = false, want true from built-in runtime credentials")
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/setup/admin", bytes.NewBufferString(`{"username":"admin","password":"secret123"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("setup admin code = %d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"username":"admin","password":"secret123"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login code = %d body=%s", w.Code, w.Body.String())
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != adminSessionCookie || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookies = %+v, want HttpOnly SameSite=Lax %s cookie", cookies, adminSessionCookie)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.AddCookie(cookies[0])
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("me code = %d body=%s", w.Code, w.Body.String())
	}
	var me model.User
	if err := json.Unmarshal(w.Body.Bytes(), &me); err != nil {
		t.Fatalf("decode me: %v", err)
	}
	if me.Username != "admin" || me.PasswordHash != "" {
		t.Fatalf("me = %+v, want admin without password hash", me)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(cookies[0])
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("logout code = %d body=%s", w.Code, w.Body.String())
	}
	cleared := w.Result().Cookies()
	if len(cleared) != 1 || cleared[0].MaxAge >= 0 || cleared[0].SameSite != http.SameSiteLaxMode {
		t.Fatalf("logout cookies = %+v, want cleared SameSite=Lax session", cleared)
	}
}

func TestSetupAPIKeyAutoGeneratesAndSkipIsDisabled(t *testing.T) {
	deps := testDeps(t)
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/setup/api-key", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("api key code = %d body=%s", w.Code, w.Body.String())
	}
	var body model.APIKeyResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode api key: %v", err)
	}
	if body.Name != "default" || len(body.Prefix) != 8 || len(body.Key) != 32 || body.Prefix != body.Key[:8] || strings.Contains(body.Key, "-") {
		t.Fatalf("api key response = %+v", body)
	}
	count, err := deps.APIKeys.CountEnabled(context.Background())
	if err != nil {
		t.Fatalf("count keys: %v", err)
	}
	if count != 1 {
		t.Fatalf("enabled key count = %d, want 1", count)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("setup status code = %d body=%s", w.Code, w.Body.String())
	}
	var status model.SetupStatus
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode setup status: %v", err)
	}
	if !status.APIKeyStepComplete {
		t.Fatalf("api key step complete = false, want true")
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/setup/api-key/skip", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound && w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("skip code = %d body=%s, want 404 or 405", w.Code, w.Body.String())
	}
}

func TestBusinessAPIAcceptsAdminSessionOrAPIKey(t *testing.T) {
	deps := testDeps(t)
	router := NewRouter(deps)
	key := createTestAPIKey(t, router)
	cookie := createAdminSession(t, router)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status without key code = %d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/status", nil)
	req.AddCookie(cookie)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status with admin session only code = %d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/status", nil)
	req.Header.Set("Authorization", "Bearer "+key)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status bearer code = %d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/status", nil)
	req.Header.Set("X-API-Key", key)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status x-api-key code = %d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/status", nil)
	req.Header.Set("X-API-Key", "invalid")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status invalid key code = %d body=%s, want 401", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/status", nil)
	req.AddCookie(cookie)
	req.Header.Set("X-API-Key", "invalid")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status admin session with invalid key code = %d body=%s", w.Code, w.Body.String())
	}
}

func TestAPIKeySettingsViewAndRegenerate(t *testing.T) {
	deps := testDeps(t)
	router := NewRouter(deps)
	first := createTestAPIKey(t, router)
	cookie := createAdminSession(t, router)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/settings/api-key", nil)
	req.AddCookie(cookie)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("settings api key code = %d body=%s", w.Code, w.Body.String())
	}
	var current model.APIKeyResponse
	if err := json.Unmarshal(w.Body.Bytes(), &current); err != nil {
		t.Fatalf("decode current key: %v", err)
	}
	if current.Key != first {
		t.Fatalf("current key = %q, want first key", current.Key)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/settings/api-key/regenerate", nil)
	req.AddCookie(cookie)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("regenerate code = %d body=%s", w.Code, w.Body.String())
	}
	var regenerated model.APIKeyResponse
	if err := json.Unmarshal(w.Body.Bytes(), &regenerated); err != nil {
		t.Fatalf("decode regenerated key: %v", err)
	}
	if regenerated.Key == first || regenerated.Prefix != regenerated.Key[:8] {
		t.Fatalf("regenerated key = %+v, first key should be invalidated", regenerated)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/status", nil)
	req.Header.Set("X-API-Key", first)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("old key code = %d body=%s, want 401", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/status", nil)
	req.Header.Set("X-API-Key", regenerated.Key)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("new key code = %d body=%s", w.Code, w.Body.String())
	}
}

func createTestAPIKey(t *testing.T, router *gin.Engine) string {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/setup/api-key", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create api key code = %d body=%s", w.Code, w.Body.String())
	}
	var body model.APIKeyResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode api key: %v", err)
	}
	return body.Key
}

func createAdminSession(t *testing.T, router *gin.Engine) *http.Cookie {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/setup/admin", bytes.NewBufferString(`{"username":"admin","password":"password123"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create admin code = %d body=%s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"username":"admin","password":"password123"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login code = %d body=%s", w.Code, w.Body.String())
	}
	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("login did not set session cookie")
	}
	return cookies[0]
}

func withAPIKey(t *testing.T, deps Dependencies, req *http.Request) *http.Request {
	t.Helper()
	req.Header.Set("X-API-Key", testAPIKey(t, deps))
	return req
}

func testAPIKey(t *testing.T, deps Dependencies) string {
	t.Helper()
	service := deps.APIKeyService
	if service == nil {
		service = apikey.NewService(deps.APIKeys, deps.Settings)
	}
	resp, err := service.EnsureActive(context.Background())
	if err != nil {
		t.Fatalf("ensure test api key: %v", err)
	}
	return resp.Key
}

func TestSetupListenRulesMarksStepConfigured(t *testing.T) {
	deps := testDeps(t)
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/setup/listen-rules", bytes.NewBufferString(`{
		"includes":["电影","网盘"],
		"excludes":["预告"],
		"message_types":["link","video","audio","file","text"],
		"link_types":["cloud_drive","magnet","ed2k","other"]
	}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("setup listen rules code = %d body=%s", w.Code, w.Body.String())
	}
	var status model.SetupStatus
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode setup status: %v", err)
	}
	if !status.ListenRulesConfigured {
		t.Fatalf("listen rules configured = false, want true")
	}
}

func TestTelegramAPISettings(t *testing.T) {
	deps := testDeps(t)
	deps.RuntimeConfig.Telegram.APIID = config.DefaultTelegramAPIID
	deps.RuntimeConfig.Telegram.APIHash = config.DefaultTelegramAPIHash
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/setup/telegram-api", bytes.NewBufferString(`{"app_id":0,"app_hash":""}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("setup telegram api skip code = %d body=%s, want 200", w.Code, w.Body.String())
	}
	assertTelegramAPISettingsResponse(t, w.Body.Bytes(), true, config.DefaultTelegramAPIID)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/setup/telegram-api", bytes.NewBufferString(`{"app_id":123456,"app_hash":""}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("partial setup telegram api code = %d body=%s, want 400", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/setup/telegram-api", bytes.NewBufferString(`{"app_id":123456,"app_hash":"hash-secret"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("setup telegram api code = %d body=%s, want 200", w.Code, w.Body.String())
	}
	assertTelegramAPISettingsResponse(t, w.Body.Bytes(), true, 123456)
	if bytes.Contains(w.Body.Bytes(), []byte("hash-secret")) {
		t.Fatalf("setup telegram api response leaked app hash: %s", w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/settings/telegram-api", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get telegram api code = %d body=%s, want 200", w.Code, w.Body.String())
	}
	assertTelegramAPISettingsResponse(t, w.Body.Bytes(), true, 123456)
	if bytes.Contains(w.Body.Bytes(), []byte("hash-secret")) {
		t.Fatalf("get telegram api response leaked app hash: %s", w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/settings/telegram-api", bytes.NewBufferString(`{"app_id":654321,"app_hash":"new-hash-secret"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("put telegram api code = %d body=%s, want 200", w.Code, w.Body.String())
	}
	assertTelegramAPISettingsResponse(t, w.Body.Bytes(), true, 654321)
	if bytes.Contains(w.Body.Bytes(), []byte("new-hash-secret")) {
		t.Fatalf("put telegram api response leaked app hash: %s", w.Body.String())
	}
}

func TestStorageUsageAPI(t *testing.T) {
	deps := testDeps(t)
	root := t.TempDir()
	writeSizedFile(t, filepath.Join(root, "tg-search.db"), 10)
	writeSizedFile(t, filepath.Join(root, "index", "fts.data"), 20)
	writeSizedFile(t, filepath.Join(root, "thumbnails", "thumb.bin"), 30)
	deps.StorageUsage = storage.NewUsageService(config.Config{
		Storage: config.StorageConfig{
			Path:          root,
			MaxDBSize:     config.Size(100),
			MaxMediaCache: config.Size(100),
		},
	})
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/storage/usage", nil)
	withAPIKey(t, deps, req)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("storage usage code = %d body=%s", w.Code, w.Body.String())
	}
	var usage model.StorageUsage
	if err := json.Unmarshal(w.Body.Bytes(), &usage); err != nil {
		t.Fatalf("decode usage: %v", err)
	}
	if usage.DBBytes != 10 || usage.IndexBytes != 20 || usage.MediaCacheBytes != 30 || usage.TotalBytes != 60 {
		t.Fatalf("usage = %+v", usage)
	}
}

func TestHealthAndReadyEndpoints(t *testing.T) {
	deps := testDeps(t)
	runtimeRoot := t.TempDir()
	deps.RuntimeConfig = config.Config{Storage: config.StorageConfig{Path: runtimeRoot}}
	if err := config.EnsureRuntimeDirs(deps.RuntimeConfig); err != nil {
		t.Fatalf("EnsureRuntimeDirs returned error: %v", err)
	}
	router := NewRouter(deps)

	for _, tc := range []struct {
		path string
		key  string
	}{
		{"/api/health", "service"},
		{"/api/ready", "ready"},
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s status = %d body=%s, want 200", tc.path, w.Code, w.Body.String())
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

func TestReadyEndpointFailsWhenRuntimeDirsAreInvalid(t *testing.T) {
	deps := testDeps(t)
	deps.RuntimeConfig = config.Config{Storage: config.StorageConfig{Path: t.TempDir()}}
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/ready", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("ready status = %d body=%s, want 503", w.Code, w.Body.String())
	}
	var body struct {
		Ready bool `json:"ready"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Ready {
		t.Fatal("ready = true, want false")
	}
}

func TestTaskAPIReturnsEmptyItemsArray(t *testing.T) {
	deps := testDeps(t)
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	withAPIKey(t, deps, req)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte(`"items":[]`)) {
		t.Fatalf("list body = %s, want empty items array", w.Body.String())
	}
}

func TestTaskAPI(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	router := NewRouter(deps)

	failed, err := deps.Tasks.Enqueue(ctx, model.TaskTypeHistorySync, map[string]any{"channel_id": 1})
	if err != nil {
		t.Fatalf("enqueue failed task: %v", err)
	}
	if err := deps.Tasks.Start(ctx, failed.ID); err != nil {
		t.Fatalf("start failed task: %v", err)
	}
	if err := deps.Tasks.Fail(ctx, failed.ID, "temporary", "temporary failure"); err != nil {
		t.Fatalf("fail task: %v", err)
	}

	running, err := deps.Tasks.Enqueue(ctx, model.TaskTypeHistorySync, map[string]any{"channel_id": 2})
	if err != nil {
		t.Fatalf("enqueue running task: %v", err)
	}
	if err := deps.Tasks.Start(ctx, running.ID); err != nil {
		t.Fatalf("start running task: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	withAPIKey(t, deps, req)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	var listBody struct {
		Items []model.Task `json:"items"`
		Total int          `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &listBody); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listBody.Items) != 2 {
		t.Fatalf("list items = %+v, want 2 tasks", listBody.Items)
	}
	if listBody.Total != 2 {
		t.Fatalf("list total = %d, want 2", listBody.Total)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/tasks/"+strconv.FormatInt(failed.ID, 10), nil)
	withAPIKey(t, deps, req)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("detail status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	var detail model.Task
	if err := json.Unmarshal(w.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if detail.ID != failed.ID || detail.Status != model.TaskStatusFailed {
		t.Fatalf("detail = %+v, want failed task", detail)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/tasks/"+strconv.FormatInt(failed.ID, 10)+"/retry", nil)
	withAPIKey(t, deps, req)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("retry status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	var retried model.Task
	if err := json.Unmarshal(w.Body.Bytes(), &retried); err != nil {
		t.Fatalf("decode retry: %v", err)
	}
	if retried.Status != model.TaskStatusQueued || retried.RetryCount != 1 {
		t.Fatalf("retried = %+v, want queued retry_count=1", retried)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/tasks/"+strconv.FormatInt(running.ID, 10)+"/cancel", nil)
	withAPIKey(t, deps, req)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("cancel status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	var canceling model.Task
	if err := json.Unmarshal(w.Body.Bytes(), &canceling); err != nil {
		t.Fatalf("decode cancel: %v", err)
	}
	if canceling.Status != model.TaskStatusCanceling {
		t.Fatalf("canceling = %+v, want canceling", canceling)
	}
}

func TestEventsAPI(t *testing.T) {
	deps := testDeps(t)
	router := NewRouter(deps)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/events?api_key="+url.QueryEscape(testAPIKey(t, deps)), nil).WithContext(ctx)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("events status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); !strings.Contains(got, "text/event-stream") {
		t.Fatalf("content-type = %q, want text/event-stream", got)
	}
}

func assertTelegramAPISettingsResponse(t *testing.T, data []byte, configured bool, appID int) {
	t.Helper()
	var body model.TelegramAPISettingsResponse
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("decode telegram api settings: %v", err)
	}
	if body.Configured != configured || body.AppID != appID || body.AppHashSet != configured {
		t.Fatalf("telegram api settings = %+v, want configured=%v app_id=%d", body, configured, appID)
	}
}

func TestGlobalListenRulesAPI(t *testing.T) {
	deps := testDeps(t)
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/listen-rules", bytes.NewBufferString(`{"includes":[" 庆余年 "],"excludes":["预告"],"message_types":["link","text"],"link_types":["cloud_drive","magnet"]}`))
	withAPIKey(t, deps, req)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("update status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	var updated model.ListenRules
	if err := json.Unmarshal(w.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode update listen rules: %v", err)
	}
	if !sameStringSlices(updated.Includes, []string{"庆余年"}) ||
		!sameStringSlices(updated.Excludes, []string{"预告"}) ||
		!sameStringSlices(updated.MessageTypes, []string{"link", "text"}) ||
		!sameStringSlices(updated.LinkTypes, []string{"cloud_drive", "magnet"}) {
		t.Fatalf("updated = %+v", updated)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/listen-rules", nil)
	withAPIKey(t, deps, req)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	var got model.ListenRules
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode get listen rules: %v", err)
	}
	if !sameStringSlices(got.Includes, []string{"庆余年"}) ||
		!sameStringSlices(got.MessageTypes, []string{"link", "text"}) {
		t.Fatalf("got = %+v", got)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/listen-rules", bytes.NewBufferString(`{"message_types":[],"link_types":["cloud_drive"]}`))
	withAPIKey(t, deps, req)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid update status = %d body=%s, want 400", w.Code, w.Body.String())
	}
}

func TestWatchRuleAPICRUD(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	accountID, _ := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := deps.Channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 1, Title: "VIP", Type: model.ChannelTypeChannel})
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/watch-rules", bytes.NewBufferString(`{"channel_id":`+strconv.FormatInt(channelID, 10)+`,"includes":[" 庆余年 "],"excludes":["预告"],"message_types":["text","file"],"link_types":["cloud_drive","magnet"]}`))
	withAPIKey(t, deps, req)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s, want 201", w.Code, w.Body.String())
	}
	var created model.WatchRule
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("invalid create JSON: %v", err)
	}
	if created.ID == 0 || created.ChannelID != channelID || !created.Enabled ||
		!sameStringSlices(created.Includes, []string{"庆余年"}) ||
		!sameStringSlices(created.MessageTypes, []string{"text", "file"}) ||
		!sameStringSlices(created.LinkTypes, []string{"cloud_drive", "magnet"}) {
		t.Fatalf("created = %+v", created)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/watch-rules/"+strconv.FormatInt(created.ID, 10), bytes.NewBufferString(`{"channel_id":`+strconv.FormatInt(channelID, 10)+`,"enabled":false,"includes":["三体"],"excludes":["花絮"],"message_types":["text"],"link_types":["http"]}`))
	withAPIKey(t, deps, req)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("update status = %d body=%s, want 200", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/watch-rules/"+strconv.FormatInt(created.ID, 10), nil)
	withAPIKey(t, deps, req)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	var got model.WatchRule
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got.Enabled || !sameStringSlices(got.Includes, []string{"三体"}) ||
		!sameStringSlices(got.Excludes, []string{"花絮"}) ||
		!sameStringSlices(got.MessageTypes, []string{"text"}) ||
		!sameStringSlices(got.LinkTypes, []string{"http"}) {
		t.Fatalf("got = %+v", got)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/watch-rules", nil)
	withAPIKey(t, deps, req)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte(`"items"`)) {
		t.Fatalf("list status=%d body=%s, want items", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/watch-rules/"+strconv.FormatInt(created.ID, 10), nil)
	withAPIKey(t, deps, req)
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
		withAPIKey(t, deps, req)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d response=%s, want 400", body, w.Code, w.Body.String())
		}
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/watch-rules", bytes.NewBufferString(`{"channel_id":`+strconv.FormatInt(channelID, 10)+`}`))
	withAPIKey(t, deps, req)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("first create status = %d body=%s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/watch-rules", bytes.NewBufferString(`{"channel_id":`+strconv.FormatInt(channelID, 10)+`}`))
	withAPIKey(t, deps, req)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d body=%s, want 409", w.Code, w.Body.String())
	}
}

func TestSearchRequiresQuery(t *testing.T) {
	deps := testDeps(t)
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/search", nil)
	withAPIKey(t, deps, req)
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
	req := httptest.NewRequest(http.MethodPost, "/api/telegram/login/send-code", bytes.NewBufferString(`{"phone":"+10000000000"}`))
	withAPIKey(t, deps, req)
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

func TestTelegramLoginRoutes(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	deps.Telegram = &fakeTelegram{}
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/telegram/login/send-code", bytes.NewBufferString(`{"phone":"+10000000000"}`))
	withAPIKey(t, deps, req)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("send-code status = %d body=%s, want 200", w.Code, w.Body.String())
	}

	deps.CodeStore.Save("+10000000000", "hash")
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/telegram/login/sign-in", bytes.NewBufferString(`{"phone":"+10000000000","code":"12345"}`))
	withAPIKey(t, deps, req)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("sign-in status = %d body=%s, want 200", w.Code, w.Body.String())
	}

	if _, err := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000001", Status: model.AccountStatusLoginRequired}); err != nil {
		t.Fatalf("save password account: %v", err)
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/telegram/login/password", bytes.NewBufferString(`{"phone":"+10000000001","password":"secret"}`))
	withAPIKey(t, deps, req)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("password status = %d body=%s, want 200", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/login/send-code", bytes.NewBufferString(`{"phone":"+10000000002"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("old send-code status = %d body=%s, want 404", w.Code, w.Body.String())
	}
}

func TestTelegramQRLoginStartAndPendingPoll(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	fake := &fakeTelegram{
		qrTokenURL: "tg://login?token=test-token",
		qrExpires:  time.Now().UTC().Add(time.Minute),
	}
	deps.Telegram = fake
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/telegram/login/qr/start", nil)
	withAPIKey(t, deps, req)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("start status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	var started struct {
		LoginID   string    `json:"login_id"`
		QRURL     string    `json:"qr_url"`
		ExpiresAt time.Time `json:"expires_at"`
		Status    string    `json:"status"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &started); err != nil {
		t.Fatalf("invalid start JSON: %v body=%s", err, w.Body.String())
	}
	if started.LoginID == "" || started.QRURL != "tg://login?token=test-token" || started.Status != "pending" {
		t.Fatalf("start body = %+v, want login id, token url, pending", started)
	}
	accounts, err := deps.Accounts.FindAll(ctx)
	if err != nil {
		t.Fatalf("find accounts: %v", err)
	}
	if len(accounts) != 0 {
		t.Fatalf("accounts len = %d, want 0 before QR confirmation", len(accounts))
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/telegram/login/qr/"+started.LoginID, nil)
	withAPIKey(t, deps, req)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("poll status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	var polled struct {
		LoginID   string    `json:"login_id"`
		QRURL     string    `json:"qr_url"`
		ExpiresAt time.Time `json:"expires_at"`
		Status    string    `json:"status"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &polled); err != nil {
		t.Fatalf("invalid poll JSON: %v body=%s", err, w.Body.String())
	}
	if polled.LoginID != started.LoginID || polled.Status != "pending" || polled.QRURL != started.QRURL {
		t.Fatalf("poll body = %+v, want same pending QR session", polled)
	}
}

func TestTelegramQRLoginCancelStopsSession(t *testing.T) {
	deps := testDeps(t)
	fake := &fakeTelegram{
		qrTokenURL: "tg://login?token=cancel",
		qrExpires:  time.Now().UTC().Add(time.Minute),
	}
	deps.Telegram = fake
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/telegram/login/qr/start", nil)
	withAPIKey(t, deps, req)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("start status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	var started struct {
		LoginID string `json:"login_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &started); err != nil {
		t.Fatalf("invalid start JSON: %v body=%s", err, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/telegram/login/qr/"+started.LoginID, nil)
	withAPIKey(t, deps, req)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("cancel status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	if fake.qrSession == nil || !fake.qrSession.cancelled {
		t.Fatal("QR session was not canceled")
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/telegram/login/qr/"+started.LoginID, nil)
	withAPIKey(t, deps, req)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("poll canceled status = %d body=%s, want 404", w.Code, w.Body.String())
	}
}

func TestTelegramQRLoginPollFinalizesConfirmedAccount(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	fake := &fakeTelegram{
		qrTokenURL: "tg://login?token=ready",
		qrExpires:  time.Now().UTC().Add(time.Minute),
	}
	deps.Telegram = fake
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/telegram/login/qr/start", nil)
	withAPIKey(t, deps, req)
	router.ServeHTTP(w, req)
	var started struct {
		LoginID string `json:"login_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &started); err != nil {
		t.Fatalf("invalid start JSON: %v", err)
	}
	if err := os.WriteFile(fake.qrSessionPath, []byte(`{"session":"qr"}`), 0o600); err != nil {
		t.Fatalf("write qr session: %v", err)
	}
	fake.qrSession.result = telegram.QRLoginPollResult{
		Status:  telegram.QRLoginStatusOnline,
		Profile: telegram.Profile{TelegramUserID: 99, Phone: "+19990000000", FirstName: "QR", LastName: "User", Username: "qr_user"},
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/telegram/login/qr/"+started.LoginID, nil)
	withAPIKey(t, deps, req)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("poll status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	var body struct {
		Status  string        `json:"status"`
		Account model.Account `json:"account"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid poll JSON: %v body=%s", err, w.Body.String())
	}
	if body.Status != model.AccountStatusOnline || body.Account.Phone != "+19990000000" || body.Account.TelegramUserID != 99 {
		t.Fatalf("body = %+v, want online QR account", body)
	}
	if _, err := os.Stat(body.Account.SessionPath); err != nil {
		t.Fatalf("final session stat: %v", err)
	}
	if _, ok := deps.QRLogins.Find(started.LoginID); ok {
		t.Fatal("completed QR login session was not removed")
	}
	account, err := deps.Accounts.FindByPhone(ctx, "+19990000000")
	if err != nil {
		t.Fatalf("find account: %v", err)
	}
	if account.Status != model.AccountStatusOnline {
		t.Fatalf("stored status = %q, want ONLINE", account.Status)
	}
}

func TestTelegramSignInStartsMetadataSyncOnly(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	fake := &fakeTelegram{
		channels: []telegram.Channel{
			{
				TelegramChannelID: 100,
				AccessHash:        200,
				Title:             "Resource Channel",
				Username:          "resources",
				Type:              model.ChannelTypeChannel,
				MemberCount:       1234,
				Description:       "resource index",
				AvatarState:       "unknown",
			},
			{
				TelegramChannelID: 101,
				AccessHash:        201,
				Title:             "Private Group",
				Type:              model.ChannelTypeSupergroup,
				MemberCount:       50,
				Description:       "invite only",
			},
		},
	}
	deps.Telegram = fake
	deps.ChannelSync = channel.NewService(deps.Channels, fake, deps.Sessions)
	deps.ChannelWebAccess = channel.NewWebAccessService(deps.Channels, &apiWebAccessChecker{results: map[string]bool{"async_resources": true}})
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/telegram/login/send-code", bytes.NewBufferString(`{"phone":"+10000000000"}`))
	withAPIKey(t, deps, req)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("send-code status = %d body=%s, want 200", w.Code, w.Body.String())
	}

	deps.CodeStore.Save("+10000000000", "hash")
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/telegram/login/sign-in", bytes.NewBufferString(`{"phone":"+10000000000","code":"12345"}`))
	withAPIKey(t, deps, req)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("sign-in status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	var body struct {
		Status       string        `json:"status"`
		Account      model.Account `json:"account"`
		MetadataSync struct {
			Status       string `json:"status"`
			ChannelCount int    `json:"channel_count"`
		} `json:"metadata_sync"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v body=%s", err, w.Body.String())
	}
	if body.Status != model.AccountStatusOnline || body.Account.Status != model.AccountStatusOnline {
		t.Fatalf("status body = %+v, want ONLINE", body)
	}
	if body.Account.LastOnlineAt == nil || body.Account.SessionPath == "" || body.Account.LastError != "" {
		t.Fatalf("account metadata = %+v, want last_online_at, session_path, empty last_error", body.Account)
	}
	if body.MetadataSync.Status != "succeeded" || body.MetadataSync.ChannelCount != 3 {
		t.Fatalf("metadata_sync = %+v, want succeeded with 3 channels", body.MetadataSync)
	}

	items, err := deps.Channels.FindByAccountID(ctx, body.Account.ID)
	if err != nil {
		t.Fatalf("find channels: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("channels len = %d, want 3", len(items))
	}
	var public model.Channel
	for _, item := range items {
		if item.Username == "resources" {
			public = item
		}
	}
	if public.MemberCount != 1234 || public.Description != "resource index" || public.SyncState != "metadata_only" {
		t.Fatalf("public channel metadata = %+v", public)
	}
	counts, err := deps.Status.Counts(ctx)
	if err != nil {
		t.Fatalf("counts: %v", err)
	}
	if counts.Messages != 0 {
		t.Fatalf("message count = %d, want 0", counts.Messages)
	}
	if fake.fetchHistoryCalls != 0 {
		t.Fatalf("FetchHistory calls = %d, want 0", fake.fetchHistoryCalls)
	}
}

func TestTelegramSignInQueuesMetadataSyncWhenRetryQueueAvailable(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	deps.SyncQueue = scheduler.NewRetryQueue(scheduler.RetryQueueOptions{
		Policy: retry.Policy{BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, MaxTries: 1, Sleep: func(context.Context, time.Duration) error { return nil }},
	})
	fake := &fakeTelegram{
		channels: []telegram.Channel{
			{
				TelegramChannelID: 200,
				AccessHash:        300,
				Title:             "Async Resources",
				Username:          "async_resources",
				Type:              model.ChannelTypeChannel,
				MemberCount:       42,
				Description:       "loaded in the background",
			},
		},
	}
	deps.Telegram = fake
	deps.ChannelSync = channel.NewService(deps.Channels, fake, deps.Sessions)
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/telegram/login/send-code", bytes.NewBufferString(`{"phone":"+10000000000"}`))
	withAPIKey(t, deps, req)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("send-code status = %d body=%s, want 200", w.Code, w.Body.String())
	}

	deps.CodeStore.Save("+10000000000", "hash")
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/telegram/login/sign-in", bytes.NewBufferString(`{"phone":"+10000000000","code":"12345"}`))
	withAPIKey(t, deps, req)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("sign-in status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	var body struct {
		Account      model.Account `json:"account"`
		MetadataSync struct {
			Status string `json:"status"`
			JobID  string `json:"job_id"`
		} `json:"metadata_sync"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v body=%s", err, w.Body.String())
	}
	if body.MetadataSync.Status != "queued" || body.MetadataSync.JobID == "" {
		t.Fatalf("metadata_sync = %+v, want queued job id", body.MetadataSync)
	}
	done, err := deps.SyncQueue.Wait(ctx, body.MetadataSync.JobID)
	if err != nil {
		t.Fatalf("wait metadata sync job: %v", err)
	}
	if done.Status != scheduler.RetryJobSucceeded {
		t.Fatalf("job status = %q error=%s, want succeeded", done.Status, done.Error)
	}
	items, err := deps.Channels.FindByAccountID(ctx, body.Account.ID)
	if err != nil {
		t.Fatalf("find channels: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("channels len = %d, want saved messages plus async channel", len(items))
	}
}

func TestTelegramSignInQueuedMetadataSyncChecksWebAccess(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	deps.SyncQueue = scheduler.NewRetryQueue(scheduler.RetryQueueOptions{
		Policy: retry.Policy{BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, MaxTries: 1, Sleep: func(context.Context, time.Duration) error { return nil }},
	})
	fake := &fakeTelegram{
		channels: []telegram.Channel{
			{
				TelegramChannelID: 201,
				AccessHash:        301,
				Title:             "Public After Login",
				Username:          "public_after_login",
				Type:              model.ChannelTypeChannel,
			},
		},
	}
	checker := &apiWebAccessChecker{results: map[string]bool{"public_after_login": true}}
	deps.Telegram = fake
	deps.ChannelSync = channel.NewService(deps.Channels, fake, deps.Sessions)
	deps.ChannelWebAccess = channel.NewWebAccessService(deps.Channels, checker)
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/telegram/login/send-code", bytes.NewBufferString(`{"phone":"+10000000000"}`))
	withAPIKey(t, deps, req)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("send-code status = %d body=%s, want 200", w.Code, w.Body.String())
	}

	deps.CodeStore.Save("+10000000000", "hash")
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/telegram/login/sign-in", bytes.NewBufferString(`{"phone":"+10000000000","code":"12345"}`))
	withAPIKey(t, deps, req)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("sign-in status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	var body struct {
		Account      model.Account `json:"account"`
		MetadataSync struct {
			JobID string `json:"job_id"`
		} `json:"metadata_sync"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v body=%s", err, w.Body.String())
	}
	done, err := deps.SyncQueue.Wait(ctx, body.MetadataSync.JobID)
	if err != nil {
		t.Fatalf("wait metadata sync job: %v", err)
	}
	if done.Status != scheduler.RetryJobSucceeded {
		t.Fatalf("job status = %q error=%s, want succeeded", done.Status, done.Error)
	}
	if !sameStringSlices(checker.calls, []string{"public_after_login"}) {
		t.Fatalf("checker calls = %v, want public_after_login", checker.calls)
	}
	items, err := deps.Channels.FindByAccountID(ctx, body.Account.ID)
	if err != nil {
		t.Fatalf("find channels: %v", err)
	}
	var public model.Channel
	for _, item := range items {
		if item.Username == "public_after_login" {
			public = item
		}
	}
	if public.ID == 0 || public.WebAccess == nil || !*public.WebAccess || public.WebAccessCheckedAt == nil {
		t.Fatalf("public channel = %+v, want web access checked true", public)
	}
}

func TestTelegramSignInKeepsAccountOnlineWhenMetadataSyncFails(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	fake := &fakeTelegram{listErr: errors.New("flood wait")}
	deps.Telegram = fake
	deps.ChannelSync = channel.NewService(deps.Channels, fake, deps.Sessions)
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/telegram/login/send-code", bytes.NewBufferString(`{"phone":"+10000000000"}`))
	withAPIKey(t, deps, req)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("send-code status = %d body=%s, want 200", w.Code, w.Body.String())
	}

	deps.CodeStore.Save("+10000000000", "hash")
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/telegram/login/sign-in", bytes.NewBufferString(`{"phone":"+10000000000","code":"12345"}`))
	withAPIKey(t, deps, req)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("sign-in status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	var body struct {
		Status       string        `json:"status"`
		Account      model.Account `json:"account"`
		MetadataSync struct {
			Status string `json:"status"`
			Error  string `json:"error"`
		} `json:"metadata_sync"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v body=%s", err, w.Body.String())
	}
	if body.Status != model.AccountStatusOnline || body.Account.Status != model.AccountStatusOnline {
		t.Fatalf("status body = %+v, want ONLINE", body)
	}
	if body.MetadataSync.Status != "failed" || !strings.Contains(body.MetadataSync.Error, "flood wait") {
		t.Fatalf("metadata_sync = %+v, want failed flood wait", body.MetadataSync)
	}
	account, err := deps.Accounts.FindByID(ctx, body.Account.ID)
	if err != nil {
		t.Fatalf("find account: %v", err)
	}
	if account.Status != model.AccountStatusOnline || !strings.Contains(account.LastError, "flood wait") {
		t.Fatalf("stored account = %+v, want ONLINE with last_error", account)
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
	withAPIKey(t, deps, req)
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

func TestLogoutAccountStopsRuntimeRemovesSessionAndKeepsAccount(t *testing.T) {
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
	req := httptest.NewRequest(http.MethodPost, "/api/accounts/"+strconv.FormatInt(accountID, 10)+"/logout", nil)
	withAPIKey(t, deps, req)
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
	var body model.Account
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.ID != accountID || body.Status != model.AccountStatusLoginRequired {
		t.Fatalf("response account = %+v, want id %d LOGIN_REQUIRED", body, accountID)
	}
	account, err := deps.Accounts.FindByID(ctx, accountID)
	if err != nil {
		t.Fatalf("find account after logout: %v", err)
	}
	if account.Status != model.AccountStatusLoginRequired {
		t.Fatalf("stored status = %q, want LOGIN_REQUIRED", account.Status)
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
	withAPIKey(t, deps, req)
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
		withAPIKey(t, deps, req)
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
	withAPIKey(t, deps, req)
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
	withAPIKey(t, deps, req)
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
	withAPIKey(t, deps, req)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("jan")) || bytes.Contains(w.Body.Bytes(), []byte("feb")) {
		t.Fatalf("date range response = %s, want jan only", w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/links?date_from=not-a-date", nil)
	withAPIKey(t, deps, req)
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
	withAPIKey(t, deps, req)
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
	withAPIKey(t, deps, req)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("january")) || bytes.Contains(w.Body.Bytes(), []byte("february")) {
		t.Fatalf("date range response = %s, want january only", w.Body.String())
	}
}

func TestReadAPIsRejectInvalidQueryParameters(t *testing.T) {
	deps := testDeps(t)
	router := NewRouter(deps)
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
		withAPIKey(t, deps, req)
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
	withAPIKey(t, deps, req)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("shared older")) || bytes.Contains(w.Body.Bytes(), []byte("shared newer")) {
		t.Fatalf("cursor response = %s, want older only", w.Body.String())
	}
}

func TestMaintenanceSQLiteAPI(t *testing.T) {
	deps := testDeps(t)
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/maintenance/sqlite", nil)
	withAPIKey(t, deps, req)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("ANALYZE")) || !bytes.Contains(w.Body.Bytes(), []byte("telegram_messages_fts optimize")) {
		t.Fatalf("maintenance response = %s", w.Body.String())
	}
}

func TestMaintenanceBackupAPI(t *testing.T) {
	deps := testDeps(t)
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/maintenance/backup", nil)
	withAPIKey(t, deps, req)
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
	deps := testDeps(t)
	router := NewRouter(deps)
	for _, body := range []string{
		`{}`,
		`{"channel_ids":[]}`,
		`{"channel_ids":[0]}`,
		`{"channel_ids":[-1]}`,
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/channels/sync", bytes.NewBufferString(body))
		withAPIKey(t, deps, req)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d body=%s, want 400", body, w.Code, w.Body.String())
		}
	}
}

func TestBatchSyncAPIValidatesMaxMessages(t *testing.T) {
	deps := testDeps(t)
	router := NewRouter(deps)
	for _, body := range []string{
		`{"channel_ids":[1],"max_messages":0}`,
		`{"channel_ids":[1],"max_messages":-1}`,
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/channels/sync", bytes.NewBufferString(body))
		withAPIKey(t, deps, req)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d body=%s, want 400", body, w.Code, w.Body.String())
		}
	}
}

func TestChannelsAPIIncludesIndexedMessageCount(t *testing.T) {
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
	now := time.Now().UTC()
	_, _ = deps.Messages.SaveBatch(ctx, []model.Message{
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1, Text: "indexed 1", RawJSON: "{}", Date: now},
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 2, Text: "indexed 2", RawJSON: "{}", Date: now},
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 3, Text: "deleted", RawJSON: "{}", Date: now, Deleted: true},
	})
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/channels", nil)
	withAPIKey(t, deps, req)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	var body struct {
		Items []struct {
			ID                  int64 `json:"id"`
			IndexedMessageCount int64 `json:"indexed_message_count"`
		} `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(body.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(body.Items))
	}
	if body.Items[0].IndexedMessageCount != 2 {
		t.Fatalf("indexed_message_count = %d, want 2", body.Items[0].IndexedMessageCount)
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
	withAPIKey(t, deps, req)
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
	withAPIKey(t, deps, req)
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

func TestChannelWebAccessCheckAPIStoresErrors(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	accountID, _ := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := deps.Channels.Save(ctx, model.Channel{
		AccountID:         accountID,
		TelegramChannelID: 32,
		Title:             "Public",
		Username:          "public_channel",
		Type:              model.ChannelTypeChannel,
	})
	checker := &apiWebAccessChecker{errors: map[string]error{"public_channel": errors.New("telegram web 500")}}
	deps.ChannelWebAccess = channel.NewWebAccessService(deps.Channels, checker)
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/channels/web-access/check", bytes.NewBufferString(`{"channel_ids":[`+strconv.FormatInt(channelID, 10)+`]}`))
	withAPIKey(t, deps, req)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", w.Code, w.Body.String())
	}

	stored, err := deps.Channels.FindByID(ctx, channelID)
	if err != nil {
		t.Fatalf("find channel: %v", err)
	}
	if stored.WebAccess == nil || *stored.WebAccess != false || !strings.Contains(stored.WebAccessError, "telegram web 500") {
		t.Fatalf("stored web access = %+v, want false with error", stored)
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
		withAPIKey(t, deps, req)
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
	withAPIKey(t, deps, req)
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

func TestChannelControlAPIUpdatesProfileAndToggles(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	accountID, _ := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := deps.Channels.Save(ctx, model.Channel{
		AccountID:         accountID,
		TelegramChannelID: 51,
		Title:             "Control",
		Type:              model.ChannelTypeChannel,
	})
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/channels/"+strconv.FormatInt(channelID, 10)+"/control", bytes.NewBufferString(`{
		"history_sync_enabled": true,
		"sync_profile": "Quick",
		"listen_enabled": true,
		"remote_search_allowed": false
	}`))
	withAPIKey(t, deps, req)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	var body model.Channel
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !body.HistorySyncEnabled || body.SyncProfile != "Quick" || !body.ListenEnabled || body.RemoteSearchAllowed {
		t.Fatalf("response control = %+v", body)
	}
	if body.SyncState != "pending" || body.ListenState != "enabled" {
		t.Fatalf("response states = sync:%q listen:%q, want pending/enabled", body.SyncState, body.ListenState)
	}

	stored, err := deps.Channels.FindByID(ctx, channelID)
	if err != nil {
		t.Fatalf("find channel: %v", err)
	}
	if !stored.HistorySyncEnabled || stored.SyncProfile != "Quick" || !stored.ListenEnabled || stored.RemoteSearchAllowed {
		t.Fatalf("stored control = %+v", stored)
	}
	if stored.SyncState != "pending" || stored.ListenState != "enabled" {
		t.Fatalf("stored states = sync:%q listen:%q, want pending/enabled", stored.SyncState, stored.ListenState)
	}
}

func TestBatchChannelControlAPIUpdatesSelectedChannels(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	accountID, _ := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channel1, _ := deps.Channels.Save(ctx, model.Channel{
		AccountID:         accountID,
		TelegramChannelID: 61,
		Title:             "Control 1",
		Type:              model.ChannelTypeChannel,
	})
	channel2, _ := deps.Channels.Save(ctx, model.Channel{
		AccountID:         accountID,
		TelegramChannelID: 62,
		Title:             "Control 2",
		Type:              model.ChannelTypeChannel,
	})
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/channels/control", bytes.NewBufferString(`{
		"channel_ids": [`+strconv.FormatInt(channel1, 10)+`,`+strconv.FormatInt(channel2, 10)+`],
		"control": {
			"history_sync_enabled": true,
			"sync_profile": "Normal",
			"listen_enabled": true,
			"remote_search_allowed": true
		}
	}`))
	withAPIKey(t, deps, req)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	var body struct {
		Items []model.Channel `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(body.Items) != 2 {
		t.Fatalf("items len = %d, want 2", len(body.Items))
	}
	for _, id := range []int64{channel1, channel2} {
		stored, err := deps.Channels.FindByID(ctx, id)
		if err != nil {
			t.Fatalf("find channel %d: %v", id, err)
		}
		if !stored.HistorySyncEnabled || stored.SyncProfile != "Normal" || !stored.ListenEnabled || !stored.RemoteSearchAllowed {
			t.Fatalf("stored channel %d control = %+v", id, stored)
		}
		if stored.SyncState != "pending" || stored.ListenState != "enabled" {
			t.Fatalf("stored channel %d states = sync:%q listen:%q, want pending/enabled", id, stored.SyncState, stored.ListenState)
		}
	}
}

func TestChannelControlAPIRejectsInvalidProfile(t *testing.T) {
	deps := testDeps(t)
	accountID, _ := deps.Accounts.Save(context.Background(), model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := deps.Channels.Save(context.Background(), model.Channel{
		AccountID:         accountID,
		TelegramChannelID: 52,
		Title:             "Control",
		Type:              model.ChannelTypeChannel,
	})
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/channels/"+strconv.FormatInt(channelID, 10)+"/control", bytes.NewBufferString(`{
		"history_sync_enabled": true,
		"sync_profile": "raw-1000",
		"listen_enabled": false,
		"remote_search_allowed": true
	}`))
	withAPIKey(t, deps, req)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s, want 400", w.Code, w.Body.String())
	}
}

func TestChannelControlAPIDeepProfileChecksStorageQuota(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	root := t.TempDir()
	writeSizedFile(t, filepath.Join(root, "tg-search.db"), 10)
	deps.StorageUsage = storage.NewUsageService(config.Config{
		Storage: config.StorageConfig{
			Path:      root,
			MaxDBSize: config.Size(10),
		},
	})
	accountID, _ := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := deps.Channels.Save(ctx, model.Channel{
		AccountID:         accountID,
		TelegramChannelID: 53,
		Title:             "Control",
		Type:              model.ChannelTypeChannel,
	})
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/channels/"+strconv.FormatInt(channelID, 10)+"/control", bytes.NewBufferString(`{
		"history_sync_enabled": true,
		"sync_profile": "Deep",
		"listen_enabled": false,
		"remote_search_allowed": true
	}`))
	withAPIKey(t, deps, req)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d body=%s, want 409", w.Code, w.Body.String())
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Error.Code != "storage_quota_exceeded" {
		t.Fatalf("error code = %q, want storage_quota_exceeded body=%s", body.Error.Code, w.Body.String())
	}
}

func TestChannelAnalyze(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	accountID, _ := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := deps.Channels.Save(ctx, model.Channel{
		AccountID:           accountID,
		TelegramChannelID:   61,
		Title:               "Analysis Channel",
		Username:            "analysis",
		Type:                model.ChannelTypeChannel,
		MemberCount:         123,
		Description:         "metadata only",
		HistorySyncEnabled:  true,
		SyncProfile:         "Normal",
		ListenEnabled:       true,
		RemoteSearchAllowed: false,
	})
	_, _ = deps.WatchRules.Create(ctx, model.WatchRule{
		ChannelID:    channelID,
		Enabled:      true,
		Includes:     []string{"电影", "课程"},
		Excludes:     []string{"广告"},
		MessageTypes: []string{"text", "file"},
		LinkTypes:    []string{"cloud_drive", "magnet"},
	})
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/channels/"+strconv.FormatInt(channelID, 10)+"/analyze", nil)
	withAPIKey(t, deps, req)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	var body model.ChannelAnalysis
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Channel.ID != channelID || body.Channel.Title != "Analysis Channel" || body.Channel.MemberCount != 123 {
		t.Fatalf("channel analysis metadata = %+v", body.Channel)
	}
	if !body.Control.HistorySyncEnabled || body.Control.SyncProfile != "Normal" || !body.Control.ListenEnabled || body.Control.RemoteSearchAllowed {
		t.Fatalf("control = %+v", body.Control)
	}
	if body.WatchRule == nil || !sameStringSlices(body.WatchRule.MessageTypes, []string{"text", "file"}) ||
		!sameStringSlices(body.WatchRule.LinkTypes, []string{"cloud_drive", "magnet"}) {
		t.Fatalf("watch rule = %+v", body.WatchRule)
	}
	if body.IndexedCounts.Messages != 0 || body.IndexedCounts.Links != 0 || body.IndexedCounts.Files != 0 {
		t.Fatalf("indexed counts = %+v, want zero counts", body.IndexedCounts)
	}
}

func TestRemoteSearchEntry(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	accountID, _ := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	allowedID, _ := deps.Channels.Save(ctx, model.Channel{
		AccountID:           accountID,
		TelegramChannelID:   71,
		Title:               "Allowed",
		Type:                model.ChannelTypeChannel,
		RemoteSearchAllowed: true,
	})
	blockedID, _ := deps.Channels.Save(ctx, model.Channel{
		AccountID:           accountID,
		TelegramChannelID:   72,
		Title:               "Blocked",
		Type:                model.ChannelTypeChannel,
		RemoteSearchAllowed: false,
	})
	syncedAt := time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC)
	syncedID, _ := deps.Channels.Save(ctx, model.Channel{
		AccountID:           accountID,
		TelegramChannelID:   73,
		Title:               "Synced",
		Type:                model.ChannelTypeChannel,
		LastMessageID:       100,
		LastSyncTime:        &syncedAt,
		RemoteSearchAllowed: true,
	})
	router := NewRouter(deps)

	for _, tc := range []struct {
		name string
		body string
		code int
		err  string
	}{
		{"empty query", `{"channel_id":` + strconv.FormatInt(allowedID, 10) + `,"query":" "}`, http.StatusBadRequest, "bad_request"},
		{"blocked", `{"channel_id":` + strconv.FormatInt(blockedID, 10) + `,"query":"ubuntu iso"}`, http.StatusConflict, "remote_search_not_allowed"},
		{"synced", `{"channel_id":` + strconv.FormatInt(syncedID, 10) + `,"query":"ubuntu iso"}`, http.StatusConflict, "remote_search_requires_unsynced_channel"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/search/remote", bytes.NewBufferString(tc.body))
			withAPIKey(t, deps, req)
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)
			if w.Code != tc.code {
				t.Fatalf("status = %d body=%s, want %d", w.Code, w.Body.String(), tc.code)
			}
			if tc.err != "" {
				var body struct {
					Error struct {
						Code string `json:"code"`
					} `json:"error"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
					t.Fatalf("invalid error JSON: %v", err)
				}
				if body.Error.Code != tc.err {
					t.Fatalf("error code = %q, want %q body=%s", body.Error.Code, tc.err, w.Body.String())
				}
			}
		})
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/search/remote", bytes.NewBufferString(`{"channel_id":`+strconv.FormatInt(allowedID, 10)+`,"query":"ubuntu iso"}`))
	withAPIKey(t, deps, req)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d body=%s, want 202", w.Code, w.Body.String())
	}
	var body model.RemoteSearchTask
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.ID == 0 || body.Status != model.RemoteSearchStatusQueued || body.Source != "remote" || body.Query != "ubuntu iso" || body.ExpiresAt.IsZero() {
		t.Fatalf("remote search task = %+v", body)
	}
}

func TestRemoteSearchResultAPI(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	accountID, _ := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := deps.Channels.Save(ctx, model.Channel{
		AccountID:           accountID,
		TelegramChannelID:   71,
		Title:               "Allowed",
		Type:                model.ChannelTypeChannel,
		RemoteSearchAllowed: true,
	})
	deps.RemoteSearchExec = search.NewRemoteService(search.RemoteOptions{
		Accounts: deps.Accounts,
		Channels: deps.Channels,
		Tasks:    deps.RemoteSearch,
		Cursors:  repository.NewSyncCursorRepository(deps.BackupDB),
		Telegram: &apiRemoteSearchClient{items: []telegram.Message{{
			TelegramMessageID: 99,
			Text:              "ubuntu remote result",
			RawJSON:           "{}",
			Date:              time.Now().UTC(),
		}}},
		Sessions: deps.Sessions,
	})
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/search/remote", bytes.NewBufferString(`{"channel_id":`+strconv.FormatInt(channelID, 10)+`,"query":"ubuntu"}`))
	withAPIKey(t, deps, req)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("create status = %d body=%s, want 202", w.Code, w.Body.String())
	}
	var task model.RemoteSearchTask
	if err := json.Unmarshal(w.Body.Bytes(), &task); err != nil {
		t.Fatalf("invalid task JSON: %v", err)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/search/remote/"+strconv.FormatInt(task.ID, 10), nil)
	withAPIKey(t, deps, req)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("result status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	var result model.RemoteSearchResults
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("invalid result JSON: %v", err)
	}
	if result.Task.ID != task.ID || len(result.Items) != 1 || result.Items[0].Source != "remote" || result.Items[0].Text != "ubuntu remote result" {
		t.Fatalf("remote result = %+v", result)
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
	withAPIKey(t, deps, req)
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
	req := httptest.NewRequest(http.MethodPost, "/api/accounts/"+strconv.FormatInt(accountID, 10)+"/channels/sync-metadata", nil)
	withAPIKey(t, deps, req)
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

type apiRemoteSearchClient struct {
	telegram.NopClient
	items []telegram.Message
}

func (f *apiRemoteSearchClient) SearchMessages(ctx context.Context, account telegram.AccountSession, channel telegram.ChannelRef, query string, limit int) ([]telegram.Message, error) {
	return f.items, nil
}

func testDeps(t *testing.T) Dependencies {
	t.Helper()
	deps, _ := testDepsWithDB(t)
	return deps
}

func testDepsWithDB(t *testing.T) (Dependencies, *sql.DB) {
	t.Helper()
	root := t.TempDir()
	runtimeConfig := config.Config{Storage: config.StorageConfig{Path: root, MaxDBSize: config.Size(10), MaxMediaCache: config.Size(20)}}
	if err := config.EnsureRuntimeDirs(runtimeConfig); err != nil {
		t.Fatalf("EnsureRuntimeDirs returned error: %v", err)
	}
	conn, err := db.Open(filepath.Join(root, "tg-search.db"))
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
	files := repository.NewFileRepository(conn)
	resourceStats := repository.NewResourceStatsRepository(conn)
	watchRules := repository.NewWatchRuleRepository(conn)
	remoteSearch := repository.NewRemoteSearchTaskRepository(conn)
	maintenance := repository.NewMaintenanceRepository(conn)
	status := repository.NewStatusRepository(conn)
	taskRepository := taskpkg.NewRepository(conn)
	taskService := taskpkg.NewService(taskRepository)
	users := repository.NewUserRepository(conn)
	apiKeys := repository.NewAPIKeyRepository(conn)
	settings := repository.NewSettingsRepository(conn)
	sessions := session.NewManager(filepath.Join(t.TempDir(), "sessions"))
	client := telegram.NopClient{}
	watchFilter := messagefilter.New(messagefilter.NewSettingsRuleStore(watchRules, settings))
	searchService := search.NewService(messages, links, files, channels)
	resourceService := resource.NewService(links, files, resourceStats)
	historyService := history.NewService(history.Options{
		DB: conn, Accounts: accounts, Channels: channels, Messages: messages, Links: links,
		Resources: resourceService,
		Telegram:  client, Sessions: sessions, Extractor: link.NewExtractor(), Filter: watchFilter, HistoryBatchSize: 100,
	})
	channelService := channel.NewService(channels, client, sessions)
	channelWebAccessService := channel.NewWebAccessService(channels, nil)
	return Dependencies{
		Users: users, APIKeys: apiKeys, Settings: settings, AdminAuth: adminauth.NewService(users),
		Accounts: accounts, Channels: channels, Messages: messages, Links: links, WatchRules: watchRules, RemoteSearch: remoteSearch, Maintenance: maintenance, Status: status,
		BackupDB: conn, BackupDir: filepath.Join(t.TempDir(), "backup"),
		RuntimeConfig: runtimeConfig,
		StorageUsage:  storage.NewUsageService(runtimeConfig),
		Search:        searchService, History: historyService, Resources: resourceService, ChannelSync: channelService, ChannelWebAccess: channelWebAccessService,
		Tasks: taskService, TaskRepository: taskRepository, Events: taskpkg.NewEventBroker(),
		Telegram: client, Sessions: sessions, CodeStore: telegram.NewCodeStore(), QRLogins: NewQRLoginStore(2 * time.Minute),
	}, conn
}

type fakeTelegram struct {
	telegram.NopClient
	channels          []telegram.Channel
	listErr           error
	fetchHistoryCalls int
	qrTokenURL        string
	qrExpires         time.Time
	qrSessionPath     string
	qrSession         *fakeQRLoginSession
}

func (fakeTelegram) SendCode(ctx context.Context, phone string, sessionPath string) (telegram.SentCode, error) {
	return telegram.SentCode{PhoneCodeHash: "hash"}, nil
}

func (fakeTelegram) SignIn(ctx context.Context, phone string, code string, phoneCodeHash string, sessionPath string) (telegram.Profile, error) {
	return telegram.Profile{TelegramUserID: 42, FirstName: "Ada", LastName: "Lovelace", Username: "ada"}, nil
}

func (fakeTelegram) Password(ctx context.Context, password string, sessionPath string) (telegram.Profile, error) {
	return telegram.Profile{TelegramUserID: 43, FirstName: "Grace", LastName: "Hopper", Username: "grace"}, nil
}

func (f *fakeTelegram) StartQRLogin(ctx context.Context, sessionPath string) (telegram.QRLoginSession, error) {
	session := &fakeQRLoginSession{
		token: telegram.QRLoginToken{URL: f.qrTokenURL, ExpiresAt: f.qrExpires},
	}
	f.qrSessionPath = sessionPath
	f.qrSession = session
	return session, nil
}

func (f *fakeTelegram) ListChannels(ctx context.Context, accountSession telegram.AccountSession) ([]telegram.Channel, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := []telegram.Channel{
		{
			TelegramChannelID: accountSession.AccountID,
			Title:             "Saved Messages",
			Type:              model.ChannelTypeSavedMessages,
		},
	}
	out = append(out, f.channels...)
	return out, nil
}

func (f *fakeTelegram) FetchHistory(ctx context.Context, account telegram.AccountSession, channel telegram.ChannelRef, offsetID int64, limit int) ([]telegram.Message, error) {
	f.fetchHistoryCalls++
	return nil, nil
}

type fakeQRLoginSession struct {
	token     telegram.QRLoginToken
	result    telegram.QRLoginPollResult
	cancelled bool
}

func (s *fakeQRLoginSession) Token() telegram.QRLoginToken {
	return s.token
}

func (s *fakeQRLoginSession) Poll(ctx context.Context) (telegram.QRLoginPollResult, error) {
	if s.result.Status == "" {
		return telegram.QRLoginPollResult{Status: telegram.QRLoginStatusPending, Token: s.token}, nil
	}
	return s.result, nil
}

func (s *fakeQRLoginSession) Cancel(ctx context.Context) error {
	s.cancelled = true
	return nil
}

type apiWebAccessChecker struct {
	results map[string]bool
	errors  map[string]error
	calls   []string
}

func (c *apiWebAccessChecker) Check(ctx context.Context, username string) (bool, error) {
	c.calls = append(c.calls, username)
	if c.errors != nil && c.errors[username] != nil {
		return false, c.errors[username]
	}
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

func writeSizedFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
