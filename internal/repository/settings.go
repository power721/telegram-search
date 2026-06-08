package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type SettingsRepository struct {
	db *sql.DB
}

func NewSettingsRepository(db *sql.DB) *SettingsRepository {
	return &SettingsRepository{db: db}
}

func (r *SettingsRepository) Set(ctx context.Context, key string, valueJSON string) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
INSERT INTO settings (key, value_json, updated_at)
VALUES (?, ?, ?)
ON CONFLICT(key) DO UPDATE SET
  value_json = excluded.value_json,
  updated_at = excluded.updated_at`,
		key, valueJSON, now)
	if err != nil {
		return fmt.Errorf("set setting: %w", err)
	}
	return nil
}

func (r *SettingsRepository) Get(ctx context.Context, key string) (string, bool, error) {
	var value string
	err := r.db.QueryRowContext(ctx, `SELECT value_json FROM settings WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get setting: %w", err)
	}
	return value, true, nil
}
