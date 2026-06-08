package task

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"tg-search/internal/model"
)

type Repository struct {
	db *sql.DB
}

type StatusUpdate struct {
	Message      string
	ErrorCode    string
	ErrorMessage string
	RetryCount   *int64
	NextRunAt    *time.Time
	StartedAt    *time.Time
	FinishedAt   *time.Time
}

type ListFilter struct {
	Status string
	Type   string
	Limit  int
	Offset int
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, item model.Task) (model.Task, error) {
	if item.Status == "" {
		item.Status = model.TaskStatusQueued
	}
	if item.PayloadJSON == "" {
		item.PayloadJSON = "{}"
	}
	now := time.Now().UTC()
	row := r.db.QueryRowContext(ctx, `
INSERT INTO sync_tasks
  (type, status, progress, total, message, error_code, error_message, retry_count, next_run_at, payload_json, started_at, finished_at, created_at, updated_at)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id, type, status, progress, total, message, error_code, error_message, retry_count, next_run_at, payload_json, started_at, finished_at, created_at, updated_at`,
		item.Type, item.Status, item.Progress, item.Total, item.Message, item.ErrorCode, item.ErrorMessage, item.RetryCount,
		item.NextRunAt, item.PayloadJSON, item.StartedAt, item.FinishedAt, now, now,
	)
	item, err := scanTask(row)
	if err != nil {
		return model.Task{}, fmt.Errorf("create task: %w", err)
	}
	return item, nil
}

func (r *Repository) FindByID(ctx context.Context, id int64) (model.Task, error) {
	return scanTask(r.db.QueryRowContext(ctx, `
SELECT id, type, status, progress, total, message, error_code, error_message, retry_count, next_run_at, payload_json, started_at, finished_at, created_at, updated_at
FROM sync_tasks
WHERE id = ?`, id))
}

func (r *Repository) UpdateStatus(ctx context.Context, id int64, status string, update StatusUpdate) error {
	now := time.Now().UTC()
	retryCount := any(nil)
	if update.RetryCount != nil {
		retryCount = *update.RetryCount
	}
	res, err := r.db.ExecContext(ctx, `
UPDATE sync_tasks
SET status = ?,
    message = CASE WHEN ? <> '' THEN ? ELSE message END,
    error_code = CASE WHEN ? <> '' THEN ? ELSE error_code END,
    error_message = CASE WHEN ? <> '' THEN ? ELSE error_message END,
    retry_count = COALESCE(?, retry_count),
    next_run_at = ?,
    started_at = COALESCE(?, started_at),
    finished_at = COALESCE(?, finished_at),
    updated_at = ?
WHERE id = ?`,
		status, update.Message, update.Message, update.ErrorCode, update.ErrorCode, update.ErrorMessage, update.ErrorMessage,
		retryCount, update.NextRunAt, update.StartedAt, update.FinishedAt, now, id,
	)
	if err != nil {
		return fmt.Errorf("update task status: %w", err)
	}
	if rows, err := res.RowsAffected(); err != nil || rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *Repository) AppendProgress(ctx context.Context, id int64, progress int64, total int64, message string) error {
	res, err := r.db.ExecContext(ctx, `
UPDATE sync_tasks
SET progress = ?, total = ?, message = ?, updated_at = ?
WHERE id = ?`, progress, total, message, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("append task progress: %w", err)
	}
	if rows, err := res.RowsAffected(); err != nil || rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *Repository) List(ctx context.Context, filter ListFilter) ([]model.Task, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	where := []string{"1=1"}
	args := []any{}
	if filter.Status != "" {
		where = append(where, "status = ?")
		args = append(args, filter.Status)
	}
	if filter.Type != "" {
		where = append(where, "type = ?")
		args = append(args, filter.Type)
	}
	args = append(args, limit, filter.Offset)
	rows, err := r.db.QueryContext(ctx, `
SELECT id, type, status, progress, total, message, error_code, error_message, retry_count, next_run_at, payload_json, started_at, finished_at, created_at, updated_at
FROM sync_tasks
WHERE `+strings.Join(where, " AND ")+`
ORDER BY updated_at DESC, id DESC
LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()
	return scanTasks(rows)
}

func (r *Repository) ListRestartable(ctx context.Context, now time.Time) ([]model.Task, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, type, status, progress, total, message, error_code, error_message, retry_count, next_run_at, payload_json, started_at, finished_at, created_at, updated_at
FROM sync_tasks
WHERE status IN (?, ?, ?, ?, ?)
  AND (next_run_at IS NULL OR next_run_at <= ?)
ORDER BY updated_at ASC, id ASC`,
		model.TaskStatusRunning,
		model.TaskStatusCanceling,
		model.TaskStatusPaused,
		model.TaskStatusFloodWait,
		model.TaskStatusReconnecting,
		now,
	)
	if err != nil {
		return nil, fmt.Errorf("list restartable tasks: %w", err)
	}
	defer rows.Close()
	return scanTasks(rows)
}

func scanTasks(rows *sql.Rows) ([]model.Task, error) {
	var out []model.Task
	for rows.Next() {
		item, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func scanTask(row interface {
	Scan(...any) error
}) (model.Task, error) {
	var item model.Task
	var nextRunAt sql.NullTime
	var startedAt sql.NullTime
	var finishedAt sql.NullTime
	if err := row.Scan(
		&item.ID,
		&item.Type,
		&item.Status,
		&item.Progress,
		&item.Total,
		&item.Message,
		&item.ErrorCode,
		&item.ErrorMessage,
		&item.RetryCount,
		&nextRunAt,
		&item.PayloadJSON,
		&startedAt,
		&finishedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return model.Task{}, err
	}
	if nextRunAt.Valid {
		item.NextRunAt = &nextRunAt.Time
	}
	if startedAt.Valid {
		item.StartedAt = &startedAt.Time
	}
	if finishedAt.Valid {
		item.FinishedAt = &finishedAt.Time
	}
	return item, nil
}
