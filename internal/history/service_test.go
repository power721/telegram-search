package history

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"tg-provider/internal/db"
	"tg-provider/internal/link"
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
