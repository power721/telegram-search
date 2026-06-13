package api

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"tg-search/internal/apikey"
	"tg-search/internal/medialimit"
	"tg-search/internal/model"
	"tg-search/internal/telegram"
)

func TestParseRange(t *testing.T) {
	tests := []struct {
		name        string
		header      string
		size        int64
		wantStart   int64
		wantEnd     int64
		wantPartial bool
		wantErr     bool
	}{
		{name: "empty", size: 1000, wantStart: 0, wantEnd: 999},
		{name: "closed", header: "bytes=100-199", size: 1000, wantStart: 100, wantEnd: 199, wantPartial: true},
		{name: "open ended", header: "bytes=900-", size: 1000, wantStart: 900, wantEnd: 999, wantPartial: true},
		{name: "large open ended reaches eof", header: "bytes=1048576-", size: 32 * 1024 * 1024, wantStart: 1048576, wantEnd: 32*1024*1024 - 1, wantPartial: true},
		{name: "suffix", header: "bytes=-200", size: 1000, wantStart: 800, wantEnd: 999, wantPartial: true},
		{name: "large suffix", header: "bytes=-2000", size: 1000, wantStart: 0, wantEnd: 999, wantPartial: true},
		{name: "end past size", header: "bytes=900-1000", size: 1000, wantErr: true},
		{name: "multi range", header: "bytes=0-1,3-4", size: 1000, wantErr: true},
		{name: "bad unit", header: "items=0-1", size: 1000, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, partial, err := parseRange(tt.header, tt.size)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseRange returned nil error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseRange returned error: %v", err)
			}
			if start != tt.wantStart || end != tt.wantEnd || partial != tt.wantPartial {
				t.Fatalf("parseRange = %d, %d, %v; want %d, %d, %v", start, end, partial, tt.wantStart, tt.wantEnd, tt.wantPartial)
			}
		})
	}
}

func TestServeTelegramVideoOpenEndedRangeReachesEOF(t *testing.T) {
	deps := testDeps(t)
	ctx := context.Background()
	accountID, err := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	if err != nil {
		t.Fatalf("save account: %v", err)
	}
	channelID, err := deps.Channels.Save(ctx, model.Channel{
		AccountID:         accountID,
		TelegramChannelID: 100,
		AccessHash:        200,
		Title:             "NewQuark",
		Username:          "NewQuark",
		Type:              model.ChannelTypeChannel,
	})
	if err != nil {
		t.Fatalf("save channel: %v", err)
	}
	stored, err := deps.Messages.SaveBatch(ctx, []model.Message{{
		AccountID: accountID, ChannelID: channelID, TelegramMessageID: 12345,
		MessageType: "video", MediaSummary: "video/mp4", Text: "clip", RawJSON: "{}", Date: time.Now().UTC(),
	}})
	if err != nil {
		t.Fatalf("save message: %v", err)
	}
	_, err = deps.Files.SaveBatch(ctx, stored[0].ID, []model.File{{TelegramFileID: 777, FileName: "clip.mp4", Extension: ".mp4", MimeType: "video/mp4", SizeBytes: 32 * 1024 * 1024}})
	if err != nil {
		t.Fatalf("save file: %v", err)
	}
	fake := &mediaProxyClient{
		file: telegram.VideoFile{ID: 1, AccessHash: 2, FileReference: []byte{3}, Size: 32 * 1024 * 1024, MIMEType: "video/mp4"},
		data: []byte{0x7a},
	}
	deps.Telegram = fake
	router := NewRouter(deps)
	key := createTestAPIKey(t, router)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v/777", nil)
	req.Header.Set("Range", "bytes=1048576-")
	req.Header.Set("X-API-Key", key)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusPartialContent {
		t.Fatalf("status = %d body=%s, want 206", w.Code, w.Body.String())
	}
	wantEnd := int64(32*1024*1024 - 1)
	if got := w.Header().Get("Content-Range"); got != "bytes 1048576-"+strconv.FormatInt(wantEnd, 10)+"/33554432" {
		t.Fatalf("Content-Range = %q", got)
	}
	wantLength := int64(32*1024*1024 - 1048576)
	if got := w.Header().Get("Content-Length"); got != strconv.FormatInt(wantLength, 10) {
		t.Fatalf("Content-Length = %q", got)
	}
	if fake.offset != 1048576 || fake.length != wantLength {
		t.Fatalf("stream range = offset %d length %d, want 1048576/%d", fake.offset, fake.length, wantLength)
	}
}

