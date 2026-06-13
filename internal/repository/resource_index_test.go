package repository

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tg-search/internal/db"
	"tg-search/internal/model"
)

func TestResourceIndexRebuildDeduplicatesLinksAndExcludesImages(t *testing.T) {
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
	files := NewFileRepository(conn)
	index := NewResourceIndexRepository(conn)

	accountID, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 100, Title: "VIP", Username: "vip", Type: model.ChannelTypeChannel})
	oldDate := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	newDate := time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC)
	stored, err := messages.SaveBatch(ctx, []model.Message{
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1, Text: "old ubuntu", RawJSON: "{}", Date: oldDate},
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 2, Text: "new ubuntu", RawJSON: "{}", Date: newDate},
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 3, Text: "image only", RawJSON: "{}", Date: newDate.Add(time.Minute)},
	})
	if err != nil {
		t.Fatalf("save messages: %v", err)
	}
	for _, msg := range stored[:2] {
		if _, err := links.SaveBatch(ctx, msg.ID, []model.Link{{
			Type: "quark", Category: "cloud_drive", URL: "https://pan.quark.cn/s/same", Note: "Ubuntu 24.04", MediaTitle: "Ubuntu",
		}}); err != nil {
			t.Fatalf("save link: %v", err)
		}
	}
	if _, err := files.SaveBatch(ctx, stored[2].ID, []model.File{{TelegramFileID: 33, FileName: "poster.jpg", Extension: ".jpg", MimeType: "image/jpeg", Category: "image"}}); err != nil {
		t.Fatalf("save image: %v", err)
	}

	if err := index.Rebuild(ctx); err != nil {
		t.Fatalf("Rebuild returned error: %v", err)
	}
	result, err := index.List(ctx, model.ResourceIndexQuery{Limit: 10})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if result.Total != 1 || len(result.Items) != 1 {
		t.Fatalf("result total=%d items=%+v, want one deduped link", result.Total, result.Items)
	}
	item := result.Items[0]
	if item.URL != "https://pan.quark.cn/s/same" || item.SourceMessageID != stored[1].ID {
		t.Fatalf("indexed item = %+v, want newest source message", item)
	}
	if item.MessageCount != 2 || item.SourceChannelCount != 1 || item.ProviderCount != 1 {
		t.Fatalf("stats = channels:%d messages:%d providers:%d, want 1/2/1", item.SourceChannelCount, item.MessageCount, item.ProviderCount)
	}
}

func TestResourceIndexListSearchesFTSAndFiltersProvider(t *testing.T) {
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
	index := NewResourceIndexRepository(conn)
	accountID, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 100, Title: "VIP", Type: model.ChannelTypeChannel})
	stored, err := messages.SaveBatch(ctx, []model.Message{
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1, Text: "ubuntu", RawJSON: "{}", Date: time.Now().UTC()},
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 2, Text: "debian", RawJSON: "{}", Date: time.Now().UTC().Add(-time.Minute)},
	})
	if err != nil {
		t.Fatalf("save messages: %v", err)
	}
	if _, err := links.SaveBatch(ctx, stored[0].ID, []model.Link{{Type: "quark", Category: "cloud_drive", URL: "https://pan.quark.cn/s/ubuntu", Note: "Ubuntu 24.04"}}); err != nil {
		t.Fatalf("save ubuntu link: %v", err)
	}
	if _, err := links.SaveBatch(ctx, stored[1].ID, []model.Link{{Type: "baidu", Category: "cloud_drive", URL: "https://pan.baidu.com/s/debian", Note: "Debian"}}); err != nil {
		t.Fatalf("save debian link: %v", err)
	}
	if err := index.Rebuild(ctx); err != nil {
		t.Fatalf("Rebuild returned error: %v", err)
	}
	result, err := index.List(ctx, model.ResourceIndexQuery{Keyword: "ubuntu", Type: "quark", Limit: 10})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if result.Total != 1 || len(result.Items) != 1 || result.Items[0].Type != "quark" {
		t.Fatalf("result = %+v, want one quark ubuntu result", result)
	}
}

