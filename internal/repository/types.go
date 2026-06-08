package repository

import (
	"context"
	"database/sql"
	"time"
)

type executor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type SearchParams struct {
	Query      string
	AccountID  int64
	ChannelID  int64
	LinkType   string
	Sort       string
	DateFrom   *time.Time
	DateTo     *time.Time
	BeforeDate *time.Time
	BeforeID   int64
	Limit      int
	Offset     int
}

type LatestParams struct {
	AccountID  int64
	ChannelID  int64
	BeforeDate *time.Time
	BeforeID   int64
	Limit      int
}

type LinkSearchParams struct {
	Type      string
	Category  string
	AccountID int64
	ChannelID int64
	Keyword   string
	Sort      string
	DateFrom  *time.Time
	DateTo    *time.Time
	Limit     int
	Offset    int
}

type FileSearchParams struct {
	Query     string
	Category  string
	Extension string
	AccountID int64
	ChannelID int64
	Sort      string
	DateFrom  *time.Time
	DateTo    *time.Time
	Limit     int
	Offset    int
}

type MergedLinkSearchParams struct {
	Type      string
	AccountID int64
	ChannelID int64
	Keyword   string
	DateFrom  *time.Time
	DateTo    *time.Time
	Limit     int
	Offset    int
}

func clampLimit(limit int, fallback int) int {
	if limit <= 0 {
		return fallback
	}
	if limit > 200 {
		return 200
	}
	return limit
}

func dateOrderBy(sort string, dateColumn string, idColumn string) string {
	if sort == "date_asc" {
		return dateColumn + " ASC, " + idColumn + " ASC"
	}
	return dateColumn + " DESC, " + idColumn + " DESC"
}
