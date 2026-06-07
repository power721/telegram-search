package history

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"tg-provider/internal/db"
	"tg-provider/internal/link"
	"tg-provider/internal/messagefilter"
	"tg-provider/internal/model"
	"tg-provider/internal/repository"
	"tg-provider/internal/retry"
	"tg-provider/internal/session"
	"tg-provider/internal/telegram"
)

func TestSyncChannelStoresBatchesLinksAndCursor(t *testing.T) {
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
	status := repository.NewStatusRepository(conn)

	accountID, err := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	if err != nil {
		t.Fatalf("save account: %v", err)
	}
	channelID, err := channels.Save(ctx, model.Channel{
		AccountID:         accountID,
		TelegramChannelID: 200,
		AccessHash:        300,
		Title:             "VIP",
		Type:              model.ChannelTypeChannel,
	})
	if err != nil {
		t.Fatalf("save channel: %v", err)
	}

	now := time.Now().UTC()
	fake := &fakeTelegramClient{batches: map[int64][]telegram.Message{
		0: {
			{TelegramMessageID: 11, SenderID: 1, Text: "最新消息", RawJSON: "{}", Date: now},
			{TelegramMessageID: 10, SenderID: 1, Text: "庆余年 https://www.alipan.com/s/abc123 提取码: abcd", RawJSON: "{}", Date: now.Add(-time.Minute)},
		},
		10: {
			{TelegramMessageID: 10, SenderID: 1, Text: "庆余年 https://www.alipan.com/s/abc123 提取码: abcd", RawJSON: "{}", Date: now.Add(-time.Minute)},
			{TelegramMessageID: 9, SenderID: 1, Text: "更早消息 magnet:?xt=urn:btih:abc", RawJSON: "{}", Date: now.Add(-2 * time.Minute)},
		},
		9: {},
	}}
	service := NewService(Options{
		DB:               conn,
		Accounts:         accounts,
		Channels:         channels,
		Messages:         messages,
		Links:            links,
		Telegram:         fake,
		Sessions:         session.NewManager(filepath.Join(t.TempDir(), "sessions")),
		Extractor:        link.NewExtractor(),
		HistoryBatchSize: 2,
	})

	result, err := service.SyncChannel(ctx, channelID)
	if err != nil {
		t.Fatalf("SyncChannel returned error: %v", err)
	}
	if result.Messages != 4 {
		t.Fatalf("result messages = %d, want 4 fetched messages including duplicate", result.Messages)
	}

	counts, err := status.Counts(ctx)
	if err != nil {
		t.Fatalf("counts: %v", err)
	}
	if counts.Messages != 3 {
		t.Fatalf("stored message count = %d, want 3 unique messages", counts.Messages)
	}
	if counts.Links != 2 {
		t.Fatalf("stored link count = %d, want 2", counts.Links)
	}

	linkResults, err := links.Search(ctx, repository.LinkSearchParams{Type: "aliyun", Limit: 10})
	if err != nil {
		t.Fatalf("search aliyun links: %v", err)
	}
	if len(linkResults) != 1 || linkResults[0].Type != "aliyun" {
		t.Fatalf("aliyun links = %+v, want 1 aliyun link", linkResults)
	}

	channel, err := channels.FindByID(ctx, channelID)
	if err != nil {
		t.Fatalf("find channel: %v", err)
	}
	if channel.LastMessageID != 11 {
		t.Fatalf("last message id = %d, want 11", channel.LastMessageID)
	}
}

type fakeTelegramClient struct {
	telegram.NopClient
	batches map[int64][]telegram.Message
}

func (f *fakeTelegramClient) FetchHistory(ctx context.Context, account telegram.AccountSession, channel telegram.ChannelRef, offsetID int64, limit int) ([]telegram.Message, error) {
	return f.batches[offsetID], nil
}

func TestSyncChannelRetriesTemporaryFetchError(t *testing.T) {
	ctx := context.Background()
	conn, accounts, channels, messages, links := setupHistoryTestStore(t)
	_, channelID := seedHistoryAccountAndChannel(t, ctx, accounts, channels)

	now := time.Now().UTC()
	fake := &retryingHistoryClient{
		failuresBeforeSuccess: 1,
		successBatch: []telegram.Message{
			{TelegramMessageID: 7, SenderID: 1, Text: "retry success", RawJSON: "{}", Date: now},
		},
	}
	service := NewService(Options{
		DB: conn, Accounts: accounts, Channels: channels, Messages: messages, Links: links,
		Telegram: fake, Sessions: session.NewManager(filepath.Join(t.TempDir(), "sessions")),
		Extractor: link.NewExtractor(), HistoryBatchSize: 10,
		RetryPolicy: retry.Policy{
			BaseDelay: time.Millisecond,
			MaxDelay:  time.Millisecond,
			MaxTries:  3,
			Sleep:     func(context.Context, time.Duration) error { return nil },
		},
	})

	result, err := service.SyncChannel(ctx, channelID)
	if err != nil {
		t.Fatalf("SyncChannel returned error: %v", err)
	}
	if result.Messages != 1 {
		t.Fatalf("messages = %d, want 1", result.Messages)
	}
	if fake.calls != 2 {
		t.Fatalf("fetch calls = %d, want 2", fake.calls)
	}
}

