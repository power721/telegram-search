package search

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"tg-search/internal/db"
	"tg-search/internal/model"
	"tg-search/internal/repository"
	"tg-search/internal/session"
	taskpkg "tg-search/internal/task"
	"tg-search/internal/telegram"
)

func TestRemoteSearchDoesNotPersistResults(t *testing.T) {
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
	tasks := repository.NewRemoteSearchTaskRepository(conn)
	accountID, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := channels.Save(ctx, model.Channel{
		AccountID:           accountID,
		TelegramChannelID:   100,
		AccessHash:          200,
		Title:               "Remote",
		Type:                model.ChannelTypeChannel,
		RemoteSearchAllowed: true,
	})
	before := tableCounts(t, ctx, conn)

	client := &remoteSearchTelegramClient{items: []telegram.Message{{
		TelegramMessageID: 10,
		SenderID:          1,
		Text:              "ubuntu iso remote",
		RawJSON:           "{}",
		Date:              time.Now().UTC(),
	}}}
	service := NewRemoteService(RemoteOptions{
		Accounts: accounts,
		Channels: channels,
		Tasks:    tasks,
		Cursors:  repository.NewSyncCursorRepository(conn),
		Telegram: client,
		Sessions: session.NewManager(filepath.Join(t.TempDir(), "sessions")),
	})

	task, err := service.Search(ctx, channelID, "ubuntu", 10)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if task.ID == 0 || task.Source != "remote" || task.Query != "ubuntu" {
		t.Fatalf("task = %+v", task)
	}
	results, err := service.Results(ctx, task.ID)
	if err != nil {
		t.Fatalf("Results returned error: %v", err)
	}
	if len(results.Items) != 1 || results.Items[0].Source != "remote" || results.Items[0].Text != "ubuntu iso remote" {
		t.Fatalf("results = %+v", results)
	}
	if client.calls != 1 || client.query != "ubuntu" || client.limit != 10 {
		t.Fatalf("telegram search call = calls %d query %q limit %d", client.calls, client.query, client.limit)
	}

	after := tableCounts(t, ctx, conn)
	if before != after {
		t.Fatalf("local table counts changed: before=%+v after=%+v", before, after)
	}
}

func TestRemoteSearchRejectsSyncedChannel(t *testing.T) {
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
	cursors := repository.NewSyncCursorRepository(conn)
	tasks := repository.NewRemoteSearchTaskRepository(conn)
	accountID, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := channels.Save(ctx, model.Channel{
		AccountID:           accountID,
		TelegramChannelID:   100,
		AccessHash:          200,
		Title:               "Synced",
		Type:                model.ChannelTypeChannel,
		RemoteSearchAllowed: true,
	})
	if err := cursors.Save(ctx, model.SyncCursor{AccountID: accountID, ChannelID: channelID, CursorType: "history", LastMessageID: 100, Date: time.Now().UTC()}); err != nil {
		t.Fatalf("save cursor: %v", err)
	}

	service := NewRemoteService(RemoteOptions{
		Accounts: accounts,
		Channels: channels,
		Tasks:    tasks,
		Cursors:  cursors,
		Telegram: &remoteSearchTelegramClient{},
		Sessions: session.NewManager(filepath.Join(t.TempDir(), "sessions")),
	})

	_, err = service.Search(ctx, channelID, "ubuntu", 10)
	if err != ErrRemoteSearchRequiresUnsynced {
		t.Fatalf("error = %v, want ErrRemoteSearchRequiresUnsynced", err)
	}
}

