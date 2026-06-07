package db

import (
	"context"
	"database/sql"
	"fmt"
)

type migration struct {
	version int
	name    string
	sql     string
}

var migrations = []migration{
	{
		version: 1,
		name:    "core_schema",
		sql: `
CREATE TABLE IF NOT EXISTS telegram_accounts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  phone TEXT UNIQUE,
  telegram_user_id INTEGER,
  first_name TEXT,
  last_name TEXT,
  username TEXT,
  status TEXT NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS telegram_channels (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  account_id INTEGER NOT NULL,
  telegram_channel_id INTEGER NOT NULL,
  access_hash INTEGER,
  title TEXT,
  username TEXT,
  type TEXT NOT NULL,
  last_message_id INTEGER NOT NULL DEFAULT 0,
  last_sync_time DATETIME,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  UNIQUE(account_id, telegram_channel_id, type),
  FOREIGN KEY(account_id) REFERENCES telegram_accounts(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS telegram_messages (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  account_id INTEGER NOT NULL,
  channel_id INTEGER NOT NULL,
  telegram_message_id INTEGER NOT NULL,
  sender_id INTEGER,
  text TEXT,
  raw_json TEXT,
  date DATETIME,
  edit_date DATETIME,
  deleted INTEGER NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  UNIQUE(channel_id, telegram_message_id),
  FOREIGN KEY(account_id) REFERENCES telegram_accounts(id) ON DELETE CASCADE,
  FOREIGN KEY(channel_id) REFERENCES telegram_channels(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS telegram_links (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  message_id INTEGER NOT NULL,
  type TEXT NOT NULL,
  url TEXT NOT NULL,
  password TEXT,
  created_at DATETIME NOT NULL,
  UNIQUE(message_id, type, url),
  FOREIGN KEY(message_id) REFERENCES telegram_messages(id) ON DELETE CASCADE
);
`,
	},
	{
		version: 2,
		name:    "indexes",
		sql: `
CREATE INDEX IF NOT EXISTS idx_telegram_messages_channel_date ON telegram_messages(channel_id, date);
CREATE INDEX IF NOT EXISTS idx_telegram_messages_telegram_message_id ON telegram_messages(telegram_message_id);
CREATE INDEX IF NOT EXISTS idx_telegram_messages_account_id ON telegram_messages(account_id);
CREATE INDEX IF NOT EXISTS idx_telegram_links_type ON telegram_links(type);
CREATE INDEX IF NOT EXISTS idx_telegram_links_message_id ON telegram_links(message_id);
CREATE INDEX IF NOT EXISTS idx_telegram_channels_account_id ON telegram_channels(account_id);
`,
	},
	{
		version: 3,
		name:    "fts5",
		sql: `
CREATE VIRTUAL TABLE IF NOT EXISTS telegram_messages_fts
USING fts5(text, content='telegram_messages', content_rowid='id');

CREATE TRIGGER IF NOT EXISTS telegram_messages_ai AFTER INSERT ON telegram_messages
WHEN new.deleted = 0
BEGIN
  INSERT INTO telegram_messages_fts(rowid, text) VALUES (new.id, new.text);
END;

CREATE TRIGGER IF NOT EXISTS telegram_messages_ad AFTER DELETE ON telegram_messages
BEGIN
  INSERT INTO telegram_messages_fts(telegram_messages_fts, rowid, text)
  VALUES ('delete', old.id, old.text);
END;

CREATE TRIGGER IF NOT EXISTS telegram_messages_au AFTER UPDATE ON telegram_messages
BEGIN
  INSERT INTO telegram_messages_fts(telegram_messages_fts, rowid, text)
  VALUES ('delete', old.id, old.text);
  INSERT INTO telegram_messages_fts(rowid, text)
  SELECT new.id, new.text WHERE new.deleted = 0;
END;
`,
	},
	{
		version: 4,
		name:    "performance_indexes",
		sql: `
CREATE INDEX IF NOT EXISTS idx_telegram_messages_account_date_id ON telegram_messages(account_id, date DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_telegram_messages_channel_date_id ON telegram_messages(channel_id, date DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_telegram_links_type_message_id ON telegram_links(type, message_id);
`,
	},
}

func Migrate(ctx context.Context, conn *sql.DB) error {
	if _, err := conn.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	for _, m := range migrations {
		applied, err := migrationApplied(ctx, conn, m.version)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		if err := WithTx(ctx, conn, func(tx *sql.Tx) error {
			if _, err := tx.ExecContext(ctx, m.sql); err != nil {
				return fmt.Errorf("apply migration %03d %s: %w", m.version, m.name, err)
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version, name) VALUES (?, ?)`, m.version, m.name); err != nil {
				return fmt.Errorf("record migration %03d: %w", m.version, err)
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func migrationApplied(ctx context.Context, conn *sql.DB, version int) (bool, error) {
	var count int
	if err := conn.QueryRowContext(ctx, `SELECT count(*) FROM schema_migrations WHERE version = ?`, version).Scan(&count); err != nil {
		return false, fmt.Errorf("check migration %03d: %w", version, err)
	}
	return count > 0, nil
}
