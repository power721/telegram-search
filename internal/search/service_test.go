package search

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"tg-provider/internal/db"
	"tg-provider/internal/model"
	"tg-provider/internal/repository"
)

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

func TestServiceRejectsEmptySearchQuery(t *testing.T) {
	service := NewService(nil, nil)

	_, err := service.Search(context.Background(), Params{})
	if err == nil {
		t.Fatal("Search returned nil error for empty query")
	}
}
