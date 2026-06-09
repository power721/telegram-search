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
  (name, key_hash, key_ciphertext, prefix, enabled, created_at, updated_at)
VALUES
  (?, ?, ?, ?, ?, ?, ?)`,
		key.Name, key.KeyHash, key.KeyCiphertext, key.Prefix, key.Enabled, now, now)
	if err != nil {
		return 0, fmt.Errorf("create api key: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("create api key id: %w", err)
	}
	return id, nil
}

func (r *APIKeyRepository) Active(ctx context.Context) (model.APIKey, error) {
	var key model.APIKey
	err := r.db.QueryRowContext(ctx, `
SELECT id, name, key_hash, key_ciphertext, prefix, enabled, usage_count, last_used_at, created_at, updated_at
FROM api_keys
WHERE enabled = 1
ORDER BY id DESC
LIMIT 1`).Scan(&key.ID, &key.Name, &key.KeyHash, &key.KeyCiphertext, &key.Prefix, &key.Enabled, &key.UsageCount, &key.LastUsedAt, &key.CreatedAt, &key.UpdatedAt)
	if err != nil {
		return model.APIKey{}, fmt.Errorf("active api key: %w", err)
	}
	return key, nil
}

func (r *APIKeyRepository) DisableEnabled(ctx context.Context) error {
	now := time.Now().UTC()
	if _, err := r.db.ExecContext(ctx, `UPDATE api_keys SET enabled = 0, updated_at = ? WHERE enabled = 1`, now); err != nil {
		return fmt.Errorf("disable enabled api keys: %w", err)
	}
	return nil
}

func (r *APIKeyRepository) EnabledByPrefix(ctx context.Context, prefix string) ([]model.APIKey, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, name, key_hash, key_ciphertext, prefix, enabled, usage_count, last_used_at, created_at, updated_at
FROM api_keys
WHERE enabled = 1 AND prefix = ?
ORDER BY id DESC`, prefix)
	if err != nil {
		return nil, fmt.Errorf("enabled api keys by prefix: %w", err)
	}
	defer rows.Close()
	var keys []model.APIKey
	for rows.Next() {
		var key model.APIKey
		if err := rows.Scan(&key.ID, &key.Name, &key.KeyHash, &key.KeyCiphertext, &key.Prefix, &key.Enabled, &key.UsageCount, &key.LastUsedAt, &key.CreatedAt, &key.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate api keys: %w", err)
	}
	return keys, nil
}

func (r *APIKeyRepository) RecordUsage(ctx context.Context, id int64, at time.Time) error {
	if _, err := r.db.ExecContext(ctx, `UPDATE api_keys SET usage_count = usage_count + 1, last_used_at = ?, updated_at = ? WHERE id = ?`, at, at, id); err != nil {
		return fmt.Errorf("record api key usage: %w", err)
	}
	return nil
}

func (r *APIKeyRepository) CountEnabled(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM api_keys WHERE enabled = 1`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count enabled api keys: %w", err)
	}
	return count, nil
}
