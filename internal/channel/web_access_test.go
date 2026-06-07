package channel

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"tg-provider/internal/db"
	"tg-provider/internal/model"
	"tg-provider/internal/repository"
)

func TestWebAccessServiceChecksPublicUsername(t *testing.T) {
	ctx := context.Background()
	accounts, channels, closeStore := setupWebAccessStore(t)
	defer closeStore()
	accountID, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := channels.Save(ctx, model.Channel{
		AccountID:         accountID,
		TelegramChannelID: 100,
		Title:             "Public",
		Username:          "public_channel",
		Type:              model.ChannelTypeChannel,
	})
	checkedAt := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	checker := &recordingWebAccessChecker{results: map[string]bool{"public_channel": true}}
	service := NewWebAccessService(channels, checker)
	service.now = func() time.Time { return checkedAt }

	results, err := service.CheckMany(ctx, []int64{channelID})
	if err != nil {
		t.Fatalf("CheckMany returned error: %v", err)
	}
	if len(results) != 1 || results[0].ChannelID != channelID || !results[0].WebAccess || !results[0].CheckedAt.Equal(checkedAt) {
		t.Fatalf("results = %+v, want checked public channel", results)
	}
	if !sameStrings(checker.calls, []string{"public_channel"}) {
		t.Fatalf("checker calls = %v, want public_channel", checker.calls)
	}
	stored, err := channels.FindByID(ctx, channelID)
	if err != nil {
		t.Fatalf("find channel: %v", err)
	}
	if stored.WebAccess == nil || *stored.WebAccess != true {
		t.Fatalf("stored web_access = %v, want true", stored.WebAccess)
	}
}

func TestWebAccessServiceStoresFalseWithoutUsername(t *testing.T) {
	ctx := context.Background()
	accounts, channels, closeStore := setupWebAccessStore(t)
	defer closeStore()
	accountID, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := channels.Save(ctx, model.Channel{
		AccountID:         accountID,
		TelegramChannelID: 101,
		Title:             "Private",
		Type:              model.ChannelTypeChannel,
	})
	checker := &recordingWebAccessChecker{results: map[string]bool{}}
	service := NewWebAccessService(channels, checker)

	results, err := service.CheckMany(ctx, []int64{channelID})
	if err != nil {
		t.Fatalf("CheckMany returned error: %v", err)
	}
	if len(results) != 1 || results[0].ChannelID != channelID || results[0].WebAccess {
		t.Fatalf("results = %+v, want false web access", results)
	}
	if len(checker.calls) != 0 {
		t.Fatalf("checker calls = %v, want none", checker.calls)
	}
	stored, err := channels.FindByID(ctx, channelID)
	if err != nil {
		t.Fatalf("find channel: %v", err)
	}
	if stored.WebAccess == nil || *stored.WebAccess != false {
		t.Fatalf("stored web_access = %v, want false", stored.WebAccess)
	}
}

func TestWebAccessServiceDeduplicatesChannelIDs(t *testing.T) {
	ctx := context.Background()
	accounts, channels, closeStore := setupWebAccessStore(t)
	defer closeStore()
	accountID, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := channels.Save(ctx, model.Channel{
		AccountID:         accountID,
		TelegramChannelID: 102,
		Title:             "Duplicate",
		Username:          "duplicate_channel",
		Type:              model.ChannelTypeChannel,
	})
	checker := &recordingWebAccessChecker{results: map[string]bool{"duplicate_channel": true}}
	service := NewWebAccessService(channels, checker)

	results, err := service.CheckMany(ctx, []int64{channelID, channelID})
	if err != nil {
		t.Fatalf("CheckMany returned error: %v", err)
	}
	if len(results) != 1 || results[0].ChannelID != channelID {
		t.Fatalf("results = %+v, want one result for duplicate id", results)
	}
	if !sameStrings(checker.calls, []string{"duplicate_channel"}) {
		t.Fatalf("checker calls = %v, want one checker call", checker.calls)
	}
}

