package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"tg-search/internal/db"
	"tg-search/internal/model"
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

func TestRepositoriesKeepAccountDataIsolated(t *testing.T) {
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

	account1, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", Username: "one", Status: model.AccountStatusOnline})
	account2, _ := accounts.Save(ctx, model.Account{Phone: "+10000000001", Username: "two", Status: model.AccountStatusOnline})
	channel1, _ := channels.Save(ctx, model.Channel{AccountID: account1, TelegramChannelID: 100, Title: "A1", Type: model.ChannelTypeChannel})
	channel2, _ := channels.Save(ctx, model.Channel{AccountID: account2, TelegramChannelID: 100, Title: "A2", Type: model.ChannelTypeChannel})
	now := time.Now().UTC()
	stored1, _ := messages.SaveBatch(ctx, []model.Message{{AccountID: account1, ChannelID: channel1, TelegramMessageID: 1, Text: "shared keyword account one", RawJSON: "{}", Date: now}})
	stored2, _ := messages.SaveBatch(ctx, []model.Message{{AccountID: account2, ChannelID: channel2, TelegramMessageID: 1, Text: "shared keyword account two", RawJSON: "{}", Date: now}})
	_, _ = links.SaveBatch(ctx, stored1[0].ID, []model.Link{{Type: "url", URL: "https://example.com/one"}})
	_, _ = links.SaveBatch(ctx, stored2[0].ID, []model.Link{{Type: "url", URL: "https://example.com/two"}})

	results, err := messages.Search(ctx, SearchParams{Query: "shared", AccountID: account1, Limit: 10})
	if err != nil {
		t.Fatalf("search account1: %v", err)
	}
	if len(results) != 1 || results[0].AccountID != account1 || results[0].AccountUsername != "one" {
		t.Fatalf("account1 search results = %+v", results)
	}
	latest, err := messages.Latest(ctx, LatestParams{AccountID: account2, Limit: 10})
	if err != nil {
		t.Fatalf("latest account2: %v", err)
	}
	if len(latest) != 1 || latest[0].AccountID != account2 {
		t.Fatalf("account2 latest = %+v", latest)
	}
	linkResults, err := links.Search(ctx, LinkSearchParams{AccountID: account1, Limit: 10})
	if err != nil {
		t.Fatalf("links account1: %v", err)
	}
	if len(linkResults) != 1 || linkResults[0].AccountID != account1 || linkResults[0].URL != "https://example.com/one" {
		t.Fatalf("account1 links = %+v", linkResults)
	}
}

func TestChannelRepositoryUpdatesWebAccess(t *testing.T) {
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

	accountID, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := channels.Save(ctx, model.Channel{
		AccountID:         accountID,
		TelegramChannelID: 100,
		Title:             "Public",
		Username:          "public_channel",
		Type:              model.ChannelTypeChannel,
	})

	unchecked, err := channels.FindByID(ctx, channelID)
	if err != nil {
		t.Fatalf("find unchecked channel: %v", err)
	}
	if unchecked.WebAccess != nil || unchecked.WebAccessCheckedAt != nil {
		t.Fatalf("unchecked web access = %v checked_at=%v, want nil values", unchecked.WebAccess, unchecked.WebAccessCheckedAt)
	}

	checkedAt := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	if err := channels.UpdateWebAccess(ctx, channelID, true, checkedAt); err != nil {
		t.Fatalf("UpdateWebAccess returned error: %v", err)
	}

	checked, err := channels.FindByID(ctx, channelID)
	if err != nil {
		t.Fatalf("find checked channel: %v", err)
	}
	if checked.WebAccess == nil || *checked.WebAccess != true {
		t.Fatalf("web access = %v, want true", checked.WebAccess)
	}
	if checked.WebAccessCheckedAt == nil || !checked.WebAccessCheckedAt.Equal(checkedAt) {
		t.Fatalf("checked_at = %v, want %v", checked.WebAccessCheckedAt, checkedAt)
	}

	_, err = channels.Save(ctx, model.Channel{
		AccountID:         accountID,
		TelegramChannelID: 100,
		Title:             "Public Renamed",
		Username:          "public_channel",
		Type:              model.ChannelTypeChannel,
	})
	if err != nil {
		t.Fatalf("save existing channel: %v", err)
	}
	afterUpsert, err := channels.FindByID(ctx, channelID)
	if err != nil {
		t.Fatalf("find upserted channel: %v", err)
	}
	if afterUpsert.WebAccess == nil || *afterUpsert.WebAccess != true || afterUpsert.WebAccessCheckedAt == nil || !afterUpsert.WebAccessCheckedAt.Equal(checkedAt) {
		t.Fatalf("upsert changed web access fields: %+v", afterUpsert)
	}
}