func TestServeTelegramVideoRange(t *testing.T) {
	deps := testDeps(t)
	ctx := context.Background()
	accountID, err := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	if err != nil {
		t.Fatalf("save account: %v", err)
	}
	channelID, err := deps.Channels.Save(ctx, model.Channel{
		AccountID:         accountID,
		TelegramChannelID: 100,
		AccessHash:        200,
		Title:             "NewQuark",
		Username:          "NewQuark",
		Type:              model.ChannelTypeChannel,
	})
	if err != nil {
		t.Fatalf("save channel: %v", err)
	}
	stored, err := deps.Messages.SaveBatch(ctx, []model.Message{{
		AccountID: accountID, ChannelID: channelID, TelegramMessageID: 12345,
		MessageType: "video", MediaSummary: "video/mp4", Text: "clip", RawJSON: "{}", Date: time.Now().UTC(),
	}})
	if err != nil {
		t.Fatalf("save message: %v", err)
	}
	files, err := deps.Files.SaveBatch(ctx, stored[0].ID, []model.File{{TelegramFileID: 777, FileName: "clip.mp4", Extension: ".mp4", MimeType: "video/mp4", SizeBytes: 4096}})
	if err != nil {
		t.Fatalf("save file: %v", err)
	}
	indexedFile := files[0]
	fake := &mediaProxyClient{
		file: telegram.VideoFile{ID: 1, AccessHash: 2, FileReference: []byte{3}, Size: 4096, MIMEType: "video/mp4"},
		data: bytes.Repeat([]byte{0x7a}, 1024),
	}
	deps.Telegram = fake
	router := NewRouter(deps)
	key := createTestAPIKey(t, router)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v/777", nil)
	req.Header.Set("Range", "bytes=1024-2047")
	req.Header.Set("X-API-Key", key)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusPartialContent {
		t.Fatalf("status = %d body=%s, want 206", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Range"); got != "bytes 1024-2047/4096" {
		t.Fatalf("Content-Range = %q", got)
	}
	if got := w.Header().Get("Content-Length"); got != "1024" {
		t.Fatalf("Content-Length = %q", got)
	}
	if got := w.Header().Get("Accept-Ranges"); got != "bytes" {
		t.Fatalf("Accept-Ranges = %q", got)
	}
	if got := w.Header().Get("ETag"); got != `W/"tg-file-777-4096-`+strconv.FormatInt(indexedFile.UpdatedAt.UnixNano(), 10)+`"` {
		t.Fatalf("ETag = %q", got)
	}
	if got := w.Header().Get("Last-Modified"); got != indexedFile.UpdatedAt.UTC().Format(http.TimeFormat) {
		t.Fatalf("Last-Modified = %q", got)
	}
	if got := w.Header().Get("Content-Disposition"); got != "inline; filename=clip.mp4" {
		t.Fatalf("Content-Disposition = %q", got)
	}
	if got := w.Header().Get("Cache-Control"); got != "public, max-age=86400" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if fake.offset != 1024 || fake.length != 1024 {
		t.Fatalf("stream range = offset %d length %d, want 1024/1024", fake.offset, fake.length)
	}
	if got := w.Body.Len(); got != 1024 {
		t.Fatalf("body length = %d, want 1024", got)
	}
	if fake.channel.Username != "NewQuark" || fake.channel.TelegramChannelID != 100 || fake.session.AccountID != accountID {
		t.Fatalf("unexpected media context: channel=%+v session=%+v", fake.channel, fake.session)
	}
}

func TestServeTelegramVideoHeadSkipsStreaming(t *testing.T) {
	deps := testDeps(t)
	ctx := context.Background()
	accountID, err := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	if err != nil {
		t.Fatalf("save account: %v", err)
	}
	channelID, err := deps.Channels.Save(ctx, model.Channel{
		AccountID:         accountID,
		TelegramChannelID: 100,
		AccessHash:        200,
		Title:             "NewQuark",
		Username:          "NewQuark",
		Type:              model.ChannelTypeChannel,
	})
	if err != nil {
		t.Fatalf("save channel: %v", err)
	}
	stored, err := deps.Messages.SaveBatch(ctx, []model.Message{{
		AccountID: accountID, ChannelID: channelID, TelegramMessageID: 12345,
		MessageType: "video", MediaSummary: "video/mp4", Text: "clip", RawJSON: "{}", Date: time.Now().UTC(),
	}})
	if err != nil {
		t.Fatalf("save message: %v", err)
	}
	files, err := deps.Files.SaveBatch(ctx, stored[0].ID, []model.File{{TelegramFileID: 777, FileName: "clip.mp4", Extension: ".mp4", MimeType: "video/mp4", SizeBytes: 4096}})
	if err != nil {
		t.Fatalf("save file: %v", err)
	}
	indexedFile := files[0]
	fake := &mediaProxyClient{
		file: telegram.VideoFile{ID: 1, AccessHash: 2, FileReference: []byte{3}, Size: 4096, MIMEType: "video/mp4"},
	}
	deps.Telegram = fake
	router := NewRouter(deps)
	key := createTestAPIKey(t, router)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodHead, "/v/777", nil)
	req.Header.Set("X-API-Key", key)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Length"); got != "4096" {
		t.Fatalf("Content-Length = %q", got)
	}
	if got := w.Header().Get("ETag"); got != `W/"tg-file-777-4096-`+strconv.FormatInt(indexedFile.UpdatedAt.UnixNano(), 10)+`"` {
		t.Fatalf("ETag = %q", got)
	}
	if got := w.Header().Get("Last-Modified"); got != indexedFile.UpdatedAt.UTC().Format(http.TimeFormat) {
		t.Fatalf("Last-Modified = %q", got)
	}
	if got := w.Header().Get("Content-Disposition"); got != "inline; filename=clip.mp4" {
		t.Fatalf("Content-Disposition = %q", got)
	}
	if got := w.Body.Len(); got != 0 {
		t.Fatalf("body length = %d, want 0", got)
	}
	if fake.streamCalls != 0 {
		t.Fatalf("stream calls = %d, want 0", fake.streamCalls)
	}
}