func TestWebAccessServiceRejectsMissingChannelWithoutPartialUpdates(t *testing.T) {
	ctx := context.Background()
	accounts, channels, closeStore := setupWebAccessStore(t)
	defer closeStore()
	accountID, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := channels.Save(ctx, model.Channel{
		AccountID:         accountID,
		TelegramChannelID: 103,
		Title:             "Existing",
		Username:          "existing_channel",
		Type:              model.ChannelTypeChannel,
	})
	checker := &recordingWebAccessChecker{results: map[string]bool{"existing_channel": true}}
	service := NewWebAccessService(channels, checker)

	_, err := service.CheckMany(ctx, []int64{channelID, 999999})
	if err == nil {
		t.Fatal("CheckMany returned nil error, want missing channel error")
	}
	if len(checker.calls) != 0 {
		t.Fatalf("checker calls = %v, want none before validation succeeds", checker.calls)
	}
	stored, err := channels.FindByID(ctx, channelID)
	if err != nil {
		t.Fatalf("find channel: %v", err)
	}
	if stored.WebAccess != nil || stored.WebAccessCheckedAt != nil {
		t.Fatalf("stored web access = %v checked_at=%v, want unchanged nil values", stored.WebAccess, stored.WebAccessCheckedAt)
	}
}

func TestWebAccessServiceChecksWithMaxConcurrencyFive(t *testing.T) {
	ctx := context.Background()
	accounts, channels, closeStore := setupWebAccessStore(t)
	defer closeStore()
	accountID, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	var channelIDs []int64
	for i := 0; i < 10; i++ {
		channelID, _ := channels.Save(ctx, model.Channel{
			AccountID:         accountID,
			TelegramChannelID: int64(200 + i),
			Title:             "Concurrent",
			Username:          "concurrent_channel_" + strconv.Itoa(i),
			Type:              model.ChannelTypeChannel,
		})
		channelIDs = append(channelIDs, channelID)
	}
	checker := &concurrencyWebAccessChecker{delay: 20 * time.Millisecond}
	service := NewWebAccessService(channels, checker)

	results, err := service.CheckMany(ctx, channelIDs)
	if err != nil {
		t.Fatalf("CheckMany returned error: %v", err)
	}
	if len(results) != len(channelIDs) {
		t.Fatalf("results length = %d, want %d", len(results), len(channelIDs))
	}
	if checker.maxActive < 2 {
		t.Fatalf("max active checks = %d, want concurrent checks", checker.maxActive)
	}
	if checker.maxActive > 5 {
		t.Fatalf("max active checks = %d, want at most 5", checker.maxActive)
	}
}

func TestHTTPWebAccessCheckerRequiresTelegramMessageWrappers(t *testing.T) {
	checker := &HTTPWebAccessChecker{
		client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/s/public_channel" || req.URL.RawQuery != "q=" {
				t.Fatalf("request URL = %s, want /s/public_channel?q=", req.URL.String())
			}
			return stringResponse(http.StatusOK, `
<html>
  <body>
    <div class="tgme_container">
      <div class="tgme_widget_message_wrap"><time datetime="2026-06-07T12:00:00Z"></time></div>
    </div>
  </body>
</html>`), nil
		})},
	}

	access, err := checker.Check(context.Background(), "public_channel")
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if !access {
		t.Fatal("Check returned false, want true when message wrappers exist")
	}
}

func TestHTTPWebAccessCheckerReturnsFalseForStatusOKWithoutMessages(t *testing.T) {
	checker := &HTTPWebAccessChecker{
		client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return stringResponse(http.StatusOK, `<html><body><div class="tgme_container"></div></body></html>`), nil
		})},
	}

	access, err := checker.Check(context.Background(), "empty_channel")
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if access {
		t.Fatal("Check returned true for 200 response without message wrappers, want false")
	}
}

func setupWebAccessStore(t *testing.T) (*repository.AccountRepository, *repository.ChannelRepository, func()) {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "telegram.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := db.Migrate(context.Background(), conn); err != nil {
		_ = conn.Close()
		t.Fatalf("Migrate returned error: %v", err)
	}
	return repository.NewAccountRepository(conn), repository.NewChannelRepository(conn), func() { _ = conn.Close() }
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func stringResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

type recordingWebAccessChecker struct {
	mu      sync.Mutex
	results map[string]bool
	calls   []string
}

func (c *recordingWebAccessChecker) Check(ctx context.Context, username string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, username)
	return c.results[username], nil
}

type concurrencyWebAccessChecker struct {
	mu        sync.Mutex
	active    int
	maxActive int
	delay     time.Duration
}

func (c *concurrencyWebAccessChecker) Check(ctx context.Context, username string) (bool, error) {
	c.mu.Lock()
	c.active++
	if c.active > c.maxActive {
		c.maxActive = c.active
	}
	c.mu.Unlock()

	time.Sleep(c.delay)

	c.mu.Lock()
	c.active--
	c.mu.Unlock()
	return true, nil
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
