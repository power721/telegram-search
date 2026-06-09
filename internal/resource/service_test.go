package resource

import (
	"context"
	"path/filepath"
	"strconv"
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

func TestResourceLibraryRanksQualityBeforeFreshness(t *testing.T) {
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

	accountID, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 1, Title: "VIP", Type: model.ChannelTypeChannel})
	oldDate := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	newDate := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	stored, err := messages.SaveBatch(ctx, []model.Message{
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1, Text: "ubuntu 24.04 完整合集", RawJSON: "{}", Date: oldDate},
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 2, Text: "ubuntu random mirror", RawJSON: "{}", Date: newDate},
	})
	if err != nil {
		t.Fatalf("save messages: %v", err)
	}
	if _, err := links.SaveBatch(ctx, stored[0].ID, []model.Link{{
		Type: "quark", Category: "cloud_drive", URL: "https://pan.quark.cn/s/high", Note: "Ubuntu 24.04 最新合集", MediaTitle: "Ubuntu 24.04",
		MediaYear: "2026", MediaQuality: "4K", MediaCategory: "software",
	}}); err != nil {
		t.Fatalf("save high quality link: %v", err)
	}
	if _, err := links.SaveBatch(ctx, stored[1].ID, []model.Link{{
		Type: "quark", Category: "cloud_drive", URL: "https://pan.quark.cn/s/weak", Note: "random mirror",
	}}); err != nil {
		t.Fatalf("save weak link: %v", err)
	}

	service := NewService(links, nil)
	result, err := service.List(ctx, Query{Keyword: "ubuntu", Limit: 10})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("items = %+v, want two ranked resources", result.Items)
	}
	if result.Items[0].URL != "https://pan.quark.cn/s/high" {
		t.Fatalf("first resource = %+v, want high quality exact match before newer weak match", result.Items[0])
	}
}

func TestResourceLibraryExcludesImageFilesByDefault(t *testing.T) {
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
	mirrorChannelID, _ := channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 2, Title: "Mirror", Type: model.ChannelTypeChannel})
	publishedAt := time.Date(2026, 6, 9, 16, 26, 0, 0, time.UTC)
	stored, err := messages.SaveBatch(ctx, []model.Message{
		{
			AccountID: accountID, ChannelID: channelID, TelegramMessageID: 227,
			MessageType: "photo", MediaSummary: "photo", Text: "吞噬星空 https://pan.quark.cn/s/abc", RawJSON: "{}", Date: publishedAt,
		},
		{
			AccountID: accountID, ChannelID: mirrorChannelID, TelegramMessageID: 228,
			MessageType: "photo", MediaSummary: "photo", Text: "telegram-photo-6143006241194709282.jpg", RawJSON: "{}", Date: publishedAt,
		},
	})
	if err != nil {
		t.Fatalf("save messages: %v", err)
	}
	if _, err := links.SaveBatch(ctx, stored[0].ID, []model.Link{{
		Type: "url", Category: "cloud_drive", URL: "https://pan.quark.cn/s/abc", MediaTitle: "吞噬星空",
	}}); err != nil {
		t.Fatalf("save link: %v", err)
	}
	if _, err := files.SaveBatch(ctx, stored[0].ID, []model.File{{
		FileName: "telegram-photo-6143006241194709282.jpg", Extension: ".jpg", MimeType: "image/jpeg", Category: "image",
	}}); err != nil {
		t.Fatalf("save image file: %v", err)
	}
	if _, err := files.SaveBatch(ctx, stored[1].ID, []model.File{{
		FileName: "telegram-photo-6143006241194709282.jpg", Extension: ".jpg", MimeType: "image/jpeg", Category: "image",
	}}); err != nil {
		t.Fatalf("save mirrored image file: %v", err)
	}

	service := NewService(links, files)
	result, err := service.List(ctx, Query{Limit: 10})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("total = %d, want only the link resource", result.Total)
	}
	if len(result.Items) != 1 || result.Items[0].Kind != "link" {
		t.Fatalf("items = %+v, want only the link resource", result.Items)
	}
	if result.Grouped["cloud_drive"] != 1 || result.Grouped["files"] != 0 {
		t.Fatalf("grouped = %+v, want cloud_drive=1 files=0", result.Grouped)
	}
}

func TestResourceLibraryCountsAndPagesBeyondInitialLinkBatch(t *testing.T) {
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

	accountID, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 1, Title: "VIP", Type: model.ChannelTypeChannel})
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)

	input := make([]model.Message, 0, 250)
	for i := 0; i < 250; i++ {
		input = append(input, model.Message{
			AccountID: accountID, ChannelID: channelID, TelegramMessageID: int64(i + 1),
			Text: "ubuntu resource", RawJSON: "{}", Date: now.Add(time.Duration(i) * time.Minute),
		})
	}
	stored, err := messages.SaveBatch(ctx, input)
	if err != nil {
		t.Fatalf("save messages: %v", err)
	}
	for i, msg := range stored {
		if _, err := links.SaveBatch(ctx, msg.ID, []model.Link{{
			Type: "url", Category: "http", URL: "https://example.com/" + strconv.Itoa(i),
		}}); err != nil {
			t.Fatalf("save link %d: %v", i, err)
		}
	}

	service := NewService(links, nil)
	result, err := service.List(ctx, Query{Keyword: "ubuntu", Limit: 50, Offset: 200})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if result.Total != 250 {
		t.Fatalf("total = %d, want all matching deduped resources", result.Total)
	}
	if len(result.Items) != 50 {
		t.Fatalf("items len = %d, want final page of 50 resources", len(result.Items))
	}
	if result.Grouped["http"] != 250 {
		t.Fatalf("grouped = %+v, want http=250", result.Grouped)
	}
}
