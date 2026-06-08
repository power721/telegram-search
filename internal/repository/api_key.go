package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"tg-search/internal/model"
)

type APIKeyRepository struct {
	db *sql.DB
}

func NewAPIKeyRepository(db *sql.DB) *APIKeyRepository {
	return &APIKeyRepository{db: db}
}

func (r *APIKeyRepository) Create(ctx context.Context, key model.APIKey) (int64, error) {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `
INSERT INTO api_keys
  (name, key_hash, prefix, enabled, created_at, updated_at)
VALUES
  (?, ?, ?, ?, ?, ?)`,
		key.Name, key.KeyHash, key.Prefix, key.Enabled, now, now)
	if err != nil {
		return 0, fmt.Errorf("create api key: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("create api key id: %w", err)
	}
	return id, nil
}

func (r *APIKeyRepository) CountEnabled(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM api_keys WHERE enabled = 1`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count enabled api keys: %w", err)
	}
	return count, nil
}
