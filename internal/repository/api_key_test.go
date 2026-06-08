package repository

import (
	"context"
	"path/filepath"
	"testing"

	"tg-search/internal/db"
	"tg-search/internal/model"
)

func TestAPIKeyRepositoryCreatesAndCountsKeys(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "tg-search.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := NewAPIKeyRepository(conn)
	id, err := repo.Create(ctx, model.APIKey{Name: "cli", KeyHash: "hash", Prefix: "abcd1234", Enabled: true})
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}
	if id == 0 {
		t.Fatal("api key id = 0")
	}
	count, err := repo.CountEnabled(ctx)
	if err != nil {
		t.Fatalf("count enabled: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
}
