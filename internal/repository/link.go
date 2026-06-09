package repository

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"tg-search/internal/model"
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
		if link.Category == "" {
			link.Category = linkCategory(link)
		}
		err := tx.QueryRowContext(ctx, `
INSERT INTO telegram_links (message_id, type, url, password, note, source_snippet, category, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(message_id, type, url) DO UPDATE SET
  password = excluded.password,
  note = excluded.note,
  source_snippet = excluded.source_snippet,
  category = excluded.category
RETURNING id, created_at`,
			messageID, link.Type, link.URL, link.Password, link.Note, link.SourceSnippet, link.Category, now,
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
	where, args := linkSearchWhere(params)
	args = append(args, limit, params.Offset)
	query := `
SELECT l.id, l.message_id, l.type, l.url, COALESCE(l.password, ''), COALESCE(l.note, ''),
       COALESCE(l.source_snippet, ''), COALESCE(l.category, ''), l.created_at,
       mc.text, m.date, m.account_id, m.channel_id, c.telegram_channel_id, c.title, c.username, m.telegram_message_id
FROM telegram_links l
JOIN telegram_messages m ON m.id = l.message_id
JOIN telegram_message_contents mc ON mc.message_id = m.id
JOIN telegram_channels c ON c.id = m.channel_id
WHERE ` + strings.Join(where, " AND ") + `
ORDER BY ` + dateOrderBy(params.Sort, "m.date", "l.id") + `
LIMIT ? OFFSET ?`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("search links: %w", err)
	}
	defer rows.Close()
	var out []model.LinkResult
	for rows.Next() {
		var item model.LinkResult
		if err := rows.Scan(&item.ID, &item.MessageID, &item.Type, &item.URL, &item.Password, &item.Note, &item.SourceSnippet, &item.Category, &item.CreatedAt, &item.MessageText, &item.MessageDate, &item.AccountID, &item.ChannelID, &item.TelegramChannelID, &item.ChannelTitle, &item.ChannelUsername, &item.TelegramMessageID); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *LinkRepository) SearchResources(ctx context.Context, params LinkSearchParams) ([]model.LinkResult, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = 50
	}
	where, args := linkSearchWhere(params)
	args = append(args, limit, params.Offset)
	query := `
SELECT id, message_id, type, url, password, note, source_snippet, category, created_at,
       message_text, message_date, account_id, channel_id, telegram_channel_id, channel_title, channel_username, telegram_message_id
FROM (
  SELECT l.id, l.message_id, l.type, l.url, COALESCE(l.password, '') AS password,
         COALESCE(l.note, '') AS note, COALESCE(l.source_snippet, '') AS source_snippet,
         CASE
           WHEN COALESCE(l.category, '') <> '' THEN l.category
           WHEN l.type = 'magnet' THEN 'magnet'
           WHEN l.type = 'ed2k' THEN 'ed2k'
           WHEN l.type = 'url' THEN 'http'
           ELSE 'cloud_drive'
         END AS category,
         l.created_at, mc.text AS message_text, m.date AS message_date, m.account_id,
         m.channel_id, c.telegram_channel_id, c.title AS channel_title, c.username AS channel_username, m.telegram_message_id,
         row_number() OVER (PARTITION BY l.url ORDER BY m.date DESC, l.id DESC) AS rn
  FROM telegram_links l
  JOIN telegram_messages m ON m.id = l.message_id
  JOIN telegram_message_contents mc ON mc.message_id = m.id
  JOIN telegram_channels c ON c.id = m.channel_id
  WHERE ` + strings.Join(where, " AND ") + `
)
WHERE rn = 1
ORDER BY ` + dateOrderBy(params.Sort, "message_date", "id") + `
LIMIT ? OFFSET ?`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("search resource links: %w", err)
	}
	defer rows.Close()
	var out []model.LinkResult
	for rows.Next() {
		var item model.LinkResult
		if err := rows.Scan(&item.ID, &item.MessageID, &item.Type, &item.URL, &item.Password, &item.Note, &item.SourceSnippet, &item.Category, &item.CreatedAt, &item.MessageText, &item.MessageDate, &item.AccountID, &item.ChannelID, &item.TelegramChannelID, &item.ChannelTitle, &item.ChannelUsername, &item.TelegramMessageID); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *LinkRepository) CountSearch(ctx context.Context, params LinkSearchParams) (int, error) {
	where, args := linkSearchWhere(params)
	query := `
SELECT count(*)
FROM telegram_links l
JOIN telegram_messages m ON m.id = l.message_id
JOIN telegram_message_contents mc ON mc.message_id = m.id
WHERE ` + strings.Join(where, " AND ")
	var total int
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("count search links: %w", err)
	}
	return total, nil
}

