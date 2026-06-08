package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"tg-search/internal/model"
)

type ChannelRepository struct {
	db *sql.DB
}

func NewChannelRepository(db *sql.DB) *ChannelRepository {
	return &ChannelRepository{db: db}
}

func (r *ChannelRepository) Save(ctx context.Context, channel model.Channel) (int64, error) {
	now := time.Now().UTC()
	if channel.Type == "" {
		channel.Type = model.ChannelTypeChannel
	}
	if channel.AvatarState == "" {
		channel.AvatarState = "unknown"
	}
	if channel.SyncState == "" {
		channel.SyncState = "metadata_only"
	}
	if channel.ListenState == "" {
		channel.ListenState = "disabled"
	}
	if channel.SyncProfile == "" {
		channel.SyncProfile = "Normal"
	}
	var id int64
	err := r.db.QueryRowContext(ctx, `
INSERT INTO telegram_channels
  (account_id, telegram_channel_id, access_hash, title, username, type, member_count, description, avatar_state, sync_state, listen_state, history_sync_enabled, sync_profile, listen_enabled, remote_search_allowed, last_message_id, last_sync_time, web_access_error, created_at, updated_at)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(account_id, telegram_channel_id, type) DO UPDATE SET
  access_hash = excluded.access_hash,
  title = excluded.title,
  username = excluded.username,
  member_count = excluded.member_count,
  description = excluded.description,
  avatar_state = excluded.avatar_state,
  sync_state = excluded.sync_state,
  listen_state = excluded.listen_state,
  history_sync_enabled = excluded.history_sync_enabled,
  sync_profile = excluded.sync_profile,
  listen_enabled = excluded.listen_enabled,
  remote_search_allowed = excluded.remote_search_allowed,
  web_access_error = excluded.web_access_error,
  updated_at = excluded.updated_at
RETURNING id`,
		channel.AccountID, channel.TelegramChannelID, channel.AccessHash, channel.Title, channel.Username, channel.Type, channel.MemberCount, channel.Description, channel.AvatarState, channel.SyncState, channel.ListenState, channel.HistorySyncEnabled, channel.SyncProfile, channel.ListenEnabled, channel.RemoteSearchAllowed, channel.LastMessageID, channel.LastSyncTime, channel.WebAccessError, now, now,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("save channel: %w", err)
	}
	return id, nil
}

func (r *ChannelRepository) UpdateControl(ctx context.Context, id int64, control model.ChannelControl) error {
	if control.SyncProfile == "" {
		control.SyncProfile = "Normal"
	}
	res, err := r.db.ExecContext(ctx, `
UPDATE telegram_channels
SET history_sync_enabled = ?, sync_profile = ?, listen_enabled = ?, remote_search_allowed = ?, updated_at = ?
WHERE id = ?`,
		control.HistorySyncEnabled, control.SyncProfile, control.ListenEnabled, control.RemoteSearchAllowed, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("update channel control: %w", err)
	}
	return requireRows(res, "channel not found")
}

func (r *ChannelRepository) UpdateCursor(ctx context.Context, channelID int64, lastMessageID int64, syncTime time.Time) error {
	return r.updateCursor(ctx, r.db, channelID, lastMessageID, syncTime)
}

func (r *ChannelRepository) UpdateCursorTx(ctx context.Context, tx *sql.Tx, channelID int64, lastMessageID int64, syncTime time.Time) error {
	return r.updateCursor(ctx, tx, channelID, lastMessageID, syncTime)
}

func (r *ChannelRepository) updateCursor(ctx context.Context, exec executor, channelID int64, lastMessageID int64, syncTime time.Time) error {
	res, err := exec.ExecContext(ctx, `
UPDATE telegram_channels
SET last_message_id = ?, last_sync_time = ?, updated_at = ?
WHERE id = ?`, lastMessageID, syncTime, time.Now().UTC(), channelID)
	if err != nil {
		return fmt.Errorf("update channel cursor: %w", err)
	}
	return requireRows(res, "channel not found")
}