func TestRemoteSearchWithProgressStopsBeforeTelegramSearch(t *testing.T) {
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
	tasks := repository.NewRemoteSearchTaskRepository(conn)
	accountID, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := channels.Save(ctx, model.Channel{
		AccountID:           accountID,
		TelegramChannelID:   100,
		AccessHash:          200,
		Title:               "Remote",
		Type:                model.ChannelTypeChannel,
		RemoteSearchAllowed: true,
	})
	client := &remoteSearchTelegramClient{}
	service := NewRemoteService(RemoteOptions{
		Accounts: accounts,
		Channels: channels,
		Tasks:    tasks,
		Cursors:  repository.NewSyncCursorRepository(conn),
		Telegram: client,
		Sessions: session.NewManager(filepath.Join(t.TempDir(), "sessions")),
	})

	_, err = service.SearchWithProgress(ctx, channelID, "ubuntu", 10, &remoteProgressSink{status: model.TaskStatusCanceling})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if client.calls != 0 {
		t.Fatalf("telegram search calls = %d, want 0", client.calls)
	}
}

func TestRemoteSearchWithProgressReportsResultCount(t *testing.T) {
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
	tasks := repository.NewRemoteSearchTaskRepository(conn)
	accountID, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := channels.Save(ctx, model.Channel{
		AccountID:           accountID,
		TelegramChannelID:   100,
		AccessHash:          200,
		Title:               "Remote",
		Type:                model.ChannelTypeChannel,
		RemoteSearchAllowed: true,
	})
	client := &remoteSearchTelegramClient{items: []telegram.Message{
		{TelegramMessageID: 10, Text: "ubuntu remote", RawJSON: "{}", Date: time.Now().UTC()},
	}}
	sink := &remoteProgressSink{status: model.TaskStatusRunning}
	service := NewRemoteService(RemoteOptions{
		Accounts: accounts,
		Channels: channels,
		Tasks:    tasks,
		Cursors:  repository.NewSyncCursorRepository(conn),
		Telegram: client,
		Sessions: session.NewManager(filepath.Join(t.TempDir(), "sessions")),
	})

	if _, err := service.SearchWithProgress(ctx, channelID, "ubuntu", 10, sink); err != nil {
		t.Fatalf("SearchWithProgress returned error: %v", err)
	}
	if len(sink.updates) != 1 || sink.updates[0].progress != 1 || sink.updates[0].total != 10 {
		t.Fatalf("progress updates = %+v, want one 1/10 update", sink.updates)
	}
}

type remoteProgressUpdate struct {
	progress int64
	total    int64
	message  string
}

type remoteProgressSink struct {
	status  string
	updates []remoteProgressUpdate
}

var _ taskpkg.ProgressSink = (*remoteProgressSink)(nil)

func (s *remoteProgressSink) Progress(ctx context.Context, progress int64, total int64, message string) error {
	s.updates = append(s.updates, remoteProgressUpdate{progress: progress, total: total, message: message})
	return nil
}

func (s *remoteProgressSink) Status(ctx context.Context) (string, error) {
	return s.status, nil
}

type remoteSearchTelegramClient struct {
	telegram.NopClient
	items []telegram.Message
	calls int
	query string
	limit int
}

func (f *remoteSearchTelegramClient) SearchMessages(ctx context.Context, session telegram.AccountSession, channel telegram.ChannelRef, query string, limit int) ([]telegram.Message, error) {
	f.calls++
	f.query = query
	f.limit = limit
	return f.items, nil
}

type localTableCounts struct {
	messages int
	contents int
	links    int
	fts      int
}

func tableCounts(t *testing.T, ctx context.Context, conn *sql.DB) localTableCounts {
	t.Helper()
	var counts localTableCounts
	for _, item := range []struct {
		query string
		dest  *int
	}{
		{`SELECT count(*) FROM telegram_messages`, &counts.messages},
		{`SELECT count(*) FROM telegram_message_contents`, &counts.contents},
		{`SELECT count(*) FROM telegram_links`, &counts.links},
		{`SELECT count(*) FROM telegram_messages_fts`, &counts.fts},
	} {
		if err := conn.QueryRowContext(ctx, item.query).Scan(item.dest); err != nil {
			t.Fatalf("count table: %v", err)
		}
	}
	return counts
}
