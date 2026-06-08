package search

import (
	"context"
	"errors"
	"strings"
	"time"

	"tg-search/internal/model"
	"tg-search/internal/repository"
)

var ErrEmptyQuery = errors.New("search query is required")

type Params struct {
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

type LinkParams struct {
	Type      string
	AccountID int64
	ChannelID int64
	Keyword   string
	Sort      string
	DateFrom  *time.Time
	DateTo    *time.Time
	Limit     int
	Offset    int
}

type SearchQuery struct {
	Query       string
	AccountID   int64
	ChannelID   int64
	MessageType string
	LinkType    string
	FileType    string
	Sort        string
	DateFrom    *time.Time
	DateTo      *time.Time
	Limit       int
	Offset      int
}

type Service struct {
	messages *repository.MessageRepository
	links    *repository.LinkRepository
	files    *repository.FileRepository
	channels *repository.ChannelRepository
}

func NewService(messages *repository.MessageRepository, links *repository.LinkRepository, extras ...any) *Service {
	service := &Service{messages: messages, links: links}
	for _, extra := range extras {
		switch repo := extra.(type) {
		case *repository.FileRepository:
			service.files = repo
		case *repository.ChannelRepository:
			service.channels = repo
		}
	}
	return service
}

func (s *Service) Global(ctx context.Context, query SearchQuery) (model.GlobalSearchResult, error) {
	messages, err := s.Messages(ctx, query)
	if err != nil {
		return model.GlobalSearchResult{}, err
	}
	links, err := s.ScopedLinks(ctx, query)
	if err != nil {
		return model.GlobalSearchResult{}, err
	}
	files, err := s.Files(ctx, query)
	if err != nil {
		return model.GlobalSearchResult{}, err
	}
	channels, err := s.Channels(ctx, query)
	if err != nil {
		return model.GlobalSearchResult{}, err
	}
	return model.GlobalSearchResult{
		Messages: messages,
		Links:    links,
		Files:    files,
		Channels: channels,
	}, nil
}

func (s *Service) Messages(ctx context.Context, query SearchQuery) (model.ListResult[model.SearchResult], error) {
	items, err := s.Search(ctx, Params{
		Query:     query.Query,
		AccountID: query.AccountID,
		ChannelID: query.ChannelID,
		LinkType:  query.LinkType,
		Sort:      query.Sort,
		DateFrom:  query.DateFrom,
		DateTo:    query.DateTo,
		Limit:     query.Limit,
		Offset:    query.Offset,
	})
	if err != nil {
		return model.ListResult[model.SearchResult]{}, err
	}
	total, err := s.messages.CountSearch(ctx, repository.SearchParams{
		Query:     strings.TrimSpace(query.Query),
		AccountID: query.AccountID,
		ChannelID: query.ChannelID,
		LinkType:  query.LinkType,
		DateFrom:  query.DateFrom,
		DateTo:    query.DateTo,
	})
	if err != nil {
		return model.ListResult[model.SearchResult]{}, err
	}
	return model.ListResult[model.SearchResult]{Items: items, Total: total}, nil
}

func (s *Service) ScopedLinks(ctx context.Context, query SearchQuery) (model.ListResult[model.LinkResult], error) {
	keyword := strings.TrimSpace(query.Query)
	if keyword == "" {
		return model.ListResult[model.LinkResult]{}, ErrEmptyQuery
	}
	items, err := s.links.Search(ctx, repository.LinkSearchParams{
		Type:      query.LinkType,
		AccountID: query.AccountID,
		ChannelID: query.ChannelID,
		Keyword:   keyword,
		Sort:      query.Sort,
		DateFrom:  query.DateFrom,
		DateTo:    query.DateTo,
		Limit:     query.Limit,
		Offset:    query.Offset,
	})
	if err != nil {
		return model.ListResult[model.LinkResult]{}, err
	}
	total, err := s.links.CountSearch(ctx, repository.LinkSearchParams{
		Type:      query.LinkType,
		AccountID: query.AccountID,
		ChannelID: query.ChannelID,
		Keyword:   keyword,
		DateFrom:  query.DateFrom,
		DateTo:    query.DateTo,
	})
	if err != nil {
		return model.ListResult[model.LinkResult]{}, err
	}
	return model.ListResult[model.LinkResult]{Items: items, Total: total}, nil
}

func (s *Service) Files(ctx context.Context, query SearchQuery) (model.ListResult[model.FileResult], error) {
	keyword := strings.TrimSpace(query.Query)
	if keyword == "" {
		return model.ListResult[model.FileResult]{}, ErrEmptyQuery
	}
	if s.files == nil {
		return model.ListResult[model.FileResult]{Items: []model.FileResult{}, Total: 0}, nil
	}
	items, err := s.files.Search(ctx, repository.FileSearchParams{
		Query:     keyword,
		Category:  query.FileType,
		AccountID: query.AccountID,
		ChannelID: query.ChannelID,
		Sort:      query.Sort,
		DateFrom:  query.DateFrom,
		DateTo:    query.DateTo,
		Limit:     query.Limit,
		Offset:    query.Offset,
	})
	if err != nil {
		return model.ListResult[model.FileResult]{}, err
	}
	total, err := s.files.CountSearch(ctx, repository.FileSearchParams{
		Query:     keyword,
		Category:  query.FileType,
		AccountID: query.AccountID,
		ChannelID: query.ChannelID,
		DateFrom:  query.DateFrom,
		DateTo:    query.DateTo,
	})
	if err != nil {
		return model.ListResult[model.FileResult]{}, err
	}
	return model.ListResult[model.FileResult]{Items: items, Total: total}, nil
}

func (s *Service) Channels(ctx context.Context, query SearchQuery) (model.ListResult[model.ChannelSearchResult], error) {
	keyword := strings.TrimSpace(query.Query)
	if keyword == "" {
		return model.ListResult[model.ChannelSearchResult]{}, ErrEmptyQuery
	}
	if s.channels == nil {
		return model.ListResult[model.ChannelSearchResult]{Items: []model.ChannelSearchResult{}, Total: 0}, nil
	}
	channels, err := s.channels.FindAll(ctx)
	if err != nil {
		return model.ListResult[model.ChannelSearchResult]{}, err
	}
	needle := strings.ToLower(keyword)
	filtered := make([]model.ChannelSearchResult, 0, len(channels))
	for _, channel := range channels {
		if query.AccountID > 0 && channel.AccountID != query.AccountID {
			continue
		}
		if query.ChannelID > 0 && channel.ID != query.ChannelID {
			continue
		}
		haystack := strings.ToLower(channel.Title + " " + channel.Username + " " + channel.Description)
		if !strings.Contains(haystack, needle) {
			continue
		}
		filtered = append(filtered, model.ChannelSearchResult{Channel: channel})
	}
	total := len(filtered)
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
	return model.ListResult[model.ChannelSearchResult]{Items: filtered[offset:end], Total: total}, nil
}

func (s *Service) Search(ctx context.Context, params Params) ([]model.SearchResult, error) {
	query := strings.TrimSpace(params.Query)
	if query == "" {
		return nil, ErrEmptyQuery
	}
	return s.messages.Search(ctx, repository.SearchParams{
		Query:      query,
		AccountID:  params.AccountID,
		ChannelID:  params.ChannelID,
		LinkType:   params.LinkType,
		Sort:       params.Sort,
		DateFrom:   params.DateFrom,
		DateTo:     params.DateTo,
		BeforeDate: params.BeforeDate,
		BeforeID:   params.BeforeID,
		Limit:      params.Limit,
		Offset:     params.Offset,
	})
}

func (s *Service) Latest(ctx context.Context, params LatestParams) ([]model.SearchResult, error) {
	return s.messages.Latest(ctx, repository.LatestParams{
		AccountID:  params.AccountID,
		ChannelID:  params.ChannelID,
		BeforeDate: params.BeforeDate,
		BeforeID:   params.BeforeID,
		Limit:      params.Limit,
	})
}

func (s *Service) Links(ctx context.Context, params LinkParams) ([]model.LinkResult, error) {
	return s.links.Search(ctx, repository.LinkSearchParams{
		Type:      params.Type,
		AccountID: params.AccountID,
		ChannelID: params.ChannelID,
		Keyword:   params.Keyword,
		Sort:      params.Sort,
		DateFrom:  params.DateFrom,
		DateTo:    params.DateTo,
		Limit:     params.Limit,
		Offset:    params.Offset,
	})
}

func (s *Service) MergedLinks(ctx context.Context, params LinkParams) (model.MergedLinksResponse, error) {
	return s.links.SearchMerged(ctx, repository.MergedLinkSearchParams{
		Type:      params.Type,
		AccountID: params.AccountID,
		ChannelID: params.ChannelID,
		Keyword:   params.Keyword,
		DateFrom:  params.DateFrom,
		DateTo:    params.DateTo,
		Limit:     params.Limit,
		Offset:    params.Offset,
	})
}