func (r *ChannelRepository) FindByID(ctx context.Context, id int64) (model.Channel, error) {
	return scanChannel(r.db.QueryRowContext(ctx, `
SELECT id, account_id, telegram_channel_id, access_hash, title, username, type, member_count, description, avatar_state, sync_state, listen_state, history_sync_enabled, sync_profile, listen_enabled, remote_search_allowed, last_message_id, last_sync_time, web_access, web_access_checked_at, web_access_error, created_at, updated_at
FROM telegram_channels WHERE id = ?`, id))
}

func (r *ChannelRepository) FindByTelegramID(ctx context.Context, accountID int64, telegramChannelID int64) (model.Channel, error) {
	return scanChannel(r.db.QueryRowContext(ctx, `
SELECT id, account_id, telegram_channel_id, access_hash, title, username, type, member_count, description, avatar_state, sync_state, listen_state, history_sync_enabled, sync_profile, listen_enabled, remote_search_allowed, last_message_id, last_sync_time, web_access, web_access_checked_at, web_access_error, created_at, updated_at
FROM telegram_channels WHERE account_id = ? AND telegram_channel_id = ?`, accountID, telegramChannelID))
}

func (r *ChannelRepository) FindAll(ctx context.Context) ([]model.Channel, error) {
	return r.find(ctx, ``, nil)
}

func (r *ChannelRepository) FindByAccountID(ctx context.Context, accountID int64) ([]model.Channel, error) {
	return r.find(ctx, `WHERE account_id = ?`, []any{accountID})
}

func (r *ChannelRepository) find(ctx context.Context, where string, args []any) ([]model.Channel, error) {
	query := `
SELECT id, account_id, telegram_channel_id, access_hash, title, username, type, member_count, description, avatar_state, sync_state, listen_state, history_sync_enabled, sync_profile, listen_enabled, remote_search_allowed, last_message_id, last_sync_time, web_access, web_access_checked_at, web_access_error, created_at, updated_at
FROM telegram_channels ` + where + ` ORDER BY title, id`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("find channels: %w", err)
	}
	defer rows.Close()
	var out []model.Channel
	for rows.Next() {
		channel, err := scanChannelRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, channel)
	}
	return out, rows.Err()
}

func (r *ChannelRepository) UpdateWebAccess(ctx context.Context, channelID int64, access bool, checkedAt time.Time) error {
	res, err := r.db.ExecContext(ctx, `
UPDATE telegram_channels
SET web_access = ?, web_access_checked_at = ?, updated_at = ?
WHERE id = ?`, access, checkedAt, time.Now().UTC(), channelID)
	if err != nil {
		return fmt.Errorf("update channel web access: %w", err)
	}
	return requireRows(res, "channel not found")
}

func scanChannel(row interface {
	Scan(...any) error
}) (model.Channel, error) {
	return scanChannelRows(row)
}

func scanChannelRows(row interface {
	Scan(...any) error
}) (model.Channel, error) {
	var channel model.Channel
	var lastSync sql.NullTime
	var webAccess sql.NullBool
	var webAccessCheckedAt sql.NullTime
	err := row.Scan(
		&channel.ID,
		&channel.AccountID,
		&channel.TelegramChannelID,
		&channel.AccessHash,
		&channel.Title,
		&channel.Username,
		&channel.Type,
		&channel.MemberCount,
		&channel.Description,
		&channel.AvatarState,
		&channel.SyncState,
		&channel.ListenState,
		&channel.HistorySyncEnabled,
		&channel.SyncProfile,
		&channel.ListenEnabled,
		&channel.RemoteSearchAllowed,
		&channel.LastMessageID,
		&lastSync,
		&webAccess,
		&webAccessCheckedAt,
		&channel.WebAccessError,
		&channel.CreatedAt,
		&channel.UpdatedAt,
	)
	if err != nil {
		return model.Channel{}, err
	}
	if lastSync.Valid {
		channel.LastSyncTime = &lastSync.Time
	}
	if webAccess.Valid {
		channel.WebAccess = &webAccess.Bool
	}
	if webAccessCheckedAt.Valid {
		channel.WebAccessCheckedAt = &webAccessCheckedAt.Time
	}
	return channel, nil
}
