package apikey

import (
	"context"
	"database/sql"
	"net/http"
	"path/filepath"
	"strconv"
	"testing"
	"time"

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

func TestServiceVerifyRecordsUsageCount(t *testing.T) {
	ctx := context.Background()
	conn := testDB(t)
	service := NewService(repository.NewAPIKeyRepository(conn), repository.NewSettingsRepository(conn))
	key, err := service.EnsureActive(ctx)
	if err != nil {
		t.Fatalf("ensure active: %v", err)
	}
	for i := 0; i < 2; i++ {
		if _, ok, err := service.Verify(ctx, key.Key); err != nil || !ok {
			t.Fatalf("verify %d ok=%v err=%v, want valid", i+1, ok, err)
		}
	}
	current, err := service.Active(ctx)
	if err != nil {
		t.Fatalf("active: %v", err)
	}
	if current.UsageCount != 2 {
		t.Fatalf("usage count = %d, want 2", current.UsageCount)
	}
	if current.LastUsedAt == nil {
		t.Fatal("last used = nil, want usage timestamp")
	}
}

func TestServiceVerifyMediaSignature(t *testing.T) {
	ctx := context.Background()
	conn := testDB(t)
	service := NewService(repository.NewAPIKeyRepository(conn), repository.NewSettingsRepository(conn))
	key, err := service.EnsureActive(ctx)
	if err != nil {
		t.Fatalf("ensure active: %v", err)
	}
	now := time.Now().UTC()
	exp := now.Add(time.Hour).Unix()
	expRaw := strconv.FormatInt(exp, 10)
	sig, err := MediaSignature(key.Key, http.MethodGet, "/v/12345", expRaw)
	if err != nil {
		t.Fatalf("media signature: %v", err)
	}
	ok, err := service.VerifyMediaSignature(ctx, http.MethodGet, "/v/12345", expRaw, sig, now)
	if err != nil || !ok {
		t.Fatalf("verify ok=%v err=%v, want valid", ok, err)
	}
	ok, err = service.VerifyMediaSignature(ctx, http.MethodGet, "/v/54321", expRaw, sig, now)
	if err != nil || ok {
		t.Fatalf("verify changed path ok=%v err=%v, want invalid", ok, err)
	}
	ok, err = service.VerifyMediaSignature(ctx, http.MethodGet, "/v/12345", strconv.FormatInt(now.Add(-time.Minute).Unix(), 10), sig, now)
	if err != nil || ok {
		t.Fatalf("verify expired ok=%v err=%v, want invalid", ok, err)
	}
	if _, err := service.Regenerate(ctx); err != nil {
		t.Fatalf("regenerate: %v", err)
	}
	ok, err = service.VerifyMediaSignature(ctx, http.MethodGet, "/v/12345", expRaw, sig, now)
	if err != nil || ok {
		t.Fatalf("verify after regenerate ok=%v err=%v, want invalid", ok, err)
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
