package repository

import (
	"context"
	"database/sql"
	"fmt"
)

type MaintenanceRepository struct {
	db *sql.DB
}

func NewMaintenanceRepository(db *sql.DB) *MaintenanceRepository {
	return &MaintenanceRepository{db: db}
}

func (r *MaintenanceRepository) OptimizeSQLite(ctx context.Context) ([]string, error) {
	ops := []string{}
	if _, err := r.db.ExecContext(ctx, `ANALYZE`); err != nil {
		return nil, fmt.Errorf("analyze sqlite: %w", err)
	}
	ops = append(ops, "ANALYZE")
	if _, err := r.db.ExecContext(ctx, `PRAGMA optimize`); err != nil {
		return nil, fmt.Errorf("pragma optimize: %w", err)
	}
	ops = append(ops, "PRAGMA optimize")
	if _, err := r.db.ExecContext(ctx, `INSERT INTO telegram_messages_fts(telegram_messages_fts) VALUES ('optimize')`); err != nil {
		return nil, fmt.Errorf("optimize fts: %w", err)
	}
	ops = append(ops, "telegram_messages_fts optimize")
	return ops, nil
}
