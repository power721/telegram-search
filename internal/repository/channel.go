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
	syncState := "metadata_only"
	if control.HistorySyncEnabled {
		syncState = "pending"
	}
	listenState := "disabled"
	if control.ListenEnabled {
		listenState = "enabled"
	}
	res, err := r.db.ExecContext(ctx, `
UPDATE telegram_channels
SET history_sync_enabled = ?, sync_profile = ?, listen_enabled = ?, remote_search_allowed = ?, sync_state = ?, listen_state = ?, updated_at = ?
WHERE id = ?`,
		control.HistorySyncEnabled, control.SyncProfile, control.ListenEnabled, control.RemoteSearchAllowed, syncState, listenState, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("update channel control: %w", err)
	}
	return requireRows(res, "channel not found")
}

func (r *ChannelRepository) UpdateControls(ctx context.Context, ids []int64, control model.ChannelControl) error {
	if len(ids) == 0 {
		return nil
	}
	if control.SyncProfile == "" {
		control.SyncProfile = "Normal"
	}
	syncState := "metadata_only"
	if control.HistorySyncEnabled {
		syncState = "pending"
	}
	listenState := "disabled"
	if control.ListenEnabled {
		listenState = "enabled"
	}
	seen := map[int64]struct{}{}
	unique := make([]int64, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin update channel controls: %w", err)
	}
	defer tx.Rollback()
	for _, id := range unique {
		res, err := tx.ExecContext(ctx, `
UPDATE telegram_channels
SET history_sync_enabled = ?, sync_profile = ?, listen_enabled = ?, remote_search_allowed = ?, sync_state = ?, listen_state = ?, updated_at = ?
WHERE id = ?`,
			control.HistorySyncEnabled, control.SyncProfile, control.ListenEnabled, control.RemoteSearchAllowed, syncState, listenState, time.Now().UTC(), id)
		if err != nil {
			return fmt.Errorf("update channel control: %w", err)
		}
		if err := requireRows(res, "channel not found"); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit update channel controls: %w", err)
	}
	return nil
}

func (r *ChannelRepository) UpdateCursor(ctx context.Context, channelID int64, lastMessageID int64, syncTime time.Time) error {
	return r.updateCursor(ctx, r.db, channelID, lastMessageID, syncTime)
}

func (r *ChannelRepository) UpdateCursorTx(ctx context.Context, tx *sql.Tx, channelID int64, lastMessageID int64, syncTime time.Time) error {
	return r.updateCursor(ctx, tx, channelID, lastMessageID, syncTime)
}

func (r *ChannelRepository) MarkSynced(ctx context.Context, channelID int64, syncTime time.Time) error {
	return r.markSynced(ctx, r.db, channelID, syncTime)
}

func (r *ChannelRepository) MarkSyncedTx(ctx context.Context, tx *sql.Tx, channelID int64, syncTime time.Time) error {
	return r.markSynced(ctx, tx, channelID, syncTime)
}

func (r *ChannelRepository) updateCursor(ctx context.Context, exec executor, channelID int64, lastMessageID int64, syncTime time.Time) error {
	res, err := exec.ExecContext(ctx, `
UPDATE telegram_channels
SET last_message_id = ?, last_sync_time = ?, sync_state = ?, updated_at = ?
WHERE id = ?`, lastMessageID, syncTime, "synced", time.Now().UTC(), channelID)
	if err != nil {
		return fmt.Errorf("update channel cursor: %w", err)
	}
	return requireRows(res, "channel not found")
}

func (r *ChannelRepository) markSynced(ctx context.Context, exec executor, channelID int64, syncTime time.Time) error {
	res, err := exec.ExecContext(ctx, `
UPDATE telegram_channels
SET sync_state = ?, last_sync_time = ?, updated_at = ?
WHERE id = ?`, "synced", syncTime, time.Now().UTC(), channelID)
	if err != nil {
		return fmt.Errorf("mark channel synced: %w", err)
	}
	return requireRows(res, "channel not found")
}

