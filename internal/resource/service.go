package resource

import (
	"context"
	"sort"
	"strings"
	"time"

	"tg-search/internal/repository"
)

type Query struct {
	Keyword   string
	Type      string
	Category  string
	AccountID int64
	ChannelID int64
	Extension string
	Sort      string
	Limit     int
	Offset    int
}

type Item struct {
	ID                string    `json:"id"`
	Kind              string    `json:"kind"`
	Type              string    `json:"type,omitempty"`
	Category          string    `json:"category"`
	URL               string    `json:"url,omitempty"`
	FileName          string    `json:"file_name,omitempty"`
	Extension         string    `json:"extension,omitempty"`
	MimeType          string    `json:"mime_type,omitempty"`
	SizeBytes         int64     `json:"size_bytes,omitempty"`
	Note              string    `json:"note,omitempty"`
	Title             string    `json:"title,omitempty"`
	SourceSnippet     string    `json:"source_snippet,omitempty"`
	Datetime          time.Time `json:"datetime"`
	ChannelID         int64     `json:"channel_id"`
	ChannelTitle      string    `json:"channel_title"`
	TelegramMessageID int64     `json:"telegram_message_id"`
}

type ListResult struct {
	Items   []Item         `json:"items"`
	Total   int            `json:"total"`
	Grouped map[string]int `json:"grouped"`
}

type Service struct {
	links *repository.LinkRepository
	files *repository.FileRepository
	stats *repository.ResourceStatsRepository
}

func NewService(links *repository.LinkRepository, files *repository.FileRepository, stats ...*repository.ResourceStatsRepository) *Service {
	service := &Service{links: links, files: files}
	if len(stats) > 0 {
		service.stats = stats[0]
	}
	return service
}

func (s *Service) List(ctx context.Context, query Query) (ListResult, error) {
	items := []Item{}
	if s.links != nil && includeLinks(query) {
		links, err := s.links.Search(ctx, repository.LinkSearchParams{
			Type:      query.Type,
			Category:  query.Category,
			AccountID: query.AccountID,
			ChannelID: query.ChannelID,
			Keyword:   query.Keyword,
			Limit:     200,
		})
		if err != nil {
			return ListResult{}, err
		}
		byURL := map[string]Item{}
		for _, link := range links {
			category := link.Category
			if category == "" {
				category = link.Type
			}
			item := Item{
				ID:                "link:" + link.URL,
				Kind:              "link",
				Type:              link.Type,
				Category:          category,
				URL:               link.URL,
				Note:              link.Note,
				Title:             firstNonEmpty(link.Note, link.URL),
				SourceSnippet:     link.SourceSnippet,
				Datetime:          link.MessageDate,
				ChannelID:         link.ChannelID,
				ChannelTitle:      link.ChannelTitle,
				TelegramMessageID: link.TelegramMessageID,
			}
			existing, ok := byURL[link.URL]
			if !ok || item.Datetime.After(existing.Datetime) {
				byURL[link.URL] = item
			}
		}
		for _, item := range byURL {
			items = append(items, item)
		}
	}
	if s.files != nil && includeFiles(query) {
		files, err := s.files.Search(ctx, repository.FileSearchParams{
			Query:     query.Keyword,
			Category:  fileCategoryFilter(query),
			Extension: query.Extension,
			AccountID: query.AccountID,
			ChannelID: query.ChannelID,
			Limit:     200,
		})
		if err != nil {
			return ListResult{}, err
		}
		for _, file := range files {
			items = append(items, Item{
				ID:                "file:" + file.FileName + ":" + file.MessageDate.Format(time.RFC3339Nano),
				Kind:              "file",
				Type:              "files",
				Category:          "files",
				FileName:          file.FileName,
				Extension:         file.Extension,
				MimeType:          file.MimeType,
				SizeBytes:         file.SizeBytes,
				Title:             file.FileName,
				Datetime:          file.MessageDate,
				ChannelID:         file.ChannelID,
				ChannelTitle:      file.ChannelTitle,
				TelegramMessageID: file.TelegramMessageID,
			})
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if !items[i].Datetime.Equal(items[j].Datetime) {
			return items[i].Datetime.After(items[j].Datetime)
		}
		return resourceKindRank(items[i].Kind) < resourceKindRank(items[j].Kind)
	})

	grouped, err := s.groupedForQuery(ctx, query)
	if err != nil {
		return ListResult{}, err
	}
	total := len(items)
	offset := query.Offset
	if offset > total {
		offset = total
	}
	limit := query.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return ListResult{Items: items[offset:end], Total: total, Grouped: grouped}, nil
}

