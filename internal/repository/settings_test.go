package repository

import (
	"context"
	"path/filepath"
	"testing"

	"tg-search/internal/db"
)

func TestSettingsRepositoryUpsertsValues(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "tg-search.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := NewSettingsRepository(conn)
	if err := repo.Set(ctx, "setup.complete", `true`); err != nil {
		t.Fatalf("set: %v", err)
	}
	value, ok, err := repo.Get(ctx, "setup.complete")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !ok || value != `true` {
		t.Fatalf("value=%q ok=%v, want true", value, ok)
	}
}
