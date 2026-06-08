package history

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"tg-search/internal/db"
	"tg-search/internal/link"
	"tg-search/internal/messagefilter"
	"tg-search/internal/model"
	"tg-search/internal/repository"
	"tg-search/internal/resource"
	"tg-search/internal/retry"
	"tg-search/internal/session"
	taskpkg "tg-search/internal/task"
	"tg-search/internal/telegram"
)

func TestSyncChannelUsesSyncProfile(t *testing.T) {
	profiles := []struct {
		name      string
		available int
		want      int
	}{
		{name: "Quick", available: 101, want: 100},
		{name: "Normal", available: 1001, want: 1000},
		{name: "Deep", available: 10001, want: 10000},
	}
	for _, tt := range profiles {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			conn, accounts, channels, messages, links := setupHistoryTestStore(t)
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
				SyncProfile:       tt.name,
			})
			if err != nil {
				t.Fatalf("save channel: %v", err)
			}

			fake := &profileLimitHistoryClient{available: tt.available, startID: 50000}
			service := NewService(Options{
				DB: conn, Accounts: accounts, Channels: channels, Messages: messages, Links: links,
				Telegram: fake, Sessions: session.NewManager(filepath.Join(t.TempDir(), "sessions")),
				Extractor: link.NewExtractor(), HistoryBatchSize: 600,
			})

			result, err := service.SyncChannel(ctx, channelID)
			if err != nil {
				t.Fatalf("SyncChannel returned error: %v", err)
			}
			if result.Messages != tt.want {
				t.Fatalf("messages = %d, want %d", result.Messages, tt.want)
			}
			if fake.fetched != tt.want {
				t.Fatalf("fetched = %d, want %d", fake.fetched, tt.want)
			}

			cursor, err := repository.NewSyncCursorRepository(conn).Find(ctx, accountID, channelID, "history")
			if err != nil {
				t.Fatalf("find sync cursor: %v", err)
			}
			if cursor.LastMessageID != 50000 {
				t.Fatalf("cursor last message id = %d, want 50000", cursor.LastMessageID)
			}
			if cursor.Date.IsZero() {
				t.Fatal("cursor date is zero")
			}

			channel, err := channels.FindByID(ctx, channelID)
			if err != nil {
				t.Fatalf("find channel: %v", err)
			}
			if channel.LastMessageID != 0 {
				t.Fatalf("channel last message id = %d, want 0 because sync cursor table owns history state", channel.LastMessageID)
			}
		})
	}
}

func TestSyncChannelFullProfileFetchesUntilEmptyBatch(t *testing.T) {
	ctx := context.Background()
	conn, accounts, channels, messages, links := setupHistoryTestStore(t)
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
		SyncProfile:       "Full",
	})
	if err != nil {
		t.Fatalf("save channel: %v", err)
	}

	fake := &scriptedHistoryClient{batches: [][]telegram.Message{
		{
			{TelegramMessageID: 3, Text: "short batch 3", RawJSON: "{}", Date: time.Now().UTC()},
			{TelegramMessageID: 2, Text: "short batch 2", RawJSON: "{}", Date: time.Now().UTC()},
		},
		{},
	}}
	service := NewService(Options{
		DB: conn, Accounts: accounts, Channels: channels, Messages: messages, Links: links,
		Telegram: fake, Sessions: session.NewManager(filepath.Join(t.TempDir(), "sessions")),
		Extractor: link.NewExtractor(), HistoryBatchSize: 5,
	})

	result, err := service.SyncChannel(ctx, channelID)
	if err != nil {
		t.Fatalf("SyncChannel returned error: %v", err)
	}
	if result.Messages != 2 {
		t.Fatalf("messages = %d, want 2", result.Messages)
	}
	if fake.calls != 2 {
		t.Fatalf("fetch calls = %d, want 2 so Full stops only after empty batch", fake.calls)
	}
	if !reflect.DeepEqual(fake.offsets, []int64{0, 2}) {
		t.Fatalf("offsets = %v, want [0 2]", fake.offsets)
	}
}

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
	resourceStats := repository.NewResourceStatsRepository(conn)
	resources := resource.NewService(links, nil, resourceStats)
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
		Resources:        resources,
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
	grouped, found, err := resourceStats.GetGrouped(ctx)
	if err != nil {
		t.Fatalf("get resource stats: %v", err)
	}
	if !found || grouped["cloud_drive"] != 1 || grouped["magnet"] != 1 {
		t.Fatalf("resource stats = %+v found=%v, want cloud_drive=1 magnet=1", grouped, found)
	}

	linkResults, err := links.Search(ctx, repository.LinkSearchParams{Type: "aliyun", Limit: 10})
	if err != nil {
		t.Fatalf("search aliyun links: %v", err)
	}
	if len(linkResults) != 1 || linkResults[0].Type != "aliyun" {
		t.Fatalf("aliyun links = %+v, want 1 aliyun link", linkResults)
	}

	cursor, err := repository.NewSyncCursorRepository(conn).Find(ctx, accountID, channelID, "history")
	if err != nil {
		t.Fatalf("find sync cursor: %v", err)
	}
	if cursor.LastMessageID != 11 {
		t.Fatalf("cursor last message id = %d, want 11", cursor.LastMessageID)
	}
	channel, err := channels.FindByID(ctx, channelID)
	if err != nil {
		t.Fatalf("find channel: %v", err)
	}
	if channel.LastMessageID != 0 {
		t.Fatalf("channel last message id = %d, want 0 because sync cursor table owns history state", channel.LastMessageID)
	}
}