func TestServeTelegramVideoBadRange(t *testing.T) {
	deps := testDeps(t)
	accountID, _ := deps.Accounts.Save(context.Background(), model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := deps.Channels.Save(context.Background(), model.Channel{
		AccountID:         accountID,
		TelegramChannelID: 100,
		AccessHash:        200,
		Title:             "NewQuark",
		Username:          "NewQuark",
		Type:              model.ChannelTypeChannel,
	})
	stored, _ := deps.Messages.SaveBatch(context.Background(), []model.Message{{
		AccountID: accountID, ChannelID: channelID, TelegramMessageID: 12345,
		MessageType: "video", MediaSummary: "video/mp4", Text: "clip", RawJSON: "{}", Date: time.Now().UTC(),
	}})
	_, _ = deps.Files.SaveBatch(context.Background(), stored[0].ID, []model.File{{TelegramFileID: 777, FileName: "clip.mp4", Extension: ".mp4", MimeType: "video/mp4", SizeBytes: 4096}})
	deps.Telegram = &mediaProxyClient{file: telegram.VideoFile{Size: 4096, MIMEType: "video/mp4"}}
	router := NewRouter(deps)
	key := createTestAPIKey(t, router)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v/777", nil)
	req.Header.Set("Range", "bytes=4096-5000")
	req.Header.Set("X-API-Key", key)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Fatalf("status = %d, want 416", w.Code)
	}
	if got := w.Header().Get("Content-Range"); got != "bytes */4096" {
		t.Fatalf("Content-Range = %q", got)
	}
}

