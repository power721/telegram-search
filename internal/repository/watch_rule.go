package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"tg-search/internal/model"
)

var ErrNotFound = sql.ErrNoRows
var ErrDuplicateWatchRule = errors.New("watch rule already exists for channel")

type WatchRuleRepository struct {
	db *sql.DB
}

func NewWatchRuleRepository(db *sql.DB) *WatchRuleRepository {
	return &WatchRuleRepository{db: db}
}

func (r *WatchRuleRepository) Create(ctx context.Context, rule model.WatchRule) (int64, error) {
	rule.Includes = normalizeTerms(rule.Includes)
	rule.Excludes = normalizeTerms(rule.Excludes)
	includes, err := json.Marshal(rule.Includes)
	if err != nil {
		return 0, fmt.Errorf("marshal includes: %w", err)
	}
	excludes, err := json.Marshal(rule.Excludes)
	if err != nil {
		return 0, fmt.Errorf("marshal excludes: %w", err)
	}
	now := time.Now().UTC()
	var id int64
	err = r.db.QueryRowContext(ctx, `
INSERT INTO telegram_watch_rules (channel_id, enabled, includes_json, excludes_json, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING id`,
		rule.ChannelID, boolInt(rule.Enabled), string(includes), string(excludes), now, now,
	).Scan(&id)
	if err != nil {
		if isWatchRuleUniqueConstraint(err) {
			return 0, ErrDuplicateWatchRule
		}
		return 0, fmt.Errorf("create watch rule: %w", err)
	}
	return id, nil
}

func (r *WatchRuleRepository) Update(ctx context.Context, rule model.WatchRule) error {
	rule.Includes = normalizeTerms(rule.Includes)
	rule.Excludes = normalizeTerms(rule.Excludes)
	includes, err := json.Marshal(rule.Includes)
	if err != nil {
		return fmt.Errorf("marshal includes: %w", err)
	}
	excludes, err := json.Marshal(rule.Excludes)
	if err != nil {
		return fmt.Errorf("marshal excludes: %w", err)
	}
	res, err := r.db.ExecContext(ctx, `
UPDATE telegram_watch_rules
SET channel_id = ?, enabled = ?, includes_json = ?, excludes_json = ?, updated_at = ?
WHERE id = ?`,
		rule.ChannelID, boolInt(rule.Enabled), string(includes), string(excludes), time.Now().UTC(), rule.ID,
	)
	if err != nil {
		if isWatchRuleUniqueConstraint(err) {
			return ErrDuplicateWatchRule
		}
		return fmt.Errorf("update watch rule: %w", err)
	}
	return requireRows(res, "watch rule not found")
}

func (r *WatchRuleRepository) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM telegram_watch_rules WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete watch rule: %w", err)
	}
	return requireRows(res, "watch rule not found")
}

func (r *WatchRuleRepository) FindByID(ctx context.Context, id int64) (model.WatchRule, error) {
	return scanWatchRule(r.db.QueryRowContext(ctx, `
SELECT id, channel_id, enabled, includes_json, excludes_json, created_at, updated_at
FROM telegram_watch_rules WHERE id = ?`, id))
}

func (r *WatchRuleRepository) FindByChannelID(ctx context.Context, channelID int64) (model.WatchRule, error) {
	return scanWatchRule(r.db.QueryRowContext(ctx, `
SELECT id, channel_id, enabled, includes_json, excludes_json, created_at, updated_at
FROM telegram_watch_rules WHERE channel_id = ?`, channelID))
}

func (r *WatchRuleRepository) FindAll(ctx context.Context) ([]model.WatchRule, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, channel_id, enabled, includes_json, excludes_json, created_at, updated_at
FROM telegram_watch_rules
ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("find watch rules: %w", err)
	}
	defer rows.Close()
	var out []model.WatchRule
	for rows.Next() {
		rule, err := scanWatchRule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rule)
	}
	return out, rows.Err()
}

func scanWatchRule(row interface{ Scan(...any) error }) (model.WatchRule, error) {
	var rule model.WatchRule
	var enabled int
	var includesRaw string
	var excludesRaw string
	if err := row.Scan(&rule.ID, &rule.ChannelID, &enabled, &includesRaw, &excludesRaw, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
		return model.WatchRule{}, err
	}
	rule.Enabled = enabled != 0
	if err := json.Unmarshal([]byte(includesRaw), &rule.Includes); err != nil {
		return model.WatchRule{}, fmt.Errorf("unmarshal includes: %w", err)
	}
	if err := json.Unmarshal([]byte(excludesRaw), &rule.Excludes); err != nil {
		return model.WatchRule{}, fmt.Errorf("unmarshal excludes: %w", err)
	}
	return rule, nil
}

func normalizeTerms(in []string) []string {
	out := make([]string, 0, len(in))
	for _, term := range in {
		trimmed := strings.TrimSpace(term)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func isWatchRuleUniqueConstraint(err error) bool {
	text := err.Error()
	return strings.Contains(text, "UNIQUE constraint failed: telegram_watch_rules.channel_id") ||
		strings.Contains(text, "constraint failed: UNIQUE constraint failed: telegram_watch_rules.channel_id")
}