func TestSyncChannelMarksAccountFloodWaitBeforeRetry(t *testing.T) {
	ctx := context.Background()
	conn, accounts, channels, messages, links := setupHistoryTestStore(t)
	accountID, channelID := seedHistoryAccountAndChannel(t, ctx, accounts, channels)

	fake := &floodThenEmptyHistoryClient{}
	service := NewService(Options{
		DB: conn, Accounts: accounts, Channels: channels, Messages: messages, Links: links,
		Telegram: fake, Sessions: session.NewManager(filepath.Join(t.TempDir(), "sessions")),
		Extractor: link.NewExtractor(), HistoryBatchSize: 10,
		RetryPolicy: retry.Policy{
			BaseDelay: time.Millisecond,
			MaxDelay:  time.Millisecond,
			MaxTries:  2,
			Sleep:     func(context.Context, time.Duration) error { return nil },
		},
	})

	_, err := service.SyncChannel(ctx, channelID)
	if err != nil {
		t.Fatalf("SyncChannel returned error: %v", err)
	}
	account, err := accounts.FindByID(ctx, accountID)
	if err != nil {
		t.Fatalf("find account: %v", err)
	}
	if account.Status != model.AccountStatusFloodWait {
		t.Fatalf("status = %q, want FLOOD_WAIT", account.Status)
	}
	if fake.calls != 2 {
		t.Fatalf("fetch calls = %d, want 2", fake.calls)
	}
}

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
		DB:               conn,
		Accounts:         accounts,
		Channels:         channels,
		Messages:         messages,
		Links:            links,
		Telegram:         fake,
		Sessions:         session.NewManager(filepath.Join(t.TempDir(), "sessions")),
		Extractor:        link.NewExtractor(),
		HistoryBatchSize: 10,
		Filter:           messagefilter.New(rules),
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
		DB:               conn,
		Accounts:         accounts,
		Channels:         channels,
		Messages:         messages,
		Links:            links,
		Telegram:         fake,
		Sessions:         session.NewManager(filepath.Join(t.TempDir(), "sessions")),
		Extractor:        link.NewExtractor(),
		HistoryBatchSize: 10,
		Filter:           messagefilter.New(rules),
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

func setupHistoryTestStore(t *testing.T) (*sql.DB, *repository.AccountRepository, *repository.ChannelRepository, *repository.MessageRepository, *repository.LinkRepository) {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "telegram.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(context.Background(), conn); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	return conn, repository.NewAccountRepository(conn), repository.NewChannelRepository(conn), repository.NewMessageRepository(conn), repository.NewLinkRepository(conn)
}

func seedHistoryAccountAndChannel(t *testing.T, ctx context.Context, accounts *repository.AccountRepository, channels *repository.ChannelRepository) (int64, int64) {
	t.Helper()
	accountID, err := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	if err != nil {
		t.Fatalf("save account: %v", err)
	}
	channelID, err := channels.Save(ctx, model.Channel{
		AccountID: accountID, TelegramChannelID: 200, AccessHash: 300, Title: "VIP", Type: model.ChannelTypeChannel,
	})
	if err != nil {
		t.Fatalf("save channel: %v", err)
	}
	return accountID, channelID
}

type retryingHistoryClient struct {
	telegram.NopClient
	calls                 int
	failuresBeforeSuccess int
	successBatch          []telegram.Message
}

func (f *retryingHistoryClient) FetchHistory(ctx context.Context, account telegram.AccountSession, channel telegram.ChannelRef, offsetID int64, limit int) ([]telegram.Message, error) {
	f.calls++
	if f.calls <= f.failuresBeforeSuccess {
		return nil, retry.Temporary(errors.New("temporary history failure"))
	}
	if offsetID > 0 {
		return nil, nil
	}
	return f.successBatch, nil
}

type floodThenEmptyHistoryClient struct {
	telegram.NopClient
	calls int
}

