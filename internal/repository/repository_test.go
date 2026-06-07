package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"tg-provider/internal/db"
	"tg-provider/internal/model"
)

func TestRepositoriesPersistSearchAndCountData(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "telegram.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}

	accounts := NewAccountRepository(conn)
	channels := NewChannelRepository(conn)
	messages := NewMessageRepository(conn)
	links := NewLinkRepository(conn)
	status := NewStatusRepository(conn)

	account := model.Account{
		Phone:          "+10000000000",
		TelegramUserID: 1001,
		FirstName:      "Main",
		Username:       "main",
		Status:         model.AccountStatusOnline,
	}
	accountID, err := accounts.Save(ctx, account)
	if err != nil {
		t.Fatalf("save account: %v", err)
	}

	channelID, err := channels.Save(ctx, model.Channel{
		AccountID:         accountID,
		TelegramChannelID: 2001,
		AccessHash:        3001,
		Title:             "VIP",
		Username:          "vip",
		Type:              model.ChannelTypeChannel,
	})
	if err != nil {
		t.Fatalf("save channel: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	stored, err := messages.SaveBatch(ctx, []model.Message{
		{
			AccountID:          accountID,
			ChannelID:          channelID,
			TelegramMessageID:  10,
			SenderID:           7,
			Text:               "庆余年 阿里云盘 https://www.aliyundrive.com/s/abc",
			RawJSON:            "{}",
			Date:               now.Add(-time.Hour),
			OriginalLinkInputs: []model.Link{{Type: "url", URL: "https://www.aliyundrive.com/s/abc"}},
		},
		{
			AccountID:         accountID,
			ChannelID:         channelID,
			TelegramMessageID: 11,
			SenderID:          7,
			Text:              "other message",
			RawJSON:           "{}",
			Date:              now,
		},
	})
	if err != nil {
		t.Fatalf("save messages: %v", err)
	}
	if len(stored) != 2 {
		t.Fatalf("stored messages length = %d, want 2", len(stored))
	}
	if _, err := links.SaveBatch(ctx, stored[0].ID, []model.Link{{Type: "url", URL: "https://www.aliyundrive.com/s/abc"}}); err != nil {
		t.Fatalf("save links: %v", err)
	}

	_, err = messages.SaveBatch(ctx, []model.Message{
		{
			AccountID:         accountID,
			ChannelID:         channelID,
			TelegramMessageID: 10,
			SenderID:          7,
			Text:              "庆余年 edited",
			RawJSON:           "{}",
			Date:              now.Add(-time.Hour),
		},
	})
	if err != nil {
		t.Fatalf("save duplicate message: %v", err)
	}

	results, err := messages.Search(ctx, SearchParams{Query: "庆余年", Limit: 10})
	if err != nil {
		t.Fatalf("search messages: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("search result length = %d, want 1", len(results))
	}
	if results[0].Text != "庆余年 edited" {
		t.Fatalf("search text = %q, want updated text", results[0].Text)
	}
	if results[0].AccountUsername != "main" || results[0].ChannelTitle != "VIP" {
		t.Fatalf("search metadata = account %q channel %q", results[0].AccountUsername, results[0].ChannelTitle)
	}

	latest, err := messages.Latest(ctx, LatestParams{Limit: 2})
	if err != nil {
		t.Fatalf("latest messages: %v", err)
	}
	if len(latest) != 2 {
		t.Fatalf("latest length = %d, want 2", len(latest))
	}
	if latest[0].TelegramMessageID != 11 {
		t.Fatalf("latest first telegram id = %d, want 11", latest[0].TelegramMessageID)
	}

	linkResults, err := links.Search(ctx, LinkSearchParams{Type: "url", Limit: 10})
	if err != nil {
		t.Fatalf("search links: %v", err)
	}
	if len(linkResults) != 1 {
		t.Fatalf("link result length = %d, want 1", len(linkResults))
	}

	counts, err := status.Counts(ctx)
	if err != nil {
		t.Fatalf("counts: %v", err)
	}
	if counts.Accounts != 1 || counts.Channels != 1 || counts.Messages != 2 || counts.Links != 1 {
		t.Fatalf("counts = %+v, want 1 account 1 channel 2 messages 1 link", counts)
	}
	secondAccountID, err := accounts.Save(ctx, model.Account{Phone: "+10000000001", Status: model.AccountStatusReconnecting})
	if err != nil {
		t.Fatalf("save reconnecting account: %v", err)
	}
	if secondAccountID == 0 {
		t.Fatal("second account id = 0")
	}
	counts, err = status.Counts(ctx)
	if err != nil {
		t.Fatalf("counts with states: %v", err)
	}
	if counts.AccountStates[model.AccountStatusOnline] != 1 || counts.AccountStates[model.AccountStatusReconnecting] != 1 {
		t.Fatalf("account state counts = %+v, want ONLINE=1 RECONNECTING=1", counts.AccountStates)
	}

	if err := messages.MarkDeleted(ctx, channelID, 10); err != nil {
		t.Fatalf("mark deleted: %v", err)
	}
	results, err = messages.Search(ctx, SearchParams{Query: "庆余年", Limit: 10})
	if err != nil {
		t.Fatalf("search after delete: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("search after delete length = %d, want 0", len(results))
	}
}