func TestMessageSearchAttachesLinksForMultipleResults(t *testing.T) {
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
	accountID, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", Username: "main", Status: model.AccountStatusOnline})
	channelID, _ := channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 1, Title: "VIP", Type: model.ChannelTypeChannel})
	now := time.Now().UTC()
	stored, _ := messages.SaveBatch(ctx, []model.Message{
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1, Text: "shared one", RawJSON: "{}", Date: now},
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 2, Text: "shared two", RawJSON: "{}", Date: now.Add(-time.Minute)},
	})
	_, _ = links.SaveBatch(ctx, stored[0].ID, []model.Link{{Type: "aliyun", URL: "https://www.alipan.com/s/one"}})
	_, _ = links.SaveBatch(ctx, stored[1].ID, []model.Link{{Type: "quark", URL: "https://pan.quark.cn/s/two"}})

	results, err := messages.Search(ctx, SearchParams{Query: "shared", Limit: 10})
	if err != nil {
		t.Fatalf("search messages: %v", err)
	}
	if len(results) != 2 || len(results[0].Links) != 1 || len(results[1].Links) != 1 {
		t.Fatalf("search results links = %+v", results)
	}
}

func TestLinkRepositoryPersistsAndLoadsNote(t *testing.T) {
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
	accountID, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", Username: "main", Status: model.AccountStatusOnline})
	channelID, _ := channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 1, Title: "VIP", Type: model.ChannelTypeChannel})
	stored, _ := messages.SaveBatch(ctx, []model.Message{{
		AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1,
		Text: "庆余年 S02 https://pan.quark.cn/s/abc123", RawJSON: "{}", Date: time.Now().UTC(),
	}})
	_, err = links.SaveBatch(ctx, stored[0].ID, []model.Link{{
		Type: "quark", URL: "https://pan.quark.cn/s/abc123", Note: "庆余年 S02",
	}})
	if err != nil {
		t.Fatalf("save links: %v", err)
	}

	searchResults, err := messages.Search(ctx, SearchParams{Query: "庆余年", Limit: 10})
	if err != nil {
		t.Fatalf("search messages: %v", err)
	}
	if len(searchResults) != 1 || len(searchResults[0].Links) != 1 {
		t.Fatalf("search results = %+v, want one result with one link", searchResults)
	}
	if searchResults[0].Links[0].Note != "庆余年 S02" {
		t.Fatalf("attached link note = %q, want persisted note", searchResults[0].Links[0].Note)
	}

	linkResults, err := links.Search(ctx, LinkSearchParams{Type: "quark", Limit: 10})
	if err != nil {
		t.Fatalf("search links: %v", err)
	}
	if len(linkResults) != 1 || linkResults[0].Note != "庆余年 S02" {
		t.Fatalf("link results = %+v, want persisted note", linkResults)
	}
}

func TestLinkRepositorySearchMergedGroupsAndDeduplicatesNewest(t *testing.T) {
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
	accountID, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", Username: "main", Status: model.AccountStatusOnline})
	channelID, _ := channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 1, Title: "VIP", Type: model.ChannelTypeChannel})
	oldDate := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	newDate := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	stored, _ := messages.SaveBatch(ctx, []model.Message{
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1, Text: "庆余年 old", RawJSON: "{}", Date: oldDate},
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 2, Text: "庆余年 new", RawJSON: "{}", Date: newDate},
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 3, Text: "庆余年 aliyun", RawJSON: "{}", Date: newDate.Add(-time.Minute)},
	})
	_, _ = links.SaveBatch(ctx, stored[0].ID, []model.Link{{Type: "quark", URL: "https://pan.quark.cn/s/same", Note: "庆余年 旧"}})
	_, _ = links.SaveBatch(ctx, stored[1].ID, []model.Link{{Type: "quark", URL: "https://pan.quark.cn/s/same", Note: "庆余年 最新合集"}})
	_, _ = links.SaveBatch(ctx, stored[2].ID, []model.Link{{Type: "aliyun", URL: "https://www.alipan.com/s/abc123", Note: "庆余年 S02"}})

	merged, err := links.SearchMerged(ctx, MergedLinkSearchParams{Keyword: "庆余年", Limit: 10})
	if err != nil {
		t.Fatalf("SearchMerged returned error: %v", err)
	}
	if merged.Total != 2 {
		t.Fatalf("total = %d, want 2: %+v", merged.Total, merged)
	}
	if len(merged.MergedByType["quark"]) != 1 {
		t.Fatalf("quark links = %+v, want one deduped link", merged.MergedByType["quark"])
	}
	quark := merged.MergedByType["quark"][0]
	if quark.Note != "庆余年 最新合集" || !quark.Datetime.Equal(newDate) || quark.Source != "tg:VIP" || quark.TelegramMessageID != 2 {
		t.Fatalf("quark merged link = %+v, want newest context", quark)
	}
	if len(merged.MergedByType["aliyun"]) != 1 || merged.MergedByType["aliyun"][0].Note != "庆余年 S02" {
		t.Fatalf("aliyun links = %+v, want one aliyun note", merged.MergedByType["aliyun"])
	}
}
