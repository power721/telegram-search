package history

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	channelpkg "tg-search/internal/channel"
	dbpkg "tg-search/internal/db"
	"tg-search/internal/link"
	"tg-search/internal/messagefilter"
	"tg-search/internal/model"
	"tg-search/internal/repository"
	"tg-search/internal/resource"
	"tg-search/internal/retry"
	"tg-search/internal/session"
	taskpkg "tg-search/internal/task"
	"tg-search/internal/telegram"
)

type Options struct {
	DB               *sql.DB
	Accounts         *repository.AccountRepository
	Channels         *repository.ChannelRepository
	Messages         *repository.MessageRepository
	Links            *repository.LinkRepository
	Resources        *resource.Service
	Cursors          *repository.SyncCursorRepository
	Telegram         telegram.Client
	Sessions         *session.Manager
	Extractor        *link.Extractor
	Filter           *messagefilter.Filter
	HistoryBatchSize int
	Workers          int
	RetryPolicy      retry.Policy
	Logger           *zap.Logger
}

type Service struct {
	db               *sql.DB
	accounts         *repository.AccountRepository
	channels         *repository.ChannelRepository
	messages         *repository.MessageRepository
	links            *repository.LinkRepository
	resources        *resource.Service
	cursors          *repository.SyncCursorRepository
	telegram         telegram.Client
	sessions         *session.Manager
	extractor        *link.Extractor
	filter           *messagefilter.Filter
	historyBatchSize int
	workers          int
	retryPolicy      retry.Policy
	logger           *zap.Logger
	mu               sync.Mutex
	runningChannels  map[int64]struct{}
}

type SyncResult struct {
	Messages int `json:"messages"`
	Links    int `json:"links"`
}

type SyncManyResult struct {
	Queued   int                  `json:"queued"`
	Skipped  int                  `json:"skipped"`
	Results  map[int64]SyncResult `json:"results"`
	Failures map[int64]string     `json:"failures"`
}

var ErrChannelSyncInProgress = errors.New("channel sync already in progress")
var ErrTaskPaused = errors.New("task is paused")

func NewService(opts Options) *Service {
	if opts.Telegram == nil {
		opts.Telegram = telegram.NopClient{}
	}
	if opts.Extractor == nil {
		opts.Extractor = link.NewExtractor()
	}
	if opts.HistoryBatchSize <= 0 {
		opts.HistoryBatchSize = 100
	}
	if opts.Workers <= 0 {
		opts.Workers = 1
	}
	if opts.RetryPolicy.MaxTries == 0 && opts.RetryPolicy.BaseDelay == 0 && opts.RetryPolicy.MaxDelay == 0 && opts.RetryPolicy.Sleep == nil {
		opts.RetryPolicy = retry.DefaultPolicy()
	}
	if opts.Cursors == nil && opts.DB != nil {
		opts.Cursors = repository.NewSyncCursorRepository(opts.DB)
	}
	if opts.Logger == nil {
		opts.Logger = zap.NewNop()
	}
	return &Service{
		db:               opts.DB,
		accounts:         opts.Accounts,
		channels:         opts.Channels,
		messages:         opts.Messages,
		links:            opts.Links,
		resources:        opts.Resources,
		cursors:          opts.Cursors,
		telegram:         opts.Telegram,
		sessions:         opts.Sessions,
		extractor:        opts.Extractor,
		filter:           opts.Filter,
		historyBatchSize: opts.HistoryBatchSize,
		workers:          opts.Workers,
		retryPolicy:      opts.RetryPolicy,
		logger:           opts.Logger,
		runningChannels:  map[int64]struct{}{},
	}
}

func (s *Service) SyncChannel(ctx context.Context, channelID int64) (SyncResult, error) {
	return s.SyncChannelWithProfile(ctx, channelID, "")
}

func (s *Service) SyncChannelWithProfile(ctx context.Context, channelID int64, profile string) (SyncResult, error) {
	return s.syncChannel(ctx, channelID, profile, 0, nil)
}

func (s *Service) SyncChannelWithMaxMessages(ctx context.Context, channelID int64, maxMessages int) (SyncResult, error) {
	return s.syncChannel(ctx, channelID, "", maxMessages, nil)
}

