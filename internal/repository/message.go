package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"tg-provider/internal/model"
)

type MessageRepository struct {
	db *sql.DB
}

func NewMessageRepository(db *sql.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

func (r *MessageRepository) SaveBatch(ctx context.Context, messages []model.Message) ([]model.Message, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	out, err := r.SaveBatchTx(ctx, tx, messages)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *MessageRepository) SaveBatchTx(ctx context.Context, tx *sql.Tx, messages []model.Message) ([]model.Message, error) {
	out := make([]model.Message, 0, len(messages))
	if len(messages) == 0 {
		return out, nil
	}
	stmt, err := tx.PrepareContext(ctx, saveMessageSQL)
	if err != nil {
		return nil, fmt.Errorf("prepare save message: %w", err)
	}
	defer stmt.Close()
	for _, msg := range messages {
		stored, err := r.savePrepared(ctx, stmt, msg)
		if err != nil {
			return nil, err
		}
		out = append(out, stored)
	}
	return out, nil
}

func (r *MessageRepository) save(ctx context.Context, exec queryRower, msg model.Message) (model.Message, error) {
	return saveMessage(msg, func(args ...any) *sql.Row {
		return exec.QueryRowContext(ctx, saveMessageSQL, args...)
	})
}

func (r *MessageRepository) savePrepared(ctx context.Context, stmt *sql.Stmt, msg model.Message) (model.Message, error) {
	return saveMessage(msg, func(args ...any) *sql.Row {
		return stmt.QueryRowContext(ctx, args...)
	})
}

func saveMessage(msg model.Message, query func(args ...any) *sql.Row) (model.Message, error) {
	now := time.Now().UTC()
	var editDate any
	if msg.EditDate != nil {
		editDate = *msg.EditDate
	}
	deleted := 0
	if msg.Deleted {
		deleted = 1
	}
	err := query(
		msg.AccountID, msg.ChannelID, msg.TelegramMessageID, msg.SenderID, msg.Text, msg.RawJSON, msg.Date, editDate, deleted, now, now,
	).Scan(&msg.ID, &msg.CreatedAt, &msg.UpdatedAt)
	if err != nil {
		return model.Message{}, fmt.Errorf("save message %d/%d: %w", msg.ChannelID, msg.TelegramMessageID, err)
	}
	return msg, nil
}

const saveMessageSQL = `
INSERT INTO telegram_messages
  (account_id, channel_id, telegram_message_id, sender_id, text, raw_json, date, edit_date, deleted, created_at, updated_at)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(channel_id, telegram_message_id) DO UPDATE SET
  account_id = excluded.account_id,
  sender_id = excluded.sender_id,
  text = excluded.text,
  raw_json = excluded.raw_json,
  date = excluded.date,
  edit_date = excluded.edit_date,
  deleted = excluded.deleted,
  updated_at = excluded.updated_at
RETURNING id, created_at, updated_at`

func (r *MessageRepository) Search(ctx context.Context, params SearchParams) ([]model.SearchResult, error) {
	limit := clampLimit(params.Limit, 50)
	where := []string{`telegram_messages_fts MATCH ?`, `m.deleted = 0`}
	args := []any{params.Query}
	if params.AccountID > 0 {
		where = append(where, `m.account_id = ?`)
		args = append(args, params.AccountID)
	}
	if params.ChannelID > 0 {
		where = append(where, `m.channel_id = ?`)
		args = append(args, params.ChannelID)
	}
	if params.LinkType != "" {
		where = append(where, `EXISTS (SELECT 1 FROM telegram_links fl WHERE fl.message_id = m.id AND fl.type = ?)`)
		args = append(args, params.LinkType)
	}
	if params.DateFrom != nil {
		where = append(where, `m.date >= ?`)
		args = append(args, *params.DateFrom)
	}
	if params.DateTo != nil {
		where = append(where, `m.date < ?`)
		args = append(args, *params.DateTo)
	}
	if params.BeforeDate != nil && params.BeforeID > 0 {
		where = append(where, `(m.date < ? OR (m.date = ? AND m.id < ?))`)
		args = append(args, *params.BeforeDate, *params.BeforeDate, params.BeforeID)
	}
	args = append(args, limit, params.Offset)
	query := `
SELECT m.id, m.account_id, m.channel_id, m.telegram_message_id, m.sender_id, m.text, m.raw_json, m.date, m.edit_date,
       m.deleted, m.created_at, m.updated_at,
       a.phone, a.username, a.first_name, c.title, c.username
FROM telegram_messages_fts
JOIN telegram_messages m ON m.id = telegram_messages_fts.rowid
JOIN telegram_accounts a ON a.id = m.account_id
JOIN telegram_channels c ON c.id = m.channel_id
WHERE ` + strings.Join(where, " AND ") + `
ORDER BY m.date DESC, m.id DESC
LIMIT ? OFFSET ?`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("search messages: %w", err)
	}
	var out []model.SearchResult
	for rows.Next() {
		item, err := scanSearchResult(rows)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return attachLinks(ctx, r.db, out)
}

func (r *MessageRepository) Latest(ctx context.Context, params LatestParams) ([]model.SearchResult, error) {
	limit := clampLimit(params.Limit, 50)
	where := []string{`m.deleted = 0`}
	args := []any{}
	if params.AccountID > 0 {
		where = append(where, `m.account_id = ?`)
		args = append(args, params.AccountID)
	}
	if params.ChannelID > 0 {
		where = append(where, `m.channel_id = ?`)
		args = append(args, params.ChannelID)
	}
	if params.BeforeDate != nil && params.BeforeID > 0 {
		where = append(where, `(m.date < ? OR (m.date = ? AND m.id < ?))`)
		args = append(args, *params.BeforeDate, *params.BeforeDate, params.BeforeID)
	}
	args = append(args, limit)
	query := `
SELECT m.id, m.account_id, m.channel_id, m.telegram_message_id, m.sender_id, m.text, m.raw_json, m.date, m.edit_date,
       m.deleted, m.created_at, m.updated_at,
       a.phone, a.username, a.first_name, c.title, c.username
FROM telegram_messages m
JOIN telegram_accounts a ON a.id = m.account_id
JOIN telegram_channels c ON c.id = m.channel_id
WHERE ` + strings.Join(where, " AND ") + `
ORDER BY m.date DESC, m.id DESC
LIMIT ?`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("latest messages: %w", err)
	}
	var out []model.SearchResult
	for rows.Next() {
		item, err := scanSearchResult(rows)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return attachLinks(ctx, r.db, out)
}

func (r *MessageRepository) MarkDeleted(ctx context.Context, channelID int64, telegramMessageID int64) error {
	res, err := r.db.ExecContext(ctx, `
UPDATE telegram_messages
SET deleted = 1, updated_at = ?
WHERE channel_id = ? AND telegram_message_id = ?`, time.Now().UTC(), channelID, telegramMessageID)
	if err != nil {
		return fmt.Errorf("mark message deleted: %w", err)
	}
	return requireRows(res, "message not found")
}

func (r *MessageRepository) MarkDeletedTx(ctx context.Context, tx *sql.Tx, channelID int64, telegramMessageID int64) error {
	res, err := tx.ExecContext(ctx, `
UPDATE telegram_messages
SET deleted = 1, updated_at = ?
WHERE channel_id = ? AND telegram_message_id = ?`, time.Now().UTC(), channelID, telegramMessageID)
	if err != nil {
		return fmt.Errorf("mark message deleted: %w", err)
	}
	return requireRows(res, "message not found")
}

func scanSearchResult(row interface {
	Scan(...any) error
}) (model.SearchResult, error) {
	var item model.SearchResult
	var editDate sql.NullTime
	var deleted int
	err := row.Scan(&item.ID, &item.AccountID, &item.ChannelID, &item.TelegramMessageID, &item.SenderID, &item.Text, &item.RawJSON, &item.Date, &editDate,
		&deleted, &item.CreatedAt, &item.UpdatedAt, &item.AccountPhone, &item.AccountUsername, &item.AccountFirstName, &item.ChannelTitle, &item.ChannelUsername)
	if err != nil {
		return model.SearchResult{}, err
	}
	if editDate.Valid {
		item.EditDate = &editDate.Time
	}
	item.Deleted = deleted != 0
	return item, nil
}

func attachLinks(ctx context.Context, db *sql.DB, items []model.SearchResult) ([]model.SearchResult, error) {
	if len(items) == 0 {
		return items, nil
	}
	ids := make([]any, 0, len(items))
	placeholders := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
		placeholders = append(placeholders, "?")
	}
	rows, err := db.QueryContext(ctx, `
SELECT id, message_id, type, url, password, created_at
FROM telegram_links
WHERE message_id IN (`+strings.Join(placeholders, ",")+`)
ORDER BY message_id, id`, ids...)
	if err != nil {
		return nil, fmt.Errorf("load links: %w", err)
	}
	defer rows.Close()

	byMessageID := map[int64][]model.Link{}
	for rows.Next() {
		var link model.Link
		if err := rows.Scan(&link.ID, &link.MessageID, &link.Type, &link.URL, &link.Password, &link.CreatedAt); err != nil {
			return nil, err
		}
		byMessageID[link.MessageID] = append(byMessageID[link.MessageID], link)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range items {
		items[i].Links = byMessageID[items[i].ID]
	}
	return items, nil
}
