package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"tg-search/internal/model"
)

type SyncCursorRepository struct {
	db *sql.DB
}

func NewSyncCursorRepository(db *sql.DB) *SyncCursorRepository {
	return &SyncCursorRepository{db: db}
}

func (r *SyncCursorRepository) Save(ctx context.Context, cursor model.SyncCursor) error {
	return r.save(ctx, r.db, cursor)
}

func (r *SyncCursorRepository) SaveTx(ctx context.Context, tx *sql.Tx, cursor model.SyncCursor) error {
	return r.save(ctx, tx, cursor)
}

func (r *SyncCursorRepository) save(ctx context.Context, exec executor, cursor model.SyncCursor) error {
	if cursor.CursorType == "" {
		cursor.CursorType = "history"
	}
	now := time.Now().UTC()
	var date any
	if !cursor.Date.IsZero() {
		date = cursor.Date
	}
	err := exec.QueryRowContext(ctx, `
INSERT INTO telegram_sync_cursors
  (account_id, channel_id, cursor_type, last_message_id, pts, qts, date, created_at, updated_at)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(account_id, channel_id, cursor_type) DO UPDATE SET
  last_message_id = excluded.last_message_id,
  pts = excluded.pts,
  qts = excluded.qts,
  date = excluded.date,
  updated_at = excluded.updated_at
RETURNING id`,
		cursor.AccountID, cursor.ChannelID, cursor.CursorType, cursor.LastMessageID, cursor.PTS, cursor.QTS, date, now, now,
	).Scan(&cursor.ID)
	if err != nil {
		return fmt.Errorf("save sync cursor: %w", err)
	}
	return nil
}

func (r *SyncCursorRepository) Find(ctx context.Context, accountID int64, channelID int64, cursorType string) (model.SyncCursor, error) {
	if cursorType == "" {
		cursorType = "history"
	}
	return scanSyncCursor(r.db.QueryRowContext(ctx, `
SELECT id, account_id, channel_id, cursor_type, last_message_id, pts, qts, date, created_at, updated_at
FROM telegram_sync_cursors
WHERE account_id = ? AND channel_id = ? AND cursor_type = ?`, accountID, channelID, cursorType))
}

func scanSyncCursor(row interface {
	Scan(...any) error
}) (model.SyncCursor, error) {
	var cursor model.SyncCursor
	var date sql.NullTime
	err := row.Scan(
		&cursor.ID,
		&cursor.AccountID,
		&cursor.ChannelID,
		&cursor.CursorType,
		&cursor.LastMessageID,
		&cursor.PTS,
		&cursor.QTS,
		&date,
		&cursor.CreatedAt,
		&cursor.UpdatedAt,
	)
	if err != nil {
		return model.SyncCursor{}, err
	}
	if date.Valid {
		cursor.Date = date.Time
	}
	return cursor, nil
}
