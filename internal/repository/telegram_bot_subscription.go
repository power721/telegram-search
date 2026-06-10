package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"tg-search/internal/model"
)

type TelegramBotSubscriptionRepository struct {
	db *sql.DB
}

func NewTelegramBotSubscriptionRepository(db *sql.DB) *TelegramBotSubscriptionRepository {
	return &TelegramBotSubscriptionRepository{db: db}
}

func (r *TelegramBotSubscriptionRepository) Create(ctx context.Context, item model.TelegramBotSubscription) (int64, error) {
	now := time.Now().UTC()
	var id int64
	err := r.db.QueryRowContext(ctx, `
INSERT INTO telegram_bot_subscriptions (chat_id, saved_search_id, enabled, created_at, updated_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(chat_id, saved_search_id) DO UPDATE SET
  enabled = excluded.enabled,
  updated_at = excluded.updated_at
RETURNING id`,
		item.ChatID, item.SavedSearchID, boolInt(item.Enabled), now, now,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create telegram bot subscription: %w", err)
	}
	return id, nil
}

func (r *TelegramBotSubscriptionRepository) DeleteForChat(ctx context.Context, chatID int64, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM telegram_bot_subscriptions WHERE chat_id = ? AND id = ?`, chatID, id)
	if err != nil {
		return fmt.Errorf("delete telegram bot subscription: %w", err)
	}
	return requireRows(res, "telegram bot subscription not found")
}

func (r *TelegramBotSubscriptionRepository) FindByID(ctx context.Context, id int64) (model.TelegramBotSubscription, error) {
	return scanTelegramBotSubscription(r.db.QueryRowContext(ctx, `
SELECT s.id, s.chat_id, s.saved_search_id, s.enabled, s.created_at, s.updated_at, ss.name, ss.keyword
FROM telegram_bot_subscriptions s
JOIN saved_searches ss ON ss.id = s.saved_search_id
WHERE s.id = ?`, id))
}

func (r *TelegramBotSubscriptionRepository) FindByChat(ctx context.Context, chatID int64) ([]model.TelegramBotSubscription, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT s.id, s.chat_id, s.saved_search_id, s.enabled, s.created_at, s.updated_at, ss.name, ss.keyword
FROM telegram_bot_subscriptions s
JOIN saved_searches ss ON ss.id = s.saved_search_id
WHERE s.chat_id = ? AND s.enabled = 1
ORDER BY s.id`, chatID)
	if err != nil {
		return nil, fmt.Errorf("find telegram bot subscriptions by chat: %w", err)
	}
	defer rows.Close()
	return scanTelegramBotSubscriptions(rows)
}

func (r *TelegramBotSubscriptionRepository) FindBySavedSearch(ctx context.Context, savedSearchID int64) ([]model.TelegramBotSubscription, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT s.id, s.chat_id, s.saved_search_id, s.enabled, s.created_at, s.updated_at, ss.name, ss.keyword
FROM telegram_bot_subscriptions s
JOIN saved_searches ss ON ss.id = s.saved_search_id
WHERE s.saved_search_id = ? AND s.enabled = 1
ORDER BY s.id`, savedSearchID)
	if err != nil {
		return nil, fmt.Errorf("find telegram bot subscriptions by saved search: %w", err)
	}
	defer rows.Close()
	return scanTelegramBotSubscriptions(rows)
}

func (r *TelegramBotSubscriptionRepository) ResourceSent(ctx context.Context, chatID int64, resourceID string) (bool, error) {
	resourceID = strings.TrimSpace(resourceID)
	if resourceID == "" {
		return false, nil
	}
	var id int64
	err := r.db.QueryRowContext(ctx, `
SELECT id
FROM telegram_bot_sent_resources
WHERE chat_id = ? AND resource_id = ?`, chatID, resourceID).Scan(&id)
	if err == nil {
		return true, nil
	}
	if err == sql.ErrNoRows {
		return false, nil
	}
	return false, fmt.Errorf("check telegram sent resource: %w", err)
}

func (r *TelegramBotSubscriptionRepository) MarkResourceSent(ctx context.Context, chatID int64, resourceID string) error {
	resourceID = strings.TrimSpace(resourceID)
	if resourceID == "" {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `
INSERT OR IGNORE INTO telegram_bot_sent_resources (chat_id, resource_id, created_at)
VALUES (?, ?, ?)`, chatID, resourceID, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("mark telegram sent resource: %w", err)
	}
	return nil
}

func scanTelegramBotSubscriptions(rows *sql.Rows) ([]model.TelegramBotSubscription, error) {
	var out []model.TelegramBotSubscription
	for rows.Next() {
		item, err := scanTelegramBotSubscription(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func scanTelegramBotSubscription(row interface{ Scan(...any) error }) (model.TelegramBotSubscription, error) {
	var item model.TelegramBotSubscription
	var enabled int
	if err := row.Scan(&item.ID, &item.ChatID, &item.SavedSearchID, &enabled, &item.CreatedAt, &item.UpdatedAt, &item.SavedSearch, &item.Keyword); err != nil {
		return model.TelegramBotSubscription{}, err
	}
	item.Enabled = enabled != 0
	return item, nil
}
