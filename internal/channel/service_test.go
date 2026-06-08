package channel

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"tg-search/internal/db"
	"tg-search/internal/model"
	"tg-search/internal/repository"
	"tg-search/internal/session"
	"tg-search/internal/telegram"
)

func TestServiceSyncAccountKeepsChannelsIsolated(t *testing.T) {
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
	account1, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	account2, _ := accounts.Save(ctx, model.Account{Phone: "+10000000001", Status: model.AccountStatusOnline})
	client := &recordingChannelClient{
		items: map[int64][]telegram.Channel{
			account1: {{TelegramChannelID: 100, Title: "One", Type: model.ChannelTypeChannel}},
			account2: {{TelegramChannelID: 100, Title: "Two", Type: model.ChannelTypeChannel}},
		},
	}
	service := NewService(channels, client, session.NewManager(filepath.Join(t.TempDir(), "sessions")))

	a1, _ := accounts.FindByID(ctx, account1)
	a2, _ := accounts.FindByID(ctx, account2)
	if _, err := service.SyncAccount(ctx, a1); err != nil {
		t.Fatalf("sync account1: %v", err)
	}
	if _, err := service.SyncAccount(ctx, a2); err != nil {
		t.Fatalf("sync account2: %v", err)
	}

	account1Channels, err := channels.FindByAccountID(ctx, account1)
	if err != nil {
		t.Fatalf("find account1 channels: %v", err)
	}
	account2Channels, err := channels.FindByAccountID(ctx, account2)
	if err != nil {
		t.Fatalf("find account2 channels: %v", err)
	}
	if len(account1Channels) != 1 || account1Channels[0].Title != "One" {
		t.Fatalf("account1 channels = %+v", account1Channels)
	}
	if len(account2Channels) != 1 || account2Channels[0].Title != "Two" {
		t.Fatalf("account2 channels = %+v", account2Channels)
	}
	if !client.sawSession(account1) || !client.sawSession(account2) {
		t.Fatalf("client sessions = %+v, want both account ids", client.sessions)
	}
}

type recordingChannelClient struct {
	telegram.NopClient
	mu       sync.Mutex
	items    map[int64][]telegram.Channel
	sessions []telegram.AccountSession
}

func (c *recordingChannelClient) ListChannels(ctx context.Context, accountSession telegram.AccountSession) ([]telegram.Channel, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessions = append(c.sessions, accountSession)
	return c.items[accountSession.AccountID], nil
}

func (c *recordingChannelClient) sawSession(accountID int64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, accountSession := range c.sessions {
		if accountSession.AccountID == accountID && accountSession.SessionPath != "" {
			return true
		}
	}
	return false
}
