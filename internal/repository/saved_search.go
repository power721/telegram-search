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

type SavedSearchRepository struct {
	db *sql.DB
}

func NewSavedSearchRepository(db *sql.DB) *SavedSearchRepository {
	return &SavedSearchRepository{db: db}
}

func (r *SavedSearchRepository) Create(ctx context.Context, item model.SavedSearch) (int64, error) {
	item = normalizeSavedSearch(item)
	filters, err := json.Marshal(item.Filters)
	if err != nil {
		return 0, fmt.Errorf("marshal saved search filters: %w", err)
	}
	now := time.Now().UTC()
	var id int64
	err = r.db.QueryRowContext(ctx, `
INSERT INTO saved_searches
  (name, keyword, filters_json, notify_rss, notify_webhook, notify_telegram, enabled, created_at, updated_at)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id`,
		item.Name, item.Keyword, string(filters), boolInt(item.NotifyRSS), boolInt(item.NotifyWebhook), boolInt(item.NotifyTelegram), boolInt(item.Enabled), now, now,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create saved search: %w", err)
	}
	return id, nil
}

func (r *SavedSearchRepository) Update(ctx context.Context, item model.SavedSearch) error {
	item = normalizeSavedSearch(item)
	filters, err := json.Marshal(item.Filters)
	if err != nil {
		return fmt.Errorf("marshal saved search filters: %w", err)
	}
	res, err := r.db.ExecContext(ctx, `
UPDATE saved_searches
SET name = ?, keyword = ?, filters_json = ?, notify_rss = ?, notify_webhook = ?, notify_telegram = ?, enabled = ?, updated_at = ?
WHERE id = ?`,
		item.Name, item.Keyword, string(filters), boolInt(item.NotifyRSS), boolInt(item.NotifyWebhook), boolInt(item.NotifyTelegram), boolInt(item.Enabled), time.Now().UTC(), item.ID,
	)
	if err != nil {
		return fmt.Errorf("update saved search: %w", err)
	}
	return requireRows(res, "saved search not found")
}

func (r *SavedSearchRepository) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM saved_searches WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete saved search: %w", err)
	}
	return requireRows(res, "saved search not found")
}

func (r *SavedSearchRepository) FindByID(ctx context.Context, id int64) (model.SavedSearch, error) {
	return scanSavedSearch(r.db.QueryRowContext(ctx, `
SELECT id, name, keyword, filters_json, notify_rss, notify_webhook, notify_telegram, enabled, created_at, updated_at
FROM saved_searches
WHERE id = ?`, id))
}

func (r *SavedSearchRepository) FindAll(ctx context.Context) ([]model.SavedSearch, error) {
	return r.find(ctx, "")
}

func (r *SavedSearchRepository) FindEnabled(ctx context.Context) ([]model.SavedSearch, error) {
	return r.find(ctx, "WHERE enabled = 1")
}

func (r *SavedSearchRepository) find(ctx context.Context, where string) ([]model.SavedSearch, error) {
	query := `
SELECT id, name, keyword, filters_json, notify_rss, notify_webhook, notify_telegram, enabled, created_at, updated_at
FROM saved_searches
` + where + `
ORDER BY id DESC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("find saved searches: %w", err)
	}
	defer rows.Close()
	var out []model.SavedSearch
	for rows.Next() {
		item, err := scanSavedSearch(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func scanSavedSearch(row interface{ Scan(...any) error }) (model.SavedSearch, error) {
	var item model.SavedSearch
	var filtersRaw string
	var notifyRSS, notifyWebhook, notifyTelegram, enabled int
	if err := row.Scan(&item.ID, &item.Name, &item.Keyword, &filtersRaw, &notifyRSS, &notifyWebhook, &notifyTelegram, &enabled, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return model.SavedSearch{}, err
	}
	if err := json.Unmarshal([]byte(filtersRaw), &item.Filters); err != nil {
		return model.SavedSearch{}, fmt.Errorf("unmarshal saved search filters: %w", err)
	}
	item.NotifyRSS = notifyRSS != 0
	item.NotifyWebhook = notifyWebhook != 0
	item.NotifyTelegram = notifyTelegram != 0
	item.Enabled = enabled != 0
	return item, nil
}

func normalizeSavedSearch(item model.SavedSearch) model.SavedSearch {
	item.Name = strings.TrimSpace(item.Name)
	item.Keyword = strings.TrimSpace(item.Keyword)
	item.Filters.Type = strings.TrimSpace(item.Filters.Type)
	item.Filters.Category = strings.TrimSpace(item.Filters.Category)
	item.Filters.CloudTypes = normalizeTerms(item.Filters.CloudTypes)
	return item
}
