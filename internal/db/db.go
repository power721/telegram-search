package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

func Open(path string) (*sql.DB, error) {
	dsn := path
	if path != ":memory:" && !strings.HasPrefix(path, "file:") {
		dsn = fmt.Sprintf("file:%s?_foreign_keys=on&_busy_timeout=5000", path)
	}
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	conn.SetMaxOpenConns(1)
	if err := conn.Ping(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	if err := applyPragmas(conn); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

func WithTx(ctx context.Context, conn *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func applyPragmas(conn *sql.DB) error {
	pragmas := []string{
		`PRAGMA journal_mode=WAL`,
		`PRAGMA synchronous=NORMAL`,
		`PRAGMA temp_store=MEMORY`,
		`PRAGMA cache_size=-200000`,
		`PRAGMA foreign_keys=ON`,
	}
	for _, pragma := range pragmas {
		if _, err := conn.Exec(pragma); err != nil {
			return fmt.Errorf("apply %s: %w", pragma, err)
		}
	}
	return nil
}
