package search

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"tg-search/internal/model"
	"tg-search/internal/repository"
	"tg-search/internal/session"
	taskpkg "tg-search/internal/task"
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
	Logger   *zap.Logger
}

type RemoteService struct {
	accounts *repository.AccountRepository
	channels *repository.ChannelRepository
	tasks    *repository.RemoteSearchTaskRepository
	cursors  *repository.SyncCursorRepository
	telegram telegram.Client
	sessions *session.Manager
	ttl      time.Duration
	logger   *zap.Logger
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
	if opts.Logger == nil {
		opts.Logger = zap.NewNop()
	}
	return &RemoteService{
		accounts: opts.Accounts,
		channels: opts.Channels,
		tasks:    opts.Tasks,
		cursors:  opts.Cursors,
		telegram: opts.Telegram,
		sessions: opts.Sessions,
		ttl:      opts.TTL,
		logger:   opts.Logger,
		results:  map[int64][]model.RemoteSearchItem{},
	}
}

func (s *RemoteService) Search(ctx context.Context, channelID int64, query string, limit int) (model.RemoteSearchTask, error) {
	return s.SearchWithProgress(ctx, channelID, query, limit, nil)
}

func (s *RemoteService) SearchWithProgress(ctx context.Context, channelID int64, query string, limit int, progress taskpkg.ProgressSink) (model.RemoteSearchTask, error) {
	started := time.Now()
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
		s.logger.Info("remote search rejected for synced channel", zap.Int64("channel_id", channelID), zap.Bool("has_last_sync_time", channel.LastSyncTime != nil), zap.Int64("last_message_id", channel.LastMessageID))
		return model.RemoteSearchTask{}, ErrRemoteSearchRequiresUnsynced
	}
	if s.cursors != nil {
		_, err := s.cursors.Find(ctx, channel.AccountID, channel.ID, "history")
		if err == nil {
			s.logger.Info("remote search rejected for channel with history cursor", zap.Int64("channel_id", channel.ID), zap.Int64("account_id", channel.AccountID))
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
	if err := checkRemoteProgressStatus(ctx, progress); err != nil {
		return model.RemoteSearchTask{}, err
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
	s.logger.Info("remote search started",
		zap.Int64("task_id", task.ID),
		zap.Int64("account_id", account.ID),
		zap.Int64("channel_id", channel.ID),
		zap.Int("query_length", len(query)),
		zap.Int("limit", limit),
	)

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
		s.logger.Error("remote search failed",
			zap.Int64("task_id", task.ID),
			zap.Int64("account_id", account.ID),
			zap.Int64("channel_id", channel.ID),
			zap.Duration("duration", time.Since(started)),
			zap.Error(err),
		)
		return model.RemoteSearchTask{}, fmt.Errorf("remote telegram search: %w", err)
	}

	results := make([]model.RemoteSearchItem, 0, len(items))
	for _, item := range items {
		results = append(results, model.RemoteSearchItem{
			Source:            "remote",
			AccountID:         account.ID,
			ChannelID:         channel.ID,
			TelegramChannelID: channel.TelegramChannelID,
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
	if progress != nil {
		if err := progress.Progress(ctx, int64(len(results)), int64(limit), "remote search completed"); err != nil {
			return model.RemoteSearchTask{}, err
		}
	}
	s.logger.Info("remote search completed",
		zap.Int64("task_id", task.ID),
		zap.Int64("account_id", account.ID),
		zap.Int64("channel_id", channel.ID),
		zap.Int("results", len(results)),
		zap.Duration("duration", time.Since(started)),
	)
	return task, nil
}

func checkRemoteProgressStatus(ctx context.Context, progress taskpkg.ProgressSink) error {
	if progress == nil {
		return nil
	}
	status, err := progress.Status(ctx)
	if err != nil {
		return err
	}
	if taskpkg.IsCancelingStatus(status) {
		return context.Canceled
	}
	return nil
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
