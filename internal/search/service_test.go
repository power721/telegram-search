package search

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"tg-search/internal/db"
	"tg-search/internal/model"
	"tg-search/internal/repository"
)

func TestGlobalSearchGroupsResults(t *testing.T) {
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
	channelID, _ := channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 1, Title: "Ubuntu Channel", Username: "ubuntu_resources", Type: model.ChannelTypeChannel})
	now := time.Now().UTC()
	stored, err := messages.SaveBatch(ctx, []model.Message{{
		AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1,
		Text: "ubuntu release mirror", RawJSON: "{}", Date: now,
	}})
	if err != nil {
		t.Fatalf("save messages: %v", err)
	}
	if _, err := links.SaveBatch(ctx, stored[0].ID, []model.Link{{Type: "url", Category: "http", URL: "https://example.com/ubuntu", Note: "ubuntu download"}}); err != nil {
		t.Fatalf("save links: %v", err)
	}
	if _, err := files.SaveBatch(ctx, stored[0].ID, []model.File{{FileName: "ubuntu.iso", Extension: ".iso", MimeType: "application/x-iso9660-image", SizeBytes: 5000}}); err != nil {
		t.Fatalf("save files: %v", err)
	}

	service := NewService(messages, links, files, channels)
	result, err := service.Global(ctx, SearchQuery{Query: "ubuntu", Limit: 5})
	if err != nil {
		t.Fatalf("Global returned error: %v", err)
	}
	if result.Messages.Total != 1 || len(result.Messages.Items) != 1 {
		t.Fatalf("messages group = %+v, want one item", result.Messages)
	}
	if result.Links.Total != 1 || len(result.Links.Items) != 1 {
		t.Fatalf("links group = %+v, want one item", result.Links)
	}
	if result.Files.Total != 1 || len(result.Files.Items) != 1 {
		t.Fatalf("files group = %+v, want one item", result.Files)
	}
	if result.Channels.Total != 1 || len(result.Channels.Items) != 1 {
		t.Fatalf("channels group = %+v, want one item", result.Channels)
	}
}

func TestGlobalSearchTotalsUseFullFilteredCounts(t *testing.T) {
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
	channelID, _ := channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 1, Title: "Ubuntu Channel", Username: "ubuntu_resources", Type: model.ChannelTypeChannel})
	now := time.Now().UTC()
	stored, err := messages.SaveBatch(ctx, []model.Message{
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1, Text: "ubuntu release one", RawJSON: "{}", Date: now},
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 2, Text: "ubuntu release two", RawJSON: "{}", Date: now.Add(-time.Minute)},
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 3, Text: "ubuntu release three", RawJSON: "{}", Date: now.Add(-2 * time.Minute)},
	})
	if err != nil {
		t.Fatalf("save messages: %v", err)
	}
	suffixes := []string{"a", "b", "c"}
	for i, message := range stored {
		if _, err := links.SaveBatch(ctx, message.ID, []model.Link{{Type: "url", Category: "http", URL: "https://example.com/ubuntu-" + suffixes[i], Note: "ubuntu download"}}); err != nil {
			t.Fatalf("save link %d: %v", i, err)
		}
		if _, err := files.SaveBatch(ctx, message.ID, []model.File{{FileName: "ubuntu-" + suffixes[i] + ".iso", Extension: ".iso", MimeType: "application/x-iso9660-image", SizeBytes: 5000}}); err != nil {
			t.Fatalf("save file %d: %v", i, err)
		}
	}

	service := NewService(messages, links, files, channels)
	result, err := service.Global(ctx, SearchQuery{Query: "ubuntu", Limit: 1, Offset: 1})
	if err != nil {
		t.Fatalf("Global returned error: %v", err)
	}
	if result.Messages.Total != 3 || len(result.Messages.Items) != 1 {
		t.Fatalf("messages group = %+v, want total 3 with one paged item", result.Messages)
	}
	if result.Links.Total != 3 || len(result.Links.Items) != 1 {
		t.Fatalf("links group = %+v, want total 3 with one paged item", result.Links)
	}
	if result.Files.Total != 3 || len(result.Files.Items) != 1 {
		t.Fatalf("files group = %+v, want total 3 with one paged item", result.Files)
	}
	if result.Channels.Total != 1 || len(result.Channels.Items) != 0 {
		t.Fatalf("channels group = %+v, want total 1 with offset beyond page", result.Channels)
	}
}

