package repository

import (
	"context"
	"database/sql"
	"fmt"

	"tg-provider/internal/model"
)

type StatusRepository struct {
	db *sql.DB
}

func NewStatusRepository(db *sql.DB) *StatusRepository {
	return &StatusRepository{db: db}
}

func (r *StatusRepository) Counts(ctx context.Context) (model.StatusCounts, error) {
	var counts model.StatusCounts
	queries := []struct {
		sql  string
		dest *int64
	}{
		{`SELECT count(*) FROM telegram_accounts`, &counts.Accounts},
		{`SELECT count(*) FROM telegram_channels`, &counts.Channels},
		{`SELECT count(*) FROM telegram_messages`, &counts.Messages},
		{`SELECT count(*) FROM telegram_links`, &counts.Links},
	}
	for _, query := range queries {
		if err := r.db.QueryRowContext(ctx, query.sql).Scan(query.dest); err != nil {
			return model.StatusCounts{}, fmt.Errorf("read status count: %w", err)
		}
	}
	return counts, nil
}
