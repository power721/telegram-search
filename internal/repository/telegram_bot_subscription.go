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

func (r *TelegramBotSubscriptionRepository) DeleteForChatSavedSearch(ctx context.Context, chatID int64, savedSearchID int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM telegram_bot_subscriptions WHERE chat_id = ? AND saved_search_id = ?`, chatID, savedSearchID)
	if err != nil {
		return fmt.Errorf("delete telegram bot subscription by saved search: %w", err)
	}
	return requireRows(res, "telegram bot subscription not found")
}

func (r *TelegramBotSubscriptionRepository) ReplaceSavedSearchChats(ctx context.Context, savedSearchID int64, chatIDs []int64) error {
	chatIDs = normalizeInt64IDs(chatIDs)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM telegram_bot_subscriptions WHERE saved_search_id = ?`, savedSearchID); err != nil {
		return fmt.Errorf("delete telegram bot subscriptions for saved search: %w", err)
	}
	now := time.Now().UTC()
	for _, chatID := range chatIDs {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO telegram_bot_subscriptions (chat_id, saved_search_id, enabled, created_at, updated_at)
VALUES (?, ?, 1, ?, ?)`, chatID, savedSearchID, now, now); err != nil {
			return fmt.Errorf("create telegram bot subscription for saved search: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
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

func (r *TelegramBotSubscriptionRepository) FindChatIDsBySavedSearch(ctx context.Context, savedSearchID int64) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT chat_id
FROM telegram_bot_subscriptions
WHERE saved_search_id = ? AND enabled = 1
ORDER BY id`, savedSearchID)
	if err != nil {
		return nil, fmt.Errorf("find telegram bot chat ids by saved search: %w", err)
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var chatID int64
		if err := rows.Scan(&chatID); err != nil {
			return nil, err
		}
		out = append(out, chatID)
	}
	return out, rows.Err()
}

func (r *TelegramBotSubscriptionRepository) FindChatIDsBySavedSearches(ctx context.Context, savedSearchIDs []int64) (map[int64][]int64, error) {
	savedSearchIDs = normalizeInt64IDs(savedSearchIDs)
	out := make(map[int64][]int64, len(savedSearchIDs))
	if len(savedSearchIDs) == 0 {
		return out, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(savedSearchIDs)), ",")
	args := make([]any, 0, len(savedSearchIDs))
	for _, id := range savedSearchIDs {
		args = append(args, id)
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT saved_search_id, chat_id
FROM telegram_bot_subscriptions
WHERE enabled = 1 AND saved_search_id IN (`+placeholders+`)
ORDER BY saved_search_id, id`, args...)
	if err != nil {
		return nil, fmt.Errorf("find telegram bot chat ids by saved searches: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var savedSearchID, chatID int64
		if err := rows.Scan(&savedSearchID, &chatID); err != nil {
			return nil, err
		}
		out[savedSearchID] = append(out[savedSearchID], chatID)
	}
	return out, rows.Err()
}

func (r *TelegramBotSubscriptionRepository) UpsertChat(ctx context.Context, item model.TelegramBotChat) error {
	if item.ChatID == 0 {
		return nil
	}
	item.Title = strings.TrimSpace(item.Title)
	item.Username = strings.TrimSpace(item.Username)
	item.FirstName = strings.TrimSpace(item.FirstName)
	item.LastName = strings.TrimSpace(item.LastName)
	item.Type = strings.TrimSpace(item.Type)
	now := time.Now().UTC()
	if item.LastSeenAt.IsZero() {
		item.LastSeenAt = now
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO telegram_bot_chats (chat_id, title, username, first_name, last_name, type, last_seen_at, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(chat_id) DO UPDATE SET
  title = excluded.title,
  username = excluded.username,
  first_name = excluded.first_name,
  last_name = excluded.last_name,
  type = excluded.type,
  last_seen_at = excluded.last_seen_at,
  updated_at = excluded.updated_at`,
		item.ChatID, item.Title, item.Username, item.FirstName, item.LastName, item.Type, item.LastSeenAt, now, now,
	)
	if err != nil {
		return fmt.Errorf("upsert telegram bot chat: %w", err)
	}
	return nil
}

func (r *TelegramBotSubscriptionRepository) FindChats(ctx context.Context) ([]model.TelegramBotChat, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT chat_id, title, username, first_name, last_name, type, last_seen_at, created_at, updated_at
FROM telegram_bot_chats
ORDER BY last_seen_at DESC, chat_id`)
	if err != nil {
		return nil, fmt.Errorf("find telegram bot chats: %w", err)
	}
	defer rows.Close()
	var out []model.TelegramBotChat
	for rows.Next() {
		item, err := scanTelegramBotChat(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
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

func scanTelegramBotChat(row interface{ Scan(...any) error }) (model.TelegramBotChat, error) {
	var item model.TelegramBotChat
	if err := row.Scan(&item.ChatID, &item.Title, &item.Username, &item.FirstName, &item.LastName, &item.Type, &item.LastSeenAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return model.TelegramBotChat{}, err
	}
	return item, nil
}

func normalizeInt64IDs(ids []int64) []int64 {
	seen := map[int64]struct{}{}
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
