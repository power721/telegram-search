package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"tg-provider/internal/model"
)

type LinkRepository struct {
	db *sql.DB
}

func NewLinkRepository(db *sql.DB) *LinkRepository {
	return &LinkRepository{db: db}
}

func (r *LinkRepository) SaveBatch(ctx context.Context, messageID int64, links []model.Link) ([]model.Link, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	out, err := r.SaveBatchTx(ctx, tx, messageID, links)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *LinkRepository) SaveBatchTx(ctx context.Context, tx *sql.Tx, messageID int64, links []model.Link) ([]model.Link, error) {
	out := make([]model.Link, 0, len(links))
	now := time.Now().UTC()
	for _, link := range links {
		if link.Type == "" {
			link.Type = "url"
		}
		err := tx.QueryRowContext(ctx, `
INSERT INTO telegram_links (message_id, type, url, password, created_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(message_id, type, url) DO UPDATE SET password = excluded.password
RETURNING id, created_at`,
			messageID, link.Type, link.URL, link.Password, now,
		).Scan(&link.ID, &link.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("save link %s: %w", link.URL, err)
		}
		link.MessageID = messageID
		out = append(out, link)
	}
	return out, nil
}

func (r *LinkRepository) ReplaceForMessageTx(ctx context.Context, tx *sql.Tx, messageID int64, links []model.Link) ([]model.Link, error) {
	if _, err := tx.ExecContext(ctx, `DELETE FROM telegram_links WHERE message_id = ?`, messageID); err != nil {
		return nil, fmt.Errorf("delete old links: %w", err)
	}
	return r.SaveBatchTx(ctx, tx, messageID, links)
}

func (r *LinkRepository) Search(ctx context.Context, params LinkSearchParams) ([]model.LinkResult, error) {
	limit := clampLimit(params.Limit, 50)
	where := []string{`m.deleted = 0`}
	args := []any{}
	if params.Type != "" {
		where = append(where, `l.type = ?`)
		args = append(args, params.Type)
	}
	if params.AccountID > 0 {
		where = append(where, `m.account_id = ?`)
		args = append(args, params.AccountID)
	}
	if params.ChannelID > 0 {
		where = append(where, `m.channel_id = ?`)
		args = append(args, params.ChannelID)
	}
	if params.Keyword != "" {
		where = append(where, `m.text LIKE ?`)
		args = append(args, "%"+params.Keyword+"%")
	}
	args = append(args, limit, params.Offset)
	query := `
SELECT l.id, l.message_id, l.type, l.url, l.password, l.created_at,
       m.text, m.date, m.account_id, m.channel_id, c.title, m.telegram_message_id
FROM telegram_links l
JOIN telegram_messages m ON m.id = l.message_id
JOIN telegram_channels c ON c.id = m.channel_id
WHERE ` + strings.Join(where, " AND ") + `
ORDER BY m.date DESC, l.id DESC
LIMIT ? OFFSET ?`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("search links: %w", err)
	}
	defer rows.Close()
	var out []model.LinkResult
	for rows.Next() {
		var item model.LinkResult
		if err := rows.Scan(&item.ID, &item.MessageID, &item.Type, &item.URL, &item.Password, &item.CreatedAt, &item.MessageText, &item.MessageDate, &item.AccountID, &item.ChannelID, &item.ChannelTitle, &item.TelegramMessageID); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func loadLinks(ctx context.Context, db *sql.DB, messageID int64) ([]model.Link, error) {
	rows, err := db.QueryContext(ctx, `
SELECT id, message_id, type, url, password, created_at
FROM telegram_links
WHERE message_id = ?
ORDER BY id`, messageID)
	if err != nil {
		return nil, fmt.Errorf("load links: %w", err)
	}
	defer rows.Close()
	var out []model.Link
	for rows.Next() {
		var link model.Link
		if err := rows.Scan(&link.ID, &link.MessageID, &link.Type, &link.URL, &link.Password, &link.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, link)
	}
	return out, rows.Err()
}