func TestTelegramMediaRequiresHeaderAPIKeyOrSignature(t *testing.T) {
	deps := testDeps(t)
	deps.Telegram = &mediaProxyClient{file: telegram.VideoFile{Size: 4096, MIMEType: "video/mp4"}}
	router := NewRouter(deps)
	key := createTestAPIKey(t, router)

	for _, tt := range []struct {
		name   string
		path   string
		header string
	}{
		{name: "missing", path: "/v/777"},
		{name: "invalid header key", path: "/v/777", header: "invalid"},
		{name: "query api key ignored", path: "/v/777?api_key=" + key},
		{name: "expired signature", path: signedMediaPath(t, key, "/v/777", time.Now().Add(-time.Minute))},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if tt.header != "" {
				req.Header.Set("X-API-Key", tt.header)
			}
			router.ServeHTTP(w, req)
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d body=%s, want 401", w.Code, w.Body.String())
			}
		})
	}
}

func TestMediaErrorStatusTreatsMissingThumbnailAsNotFound(t *testing.T) {
	if got := mediaErrorStatus(errors.New("no usable photo size")); got != http.StatusNotFound {
		t.Fatalf("mediaErrorStatus = %d, want 404", got)
	}
}

func TestServeTelegramImage(t *testing.T) {
	deps := testDeps(t)
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	accountID, _ := deps.Accounts.Save(context.Background(), model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := deps.Channels.Save(context.Background(), model.Channel{
		AccountID:         accountID,
		TelegramChannelID: 100,
		AccessHash:        200,
		Title:             "NewQuark",
		Username:          "NewQuark",
		Type:              model.ChannelTypeChannel,
	})
	stored, _ := deps.Messages.SaveBatch(context.Background(), []model.Message{{
		AccountID: accountID, ChannelID: channelID, TelegramMessageID: 12345,
		MessageType: "photo", MediaSummary: "photo", Text: "poster", RawJSON: "{}", Date: time.Now().UTC(),
	}})
	files, err := deps.Files.SaveBatch(context.Background(), stored[0].ID, []model.File{{TelegramFileID: 888, FileName: "poster.jpg", Extension: ".jpg", MimeType: "image/jpeg", SizeBytes: int64(len(png))}})
	if err != nil {
		t.Fatalf("save file: %v", err)
	}
	indexedFile := files[0]
	deps.Telegram = &mediaProxyClient{image: telegram.ImageFile{Data: png, MIMEType: "image/png"}}
	router := NewRouter(deps)
	key := createTestAPIKey(t, router)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, signedMediaPath(t, key, "/i/888", time.Now().Add(time.Hour)), nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if got := w.Header().Get("Content-Type"); !strings.HasPrefix(got, "image/png") {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := w.Header().Get("Content-Length"); got != strconv.Itoa(len(png)) {
		t.Fatalf("Content-Length = %q", got)
	}
	if got := w.Header().Get("Cache-Control"); got != "public, max-age=86400" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := w.Header().Get("ETag"); got != `W/"tg-file-888-8-`+strconv.FormatInt(indexedFile.UpdatedAt.UnixNano(), 10)+`"` {
		t.Fatalf("ETag = %q", got)
	}
	if got := w.Header().Get("Last-Modified"); got != indexedFile.UpdatedAt.UTC().Format(http.TimeFormat) {
		t.Fatalf("Last-Modified = %q", got)
	}
	if got := w.Header().Get("Content-Disposition"); got != "inline; filename=poster.jpg" {
		t.Fatalf("Content-Disposition = %q", got)
	}
}

func TestServeTelegramImageUsesLocalCache(t *testing.T) {
	deps := testDeps(t)
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	accountID, _ := deps.Accounts.Save(context.Background(), model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := deps.Channels.Save(context.Background(), model.Channel{
		AccountID:         accountID,
		TelegramChannelID: 100,
		AccessHash:        200,
		Title:             "NewQuark",
		Username:          "NewQuark",
		Type:              model.ChannelTypeChannel,
	})
	stored, _ := deps.Messages.SaveBatch(context.Background(), []model.Message{{
		AccountID: accountID, ChannelID: channelID, TelegramMessageID: 12345,
		MessageType: "photo", MediaSummary: "photo", Text: "poster", RawJSON: "{}", Date: time.Now().UTC(),
	}})
	_, err := deps.Files.SaveBatch(context.Background(), stored[0].ID, []model.File{{TelegramFileID: 888, FileName: "poster.jpg", Extension: ".jpg", MimeType: "image/jpeg", SizeBytes: int64(len(png))}})
	if err != nil {
		t.Fatalf("save file: %v", err)
	}
	fake := &mediaProxyClient{image: telegram.ImageFile{Data: png, MIMEType: "image/png"}}
	deps.Telegram = fake
	router := NewRouter(deps)
	key := createTestAPIKey(t, router)

	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, signedMediaPath(t, key, "/i/888", time.Now().Add(time.Hour)), nil)
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d status = %d body=%s, want 200", i+1, w.Code, w.Body.String())
		}
		if !bytes.Equal(w.Body.Bytes(), png) {
			t.Fatalf("request %d body = %x, want cached png", i+1, w.Body.Bytes())
		}
	}
	if got := fake.downloadImageCalls(); got != 1 {
		t.Fatalf("image download calls = %d, want 1", got)
	}
}