type fakeTelegramClient struct {
	telegram.NopClient
	batches map[int64][]telegram.Message
}

func (f *fakeTelegramClient) FetchHistory(ctx context.Context, account telegram.AccountSession, channel telegram.ChannelRef, offsetID int64, limit int) ([]telegram.Message, error) {
	return f.batches[offsetID], nil
}

type profileLimitHistoryClient struct {
	telegram.NopClient
	available int
	startID   int64
	fetched   int
}

func (f *profileLimitHistoryClient) FetchHistory(ctx context.Context, account telegram.AccountSession, channel telegram.ChannelRef, offsetID int64, limit int) ([]telegram.Message, error) {
	if f.fetched >= f.available {
		return nil, nil
	}
	remaining := f.available - f.fetched
	if limit > remaining {
		limit = remaining
	}
	out := make([]telegram.Message, 0, limit)
	now := time.Now().UTC()
	for i := 0; i < limit; i++ {
		out = append(out, telegram.Message{
			TelegramMessageID: f.startID - int64(f.fetched+i),
			Text:              "profile message",
			RawJSON:           "{}",
			Date:              now,
		})
	}
	f.fetched += limit
	return out, nil
}

type scriptedHistoryClient struct {
	telegram.NopClient
	batches [][]telegram.Message
	calls   int
	offsets []int64
}

