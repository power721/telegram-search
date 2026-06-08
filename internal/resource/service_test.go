package resource

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"tg-search/internal/db"
	"tg-search/internal/model"
	"tg-search/internal/repository"
)

func TestResourceLibraryDeduplicatesLinks(t *testing.T) {
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
	files := repository.NewFileRepository(conn)

	accountID, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 1, Title: "VIP", Type: model.ChannelTypeChannel})
	oldDate := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	newDate := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	stored, err := messages.SaveBatch(ctx, []model.Message{
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1, Text: "ubuntu old", RawJSON: "{}", Date: oldDate},
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 2, Text: "ubuntu new", RawJSON: "{}", Date: newDate},
	})
	if err != nil {
		t.Fatalf("save messages: %v", err)
	}
	for i, msg := range stored {
		note := "ubuntu old"
		if i == 1 {
			note = "ubuntu latest"
		}
		if _, err := links.SaveBatch(ctx, msg.ID, []model.Link{{Type: "url", Category: "http", URL: "https://example.com/ubuntu", Note: note}}); err != nil {
			t.Fatalf("save link: %v", err)
		}
	}
	if _, err := files.SaveBatch(ctx, stored[1].ID, []model.File{{FileName: "ubuntu.iso", Extension: ".iso", SizeBytes: 5000, Category: "software"}}); err != nil {
		t.Fatalf("save file: %v", err)
	}

	service := NewService(links, files)
	result, err := service.List(ctx, Query{Keyword: "ubuntu", Limit: 10})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if result.Total != 2 {
		t.Fatalf("total = %d, want deduped link plus file", result.Total)
	}
	if len(result.Items) != 2 {
		t.Fatalf("items len = %d, want 2", len(result.Items))
	}
	if result.Items[0].Kind != "link" || result.Items[0].URL != "https://example.com/ubuntu" || result.Items[0].Note != "ubuntu latest" || !result.Items[0].Datetime.Equal(newDate) {
		t.Fatalf("first item = %+v, want newest deduped link", result.Items[0])
	}
	if result.Grouped["http"] != 1 || result.Grouped["files"] != 1 {
		t.Fatalf("grouped = %+v, want http=1 files=1", result.Grouped)
	}
}
