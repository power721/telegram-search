package apikey

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"tg-search/internal/db"
	"tg-search/internal/repository"
)

func TestServiceEnsureActiveCreatesViewableKey(t *testing.T) {
	ctx := context.Background()
	conn := testDB(t)
	service := NewService(repository.NewAPIKeyRepository(conn), repository.NewSettingsRepository(conn))

	resp, err := service.EnsureActive(ctx)
	if err != nil {
		t.Fatalf("ensure active: %v", err)
	}
	if len(resp.Key) != 32 || len(resp.Prefix) != 8 || resp.Prefix != resp.Key[:8] {
		t.Fatalf("response = %+v, want 32-character viewable key with prefix", resp)
	}

	again, err := service.EnsureActive(ctx)
	if err != nil {
		t.Fatalf("ensure active again: %v", err)
	}
	if again.Key != resp.Key || again.ID != resp.ID {
		t.Fatalf("ensure active again = %+v, want same key id %d", again, resp.ID)
	}
}

func TestServiceRegenerateInvalidatesOldKey(t *testing.T) {
	ctx := context.Background()
	conn := testDB(t)
	service := NewService(repository.NewAPIKeyRepository(conn), repository.NewSettingsRepository(conn))
	first, err := service.EnsureActive(ctx)
	if err != nil {
		t.Fatalf("ensure active: %v", err)
	}
	second, err := service.Regenerate(ctx)
	if err != nil {
		t.Fatalf("regenerate: %v", err)
	}
	if second.Key == first.Key || second.ID == first.ID {
		t.Fatalf("regenerated key = %+v, first = %+v", second, first)
	}
	if _, ok, err := service.Verify(ctx, first.Key); err != nil || ok {
		t.Fatalf("old verify ok=%v err=%v, want invalid", ok, err)
	}
	if id, ok, err := service.Verify(ctx, second.Key); err != nil || !ok || id != second.ID {
		t.Fatalf("new verify id=%d ok=%v err=%v, want id %d", id, ok, err, second.ID)
	}
}

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "tg-search.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(context.Background(), conn); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	return conn
}
