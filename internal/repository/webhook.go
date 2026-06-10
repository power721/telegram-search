package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"tg-search/internal/model"
)

type WebhookRepository struct {
	db *sql.DB
}

func NewWebhookRepository(db *sql.DB) *WebhookRepository {
	return &WebhookRepository{db: db}
}

func (r *WebhookRepository) Create(ctx context.Context, item model.Webhook) (int64, error) {
	item = normalizeWebhook(item)
	events, err := json.Marshal(item.Events)
	if err != nil {
		return 0, fmt.Errorf("marshal webhook events: %w", err)
	}
	now := time.Now().UTC()
	var id int64
	err = r.db.QueryRowContext(ctx, `
INSERT INTO webhooks
  (name, url, events_json, secret, enabled, created_at, updated_at)
VALUES
  (?, ?, ?, ?, ?, ?, ?)
RETURNING id`,
		item.Name, item.URL, string(events), item.Secret, boolInt(item.Enabled), now, now,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create webhook: %w", err)
	}
	return id, nil
}

func (r *WebhookRepository) Update(ctx context.Context, item model.Webhook) error {
	item = normalizeWebhook(item)
	events, err := json.Marshal(item.Events)
	if err != nil {
		return fmt.Errorf("marshal webhook events: %w", err)
	}
	res, err := r.db.ExecContext(ctx, `
UPDATE webhooks
SET name = ?, url = ?, events_json = ?, secret = ?, enabled = ?, updated_at = ?
WHERE id = ?`,
		item.Name, item.URL, string(events), item.Secret, boolInt(item.Enabled), time.Now().UTC(), item.ID,
	)
	if err != nil {
		return fmt.Errorf("update webhook: %w", err)
	}
	return requireRows(res, "webhook not found")
}

func (r *WebhookRepository) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM webhooks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete webhook: %w", err)
	}
	return requireRows(res, "webhook not found")
}

func (r *WebhookRepository) FindByID(ctx context.Context, id int64) (model.Webhook, error) {
	return scanWebhook(r.db.QueryRowContext(ctx, `
SELECT id, name, url, events_json, secret, enabled, created_at, updated_at
FROM webhooks
WHERE id = ?`, id))
}

func (r *WebhookRepository) FindAll(ctx context.Context) ([]model.Webhook, error) {
	return r.find(ctx, "")
}

func (r *WebhookRepository) FindEnabledForEvent(ctx context.Context, eventType string) ([]model.Webhook, error) {
	items, err := r.find(ctx, "WHERE enabled = 1")
	if err != nil {
		return nil, err
	}
	var out []model.Webhook
	for _, item := range items {
		if webhookWantsEvent(item, eventType) {
			out = append(out, item)
		}
	}
	return out, nil
}

func (r *WebhookRepository) find(ctx context.Context, where string) ([]model.Webhook, error) {
	query := `
SELECT id, name, url, events_json, secret, enabled, created_at, updated_at
FROM webhooks
` + where + `
ORDER BY id DESC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("find webhooks: %w", err)
	}
	defer rows.Close()
	var out []model.Webhook
	for rows.Next() {
		item, err := scanWebhook(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func scanWebhook(row interface{ Scan(...any) error }) (model.Webhook, error) {
	var item model.Webhook
	var eventsRaw string
	var enabled int
	if err := row.Scan(&item.ID, &item.Name, &item.URL, &eventsRaw, &item.Secret, &enabled, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return model.Webhook{}, err
	}
	if err := json.Unmarshal([]byte(eventsRaw), &item.Events); err != nil {
		return model.Webhook{}, fmt.Errorf("unmarshal webhook events: %w", err)
	}
	item.Enabled = enabled != 0
	return item, nil
}

func normalizeWebhook(item model.Webhook) model.Webhook {
	item.Name = strings.TrimSpace(item.Name)
	item.URL = strings.TrimSpace(item.URL)
	item.Secret = strings.TrimSpace(item.Secret)
	item.Events = normalizeTerms(item.Events)
	return item
}

func webhookWantsEvent(item model.Webhook, eventType string) bool {
	for _, event := range item.Events {
		if event == eventType || event == "*" {
			return true
		}
	}
	return false
}
