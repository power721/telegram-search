package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"tg-search/internal/db"
	"tg-search/internal/model"
)

func TestRemoteSearchTaskRepository(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "telegram.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	accounts := NewAccountRepository(conn)
	channels := NewChannelRepository(conn)
	tasks := NewRemoteSearchTaskRepository(conn)
	accountID, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := channels.Save(ctx, model.Channel{
		AccountID:         accountID,
		TelegramChannelID: 301,
		Title:             "Remote",
		Type:              model.ChannelTypeChannel,
	})
	expiresAt := time.Date(2026, 6, 8, 10, 30, 0, 0, time.UTC)

	id, err := tasks.Create(ctx, model.RemoteSearchTask{
		AccountID: accountID,
		ChannelID: channelID,
		Query:     "ubuntu iso",
		Status:    model.RemoteSearchStatusQueued,
		Source:    "remote",
		ExpiresAt: expiresAt,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	got, err := tasks.FindByID(ctx, id)
	if err != nil {
		t.Fatalf("FindByID returned error: %v", err)
	}
	if got.AccountID != accountID || got.ChannelID != channelID || got.Query != "ubuntu iso" ||
		got.Status != model.RemoteSearchStatusQueued || got.Source != "remote" || !got.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("task = %+v", got)
	}
}
