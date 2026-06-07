package search

import (
	"context"
	"errors"
	"strings"

	"tg-provider/internal/model"
	"tg-provider/internal/repository"
)

var ErrEmptyQuery = errors.New("search query is required")

type Params struct {
	Query     string
	AccountID int64
	ChannelID int64
	LinkType  string
	Limit     int
	Offset    int
}

type LatestParams struct {
	AccountID int64
	ChannelID int64
	Limit     int
}

type LinkParams struct {
	Type      string
	AccountID int64
	ChannelID int64
	Keyword   string
	Limit     int
	Offset    int
}

type Service struct {
	messages *repository.MessageRepository
	links    *repository.LinkRepository
}

func NewService(messages *repository.MessageRepository, links *repository.LinkRepository) *Service {
	return &Service{messages: messages, links: links}
}

func (s *Service) Search(ctx context.Context, params Params) ([]model.SearchResult, error) {
	query := strings.TrimSpace(params.Query)
	if query == "" {
		return nil, ErrEmptyQuery
	}
	return s.messages.Search(ctx, repository.SearchParams{
		Query:     query,
		AccountID: params.AccountID,
		ChannelID: params.ChannelID,
		LinkType:  params.LinkType,
		Limit:     params.Limit,
		Offset:    params.Offset,
	})
}

func (s *Service) Latest(ctx context.Context, params LatestParams) ([]model.SearchResult, error) {
	return s.messages.Latest(ctx, repository.LatestParams{
		AccountID: params.AccountID,
		ChannelID: params.ChannelID,
		Limit:     params.Limit,
	})
}

func (s *Service) Links(ctx context.Context, params LinkParams) ([]model.LinkResult, error) {
	return s.links.Search(ctx, repository.LinkSearchParams{
		Type:      params.Type,
		AccountID: params.AccountID,
		ChannelID: params.ChannelID,
		Keyword:   params.Keyword,
		Limit:     params.Limit,
		Offset:    params.Offset,
	})
}
