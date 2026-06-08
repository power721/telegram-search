package repository

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"tg-search/internal/db"
	"tg-search/internal/model"
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

func TestSearchBenchmarkSeedCanGenerateLinksAndChannels(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "telegram.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}

	messages := seedSearchBenchmarkDataWithOptions(t, ctx, conn, searchBenchmarkSeedOptions{
		Messages:  600,
		Channels:  3,
		WithLinks: true,
	})

	results, err := messages.Search(ctx, SearchParams{Query: "target", Limit: 25})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) == 0 || len(results) > 25 {
		t.Fatalf("results len = %d, want 1..25", len(results))
	}
	foundLink := false
	for _, result := range results {
		if len(result.Links) > 0 {
			foundLink = true
			break
		}
	}
	if !foundLink {
		t.Fatalf("results = %+v, want at least one linked result", results)
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

func BenchmarkMessageRepositorySearchMillion(b *testing.B) {
	if os.Getenv("TG_PROVIDER_MILLION_BENCH") != "1" {
		b.Skip("set TG_PROVIDER_MILLION_BENCH=1 to seed 1,000,000 messages")
	}
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(b.TempDir(), "telegram.db"))
	if err != nil {
		b.Fatalf("Open returned error: %v", err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		b.Fatalf("Migrate returned error: %v", err)
	}
	messages := seedSearchBenchmarkDataWithOptions(b, ctx, conn, searchBenchmarkSeedOptions{
		Messages:  1_000_000,
		Channels:  50,
		WithLinks: true,
	})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := messages.Search(ctx, SearchParams{Query: "target", Limit: 20})
		if err != nil {
			b.Fatalf("Search returned error: %v", err)
		}
		if len(results) == 0 || len(results) > 20 {
			b.Fatalf("results len = %d, want 1..20", len(results))
		}
	}
}

type searchBenchmarkSeedOptions struct {
	Messages  int
	Channels  int
	WithLinks bool
}

func seedSearchBenchmarkData(tb testing.TB, ctx context.Context, conn *sql.DB, count int) *MessageRepository {
	return seedSearchBenchmarkDataWithOptions(tb, ctx, conn, searchBenchmarkSeedOptions{Messages: count, Channels: 1})
}

func seedSearchBenchmarkDataWithOptions(tb testing.TB, ctx context.Context, conn *sql.DB, opts searchBenchmarkSeedOptions) *MessageRepository {
	tb.Helper()
	if opts.Messages <= 0 {
		opts.Messages = 1
	}
	if opts.Channels <= 0 {
		opts.Channels = 1
	}
	accounts := NewAccountRepository(conn)
	channels := NewChannelRepository(conn)
	messages := NewMessageRepository(conn)
	links := NewLinkRepository(conn)
	accountID, err := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	if err != nil {
		tb.Fatalf("save account: %v", err)
	}
	channelIDs := make([]int64, 0, opts.Channels)
	for i := 0; i < opts.Channels; i++ {
		channelID, err := channels.Save(ctx, model.Channel{
			AccountID: accountID, TelegramChannelID: int64(i + 1), Title: "Bench " + strconv.Itoa(i+1), Type: model.ChannelTypeChannel,
		})
		if err != nil {
			tb.Fatalf("save channel: %v", err)
		}
		channelIDs = append(channelIDs, channelID)
	}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	batch := make([]model.Message, 0, 200)
	for i := 0; i < opts.Messages; i++ {
		text := "ordinary message " + strconv.Itoa(i)
		if i%5 == 0 {
			text = "target resource message " + strconv.Itoa(i)
		}
		batch = append(batch, model.Message{
			AccountID: accountID, ChannelID: channelIDs[i%len(channelIDs)], TelegramMessageID: int64(i/len(channelIDs) + 1),
			Text: text, RawJSON: "{}", Date: base.Add(time.Duration(i) * time.Second),
		})
		if len(batch) == cap(batch) {
			saveBenchmarkBatch(tb, ctx, messages, links, batch, opts.WithLinks)
			batch = batch[:0]
		}
	}
	if len(batch) > 0 {
		saveBenchmarkBatch(tb, ctx, messages, links, batch, opts.WithLinks)
	}
	return messages
}

func saveBenchmarkBatch(tb testing.TB, ctx context.Context, messages *MessageRepository, links *LinkRepository, batch []model.Message, withLinks bool) {
	tb.Helper()
	stored, err := messages.SaveBatch(ctx, batch)
	if err != nil {
		tb.Fatalf("save batch: %v", err)
	}
	if !withLinks {
		return
	}
	for i, msg := range stored {
		if i%5 != 0 {
			continue
		}
		if _, err := links.SaveBatch(ctx, msg.ID, []model.Link{{Type: "quark", URL: "https://pan.quark.cn/s/" + strconv.FormatInt(msg.ID, 10)}}); err != nil {
			tb.Fatalf("save benchmark link: %v", err)
		}
	}
}