func (f *floodThenEmptyHistoryClient) FetchHistory(ctx context.Context, account telegram.AccountSession, channel telegram.ChannelRef, offsetID int64, limit int) ([]telegram.Message, error) {
	f.calls++
	if f.calls == 1 {
		return nil, retry.FloodWait(60, errors.New("FLOOD_WAIT_60"))
	}
	return nil, nil
}

func TestSyncManyDeduplicatesChannelIDsAndRespectsWorkerLimit(t *testing.T) {
	ctx := context.Background()
	conn, accounts, channels, messages, links := setupHistoryTestStore(t)
	accountID, channel1 := seedHistoryAccountAndChannel(t, ctx, accounts, channels)
	channel2, err := channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 201, AccessHash: 301, Title: "VIP 2", Type: model.ChannelTypeChannel})
	if err != nil {
		t.Fatalf("save channel2: %v", err)
	}
	channel3, err := channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 202, AccessHash: 302, Title: "VIP 3", Type: model.ChannelTypeChannel})
	if err != nil {
		t.Fatalf("save channel3: %v", err)
	}
	fake := &concurrentHistoryClient{delay: 5 * time.Millisecond}
	service := NewService(Options{
		DB: conn, Accounts: accounts, Channels: channels, Messages: messages, Links: links,
		Telegram: fake, Sessions: session.NewManager(filepath.Join(t.TempDir(), "sessions")),
		Extractor: link.NewExtractor(), HistoryBatchSize: 10, Workers: 2,
		RetryPolicy: retry.Policy{BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, MaxTries: 1, Sleep: func(context.Context, time.Duration) error { return nil }},
	})

	result := service.SyncMany(ctx, []int64{channel1, channel1, channel2, channel3})
	if result.Queued != 3 {
		t.Fatalf("queued = %d, want 3 unique channels", result.Queued)
	}
	if result.Skipped != 1 {
		t.Fatalf("skipped = %d, want 1 duplicate", result.Skipped)
	}
	if len(result.Failures) != 0 {
		t.Fatalf("failures = %+v, want none", result.Failures)
	}
	if fake.maxActive > 2 {
		t.Fatalf("max active = %d, want <= 2", fake.maxActive)
	}
}

func TestSyncManySkipsChannelAlreadySyncing(t *testing.T) {
	ctx := context.Background()
	conn, accounts, channels, messages, links := setupHistoryTestStore(t)
	_, channelID := seedHistoryAccountAndChannel(t, ctx, accounts, channels)
	fake := &blockingHistoryClient{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	service := NewService(Options{
		DB: conn, Accounts: accounts, Channels: channels, Messages: messages, Links: links,
		Telegram: fake, Sessions: session.NewManager(filepath.Join(t.TempDir(), "sessions")),
		Extractor: link.NewExtractor(), HistoryBatchSize: 10, Workers: 1,
		RetryPolicy: retry.Policy{BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, MaxTries: 1, Sleep: func(context.Context, time.Duration) error { return nil }},
	})

	done := make(chan error, 1)
	go func() {
		_, err := service.SyncChannel(ctx, channelID)
		done <- err
	}()
	select {
	case <-fake.started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first sync to start")
	}

	result := service.SyncMany(ctx, []int64{channelID})
	if result.Queued != 0 || result.Skipped != 1 {
		t.Fatalf("result = %+v, want queued=0 skipped=1", result)
	}

	close(fake.release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("first SyncChannel returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first sync")
	}
}

type concurrentHistoryClient struct {
	telegram.NopClient
	mu        sync.Mutex
	active    int
	maxActive int
	delay     time.Duration
}

func (f *concurrentHistoryClient) FetchHistory(ctx context.Context, account telegram.AccountSession, channel telegram.ChannelRef, offsetID int64, limit int) ([]telegram.Message, error) {
	f.mu.Lock()
	f.active++
	if f.active > f.maxActive {
		f.maxActive = f.active
	}
	f.mu.Unlock()
	time.Sleep(f.delay)
	f.mu.Lock()
	f.active--
	f.mu.Unlock()
	return nil, nil
}

type blockingHistoryClient struct {
	telegram.NopClient
	once    sync.Once
	started chan struct{}
	release chan struct{}
}

func (f *blockingHistoryClient) FetchHistory(ctx context.Context, account telegram.AccountSession, channel telegram.ChannelRef, offsetID int64, limit int) ([]telegram.Message, error) {
	f.once.Do(func() { close(f.started) })
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-f.release:
		return nil, nil
	}
}
