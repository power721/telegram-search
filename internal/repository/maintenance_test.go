package repository

import (
	"context"
	"path/filepath"
	"testing"

	"tg-provider/internal/db"
)

func TestMaintenanceRepositoryOptimizeSQLite(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "telegram.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	ops, err := NewMaintenanceRepository(conn).OptimizeSQLite(ctx)
	if err != nil {
		t.Fatalf("OptimizeSQLite returned error: %v", err)
	}
	want := []string{"ANALYZE", "PRAGMA optimize", "telegram_messages_fts optimize"}
	if len(ops) != len(want) {
		t.Fatalf("ops = %+v, want %+v", ops, want)
	}
	for i := range want {
		if ops[i] != want[i] {
			t.Fatalf("ops = %+v, want %+v", ops, want)
		}
	}
}
