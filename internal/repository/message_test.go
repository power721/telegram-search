package repository

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"tg-search/internal/db"
	"tg-search/internal/model"
)

func TestMessageContentSplit(t *testing.T) {
	ctx := context.Background()
	conn := openRepositoryTestDB(t)

	accounts := NewAccountRepository(conn)
	channels := NewChannelRepository(conn)
	messages := NewMessageRepository(conn)

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

	now := time.Now().UTC().Truncate(time.Second)
	stored, err := messages.SaveBatch(ctx, []model.Message{{
		AccountID:         accountID,
		ChannelID:         channelID,
		TelegramMessageID: 10,
		SenderID:          7,
		Text:              "庆余年 阿里云盘",
		RawJSON:           `{"id":10}`,
		Date:              now,
		MessageType:       "text",
		MediaSummary:      "plain text",
	}})
	if err != nil {
		t.Fatalf("save message: %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("stored length = %d, want 1", len(stored))
	}

	assertColumnMissing(t, conn, "telegram_messages", "text")
	assertColumnMissing(t, conn, "telegram_messages", "raw_json")

	var metadataRows int
	if err := conn.QueryRowContext(ctx, `SELECT count(*) FROM telegram_messages`).Scan(&metadataRows); err != nil {
		t.Fatalf("count message metadata rows: %v", err)
	}
	if metadataRows != 1 {
		t.Fatalf("metadata rows = %d, want 1", metadataRows)
	}

	var contentRows int
	var text string
	var rawJSON string
	if err := conn.QueryRowContext(ctx, `
SELECT count(*), COALESCE(max(text), ''), COALESCE(max(raw_json), '')
FROM telegram_message_contents
WHERE message_id = ?`, stored[0].ID).Scan(&contentRows, &text, &rawJSON); err != nil {
		t.Fatalf("load message content: %v", err)
	}
	if contentRows != 1 || text != "庆余年 阿里云盘" || rawJSON != `{"id":10}` {
		t.Fatalf("content rows=%d text=%q raw_json=%q", contentRows, text, rawJSON)
	}

	results, err := messages.Search(ctx, SearchParams{Query: "庆余年", Limit: 10})
	if err != nil {
		t.Fatalf("search messages: %v", err)
	}
	if len(results) != 1 || results[0].Text != "庆余年 阿里云盘" || results[0].RawJSON != `{"id":10}` {
		t.Fatalf("search results = %+v", results)
	}

	latest, err := messages.Latest(ctx, LatestParams{Limit: 10})
	if err != nil {
		t.Fatalf("latest messages: %v", err)
	}
	if len(latest) != 1 || latest[0].Text != "庆余年 阿里云盘" || latest[0].RawJSON != `{"id":10}` {
		t.Fatalf("latest results = %+v", latest)
	}

	stored, err = messages.SaveBatch(ctx, []model.Message{{
		AccountID:         accountID,
		ChannelID:         channelID,
		TelegramMessageID: 10,
		SenderID:          7,
		Text:              "庆余年 edited",
		RawJSON:           `{"id":10,"edited":true}`,
		Date:              now,
		MessageType:       "text",
		MediaSummary:      "edited text",
	}})
	if err != nil {
		t.Fatalf("save duplicate message: %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("updated length = %d, want 1", len(stored))
	}

	if err := conn.QueryRowContext(ctx, `
SELECT count(*), COALESCE(max(text), ''), COALESCE(max(raw_json), '')
FROM telegram_message_contents
WHERE message_id = ?`, stored[0].ID).Scan(&contentRows, &text, &rawJSON); err != nil {
		t.Fatalf("load updated message content: %v", err)
	}
	if contentRows != 1 || text != "庆余年 edited" || rawJSON != `{"id":10,"edited":true}` {
		t.Fatalf("updated content rows=%d text=%q raw_json=%q", contentRows, text, rawJSON)
	}

	results, err = messages.Search(ctx, SearchParams{Query: "edited", Limit: 10})
	if err != nil {
		t.Fatalf("search updated messages: %v", err)
	}
	if len(results) != 1 || results[0].Text != "庆余年 edited" {
		t.Fatalf("updated search results = %+v", results)
	}

	results, err = messages.Search(ctx, SearchParams{Query: "阿里云盘", Limit: 10})
	if err != nil {
		t.Fatalf("search stale text: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("stale search results length = %d, want 0", len(results))
	}
}

func openRepositoryTestDB(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "telegram.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	})
	if err := db.Migrate(context.Background(), conn); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	return conn
}

func assertColumnMissing(t *testing.T, conn *sql.DB, table string, column string) {
	t.Helper()
	rows, err := conn.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		t.Fatalf("table_info %s: %v", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name string
		var typ string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			t.Fatalf("scan table_info %s: %v", table, err)
		}
		if name == column {
			t.Fatalf("column %s.%s exists, want split content table", table, column)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate table_info %s: %v", table, err)
	}
}