func (s *Service) SyncChannelWithProgress(ctx context.Context, channelID int64, profile string, progress taskpkg.ProgressSink) (SyncResult, error) {
	return s.syncChannel(ctx, channelID, profile, 0, progress)
}

func (s *Service) syncChannel(ctx context.Context, channelID int64, profile string, maxMessages int, progress taskpkg.ProgressSink) (SyncResult, error) {
	if !s.tryAcquireChannel(channelID) {
		return SyncResult{}, ErrChannelSyncInProgress
	}
	defer s.releaseChannel(channelID)
	return s.syncChannelWithRetry(ctx, channelID, profile, maxMessages, progress)
}

func (s *Service) SyncMany(ctx context.Context, channelIDs []int64) SyncManyResult {
	return s.SyncManyWithMaxMessages(ctx, channelIDs, 0)
}

func (s *Service) SyncManyWithMaxMessages(ctx context.Context, channelIDs []int64, maxMessages int) SyncManyResult {
	started := time.Now()
	result := SyncManyResult{
		Results:  map[int64]SyncResult{},
		Failures: map[int64]string{},
	}
	unique := make([]int64, 0, len(channelIDs))
	seen := map[int64]struct{}{}
	for _, channelID := range channelIDs {
		if channelID <= 0 {
			result.Skipped++
			continue
		}
		if _, ok := seen[channelID]; ok {
			result.Skipped++
			continue
		}
		seen[channelID] = struct{}{}
		unique = append(unique, channelID)
	}
	if len(unique) == 0 {
		s.logger.Info("history sync skipped", zap.Int("requested_channels", len(channelIDs)), zap.Int("skipped", result.Skipped))
		return result
	}

	workers := s.workers
	if workers <= 0 {
		workers = 1
	}
	if workers > len(unique) {
		workers = len(unique)
	}

	jobs := make(chan int64)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for channelID := range jobs {
				if !s.tryAcquireChannel(channelID) {
					s.logger.Info("history sync channel skipped because already running", zap.Int64("channel_id", channelID))
					mu.Lock()
					result.Skipped++
					mu.Unlock()
					continue
				}
				syncResult, err := s.syncChannelWithRetry(ctx, channelID, "", maxMessages, nil)
				s.releaseChannel(channelID)
				mu.Lock()
				if err != nil {
					s.logger.Warn("history sync channel failed", zap.Int64("channel_id", channelID), zap.Error(err))
					result.Failures[channelID] = err.Error()
				} else {
					s.logger.Info("history sync channel completed", zap.Int64("channel_id", channelID), zap.Int("messages", syncResult.Messages), zap.Int("links", syncResult.Links))
					result.Queued++
					result.Results[channelID] = syncResult
				}
				mu.Unlock()
			}
		}()
	}
	for _, channelID := range unique {
		select {
		case <-ctx.Done():
			mu.Lock()
			result.Failures[channelID] = ctx.Err().Error()
			mu.Unlock()
		case jobs <- channelID:
		}
	}
	close(jobs)
	wg.Wait()
	s.logger.Info("history sync many completed",
		zap.Int("requested_channels", len(channelIDs)),
		zap.Int("unique_channels", len(unique)),
		zap.Int("queued", result.Queued),
		zap.Int("skipped", result.Skipped),
		zap.Int("failures", len(result.Failures)),
		zap.Duration("duration", time.Since(started)),
	)
	return result
}

