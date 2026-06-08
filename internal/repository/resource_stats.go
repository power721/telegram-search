package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type ResourceStatsRepository struct {
	db *sql.DB
}

func NewResourceStatsRepository(db *sql.DB) *ResourceStatsRepository {
	return &ResourceStatsRepository{db: db}
}

func (r *ResourceStatsRepository) GetGrouped(ctx context.Context) (map[string]int, bool, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT category, count
FROM resource_group_counts`)
	if err != nil {
		return nil, false, fmt.Errorf("get resource group counts: %w", err)
	}
	defer rows.Close()

	grouped := map[string]int{}
	for rows.Next() {
		var category string
		var count int
		if err := rows.Scan(&category, &count); err != nil {
			return nil, false, err
		}
		grouped[category] = count
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	return grouped, len(grouped) > 0, nil
}

func (r *ResourceStatsRepository) SaveGrouped(ctx context.Context, grouped map[string]int) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM resource_group_counts`); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("clear resource group counts: %w", err)
	}
	now := time.Now().UTC()
	for category, count := range grouped {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO resource_group_counts(category, count, updated_at)
VALUES (?, ?, ?)`, category, count, now); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("save resource group count %s: %w", category, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}