func TestServeTelegramImageDoesNotBlockOnMediaLimiter(t *testing.T) {
	deps := testDeps(t)
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	ctx := context.Background()
	accountID, _ := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := deps.Channels.Save(ctx, model.Channel{
		AccountID:         accountID,
		TelegramChannelID: 100,
		AccessHash:        200,
		Title:             "NewQuark",
		Username:          "NewQuark",
		Type:              model.ChannelTypeChannel,
	})
	stored, _ := deps.Messages.SaveBatch(ctx, []model.Message{{
		AccountID: accountID, ChannelID: channelID, TelegramMessageID: 12345,
		MessageType: "photo", MediaSummary: "photo", Text: "poster", RawJSON: "{}", Date: time.Now().UTC(),
	}})
	_, err := deps.Files.SaveBatch(ctx, stored[0].ID, []model.File{{TelegramFileID: 888, FileName: "poster.jpg", Extension: ".jpg", MimeType: "image/jpeg", SizeBytes: int64(len(png))}})
	if err != nil {
		t.Fatalf("save file: %v", err)
	}
	// Images bypass the MediaLimiter, so even with concurrency=1 both requests run concurrently.
	fake := &mediaProxyClient{image: telegram.ImageFile{Data: png, MIMEType: "image/png"}, delay: 20 * time.Millisecond}
	deps.Telegram = fake
	deps.MediaLimiter = medialimit.New(1)
	router := NewRouter(deps)
	key := createTestAPIKey(t, router)

	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, signedMediaPath(t, key, "/i/888", time.Now().Add(time.Hour)), nil)
			router.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Errorf("status = %d body=%s, want 200", w.Code, w.Body.String())
			}
		}()
	}
	close(start)
	wg.Wait()

	// Both image requests ran concurrently (bypassing the limiter), so max active should be 2.
	if got := fake.maxActive(); got != 2 {
		t.Fatalf("max active image downloads = %d, want 2 (images bypass MediaLimiter)", got)
	}
}

func TestUpdateRuntimeSettingsAppliesMediaConcurrencyImmediately(t *testing.T) {
	deps := testDeps(t)
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	ctx := context.Background()
	accountID, _ := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := deps.Channels.Save(ctx, model.Channel{
		AccountID:         accountID,
		TelegramChannelID: 100,
		AccessHash:        200,
		Title:             "NewQuark",
		Username:          "NewQuark",
		Type:              model.ChannelTypeChannel,
	})
	stored, _ := deps.Messages.SaveBatch(ctx, []model.Message{{
		AccountID: accountID, ChannelID: channelID, TelegramMessageID: 12345,
		MessageType: "photo", MediaSummary: "photo", Text: "poster", RawJSON: "{}", Date: time.Now().UTC(),
	}})
	_, err := deps.Files.SaveBatch(ctx, stored[0].ID, []model.File{{TelegramFileID: 888, FileName: "poster.jpg", Extension: ".jpg", MimeType: "image/jpeg", SizeBytes: int64(len(png))}})
	if err != nil {
		t.Fatalf("save file: %v", err)
	}
	fake := &mediaProxyClient{image: telegram.ImageFile{Data: png, MIMEType: "image/png"}, delay: 20 * time.Millisecond}
	deps.Telegram = fake
	deps.MediaLimiter = medialimit.New(1)
	router := NewRouter(deps)
	key := createTestAPIKey(t, router)
	cookie := createAdminSession(t, router)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/settings/runtime", strings.NewReader(`{
		"sync":{"workers":8,"history_batch_size":250,"telegram_request_interval":"1500ms"},
		"storage":{"max_db_size":30000000000,"max_media_cache":40000000000},
		"telegram":{
			"proxy":"",
			"reconnect_timeout":"6m",
			"dial_timeout":"15s",
			"rate_limit":{"enabled":true,"rate_per_second":12,"burst":6},
			"stream":{"concurrency":4,"buffers":8,"chunk_timeout":"30s"},
			"media":{"concurrency":2}
		}
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("put runtime settings status = %d body=%s, want 200", w.Code, w.Body.String())
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, signedMediaPath(t, key, "/i/888", time.Now().Add(time.Hour)), nil)
			router.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Errorf("status = %d body=%s, want 200", w.Code, w.Body.String())
			}
		}()
	}
	close(start)
	wg.Wait()

	if got := fake.maxActive(); got != 2 {
		t.Fatalf("max active media downloads = %d, want 2", got)
	}
}