func (s *Service) syncChannelWithRetry(ctx context.Context, channelID int64, profile string, maxMessages int, progress taskpkg.ProgressSink) (SyncResult, error) {
	started := time.Now()
	s.logger.Info("history sync channel started", zap.Int64("channel_id", channelID), zap.String("profile", profile))
	var result SyncResult
	err := s.retryPolicy.Run(ctx, func() error {
		next, err := s.syncChannelOnce(ctx, channelID, profile, maxMessages, progress)
		result = next
		return err
	}, func(ctx context.Context, attempt retry.Attempt) {
		s.logger.Warn("history sync retry scheduled",
			zap.Int64("channel_id", channelID),
			zap.Int("attempt", attempt.Number),
			zap.Duration("delay", attempt.Delay),
			zap.String("classification", string(attempt.Classification.Kind)),
			zap.Error(attempt.Classification.Err),
		)
		if attempt.Classification.Kind == retry.KindFloodWait {
			s.markChannelAccountStatus(ctx, channelID, model.AccountStatusFloodWait)
			if progress != nil {
				if sink, ok := progress.(taskpkg.FloodWaitSink); ok {
					_ = sink.FloodWait(ctx, time.Now().UTC().Add(attempt.Delay), attempt.Classification.Err.Error())
				}
			}
		}
	})
	if err != nil {
		s.logger.Error("history sync channel failed",
			zap.Int64("channel_id", channelID),
			zap.Int("messages", result.Messages),
			zap.Int("links", result.Links),
			zap.Duration("duration", time.Since(started)),
			zap.Error(err),
		)
		return result, err
	}
	if result.Messages > 0 {
		if err := s.refreshResourceStats(ctx); err != nil {
			s.logger.Error("history sync refresh resource stats failed", zap.Int64("channel_id", channelID), zap.Error(err))
			return result, err
		}
	}
	if err := s.channels.MarkSynced(ctx, channelID, time.Now().UTC()); err != nil {
		s.logger.Error("history sync mark channel synced failed", zap.Int64("channel_id", channelID), zap.Error(err))
		return result, err
	}
	s.logger.Info("history sync channel completed",
		zap.Int64("channel_id", channelID),
		zap.Int("messages", result.Messages),
		zap.Int("links", result.Links),
		zap.Duration("duration", time.Since(started)),
	)
	return result, nil
}

func (s *Service) syncChannelOnce(ctx context.Context, channelID int64, requestedProfile string, maxMessages int, progress taskpkg.ProgressSink) (SyncResult, error) {
	channel, err := s.channels.FindByID(ctx, channelID)
	if err != nil {
		return SyncResult{}, fmt.Errorf("load channel: %w", err)
	}
	account, err := s.accounts.FindByID(ctx, channel.AccountID)
	if err != nil {
		return SyncResult{}, fmt.Errorf("load account: %w", err)
	}

	sessionPath := ""
	if s.sessions != nil {
		sessionPath = s.sessions.PathForAccount(account.ID)
	}
	accountSession := telegram.AccountSession{
		AccountID:   account.ID,
		Phone:       account.Phone,
		SessionPath: sessionPath,
	}
	ref := telegram.ChannelRef{
		TelegramChannelID: channel.TelegramChannelID,
		AccessHash:        channel.AccessHash,
		Type:              channel.Type,
	}
	profile := requestedProfile
	if profile == "" {
		profile = channel.SyncProfile
	}
	if profile == "" {
		profile = channelpkg.SyncProfileNormal
	}
	profileLimit, err := channelpkg.ProfileLimit(profile)
	if err != nil {
		return SyncResult{}, err
	}
	if maxMessages > 0 {
		profileLimit = maxMessages
	}

	var result SyncResult
	var maxSeen int64
	offsetID := int64(0)
	for {
		if err := checkTaskStatus(ctx, progress); err != nil {
			return result, err
		}
		fetchLimit := s.historyBatchSize
		if profileLimit > 0 {
			remaining := profileLimit - result.Messages
			if remaining <= 0 {
				break
			}
			if fetchLimit > remaining {
				fetchLimit = remaining
			}
		}
		batch, err := s.telegram.FetchHistory(ctx, accountSession, ref, offsetID, fetchLimit)
		if err != nil {
			return result, fmt.Errorf("fetch history: %w", err)
		}
		if len(batch) == 0 {
			break
		}
		result.Messages += len(batch)
		minID := int64(0)
		modelMessages := make([]model.Message, 0, len(batch))
		for _, item := range batch {
			if item.TelegramMessageID <= 0 {
				continue
			}
			if minID == 0 || item.TelegramMessageID < minID {
				minID = item.TelegramMessageID
			}
			if item.TelegramMessageID > maxSeen {
				maxSeen = item.TelegramMessageID
			}
			modelMessages = append(modelMessages, model.Message{
				AccountID:         account.ID,
				ChannelID:         channel.ID,
				TelegramMessageID: item.TelegramMessageID,
				SenderID:          item.SenderID,
				Text:              item.Text,
				RawJSON:           item.RawJSON,
				Date:              item.Date,
				EditDate:          item.EditDate,
			})
		}
		if len(modelMessages) > 0 {
			links, err := s.storeBatch(ctx, account.ID, channel.ID, maxSeen, time.Now().UTC(), modelMessages)
			if err != nil {
				return result, err
			}
			result.Links += links
		}
		if err := reportTaskProgress(ctx, progress, result.Messages, profileLimit, "history sync batch stored"); err != nil {
			return result, err
		}
		if minID == 0 || minID == offsetID {
			break
		}
		if profileLimit > 0 {
			if result.Messages >= profileLimit || len(batch) < fetchLimit {
				break
			}
		}
		offsetID = minID
	}
	return result, nil
}

