package repository

import (
	"context"
	"testing"
	"time"

	"tg-search/internal/model"
)

func TestLinkRepositoryPersistsResourceFields(t *testing.T) {
	ctx := context.Background()
	conn := openRepositoryTestDB(t)
	accounts := NewAccountRepository(conn)
	channels := NewChannelRepository(conn)
	messages := NewMessageRepository(conn)
	links := NewLinkRepository(conn)

	accountID, err := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	if err != nil {
		t.Fatalf("save account: %v", err)
	}
	channelID, err := channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 1, Title: "VIP", Type: model.ChannelTypeChannel})
	if err != nil {
		t.Fatalf("save channel: %v", err)
	}
	stored, err := messages.SaveBatch(ctx, []model.Message{{
		AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1,
		Text: "Ubuntu ISO https://example.com/ubuntu.iso", RawJSON: "{}", Date: time.Now().UTC(),
	}})
	if err != nil {
		t.Fatalf("save message: %v", err)
	}

	_, err = links.SaveBatch(ctx, stored[0].ID, []model.Link{{
		Type:          "url",
		Category:      "http",
		URL:           "https://example.com/ubuntu.iso",
		SourceSnippet: "Ubuntu ISO https://example.com/ubuntu.iso",
		Note:          "Ubuntu ISO",
	}})
	if err != nil {
		t.Fatalf("save links: %v", err)
	}

	results, err := links.Search(ctx, LinkSearchParams{Keyword: "Ubuntu", Limit: 10})
	if err != nil {
		t.Fatalf("search links: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results len = %d, want 1", len(results))
	}
	if results[0].Category != "http" || results[0].SourceSnippet != "Ubuntu ISO https://example.com/ubuntu.iso" {
		t.Fatalf("resource fields = category %q snippet %q", results[0].Category, results[0].SourceSnippet)
	}
}