func TestServeTelegramImageHead(t *testing.T) {
	deps := testDeps(t)
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	accountID, _ := deps.Accounts.Save(context.Background(), model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := deps.Channels.Save(context.Background(), model.Channel{
		AccountID:         accountID,
		TelegramChannelID: 100,
		AccessHash:        200,
		Title:             "NewQuark",
		Username:          "NewQuark",
		Type:              model.ChannelTypeChannel,
	})
	stored, _ := deps.Messages.SaveBatch(context.Background(), []model.Message{{
		AccountID: accountID, ChannelID: channelID, TelegramMessageID: 12345,
		MessageType: "photo", MediaSummary: "photo", Text: "poster", RawJSON: "{}", Date: time.Now().UTC(),
	}})
	files, err := deps.Files.SaveBatch(context.Background(), stored[0].ID, []model.File{{TelegramFileID: 888, FileName: "poster.jpg", Extension: ".jpg", MimeType: "image/jpeg", SizeBytes: int64(len(png))}})
	if err != nil {
		t.Fatalf("save file: %v", err)
	}
	indexedFile := files[0]
	deps.Telegram = &mediaProxyClient{image: telegram.ImageFile{Data: png, MIMEType: "image/png"}}
	router := NewRouter(deps)
	key := createTestAPIKey(t, router)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodHead, "/i/888", nil)
	req.Header.Set("X-API-Key", key)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); !strings.HasPrefix(got, "image/png") {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := w.Header().Get("Content-Length"); got != strconv.Itoa(len(png)) {
		t.Fatalf("Content-Length = %q", got)
	}
	if got := w.Header().Get("ETag"); got != `W/"tg-file-888-8-`+strconv.FormatInt(indexedFile.UpdatedAt.UnixNano(), 10)+`"` {
		t.Fatalf("ETag = %q", got)
	}
	if got := w.Header().Get("Last-Modified"); got != indexedFile.UpdatedAt.UTC().Format(http.TimeFormat) {
		t.Fatalf("Last-Modified = %q", got)
	}
	if got := w.Header().Get("Content-Disposition"); got != "inline; filename=poster.jpg" {
		t.Fatalf("Content-Disposition = %q", got)
	}
	if got := w.Body.Len(); got != 0 {
		t.Fatalf("body length = %d, want 0", got)
	}
}

