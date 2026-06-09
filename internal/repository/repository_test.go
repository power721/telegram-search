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
	if err := channels.UpdateWebAccessResult(ctx, channelID, true, checkedAt, ""); err != nil {
		t.Fatalf("UpdateWebAccessResult returned error: %v", err)
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
	if checked.WebAccessError != "" {
		t.Fatalf("web_access_error = %q, want empty", checked.WebAccessError)
	}

	errorAt := checkedAt.Add(time.Minute)
	if err := channels.UpdateWebAccessResult(ctx, channelID, false, errorAt, "http 500"); err != nil {
		t.Fatalf("UpdateWebAccessResult error update returned error: %v", err)
	}
	errored, err := channels.FindByID(ctx, channelID)
	if err != nil {
		t.Fatalf("find errored channel: %v", err)
	}
	if errored.WebAccess == nil || *errored.WebAccess != false || errored.WebAccessError != "http 500" {
		t.Fatalf("errored web access fields = %+v", errored)
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
	if afterUpsert.WebAccess == nil || *afterUpsert.WebAccess != false ||
		afterUpsert.WebAccessCheckedAt == nil || !afterUpsert.WebAccessCheckedAt.Equal(errorAt) ||
		afterUpsert.WebAccessError != "http 500" {
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

func TestLinkRepositorySearchMergedRanksQualityBeforeFreshness(t *testing.T) {
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
	oldDate := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	newDate := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	stored, _ := messages.SaveBatch(ctx, []model.Message{
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1, Text: "庆余年 第二季 完整合集", RawJSON: "{}", Date: oldDate},
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 2, Text: "庆余年 网盘搬运", RawJSON: "{}", Date: newDate},
	})
	_, _ = links.SaveBatch(ctx, stored[0].ID, []model.Link{{
		Type: "quark", URL: "https://pan.quark.cn/s/high", Note: "庆余年 第二季 最新合集", MediaTitle: "庆余年 第二季",
		MediaYear: "2026", MediaQuality: "4K", MediaCategory: "series",
	}})
	_, _ = links.SaveBatch(ctx, stored[1].ID, []model.Link{{Type: "quark", URL: "https://pan.quark.cn/s/weak", Note: "网盘搬运"}})

	merged, err := links.SearchMerged(ctx, MergedLinkSearchParams{Keyword: "庆余年", Type: "quark", Limit: 10})
	if err != nil {
		t.Fatalf("SearchMerged returned error: %v", err)
	}
	items := merged.MergedByType["quark"]
	if len(items) != 2 {
		t.Fatalf("quark links = %+v, want two links", items)
	}
	if items[0].URL != "https://pan.quark.cn/s/high" {
		t.Fatalf("first quark link = %+v, want high quality exact match before newer weak match", items[0])
	}
}

func TestAccountRepositoryPersistsExpandedMetadata(t *testing.T) {
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

	now := time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC)
	accountID, err := accounts.Save(ctx, model.Account{
		Phone:        "+10000000000",
		Status:       model.AccountStatusOnline,
		SessionPath:  "/data/tg-search/sessions/1.session",
		LastOnlineAt: &now,
		LastError:    "",
	})
	if err != nil {
		t.Fatalf("save account: %v", err)
	}

	got, err := accounts.FindByID(ctx, accountID)
	if err != nil {
		t.Fatalf("find account: %v", err)
	}
	if got.SessionPath != "/data/tg-search/sessions/1.session" {
		t.Fatalf("session_path = %q", got.SessionPath)
	}
	if got.LastOnlineAt == nil || !got.LastOnlineAt.Equal(now) {
		t.Fatalf("last_online_at = %v, want %v", got.LastOnlineAt, now)
	}
	if got.LastError != "" {
		t.Fatalf("last_error = %q, want empty", got.LastError)
	}
}

