package api

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"tg-search/internal/apikey"
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

func TestServeTelegramVideoRange(t *testing.T) {
	deps := testDeps(t)
	ctx := context.Background()
	accountID, err := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	if err != nil {
		t.Fatalf("save account: %v", err)
	}
	_, err = deps.Channels.Save(ctx, model.Channel{
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
	fake := &mediaProxyClient{
		file: telegram.VideoFile{ID: 1, AccessHash: 2, FileReference: []byte{3}, Size: 4096, MIMEType: "video/mp4"},
		data: bytes.Repeat([]byte{0x7a}, 1024),
	}
	deps.Telegram = fake
	router := NewRouter(deps)
	key := createTestAPIKey(t, router)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v/NewQuark/12345", nil)
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

func TestServeTelegramVideoBadRange(t *testing.T) {
	deps := testDeps(t)
	accountID, _ := deps.Accounts.Save(context.Background(), model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	_, _ = deps.Channels.Save(context.Background(), model.Channel{
		AccountID:         accountID,
		TelegramChannelID: 100,
		AccessHash:        200,
		Title:             "NewQuark",
		Username:          "NewQuark",
		Type:              model.ChannelTypeChannel,
	})
	deps.Telegram = &mediaProxyClient{file: telegram.VideoFile{Size: 4096, MIMEType: "video/mp4"}}
	router := NewRouter(deps)
	key := createTestAPIKey(t, router)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v/NewQuark/12345", nil)
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
		{name: "missing", path: "/v/NewQuark/12345"},
		{name: "invalid header key", path: "/v/NewQuark/12345", header: "invalid"},
		{name: "query api key ignored", path: "/v/NewQuark/12345?api_key=" + key},
		{name: "expired signature", path: signedMediaPath(t, key, "/v/NewQuark/12345", time.Now().Add(-time.Minute))},
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

func TestServeTelegramImage(t *testing.T) {
	deps := testDeps(t)
	accountID, _ := deps.Accounts.Save(context.Background(), model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	_, _ = deps.Channels.Save(context.Background(), model.Channel{
		AccountID:         accountID,
		TelegramChannelID: 100,
		AccessHash:        200,
		Title:             "NewQuark",
		Username:          "NewQuark",
		Type:              model.ChannelTypeChannel,
	})
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	deps.Telegram = &mediaProxyClient{image: telegram.ImageFile{Data: png, MIMEType: "image/png"}}
	router := NewRouter(deps)
	key := createTestAPIKey(t, router)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, signedMediaPath(t, key, "/i/NewQuark/12345", time.Now().Add(time.Hour)), nil)
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
	file    telegram.VideoFile
	image   telegram.ImageFile
	data    []byte
	session telegram.AccountSession
	channel telegram.MediaChannelRef
	msgID   int
	offset  int64
	length  int64
}

func (f *mediaProxyClient) VideoFile(ctx context.Context, session telegram.AccountSession, channel telegram.MediaChannelRef, messageID int) (telegram.VideoFile, error) {
	f.session = session
	f.channel = channel
	f.msgID = messageID
	return f.file, nil
}

func (f *mediaProxyClient) StreamVideoRange(ctx context.Context, session telegram.AccountSession, channel telegram.MediaChannelRef, messageID int, file telegram.VideoFile, offset int64, length int64, w io.Writer) error {
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
	f.session = session
	f.channel = channel
	f.msgID = messageID
	return f.image, nil
}