func (r *LinkRepository) CountByResourceCategory(ctx context.Context, params LinkSearchParams) (map[string]int, error) {
	where, args := linkSearchWhere(params)
	query := `
SELECT category, count(*)
FROM (
  SELECT
    l.url,
    CASE
      WHEN COALESCE(l.category, '') <> '' THEN l.category
      WHEN l.type = 'magnet' THEN 'magnet'
      WHEN l.type = 'ed2k' THEN 'ed2k'
      WHEN l.type = 'url' THEN 'http'
      ELSE 'cloud_drive'
    END AS category,
    row_number() OVER (PARTITION BY l.url ORDER BY m.date DESC, l.id DESC) AS rn
  FROM telegram_links l
  JOIN telegram_messages m ON m.id = l.message_id
  JOIN telegram_message_contents mc ON mc.message_id = m.id
  WHERE ` + strings.Join(where, " AND ") + `
)
WHERE rn = 1
GROUP BY category`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("count resource links by category: %w", err)
	}
	defer rows.Close()

	grouped := map[string]int{}
	for rows.Next() {
		var category string
		var count int
		if err := rows.Scan(&category, &count); err != nil {
			return nil, err
		}
		grouped[category] = count
	}
	return grouped, rows.Err()
}

func (r *LinkRepository) CountByType(ctx context.Context) (map[string]int, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT COALESCE(NULLIF(type, ''), 'url') AS type, count(*)
FROM telegram_links
JOIN telegram_messages m ON m.id = telegram_links.message_id
WHERE m.deleted = 0
GROUP BY COALESCE(NULLIF(type, ''), 'url')`)
	if err != nil {
		return nil, fmt.Errorf("count links by type: %w", err)
	}
	defer rows.Close()

	grouped := map[string]int{}
	for rows.Next() {
		var typ string
		var count int
		if err := rows.Scan(&typ, &count); err != nil {
			return nil, err
		}
		grouped[typ] = count
	}
	return grouped, rows.Err()
}

func linkSearchWhere(params LinkSearchParams) ([]string, []any) {
	where := []string{`m.deleted = 0`}
	args := []any{}
	if params.Type != "" {
		where = append(where, `l.type = ?`)
		args = append(args, params.Type)
	}
	if params.Category != "" {
		where = append(where, `l.category = ?`)
		args = append(args, params.Category)
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
		where = append(where, `mc.text LIKE ?`)
		args = append(args, "%"+params.Keyword+"%")
	}
	if params.DateFrom != nil {
		where = append(where, `m.date >= ?`)
		args = append(args, *params.DateFrom)
	}
	if params.DateTo != nil {
		where = append(where, `m.date < ?`)
		args = append(args, *params.DateTo)
	}
	return where, args
}

func (r *LinkRepository) SearchMerged(ctx context.Context, params MergedLinkSearchParams) (model.MergedLinksResponse, error) {
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
		where = append(where, `(mc.text LIKE ? OR COALESCE(l.note, '') LIKE ?)`)
		like := "%" + params.Keyword + "%"
		args = append(args, like, like)
	}
	if params.DateFrom != nil {
		where = append(where, `m.date >= ?`)
		args = append(args, *params.DateFrom)
	}
	if params.DateTo != nil {
		where = append(where, `m.date < ?`)
		args = append(args, *params.DateTo)
	}
	query := `
SELECT l.id, l.type, l.url, COALESCE(l.password, ''), COALESCE(l.note, ''),
       m.date, m.channel_id, c.title, c.username, m.telegram_message_id
FROM telegram_links l
JOIN telegram_messages m ON m.id = l.message_id
JOIN telegram_message_contents mc ON mc.message_id = m.id
JOIN telegram_channels c ON c.id = m.channel_id
WHERE ` + strings.Join(where, " AND ") + `
ORDER BY m.date DESC, l.id DESC`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return model.MergedLinksResponse{}, fmt.Errorf("search merged links: %w", err)
	}
	defer rows.Close()

	type candidate struct {
		id  int64
		typ string
		model.MergedLink
	}
	byURL := map[string]candidate{}
	for rows.Next() {
		var item candidate
		var channelTitle string
		var channelUsername string
		if err := rows.Scan(&item.id, &item.typ, &item.URL, &item.Password, &item.Note, &item.Datetime, &item.ChannelID, &channelTitle, &channelUsername, &item.TelegramMessageID); err != nil {
			return model.MergedLinksResponse{}, err
		}
		item.Source = sourceFromChannel(channelTitle, channelUsername)
		existing, ok := byURL[item.URL]
		if !ok || item.Datetime.After(existing.Datetime) || (item.Datetime.Equal(existing.Datetime) && item.id > existing.id) {
			byURL[item.URL] = item
		}
	}
	if err := rows.Err(); err != nil {
		return model.MergedLinksResponse{}, err
	}

	items := make([]candidate, 0, len(byURL))
	for _, item := range byURL {
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		leftScore := titleMarkerScore(items[i].Note)
		rightScore := titleMarkerScore(items[j].Note)
		if leftScore != rightScore {
			return leftScore > rightScore
		}
		if !items[i].Datetime.Equal(items[j].Datetime) {
			return items[i].Datetime.After(items[j].Datetime)
		}
		return items[i].id > items[j].id
	})

	offset := params.Offset
	if offset > len(items) {
		offset = len(items)
	}
	limit := clampLimit(params.Limit, 50)
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	page := items[offset:end]
	response := model.MergedLinksResponse{
		Total:        len(page),
		MergedByType: model.MergedLinks{},
	}
	for _, item := range page {
		response.MergedByType[item.typ] = append(response.MergedByType[item.typ], item.MergedLink)
	}
	return response, nil
}

func sourceFromChannel(title string, username string) string {
	if title != "" {
		return "tg:" + title
	}
	if username != "" {
		return "tg:" + username
	}
	return "tg"
}

func linkCategory(link model.Link) string {
	switch link.Type {
	case "magnet":
		return "magnet"
	case "ed2k":
		return "ed2k"
	case "url":
		return "http"
	default:
		return "cloud_drive"
	}
}

func titleMarkerScore(note string) int {
	lower := strings.ToLower(note)
	markers := []string{"合集", "系列", "全", "完", "最新", "complete"}
	for i, marker := range markers {
		if strings.Contains(lower, marker) {
			return len(markers) - i
		}
	}
	return 0
}

func loadLinks(ctx context.Context, db *sql.DB, messageID int64) ([]model.Link, error) {
	rows, err := db.QueryContext(ctx, `
SELECT id, message_id, type, url, COALESCE(password, ''), COALESCE(note, ''),
       COALESCE(source_snippet, ''), COALESCE(category, ''), created_at
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
		if err := rows.Scan(&link.ID, &link.MessageID, &link.Type, &link.URL, &link.Password, &link.Note, &link.SourceSnippet, &link.Category, &link.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, link)
	}
	return out, rows.Err()
}
