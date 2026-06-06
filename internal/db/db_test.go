package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func TestMigrationsAreIdempotentAndCreateFTS(t *testing.T) {
	ctx := context.Background()
	conn, err := Open(filepath.Join(t.TempDir(), "telegram.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer conn.Close()

	if err := Migrate(ctx, conn); err != nil {
		t.Fatalf("first Migrate returned error: %v", err)
	}
	if err := Migrate(ctx, conn); err != nil {
		t.Fatalf("second Migrate returned error: %v", err)
	}

	assertTableExists(t, conn, "telegram_accounts")
	assertTableExists(t, conn, "telegram_channels")
	assertTableExists(t, conn, "telegram_messages")
	assertTableExists(t, conn, "telegram_links")
	assertTableExists(t, conn, "telegram_messages_fts")
}

func TestFTSTriggersIndexUpdateAndSoftDelete(t *testing.T) {
	ctx := context.Background()
	conn, err := Open(filepath.Join(t.TempDir(), "telegram.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer conn.Close()

	if err := Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}

	now := time.Now().UTC()
	if _, err := conn.ExecContext(ctx, `
INSERT INTO telegram_accounts (id, phone, status, created_at, updated_at)
VALUES (1, '+10000000000', 'ONLINE', ?, ?)`, now, now); err != nil {
		t.Fatalf("insert account: %v", err)
	}
	if _, err := conn.ExecContext(ctx, `
INSERT INTO telegram_channels
  (id, account_id, telegram_channel_id, title, type, created_at, updated_at)
VALUES
  (1, 1, 100, 'VIP', 'channel', ?, ?)`, now, now); err != nil {
		t.Fatalf("insert channel: %v", err)
	}
	res, err := conn.ExecContext(ctx, `
INSERT INTO telegram_messages
  (account_id, channel_id, telegram_message_id, sender_id, text, raw_json, date, deleted, created_at, updated_at)
VALUES
  (1, 1, 10, 20, '庆余年 阿里云盘 link', '{}', ?, 0, ?, ?)`, now, now, now)
	if err != nil {
		t.Fatalf("insert message: %v", err)
	}
	rowID, _ := res.LastInsertId()

	var count int
	if err := conn.QueryRowContext(ctx, `SELECT count(*) FROM telegram_messages_fts WHERE telegram_messages_fts MATCH '庆余年'`).Scan(&count); err != nil {
		t.Fatalf("query fts: %v", err)
	}
	if count != 1 {
		t.Fatalf("fts count after insert = %d, want 1", count)
	}

	if _, err := conn.ExecContext(ctx, `UPDATE telegram_messages SET deleted = 1, updated_at = ? WHERE id = ?`, now, rowID); err != nil {
		t.Fatalf("soft delete message: %v", err)
	}

	if err := conn.QueryRowContext(ctx, `SELECT count(*) FROM telegram_messages_fts WHERE telegram_messages_fts MATCH '庆余年'`).Scan(&count); err != nil {
		t.Fatalf("query fts after delete: %v", err)
	}
	if count != 0 {
		t.Fatalf("fts count after soft delete = %d, want 0", count)
	}
}

func assertTableExists(t *testing.T, conn *sql.DB, name string) {
	t.Helper()
	var count int
	err := conn.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type IN ('table', 'view') AND name = ?`, name).Scan(&count)
	if err != nil {
		t.Fatalf("check table %s: %v", name, err)
	}
	if count != 1 {
		t.Fatalf("table %s count = %d, want 1", name, count)
	}
}