func TestResourceIndexListSupportsORResourceFilters(t *testing.T) {
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
	index := NewResourceIndexRepository(conn)
	accountID, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 100, Title: "VIP", Type: model.ChannelTypeChannel})
	now := time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC)
	stored, err := messages.SaveBatch(ctx, []model.Message{
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1, Text: "ubuntu quark", RawJSON: "{}", Date: now},
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 2, Text: "ubuntu magnet", RawJSON: "{}", Date: now.Add(-time.Minute)},
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 3, Text: "ubuntu baidu", RawJSON: "{}", Date: now.Add(-2 * time.Minute)},
	})
	if err != nil {
		t.Fatalf("save messages: %v", err)
	}
	if _, err := links.SaveBatch(ctx, stored[0].ID, []model.Link{{Type: "quark", Category: "cloud_drive", URL: "https://pan.quark.cn/s/ubuntu", Note: "Ubuntu Quark"}}); err != nil {
		t.Fatalf("save quark link: %v", err)
	}
	if _, err := links.SaveBatch(ctx, stored[1].ID, []model.Link{{Type: "magnet", Category: "magnet", URL: "magnet:?xt=urn:btih:ubuntu", Note: "Ubuntu Magnet"}}); err != nil {
		t.Fatalf("save magnet link: %v", err)
	}
	if _, err := links.SaveBatch(ctx, stored[2].ID, []model.Link{{Type: "baidu", Category: "cloud_drive", URL: "https://pan.baidu.com/s/ubuntu", Note: "Ubuntu Baidu"}}); err != nil {
		t.Fatalf("save baidu link: %v", err)
	}
	if err := index.Rebuild(ctx); err != nil {
		t.Fatalf("Rebuild returned error: %v", err)
	}

	result, err := index.List(ctx, model.ResourceIndexQuery{
		Keyword: "ubuntu",
		Filters: []model.ResourceIndexFilter{
			{Category: "cloud_drive", Type: "quark"},
			{Category: "magnet"},
		},
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if result.Total != 2 || len(result.Items) != 2 {
		t.Fatalf("result = %+v, want quark and magnet only", result)
	}
	got := map[string]bool{}
	for _, item := range result.Items {
		got[item.Type] = true
	}
	if !got["quark"] || !got["magnet"] || got["baidu"] {
		t.Fatalf("types = %+v, want quark and magnet without baidu", got)
	}
}

func TestResourceIndexStatsAfterRebuild(t *testing.T) {
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
	index := NewResourceIndexRepository(conn)
	accountID, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 100, Title: "VIP", Type: model.ChannelTypeChannel})
	stored, err := messages.SaveBatch(ctx, []model.Message{{
		AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1,
		Text: "ubuntu", RawJSON: "{}", Date: time.Now().UTC(),
	}})
	if err != nil {
		t.Fatalf("save message: %v", err)
	}
	if _, err := links.SaveBatch(ctx, stored[0].ID, []model.Link{{Type: "quark", Category: "cloud_drive", URL: "https://pan.quark.cn/s/ubuntu", Note: "Ubuntu"}}); err != nil {
		t.Fatalf("save link: %v", err)
	}
	if err := index.Rebuild(ctx); err != nil {
		t.Fatalf("Rebuild returned error: %v", err)
	}

	stats, err := index.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats returned error: %v", err)
	}
	if stats.IndexedRows != 1 || stats.UpdatedAt.IsZero() {
		t.Fatalf("stats = %+v, want one indexed row and updated_at", stats)
	}
}

func TestResourceIndexListQueryPlanAvoidsTelegramLinksWindowScan(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "telegram.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}

	rows, err := conn.QueryContext(ctx, `EXPLAIN QUERY PLAN
SELECT ri.id
FROM resource_index ri
WHERE ri.category = ?
ORDER BY ri.datetime DESC, ri.resource_id DESC
LIMIT 50`, "cloud_drive")
	if err != nil {
		t.Fatalf("explain query plan: %v", err)
	}
	defer rows.Close()
	plan := ""
	for rows.Next() {
		var id, parent, notUsed int
		var detail string
		if err := rows.Scan(&id, &parent, &notUsed, &detail); err != nil {
			t.Fatalf("scan plan: %v", err)
		}
		plan += detail + "\n"
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("query plan rows: %v", err)
	}
	if strings.Contains(plan, "telegram_links") || strings.Contains(plan, "USE TEMP B-TREE") {
		t.Fatalf("resource index plan = %s, want indexed resource_index plan without normalized link scan or temp sort", plan)
	}
}

func TestResourceIndexRefreshAfterSoftDeleteSelectsNextNewestLink(t *testing.T) {
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
	index := NewResourceIndexRepository(conn)
	accountID, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 100, Title: "VIP", Type: model.ChannelTypeChannel})
	oldDate := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	newDate := time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC)
	stored, err := messages.SaveBatch(ctx, []model.Message{
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1, Text: "same", RawJSON: "{}", Date: oldDate},
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 2, Text: "same", RawJSON: "{}", Date: newDate},
	})
	if err != nil {
		t.Fatalf("save messages: %v", err)
	}
	for _, msg := range stored {
		if _, err := links.SaveBatch(ctx, msg.ID, []model.Link{{Type: "quark", Category: "cloud_drive", URL: "https://pan.quark.cn/s/same", Note: "same resource"}}); err != nil {
			t.Fatalf("save link: %v", err)
		}
	}
	if err := index.Rebuild(ctx); err != nil {
		t.Fatalf("Rebuild returned error: %v", err)
	}
	if err := messages.MarkDeleted(ctx, channelID, 2); err != nil {
		t.Fatalf("MarkDeleted returned error: %v", err)
	}
	if err := index.RefreshMessage(ctx, stored[1].ID); err != nil {
		t.Fatalf("RefreshMessage returned error: %v", err)
	}
	result, err := index.List(ctx, model.ResourceIndexQuery{Keyword: "same", Limit: 10})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if result.Total != 1 || result.Items[0].SourceMessageID != stored[0].ID {
		t.Fatalf("result = %+v, want old source after new source deleted", result)
	}
}
