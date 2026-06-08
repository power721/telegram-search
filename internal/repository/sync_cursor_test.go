package repository

import (
	"context"
	"testing"
	"time"

	"tg-search/internal/model"
)

func TestSyncCursorRepository(t *testing.T) {
	ctx := context.Background()
	conn := openRepositoryTestDB(t)

	accounts := NewAccountRepository(conn)
	channels := NewChannelRepository(conn)
	cursors := NewSyncCursorRepository(conn)

	accountID, err := accounts.Save(ctx, model.Account{
		Phone:  "+10000000000",
		Status: model.AccountStatusOnline,
	})
	if err != nil {
		t.Fatalf("save account: %v", err)
	}
	channelID, err := channels.Save(ctx, model.Channel{
		AccountID:         accountID,
		TelegramChannelID: 2001,
		Title:             "VIP",
		Type:              model.ChannelTypeChannel,
	})
	if err != nil {
		t.Fatalf("save channel: %v", err)
	}

	cursorDate := time.Date(2026, 6, 8, 10, 30, 0, 0, time.UTC)
	if err := cursors.Save(ctx, model.SyncCursor{
		AccountID:     accountID,
		ChannelID:     channelID,
		CursorType:    "history",
		LastMessageID: 123,
		PTS:           10,
		QTS:           20,
		Date:          cursorDate,
	}); err != nil {
		t.Fatalf("save cursor: %v", err)
	}

	stored, err := cursors.Find(ctx, accountID, channelID, "history")
	if err != nil {
		t.Fatalf("find cursor: %v", err)
	}
	if stored.ID == 0 || stored.AccountID != accountID || stored.ChannelID != channelID || stored.CursorType != "history" {
		t.Fatalf("stored cursor identity = %+v", stored)
	}
	if stored.LastMessageID != 123 || stored.PTS != 10 || stored.QTS != 20 || !stored.Date.Equal(cursorDate) {
		t.Fatalf("stored cursor values = %+v", stored)
	}
	if stored.CreatedAt.IsZero() || stored.UpdatedAt.IsZero() {
		t.Fatalf("stored cursor timestamps = created %v updated %v", stored.CreatedAt, stored.UpdatedAt)
	}

	updatedDate := cursorDate.Add(time.Hour)
	if err := cursors.Save(ctx, model.SyncCursor{
		AccountID:     accountID,
		ChannelID:     channelID,
		CursorType:    "history",
		LastMessageID: 456,
		PTS:           30,
		QTS:           40,
		Date:          updatedDate,
	}); err != nil {
		t.Fatalf("update cursor: %v", err)
	}

	var rows int
	if err := conn.QueryRowContext(ctx, `
SELECT count(*)
FROM telegram_sync_cursors
WHERE account_id = ? AND channel_id = ? AND cursor_type = ?`, accountID, channelID, "history").Scan(&rows); err != nil {
		t.Fatalf("count cursors: %v", err)
	}
	if rows != 1 {
		t.Fatalf("cursor rows = %d, want 1", rows)
	}

	updated, err := cursors.Find(ctx, accountID, channelID, "history")
	if err != nil {
		t.Fatalf("find updated cursor: %v", err)
	}
	if updated.ID != stored.ID {
		t.Fatalf("updated id = %d, want %d", updated.ID, stored.ID)
	}
	if updated.LastMessageID != 456 || updated.PTS != 30 || updated.QTS != 40 || !updated.Date.Equal(updatedDate) {
		t.Fatalf("updated cursor values = %+v", updated)
	}
}
