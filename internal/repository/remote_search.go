package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"tg-search/internal/model"
)

type RemoteSearchTaskRepository struct {
	db *sql.DB
}

func NewRemoteSearchTaskRepository(db *sql.DB) *RemoteSearchTaskRepository {
	return &RemoteSearchTaskRepository{db: db}
}

func (r *RemoteSearchTaskRepository) Create(ctx context.Context, task model.RemoteSearchTask) (int64, error) {
	now := time.Now().UTC()
	if task.Status == "" {
		task.Status = model.RemoteSearchStatusQueued
	}
	if task.Source == "" {
		task.Source = "remote"
	}
	var id int64
	err := r.db.QueryRowContext(ctx, `
INSERT INTO remote_search_tasks (account_id, channel_id, query, status, source, expires_at, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id`,
		task.AccountID, task.ChannelID, task.Query, task.Status, task.Source, task.ExpiresAt, now, now,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create remote search task: %w", err)
	}
	return id, nil
}

func (r *RemoteSearchTaskRepository) FindByID(ctx context.Context, id int64) (model.RemoteSearchTask, error) {
	return scanRemoteSearchTask(r.db.QueryRowContext(ctx, `
SELECT id, account_id, channel_id, query, status, source, expires_at, created_at, updated_at
FROM remote_search_tasks WHERE id = ?`, id))
}

func scanRemoteSearchTask(row interface{ Scan(...any) error }) (model.RemoteSearchTask, error) {
	var task model.RemoteSearchTask
	if err := row.Scan(
		&task.ID,
		&task.AccountID,
		&task.ChannelID,
		&task.Query,
		&task.Status,
		&task.Source,
		&task.ExpiresAt,
		&task.CreatedAt,
		&task.UpdatedAt,
	); err != nil {
		return model.RemoteSearchTask{}, err
	}
	return task, nil
}
