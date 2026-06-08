package repository

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

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

func TestAPIKeyRepositoryActiveLifecycle(t *testing.T) {
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

	id, err := repo.Create(ctx, model.APIKey{
		Name: "default", KeyHash: "hash1", KeyCiphertext: "cipher1", Prefix: "abcd1234", Enabled: true,
	})
	if err != nil {
		t.Fatalf("create key: %v", err)
	}
	active, err := repo.Active(ctx)
	if err != nil {
		t.Fatalf("active key: %v", err)
	}
	if active.ID != id || active.KeyCiphertext != "cipher1" || !active.Enabled {
		t.Fatalf("active = %+v, want id %d with ciphertext", active, id)
	}

	if err := repo.DisableEnabled(ctx); err != nil {
		t.Fatalf("disable enabled: %v", err)
	}
	if _, err := repo.Active(ctx); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("active after disable err = %v, want sql.ErrNoRows", err)
	}
}

func TestAPIKeyRepositoryVerificationCandidatesAndLastUsed(t *testing.T) {
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
	now := time.Now().UTC()

	id, err := repo.Create(ctx, model.APIKey{
		Name: "default", KeyHash: "hash1", KeyCiphertext: "cipher1", Prefix: "abcd1234", Enabled: true,
	})
	if err != nil {
		t.Fatalf("create key: %v", err)
	}
	candidates, err := repo.EnabledByPrefix(ctx, "abcd1234")
	if err != nil {
		t.Fatalf("enabled by prefix: %v", err)
	}
	if len(candidates) != 1 || candidates[0].ID != id {
		t.Fatalf("candidates = %+v, want created key", candidates)
	}
	if err := repo.UpdateLastUsed(ctx, id, now); err != nil {
		t.Fatalf("update last used: %v", err)
	}
	active, err := repo.Active(ctx)
	if err != nil {
		t.Fatalf("active key: %v", err)
	}
	if active.LastUsedAt == nil || !active.LastUsedAt.Equal(now) {
		t.Fatalf("last used = %v, want %v", active.LastUsedAt, now)
	}
}
