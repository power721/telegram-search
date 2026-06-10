package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"tg-search/internal/model"
)

type NotificationDeliveryRepository struct {
	db *sql.DB
}

type NotificationDeliveryListParams struct {
	Status string
	Limit  int
	Offset int
}

func NewNotificationDeliveryRepository(db *sql.DB) *NotificationDeliveryRepository {
	return &NotificationDeliveryRepository{db: db}
}

func (r *NotificationDeliveryRepository) Create(ctx context.Context, item model.NotificationDelivery) (int64, error) {
	now := time.Now().UTC()
	if item.Status == "" {
		item.Status = model.NotificationDeliveryPending
	}
	if item.PayloadJSON == "" {
		item.PayloadJSON = "{}"
	}
	var id int64
	err := r.db.QueryRowContext(ctx, `
INSERT INTO notification_deliveries
  (event_type, target_type, target_id, payload_json, status, retry_count, last_error, next_run_at, delivered_at, created_at, updated_at)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id`,
		item.EventType, item.TargetType, item.TargetID, item.PayloadJSON, item.Status, item.RetryCount, item.LastError, item.NextRunAt, item.DeliveredAt, now, now,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create notification delivery: %w", err)
	}
	return id, nil
}

func (r *NotificationDeliveryRepository) FindByID(ctx context.Context, id int64) (model.NotificationDelivery, error) {
	return scanNotificationDelivery(r.db.QueryRowContext(ctx, `
SELECT id, event_type, target_type, target_id, payload_json, status, retry_count, last_error, next_run_at, delivered_at, created_at, updated_at
FROM notification_deliveries
WHERE id = ?`, id))
}

func (r *NotificationDeliveryRepository) List(ctx context.Context, params NotificationDeliveryListParams) ([]model.NotificationDelivery, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	offset := params.Offset
	if offset < 0 {
		offset = 0
	}
	where := "1=1"
	args := []any{}
	if params.Status != "" {
		where = "status = ?"
		args = append(args, params.Status)
	}
	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, `
SELECT id, event_type, target_type, target_id, payload_json, status, retry_count, last_error, next_run_at, delivered_at, created_at, updated_at
FROM notification_deliveries
WHERE `+where+`
ORDER BY id DESC
LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("list notification deliveries: %w", err)
	}
	defer rows.Close()
	var out []model.NotificationDelivery
	for rows.Next() {
		item, err := scanNotificationDelivery(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *NotificationDeliveryRepository) MarkSucceeded(ctx context.Context, id int64, deliveredAt time.Time) error {
	res, err := r.db.ExecContext(ctx, `
UPDATE notification_deliveries
SET status = ?, delivered_at = ?, last_error = '', updated_at = ?
WHERE id = ?`,
		model.NotificationDeliverySucceeded, deliveredAt, deliveredAt, id,
	)
	if err != nil {
		return fmt.Errorf("mark notification delivery succeeded: %w", err)
	}
	return requireRows(res, "notification delivery not found")
}

func (r *NotificationDeliveryRepository) MarkFailed(ctx context.Context, id int64, lastError string, nextRunAt *time.Time) error {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `
UPDATE notification_deliveries
SET status = ?, retry_count = retry_count + 1, last_error = ?, next_run_at = ?, updated_at = ?
WHERE id = ?`,
		model.NotificationDeliveryFailed, lastError, nextRunAt, now, id,
	)
	if err != nil {
		return fmt.Errorf("mark notification delivery failed: %w", err)
	}
	return requireRows(res, "notification delivery not found")
}

func scanNotificationDelivery(row interface{ Scan(...any) error }) (model.NotificationDelivery, error) {
	var item model.NotificationDelivery
	if err := row.Scan(&item.ID, &item.EventType, &item.TargetType, &item.TargetID, &item.PayloadJSON, &item.Status, &item.RetryCount, &item.LastError, &item.NextRunAt, &item.DeliveredAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return model.NotificationDelivery{}, err
	}
	return item, nil
}