func reportTaskProgress(ctx context.Context, progress taskpkg.ProgressSink, current int, total int, message string) error {
	if progress == nil {
		return nil
	}
	return progress.Progress(ctx, int64(current), int64(total), message)
}

func checkTaskStatus(ctx context.Context, progress taskpkg.ProgressSink) error {
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
	if status == model.TaskStatusPaused {
		return ErrTaskPaused
	}
	return nil
}

func (s *Service) refreshResourceStats(ctx context.Context) error {
	if s.resources == nil {
		return nil
	}
	return s.resources.RefreshGlobalGrouped(ctx)
}

func (s *Service) tryAcquireChannel(channelID int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.runningChannels[channelID]; ok {
		return false
	}
	s.runningChannels[channelID] = struct{}{}
	return true
}

func (s *Service) releaseChannel(channelID int64) {
	s.mu.Lock()
	delete(s.runningChannels, channelID)
	s.mu.Unlock()
}

func (s *Service) markChannelAccountStatus(ctx context.Context, channelID int64, status string) {
	if s.accounts == nil || s.channels == nil {
		return
	}
	channel, err := s.channels.FindByID(ctx, channelID)
	if err != nil {
		return
	}
	_ = s.accounts.UpdateStatus(ctx, channel.AccountID, status)
}

func (s *Service) storeBatch(ctx context.Context, accountID int64, channelID int64, cursor int64, cursorDate time.Time, messages []model.Message) (int, error) {
	filtered := make([]model.Message, 0, len(messages))
	linksByTelegramID := map[int64][]model.Link{}
	for _, msg := range messages {
		extracted := s.extractor.Extract(msg.Text)
		if s.filter != nil {
			result, err := s.filter.Apply(ctx, messagefilter.Request{
				ChannelID:      msg.ChannelID,
				Text:           msg.Text,
				RequireRule:    false,
				RequireEnabled: false,
			})
			if err != nil {
				return 0, fmt.Errorf("filter history message: %w", err)
			}
			if result.RuleApplied {
				if !result.Keep {
					continue
				}
				extracted = result.Links
			}
		}
		filtered = append(filtered, msg)
		linksByTelegramID[msg.TelegramMessageID] = extracted
	}

	var linkCount int
	err := dbpkg.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		if len(filtered) > 0 {
			stored, err := s.messages.SaveBatchTx(ctx, tx, filtered)
			if err != nil {
				return err
			}
			for _, msg := range stored {
				extracted := linksByTelegramID[msg.TelegramMessageID]
				_, err := s.links.ReplaceForMessageTx(ctx, tx, msg.ID, extracted)
				if err != nil {
					return err
				}
				linkCount += len(extracted)
			}
		}
		if cursor > 0 && s.cursors != nil {
			if err := s.cursors.SaveTx(ctx, tx, model.SyncCursor{
				AccountID:     accountID,
				ChannelID:     channelID,
				CursorType:    "history",
				LastMessageID: cursor,
				Date:          cursorDate,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("store history batch: %w", err)
	}
	return linkCount, nil
}
