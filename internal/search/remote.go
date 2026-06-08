package search

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"tg-search/internal/model"
	"tg-search/internal/repository"
	"tg-search/internal/session"
	"tg-search/internal/telegram"
)

var ErrRemoteSearchNotAllowed = errors.New("remote search is not allowed for this channel")
var ErrRemoteSearchRequiresUnsynced = errors.New("remote search requires an unsynced channel")

type RemoteOptions struct {
	Accounts *repository.AccountRepository
	Channels *repository.ChannelRepository
	Tasks    *repository.RemoteSearchTaskRepository
	Cursors  *repository.SyncCursorRepository
	Telegram telegram.Client
	Sessions *session.Manager
	TTL      time.Duration
}

type RemoteService struct {
	accounts *repository.AccountRepository
	channels *repository.ChannelRepository
	tasks    *repository.RemoteSearchTaskRepository
	cursors  *repository.SyncCursorRepository
	telegram telegram.Client
	sessions *session.Manager
	ttl      time.Duration
	mu       sync.Mutex
	results  map[int64][]model.RemoteSearchItem
}

func NewRemoteService(opts RemoteOptions) *RemoteService {
	if opts.Telegram == nil {
		opts.Telegram = telegram.NopClient{}
	}
	if opts.TTL <= 0 {
		opts.TTL = 30 * time.Minute
	}
	return &RemoteService{
		accounts: opts.Accounts,
		channels: opts.Channels,
		tasks:    opts.Tasks,
		cursors:  opts.Cursors,
		telegram: opts.Telegram,
		sessions: opts.Sessions,
		ttl:      opts.TTL,
		results:  map[int64][]model.RemoteSearchItem{},
	}
}

func (s *RemoteService) Search(ctx context.Context, channelID int64, query string, limit int) (model.RemoteSearchTask, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return model.RemoteSearchTask{}, ErrEmptyQuery
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	channel, err := s.channels.FindByID(ctx, channelID)
	if err != nil {
		return model.RemoteSearchTask{}, fmt.Errorf("load channel: %w", err)
	}
	if !channel.RemoteSearchAllowed {
		return model.RemoteSearchTask{}, ErrRemoteSearchNotAllowed
	}
	if channel.LastMessageID > 0 || channel.LastSyncTime != nil {
		return model.RemoteSearchTask{}, ErrRemoteSearchRequiresUnsynced
	}
	if s.cursors != nil {
		_, err := s.cursors.Find(ctx, channel.AccountID, channel.ID, "history")
		if err == nil {
			return model.RemoteSearchTask{}, ErrRemoteSearchRequiresUnsynced
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return model.RemoteSearchTask{}, fmt.Errorf("load sync cursor: %w", err)
		}
	}
	account, err := s.accounts.FindByID(ctx, channel.AccountID)
	if err != nil {
		return model.RemoteSearchTask{}, fmt.Errorf("load account: %w", err)
	}
	sessionPath := ""
	if s.sessions != nil {
		sessionPath = s.sessions.PathForAccount(account.ID)
	}
	task := model.RemoteSearchTask{
		AccountID: channel.AccountID,
		ChannelID: channel.ID,
		Query:     query,
		Status:    model.RemoteSearchStatusQueued,
		Source:    "remote",
		ExpiresAt: time.Now().UTC().Add(s.ttl),
	}
	id, err := s.tasks.Create(ctx, task)
	if err != nil {
		return model.RemoteSearchTask{}, err
	}
	task, err = s.tasks.FindByID(ctx, id)
	if err != nil {
		return model.RemoteSearchTask{}, err
	}

	items, err := s.telegram.SearchMessages(ctx, telegram.AccountSession{
		AccountID:   account.ID,
		Phone:       account.Phone,
		SessionPath: sessionPath,
	}, telegram.ChannelRef{
		TelegramChannelID: channel.TelegramChannelID,
		AccessHash:        channel.AccessHash,
		Type:              channel.Type,
	}, query, limit)
	if err != nil {
		return model.RemoteSearchTask{}, fmt.Errorf("remote telegram search: %w", err)
	}

	results := make([]model.RemoteSearchItem, 0, len(items))
	for _, item := range items {
		results = append(results, model.RemoteSearchItem{
			Source:            "remote",
			AccountID:         account.ID,
			ChannelID:         channel.ID,
			ChannelTitle:      channel.Title,
			ChannelUsername:   channel.Username,
			TelegramMessageID: item.TelegramMessageID,
			SenderID:          item.SenderID,
			Text:              item.Text,
			RawJSON:           item.RawJSON,
			Date:              item.Date,
			EditDate:          item.EditDate,
		})
	}
	s.mu.Lock()
	s.results[task.ID] = results
	s.mu.Unlock()
	return task, nil
}

func (s *RemoteService) Results(ctx context.Context, taskID int64) (model.RemoteSearchResults, error) {
	task, err := s.tasks.FindByID(ctx, taskID)
	if err != nil {
		return model.RemoteSearchResults{}, err
	}
	if time.Now().UTC().After(task.ExpiresAt) {
		return model.RemoteSearchResults{Task: task, Items: []model.RemoteSearchItem{}}, nil
	}
	s.mu.Lock()
	items := append([]model.RemoteSearchItem(nil), s.results[taskID]...)
	s.mu.Unlock()
	if items == nil {
		items = []model.RemoteSearchItem{}
	}
	return model.RemoteSearchResults{Task: task, Items: items}, nil
}