func TestChannelRepositoryPersistsExpandedMetadata(t *testing.T) {
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

	accountID, err := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	if err != nil {
		t.Fatalf("save account: %v", err)
	}
	channelID, err := channels.Save(ctx, model.Channel{
		AccountID:         accountID,
		TelegramChannelID: 200,
		AccessHash:        300,
		Title:             "VIP",
		Username:          "vip",
		Type:              model.ChannelTypeChannel,
		MemberCount:       1234,
		Description:       "private resource channel",
		AvatarState:       "unknown",
		SyncState:         "metadata_only",
		ListenState:       "disabled",
	})
	if err != nil {
		t.Fatalf("save channel: %v", err)
	}

	got, err := channels.FindByID(ctx, channelID)
	if err != nil {
		t.Fatalf("find channel: %v", err)
	}
	if got.MemberCount != 1234 || got.Description != "private resource channel" ||
		got.AvatarState != "unknown" || got.SyncState != "metadata_only" || got.ListenState != "disabled" {
		t.Fatalf("channel metadata = %+v", got)
	}
}

func TestChannelControlFields(t *testing.T) {
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

	accountID, err := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	if err != nil {
		t.Fatalf("save account: %v", err)
	}
	channelID, err := channels.Save(ctx, model.Channel{
		AccountID:           accountID,
		TelegramChannelID:   201,
		Title:               "Control Channel",
		Type:                model.ChannelTypeChannel,
		HistorySyncEnabled:  true,
		SyncProfile:         "Deep",
		ListenEnabled:       false,
		RemoteSearchAllowed: true,
	})
	if err != nil {
		t.Fatalf("save channel: %v", err)
	}

	got, err := channels.FindByID(ctx, channelID)
	if err != nil {
		t.Fatalf("find channel: %v", err)
	}
	if !got.HistorySyncEnabled || got.SyncProfile != "Deep" || got.ListenEnabled || !got.RemoteSearchAllowed {
		t.Fatalf("saved control = %+v", got)
	}

	if err := channels.UpdateControl(ctx, channelID, model.ChannelControl{
		HistorySyncEnabled:  false,
		SyncProfile:         "Quick",
		ListenEnabled:       true,
		RemoteSearchAllowed: false,
	}); err != nil {
		t.Fatalf("UpdateControl returned error: %v", err)
	}
	got, err = channels.FindByID(ctx, channelID)
	if err != nil {
		t.Fatalf("find updated channel: %v", err)
	}
	if got.HistorySyncEnabled || got.SyncProfile != "Quick" || !got.ListenEnabled || got.RemoteSearchAllowed {
		t.Fatalf("updated control = %+v", got)
	}
	if got.SyncState != "metadata_only" || got.ListenState != "enabled" {
		t.Fatalf("updated states = sync:%q listen:%q, want metadata_only/enabled", got.SyncState, got.ListenState)
	}

	if err := channels.UpdateControl(ctx, channelID, model.ChannelControl{
		HistorySyncEnabled:  true,
		SyncProfile:         "Normal",
		ListenEnabled:       false,
		RemoteSearchAllowed: true,
	}); err != nil {
		t.Fatalf("UpdateControl enable history returned error: %v", err)
	}
	got, err = channels.FindByID(ctx, channelID)
	if err != nil {
		t.Fatalf("find history-enabled channel: %v", err)
	}
	if got.SyncState != "pending" || got.ListenState != "disabled" {
		t.Fatalf("history-enabled states = sync:%q listen:%q, want pending/disabled", got.SyncState, got.ListenState)
	}

	syncTime := time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC)
	if err := channels.UpdateCursor(ctx, channelID, 9001, syncTime); err != nil {
		t.Fatalf("UpdateCursor returned error: %v", err)
	}
	got, err = channels.FindByID(ctx, channelID)
	if err != nil {
		t.Fatalf("find synced channel: %v", err)
	}
	if got.SyncState != "synced" {
		t.Fatalf("synced channel state = %q, want synced", got.SyncState)
	}

	_, err = channels.Save(ctx, model.Channel{
		AccountID:         accountID,
		TelegramChannelID: 201,
		Title:             "Control Channel Renamed",
		Type:              model.ChannelTypeChannel,
		MemberCount:       50,
		Description:       "fresh metadata",
		SyncState:         "metadata_only",
		ListenState:       "disabled",
	})
	if err != nil {
		t.Fatalf("metadata upsert returned error: %v", err)
	}
	got, err = channels.FindByID(ctx, channelID)
	if err != nil {
		t.Fatalf("find metadata-upserted channel: %v", err)
	}
	if got.Title != "Control Channel Renamed" || got.MemberCount != 50 || got.Description != "fresh metadata" {
		t.Fatalf("metadata fields after upsert = %+v", got)
	}
	if got.SyncState != "synced" || got.ListenState != "disabled" ||
		!got.HistorySyncEnabled || got.SyncProfile != "Normal" || got.ListenEnabled || !got.RemoteSearchAllowed {
		t.Fatalf("control fields changed by metadata upsert: %+v", got)
	}
}

