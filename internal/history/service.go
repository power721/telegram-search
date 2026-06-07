package history

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	dbpkg "tg-provider/internal/db"
	"tg-provider/internal/link"
	"tg-provider/internal/model"
	"tg-provider/internal/repository"
	"tg-provider/internal/session"
	"tg-provider/internal/telegram"
)

type Options struct {
	DB               *sql.DB
	Accounts         *repository.AccountRepository
	Channels         *repository.ChannelRepository
	Messages         *repository.MessageRepository
	Links            *repository.LinkRepository
	Telegram         telegram.Client
	Sessions         *session.Manager
	Extractor        *link.Extractor
	HistoryBatchSize int
}

type Service struct {
	db               *sql.DB
	accounts         *repository.AccountRepository
	channels         *repository.ChannelRepository
	messages         *repository.MessageRepository
	links            *repository.LinkRepository
	telegram         telegram.Client
	sessions         *session.Manager
	extractor        *link.Extractor
	historyBatchSize int
}

type SyncResult struct {
	Messages int `json:"messages"`
	Links    int `json:"links"`
}

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
	return &Service{
		db:               opts.DB,
		accounts:         opts.Accounts,
		channels:         opts.Channels,
		messages:         opts.Messages,
		links:            opts.Links,
		telegram:         opts.Telegram,
		sessions:         opts.Sessions,
		extractor:        opts.Extractor,
		historyBatchSize: opts.HistoryBatchSize,
	}
}

func (s *Service) SyncChannel(ctx context.Context, channelID int64) (SyncResult, error) {
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

	var result SyncResult
	var maxSeen int64
	offsetID := int64(0)
	for {
		batch, err := s.telegram.FetchHistory(ctx, accountSession, ref, offsetID, s.historyBatchSize)
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
			links, err := s.storeBatch(ctx, channel.ID, maxSeen, modelMessages)
			if err != nil {
				return result, err
			}
			result.Links += links
		}
		if len(batch) < s.historyBatchSize || minID == 0 || minID == offsetID {
			break
		}
		offsetID = minID
	}
	return result, nil
}

func (s *Service) storeBatch(ctx context.Context, channelID int64, cursor int64, messages []model.Message) (int, error) {
	var linkCount int
	err := dbpkg.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		stored, err := s.messages.SaveBatchTx(ctx, tx, messages)
		if err != nil {
			return err
		}
		for _, msg := range stored {
			extracted := s.extractor.Extract(msg.Text)
			_, err := s.links.ReplaceForMessageTx(ctx, tx, msg.ID, extracted)
			if err != nil {
				return err
			}
			linkCount += len(extracted)
		}
		if cursor > 0 {
			if err := s.channels.UpdateCursorTx(ctx, tx, channelID, cursor, time.Now().UTC()); err != nil {
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