func (f *scriptedHistoryClient) FetchHistory(ctx context.Context, account telegram.AccountSession, channel telegram.ChannelRef, offsetID int64, limit int) ([]telegram.Message, error) {
	f.offsets = append(f.offsets, offsetID)
	if f.calls >= len(f.batches) {
		f.calls++
		return nil, nil
	}
	batch := f.batches[f.calls]
	f.calls++
	return batch, nil
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

func TestHistorySyncTaskProgressUsesProfileLimit(t *testing.T) {
	ctx := context.Background()
	conn, accounts, channels, messages, links := setupHistoryTestStore(t)
	accountID, err := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	if err != nil {
		t.Fatalf("save account: %v", err)
	}
	channelID, err := channels.Save(ctx, model.Channel{
		AccountID: accountID, TelegramChannelID: 200, AccessHash: 300, Title: "VIP", Type: model.ChannelTypeChannel,
		SyncProfile: "Quick",
	})
	if err != nil {
		t.Fatalf("save channel: %v", err)
	}
	now := time.Now().UTC()
	fake := &fakeTelegramClient{batches: map[int64][]telegram.Message{
		0: {{TelegramMessageID: 3, Text: "first", RawJSON: "{}", Date: now}},
		3: {{TelegramMessageID: 2, Text: "second", RawJSON: "{}", Date: now}},
		2: {},
	}}
	sink := &recordingProgressSink{}
	service := NewService(Options{
		DB: conn, Accounts: accounts, Channels: channels, Messages: messages, Links: links,
		Telegram: fake, Sessions: session.NewManager(filepath.Join(t.TempDir(), "sessions")),
		Extractor: link.NewExtractor(), HistoryBatchSize: 1,
	})

	result, err := service.SyncChannelWithProgress(ctx, channelID, "", sink)
	if err != nil {
		t.Fatalf("SyncChannelWithProgress returned error: %v", err)
	}
	if result.Messages != 2 {
		t.Fatalf("messages = %d, want 2", result.Messages)
	}
	if len(sink.updates) != 2 {
		t.Fatalf("progress updates = %+v, want 2 updates", sink.updates)
	}
	if sink.updates[0].progress != 1 || sink.updates[0].total != 100 {
		t.Fatalf("first progress = %+v, want progress=1 total=100", sink.updates[0])
	}
	if sink.updates[1].progress != 2 || sink.updates[1].total != 100 {
		t.Fatalf("second progress = %+v, want progress=2 total=100", sink.updates[1])
	}
}

func TestHistorySyncTaskProgressFullProfileUsesUnknownTotal(t *testing.T) {
	ctx := context.Background()
	conn, accounts, channels, messages, links := setupHistoryTestStore(t)
	_, channelID := seedHistoryAccountAndChannel(t, ctx, accounts, channels)
	if err := channels.UpdateControl(ctx, channelID, model.ChannelControl{SyncProfile: "Full"}); err != nil {
		t.Fatalf("update controls: %v", err)
	}
	now := time.Now().UTC()
	fake := &fakeTelegramClient{batches: map[int64][]telegram.Message{
		0: {{TelegramMessageID: 3, Text: "first", RawJSON: "{}", Date: now}},
		3: {},
	}}
	sink := &recordingProgressSink{}
	service := NewService(Options{
		DB: conn, Accounts: accounts, Channels: channels, Messages: messages, Links: links,
		Telegram: fake, Sessions: session.NewManager(filepath.Join(t.TempDir(), "sessions")),
		Extractor: link.NewExtractor(), HistoryBatchSize: 1,
	})

	if _, err := service.SyncChannelWithProgress(ctx, channelID, "", sink); err != nil {
		t.Fatalf("SyncChannelWithProgress returned error: %v", err)
	}
	if len(sink.updates) != 1 || sink.updates[0].total != 0 {
		t.Fatalf("progress updates = %+v, want one unknown-total update", sink.updates)
	}
}

func TestHistorySyncTaskCancelStopsFutureBatches(t *testing.T) {
	ctx := context.Background()
	conn, accounts, channels, messages, links := setupHistoryTestStore(t)
	_, channelID := seedHistoryAccountAndChannel(t, ctx, accounts, channels)
	now := time.Now().UTC()
	fake := &fakeTelegramClient{batches: map[int64][]telegram.Message{
		0: {{TelegramMessageID: 3, Text: "first", RawJSON: "{}", Date: now}},
		3: {{TelegramMessageID: 2, Text: "second", RawJSON: "{}", Date: now}},
	}}
	sink := &recordingProgressSink{statusAfterUpdates: model.TaskStatusCanceling}
	service := NewService(Options{
		DB: conn, Accounts: accounts, Channels: channels, Messages: messages, Links: links,
		Telegram: fake, Sessions: session.NewManager(filepath.Join(t.TempDir(), "sessions")),
		Extractor: link.NewExtractor(), HistoryBatchSize: 1,
	})

	result, err := service.SyncChannelWithProgress(ctx, channelID, "", sink)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if result.Messages != 1 {
		t.Fatalf("messages = %d, want first batch only", result.Messages)
	}
	latest, err := messages.Latest(ctx, repository.LatestParams{Limit: 10})
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	if len(latest) != 1 || latest[0].TelegramMessageID != 3 {
		t.Fatalf("stored messages = %+v, want only first batch", latest)
	}
}

func TestHistorySyncTaskFloodWaitNotifiesSink(t *testing.T) {
	ctx := context.Background()
	conn, accounts, channels, messages, links := setupHistoryTestStore(t)
	_, channelID := seedHistoryAccountAndChannel(t, ctx, accounts, channels)
	fake := &floodThenEmptyHistoryClient{}
	sink := &recordingProgressSink{}
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

	before := time.Now().UTC()
	_, err := service.SyncChannelWithProgress(ctx, channelID, "", sink)
	if err != nil {
		t.Fatalf("SyncChannelWithProgress returned error: %v", err)
	}
	if len(sink.floodWaits) != 1 {
		t.Fatalf("flood waits = %+v, want one", sink.floodWaits)
	}
	if sink.floodWaits[0].nextRunAt.Before(before) || sink.floodWaits[0].message == "" {
		t.Fatalf("flood wait = %+v, want future next run and message", sink.floodWaits[0])
	}
}

type progressUpdate struct {
	progress int64
	total    int64
	message  string
}

type floodWaitUpdate struct {
	nextRunAt time.Time
	message   string
}

type recordingProgressSink struct {
	updates            []progressUpdate
	statusAfterUpdates string
	floodWaits         []floodWaitUpdate
}

var _ taskpkg.ProgressSink = (*recordingProgressSink)(nil)

func (s *recordingProgressSink) Progress(ctx context.Context, progress int64, total int64, message string) error {
	s.updates = append(s.updates, progressUpdate{progress: progress, total: total, message: message})
	return nil
}

func (s *recordingProgressSink) Status(ctx context.Context) (string, error) {
	if s.statusAfterUpdates != "" && len(s.updates) > 0 {
		return s.statusAfterUpdates, nil
	}
	return model.TaskStatusRunning, nil
}

func (s *recordingProgressSink) FloodWait(ctx context.Context, nextRunAt time.Time, message string) error {
	s.floodWaits = append(s.floodWaits, floodWaitUpdate{nextRunAt: nextRunAt, message: message})
	return nil
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