func TestServeTelegramImageMarksAccountLoginRequiredOnAuthFailure(t *testing.T) {
	deps := testDeps(t)
	ctx := context.Background()
	accountID, _ := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := deps.Channels.Save(ctx, model.Channel{
		AccountID:         accountID,
		TelegramChannelID: 100,
		AccessHash:        200,
		Title:             "NewQuark",
		Username:          "NewQuark",
		Type:              model.ChannelTypeChannel,
	})
	stored, _ := deps.Messages.SaveBatch(ctx, []model.Message{{
		AccountID: accountID, ChannelID: channelID, TelegramMessageID: 12345,
		MessageType: "photo", MediaSummary: "photo", Text: "poster", RawJSON: "{}", Date: time.Now().UTC(),
	}})
	_, _ = deps.Files.SaveBatch(ctx, stored[0].ID, []model.File{{TelegramFileID: 888, FileName: "poster.jpg", Extension: ".jpg", MimeType: "image/jpeg", SizeBytes: 4096}})
	deps.Telegram = &mediaProxyClient{imageErr: errors.New("callback: rpcDoRequest: rpc error code 401: AUTH_KEY_UNREGISTERED")}
	router := NewRouter(deps)
	key := createTestAPIKey(t, router)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, signedMediaPath(t, key, "/i/888", time.Now().Add(time.Hour)), nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d body=%s, want 503", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Telegram 登录已失效，请重新登录") {
		t.Fatalf("body = %s, want localized auth failure", w.Body.String())
	}
	account, err := deps.Accounts.FindByID(ctx, accountID)
	if err != nil {
		t.Fatalf("find account: %v", err)
	}
	if account.Status != model.AccountStatusLoginRequired {
		t.Fatalf("status = %q, want LOGIN_REQUIRED", account.Status)
	}
}

func signedMediaPath(t *testing.T, key string, path string, expiresAt time.Time) string {
	t.Helper()
	exp := strconv.FormatInt(expiresAt.Unix(), 10)
	sig, err := apikey.MediaSignature(key, http.MethodGet, path, exp)
	if err != nil {
		t.Fatalf("sign media path: %v", err)
	}
	return path + "?exp=" + exp + "&sig=" + sig
}

type mediaProxyClient struct {
	telegram.NopClient
	mu          sync.Mutex
	file        telegram.VideoFile
	image       telegram.ImageFile
	videoErr    error
	imageErr    error
	imageCalls  int
	data        []byte
	session     telegram.AccountSession
	channel     telegram.MediaChannelRef
	msgID       int
	offset      int64
	length      int64
	streamCalls int
	delay       time.Duration
	active      int
	maxActiveN  int
}

func (f *mediaProxyClient) VideoFile(ctx context.Context, session telegram.AccountSession, channel telegram.MediaChannelRef, messageID int) (telegram.VideoFile, error) {
	if err := f.enter(ctx); err != nil {
		return telegram.VideoFile{}, err
	}
	defer f.leave()
	f.session = session
	f.channel = channel
	f.msgID = messageID
	if f.videoErr != nil {
		return telegram.VideoFile{}, f.videoErr
	}
	return f.file, nil
}

func (f *mediaProxyClient) StreamVideoRange(ctx context.Context, session telegram.AccountSession, channel telegram.MediaChannelRef, messageID int, file telegram.VideoFile, offset int64, length int64, w io.Writer) error {
	if err := f.enter(ctx); err != nil {
		return err
	}
	defer f.leave()
	f.streamCalls++
	f.session = session
	f.channel = channel
	f.msgID = messageID
	f.offset = offset
	f.length = length
	data := f.data
	if len(data) == 0 {
		data = bytes.Repeat([]byte{0x30}, int(length))
	}
	_, err := w.Write(data[:min(len(data), int(length))])
	return err
}

func (f *mediaProxyClient) DownloadMessageImage(ctx context.Context, session telegram.AccountSession, channel telegram.MediaChannelRef, messageID int) (telegram.ImageFile, error) {
	if err := f.enter(ctx); err != nil {
		return telegram.ImageFile{}, err
	}
	defer f.leave()
	f.mu.Lock()
	f.imageCalls++
	f.mu.Unlock()
	f.session = session
	f.channel = channel
	f.msgID = messageID
	if f.imageErr != nil {
		return telegram.ImageFile{}, f.imageErr
	}
	return f.image, nil
}

func (f *mediaProxyClient) downloadImageCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.imageCalls
}

func (f *mediaProxyClient) enter(ctx context.Context) error {
	f.mu.Lock()
	f.active++
	if f.active > f.maxActiveN {
		f.maxActiveN = f.active
	}
	delay := f.delay
	f.mu.Unlock()
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		f.leave()
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (f *mediaProxyClient) leave() {
	f.mu.Lock()
	f.active--
	f.mu.Unlock()
}

func (f *mediaProxyClient) maxActive() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.maxActiveN
}
