package repository

import (
	"context"
	"database/sql"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"tg-provider/internal/db"
	"tg-provider/internal/model"
)

func TestSearchBenchmarkSeedReturnsBoundedResults(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "telegram.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	messages := seedSearchBenchmarkData(t, ctx, conn, 250)
	results, err := messages.Search(ctx, SearchParams{Query: "target", Limit: 5})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 5 {
		t.Fatalf("results len = %d, want bounded limit 5", len(results))
	}
}

func BenchmarkMessageRepositorySearch(b *testing.B) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(b.TempDir(), "telegram.db"))
	if err != nil {
		b.Fatalf("Open returned error: %v", err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		b.Fatalf("Migrate returned error: %v", err)
	}
	messages := seedSearchBenchmarkData(b, ctx, conn, 5000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := messages.Search(ctx, SearchParams{Query: "target", Limit: 20})
		if err != nil {
			b.Fatalf("Search returned error: %v", err)
		}
		if len(results) == 0 {
			b.Fatal("Search returned no results")
		}
	}
}

func seedSearchBenchmarkData(tb testing.TB, ctx context.Context, conn *sql.DB, count int) *MessageRepository {
	tb.Helper()
	accounts := NewAccountRepository(conn)
	channels := NewChannelRepository(conn)
	messages := NewMessageRepository(conn)
	accountID, err := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	if err != nil {
		tb.Fatalf("save account: %v", err)
	}
	channelID, err := channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 1, Title: "Bench", Type: model.ChannelTypeChannel})
	if err != nil {
		tb.Fatalf("save channel: %v", err)
	}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	batch := make([]model.Message, 0, 200)
	for i := 0; i < count; i++ {
		text := "ordinary message " + strconv.Itoa(i)
		if i%5 == 0 {
			text = "target resource message " + strconv.Itoa(i)
		}
		batch = append(batch, model.Message{
			AccountID: accountID, ChannelID: channelID, TelegramMessageID: int64(i + 1),
			Text: text, RawJSON: "{}", Date: base.Add(time.Duration(i) * time.Second),
		})
		if len(batch) == cap(batch) {
			if _, err := messages.SaveBatch(ctx, batch); err != nil {
				tb.Fatalf("save batch: %v", err)
			}
			batch = batch[:0]
		}
	}
	if len(batch) > 0 {
		if _, err := messages.SaveBatch(ctx, batch); err != nil {
			tb.Fatalf("save final batch: %v", err)
		}
	}
	return messages
}