func TestServiceSearchLatestAndLinks(t *testing.T) {
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
	now := time.Now().UTC()
	stored, err := messages.SaveBatch(ctx, []model.Message{
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1, Text: "庆余年 https://www.alipan.com/s/abc123", RawJSON: "{}", Date: now},
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 2, Text: "庆余年 https://pan.quark.cn/s/quark123", RawJSON: "{}", Date: now.Add(-time.Minute)},
	})
	if err != nil {
		t.Fatalf("save messages: %v", err)
	}
	if _, err := links.SaveBatch(ctx, stored[0].ID, []model.Link{{Type: "aliyun", URL: "https://www.alipan.com/s/abc123"}}); err != nil {
		t.Fatalf("save aliyun links: %v", err)
	}
	if _, err := links.SaveBatch(ctx, stored[1].ID, []model.Link{{Type: "quark", URL: "https://pan.quark.cn/s/quark123"}}); err != nil {
		t.Fatalf("save quark links: %v", err)
	}

	service := NewService(messages, links)
	results, err := service.Search(ctx, Params{Query: "庆余年", Limit: 10})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 2 || len(results[0].Links) != 1 || len(results[1].Links) != 1 {
		t.Fatalf("search results = %+v", results)
	}

	results, err = service.Search(ctx, Params{Query: "庆余年", LinkType: "aliyun", Limit: 10})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 1 || results[0].Links[0].Type != "aliyun" {
		t.Fatalf("filtered search results = %+v, want only aliyun", results)
	}

	latest, err := service.Latest(ctx, LatestParams{Limit: 10})
	if err != nil {
		t.Fatalf("Latest returned error: %v", err)
	}
	if len(latest) != 2 {
		t.Fatalf("latest len = %d, want 2", len(latest))
	}

	linkResults, err := service.Links(ctx, LinkParams{Type: "aliyun", Limit: 10})
	if err != nil {
		t.Fatalf("Links returned error: %v", err)
	}
	if len(linkResults) != 1 {
		t.Fatalf("links len = %d, want 1", len(linkResults))
	}
}

func TestServiceLinksFiltersByMessageDateRange(t *testing.T) {
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
	january := time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC)
	february := time.Date(2026, 2, 5, 12, 0, 0, 0, time.UTC)
	stored, err := messages.SaveBatch(ctx, []model.Message{
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1, Text: "jan aliyun", RawJSON: "{}", Date: january},
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 2, Text: "feb aliyun", RawJSON: "{}", Date: february},
	})
	if err != nil {
		t.Fatalf("save messages: %v", err)
	}
	if _, err := links.SaveBatch(ctx, stored[0].ID, []model.Link{{Type: "aliyun", URL: "https://www.alipan.com/s/jan"}}); err != nil {
		t.Fatalf("save jan link: %v", err)
	}
	if _, err := links.SaveBatch(ctx, stored[1].ID, []model.Link{{Type: "aliyun", URL: "https://www.alipan.com/s/feb"}}); err != nil {
		t.Fatalf("save feb link: %v", err)
	}

	service := NewService(messages, links)
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	linkResults, err := service.Links(ctx, LinkParams{Type: "aliyun", DateFrom: &from, DateTo: &to, Limit: 10})
	if err != nil {
		t.Fatalf("Links date range returned error: %v", err)
	}
	if len(linkResults) != 1 || linkResults[0].MessageDate.Month() != time.January {
		t.Fatalf("date filtered links = %+v, want only January aliyun link", linkResults)
	}
}

func TestServiceSearchFiltersByMessageDateRange(t *testing.T) {
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
	january := time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC)
	february := time.Date(2026, 2, 5, 12, 0, 0, 0, time.UTC)
	if _, err := messages.SaveBatch(ctx, []model.Message{
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1, Text: "shared keyword january", RawJSON: "{}", Date: january},
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 2, Text: "shared keyword february", RawJSON: "{}", Date: february},
	}); err != nil {
		t.Fatalf("save messages: %v", err)
	}

	service := NewService(messages, links)
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	results, err := service.Search(ctx, Params{Query: "shared", DateFrom: &from, DateTo: &to, Limit: 10})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 1 || results[0].Date.Month() != time.January {
		t.Fatalf("date filtered search results = %+v, want January only", results)
	}
}

func TestServiceSearchCursorReturnsOlderRows(t *testing.T) {
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
	newer := time.Date(2026, 2, 5, 12, 0, 0, 0, time.UTC)
	older := time.Date(2026, 2, 4, 12, 0, 0, 0, time.UTC)
	stored, err := messages.SaveBatch(ctx, []model.Message{
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1, Text: "shared newer", RawJSON: "{}", Date: newer},
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 2, Text: "shared older", RawJSON: "{}", Date: older},
	})
	if err != nil {
		t.Fatalf("save messages: %v", err)
	}

	service := NewService(messages, links)
	results, err := service.Search(ctx, Params{Query: "shared", BeforeDate: &newer, BeforeID: stored[0].ID, Limit: 10})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 1 || results[0].Text != "shared older" {
		t.Fatalf("cursor search results = %+v, want older row only", results)
	}
}

func TestServiceRejectsEmptySearchQuery(t *testing.T) {
	service := NewService(nil, nil)

	_, err := service.Search(context.Background(), Params{})
	if err == nil {
		t.Fatal("Search returned nil error for empty query")
	}
}