func (r *ChannelRepository) FindByID(ctx context.Context, id int64) (model.Channel, error) {
	return scanChannel(r.db.QueryRowContext(ctx, `
SELECT c.id, c.account_id, c.telegram_channel_id, c.access_hash, c.title, c.username, c.type, c.member_count, c.description, c.avatar_state, c.sync_state, c.listen_state, c.history_sync_enabled, c.sync_profile, c.listen_enabled, c.remote_search_allowed, c.last_message_id, c.last_sync_time, c.web_access, c.web_access_checked_at, c.web_access_error, COALESCE(message_counts.indexed_message_count, 0), c.created_at, c.updated_at
FROM telegram_channels c
LEFT JOIN (
  SELECT channel_id, COUNT(*) AS indexed_message_count
  FROM telegram_messages
  WHERE deleted = 0
  GROUP BY channel_id
) message_counts ON message_counts.channel_id = c.id
WHERE c.id = ?`, id))
}

func (r *ChannelRepository) FindByTelegramID(ctx context.Context, accountID int64, telegramChannelID int64) (model.Channel, error) {
	return scanChannel(r.db.QueryRowContext(ctx, `
SELECT c.id, c.account_id, c.telegram_channel_id, c.access_hash, c.title, c.username, c.type, c.member_count, c.description, c.avatar_state, c.sync_state, c.listen_state, c.history_sync_enabled, c.sync_profile, c.listen_enabled, c.remote_search_allowed, c.last_message_id, c.last_sync_time, c.web_access, c.web_access_checked_at, c.web_access_error, COALESCE(message_counts.indexed_message_count, 0), c.created_at, c.updated_at
FROM telegram_channels c
LEFT JOIN (
  SELECT channel_id, COUNT(*) AS indexed_message_count
  FROM telegram_messages
  WHERE deleted = 0
  GROUP BY channel_id
) message_counts ON message_counts.channel_id = c.id
WHERE c.account_id = ? AND c.telegram_channel_id = ?`, accountID, telegramChannelID))
}

func (r *ChannelRepository) FindAll(ctx context.Context) ([]model.Channel, error) {
	return r.find(ctx, ``, nil)
}

func (r *ChannelRepository) FindByAccountID(ctx context.Context, accountID int64) ([]model.Channel, error) {
	return r.find(ctx, `WHERE c.account_id = ?`, []any{accountID})
}

func (r *ChannelRepository) find(ctx context.Context, where string, args []any) ([]model.Channel, error) {
	query := `
SELECT c.id, c.account_id, c.telegram_channel_id, c.access_hash, c.title, c.username, c.type, c.member_count, c.description, c.avatar_state, c.sync_state, c.listen_state, c.history_sync_enabled, c.sync_profile, c.listen_enabled, c.remote_search_allowed, c.last_message_id, c.last_sync_time, c.web_access, c.web_access_checked_at, c.web_access_error, COALESCE(message_counts.indexed_message_count, 0), c.created_at, c.updated_at
FROM telegram_channels c
LEFT JOIN (
  SELECT channel_id, COUNT(*) AS indexed_message_count
  FROM telegram_messages
  WHERE deleted = 0
  GROUP BY channel_id
) message_counts ON message_counts.channel_id = c.id
` + where + ` ORDER BY c.title, c.id`
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
	return r.UpdateWebAccessResult(ctx, channelID, access, checkedAt, "")
}

func (r *ChannelRepository) UpdateWebAccessResult(ctx context.Context, channelID int64, access bool, checkedAt time.Time, errorText string) error {
	res, err := r.db.ExecContext(ctx, `
UPDATE telegram_channels
SET web_access = ?, web_access_checked_at = ?, web_access_error = ?, updated_at = ?
WHERE id = ?`, access, checkedAt, errorText, time.Now().UTC(), channelID)
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
		&channel.IndexedMessageCount,
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