func (s *Service) groupedForQuery(ctx context.Context, query Query) (map[string]int, error) {
	grouped := defaultGrouped()
	if s.links != nil && includeLinks(query) {
		linkCounts, err := s.links.CountByResourceCategory(ctx, repository.LinkSearchParams{
			Type:      query.Type,
			Category:  query.Category,
			AccountID: query.AccountID,
			ChannelID: query.ChannelID,
			Keyword:   query.Keyword,
		})
		if err != nil {
			return nil, err
		}
		for category, count := range linkCounts {
			grouped[category] += count
		}
	}
	if s.files != nil && includeFiles(query) {
		fileCount, err := s.files.CountResources(ctx, repository.FileSearchParams{
			Query:     query.Keyword,
			Category:  fileCategoryFilter(query),
			Extension: query.Extension,
			AccountID: query.AccountID,
			ChannelID: query.ChannelID,
		})
		if err != nil {
			return nil, err
		}
		grouped["files"] = fileCount
	}
	return grouped, nil
}

func (s *Service) GlobalGrouped(ctx context.Context) (map[string]int, error) {
	if s.stats == nil {
		return s.computeGlobalGrouped(ctx)
	}
	grouped, found, err := s.stats.GetGrouped(ctx)
	if err != nil {
		return nil, err
	}
	if found {
		return normalizeGrouped(grouped), nil
	}
	if err := s.RefreshGlobalGrouped(ctx); err != nil {
		return nil, err
	}
	grouped, _, err = s.stats.GetGrouped(ctx)
	if err != nil {
		return nil, err
	}
	return normalizeGrouped(grouped), nil
}

func (s *Service) RefreshGlobalGrouped(ctx context.Context) error {
	if s.stats == nil {
		return nil
	}
	grouped, err := s.computeGlobalGrouped(ctx)
	if err != nil {
		return err
	}
	return s.stats.SaveGrouped(ctx, grouped)
}

func (s *Service) computeGlobalGrouped(ctx context.Context) (map[string]int, error) {
	grouped := defaultGrouped()
	if s.links != nil {
		linkCounts, err := s.links.CountByResourceCategory(ctx, repository.LinkSearchParams{})
		if err != nil {
			return nil, err
		}
		for category, count := range linkCounts {
			grouped[category] += count
		}
	}
	if s.files != nil {
		fileCount, err := s.files.CountResources(ctx, repository.FileSearchParams{})
		if err != nil {
			return nil, err
		}
		grouped["files"] = fileCount
	}
	return grouped, nil
}

func normalizeGrouped(grouped map[string]int) map[string]int {
	out := defaultGrouped()
	for category, count := range grouped {
		out[category] = count
	}
	return out
}

func defaultGrouped() map[string]int {
	return map[string]int{"cloud_drive": 0, "magnet": 0, "ed2k": 0, "http": 0, "files": 0}
}

func includeLinks(query Query) bool {
	return query.Type != "files" && query.Category != "files"
}

func includeFiles(query Query) bool {
	return (query.Type == "" || query.Type == "files") && (query.Category == "" || query.Category == "files")
}

func fileCategoryFilter(query Query) string {
	if query.Category == "files" {
		return ""
	}
	return query.Category
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func resourceKindRank(kind string) int {
	if kind == "link" {
		return 0
	}
	return 1
}
