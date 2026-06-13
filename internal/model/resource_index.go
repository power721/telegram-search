package model

import "time"

type ResourceIndexItem struct {
	ID                 int64
	ResourceID         string
	Kind               string
	SourceKey          string
	SourceMessageID    int64
	URL                string
	Type               string
	Category           string
	Password           string
	Note               string
	Title              string
	SourceSnippet      string
	TelegramFileID     int64
	FileName           string
	Extension          string
	MimeType           string
	SizeBytes          int64
	MediaTitle         string
	MediaYear          string
	MediaSeason        string
	MediaEpisode       string
	MediaQuality       string
	MediaSize          string
	MediaTMDBID        string
	MediaCategory      string
	MediaTags          string
	MediaSummary       string
	Datetime           time.Time
	AccountID          int64
	ChannelID          int64
	TelegramChannelID  int64
	ChannelTitle       string
	ChannelUsername    string
	TelegramMessageID  int64
	MessageType        string
	SourceChannelCount int
	MessageCount       int
	ProviderCount      int
	Score              int
	UpdatedAt          time.Time
}

type ResourceIndexQuery struct {
	Keyword    string
	Type       string
	Types      []string
	Category   string
	Categories []string
	AccountID  int64
	ChannelID  int64
	Extension  string
	Sort       string
	DateFrom   *time.Time
	DateTo     *time.Time
	Limit      int
	Offset     int
	MaxLimit   int
}

type ResourceIndexListResult struct {
	Items   []ResourceIndexItem
	Total   int
	Grouped map[string]int
}

type ResourceIndexStats struct {
	IndexedRows int
	UpdatedAt   time.Time
}