func TestChannelControlDisablesDuplicateChannelOwners(t *testing.T) {
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

	firstAccountID, err := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	if err != nil {
		t.Fatalf("save first account: %v", err)
	}
	secondAccountID, err := accounts.Save(ctx, model.Account{Phone: "+10000000001", Status: model.AccountStatusOnline})
	if err != nil {
		t.Fatalf("save second account: %v", err)
	}
	firstChannelID, err := channels.Save(ctx, model.Channel{
		AccountID:           firstAccountID,
		TelegramChannelID:   201,
		Title:               "Duplicate Channel",
		Type:                model.ChannelTypeChannel,
		HistorySyncEnabled:  true,
		SyncProfile:         "Deep",
		ListenEnabled:       true,
		RemoteSearchAllowed: true,
	})
	if err != nil {
		t.Fatalf("save first channel: %v", err)
	}
	secondChannelID, err := channels.Save(ctx, model.Channel{
		AccountID:           secondAccountID,
		TelegramChannelID:   201,
		Title:               "Duplicate Channel",
		Type:                model.ChannelTypeChannel,
		HistorySyncEnabled:  false,
		SyncProfile:         "Normal",
		ListenEnabled:       false,
		RemoteSearchAllowed: true,
	})
	if err != nil {
		t.Fatalf("save second channel: %v", err)
	}

	if err := channels.UpdateControl(ctx, secondChannelID, model.ChannelControl{
		HistorySyncEnabled:  true,
		SyncProfile:         "Normal",
		ListenEnabled:       true,
		RemoteSearchAllowed: true,
	}); err != nil {
		t.Fatalf("UpdateControl returned error: %v", err)
	}

	first, err := channels.FindByID(ctx, firstChannelID)
	if err != nil {
		t.Fatalf("find first channel: %v", err)
	}
	if first.HistorySyncEnabled || first.ListenEnabled {
		t.Fatalf("first duplicate control = history:%v listen:%v, want both disabled", first.HistorySyncEnabled, first.ListenEnabled)
	}
	if first.SyncState != "metadata_only" || first.ListenState != "disabled" {
		t.Fatalf("first duplicate states = sync:%q listen:%q, want metadata_only/disabled", first.SyncState, first.ListenState)
	}

	second, err := channels.FindByID(ctx, secondChannelID)
	if err != nil {
		t.Fatalf("find second channel: %v", err)
	}
	if !second.HistorySyncEnabled || !second.ListenEnabled {
		t.Fatalf("second duplicate control = history:%v listen:%v, want both enabled", second.HistorySyncEnabled, second.ListenEnabled)
	}
	if second.SyncState != "pending" || second.ListenState != "enabled" {
		t.Fatalf("second duplicate states = sync:%q listen:%q, want pending/enabled", second.SyncState, second.ListenState)
	}
}
