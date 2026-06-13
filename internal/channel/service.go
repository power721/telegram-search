package channel

import (
	"context"
	"fmt"

	"tg-search/internal/model"
	"tg-search/internal/repository"
	"tg-search/internal/session"
	taskpkg "tg-search/internal/task"
	"tg-search/internal/telegram"
)

type Service struct {
	channels *repository.ChannelRepository
	telegram telegram.Client
	sessions *session.Manager
}

func NewService(channels *repository.ChannelRepository, client telegram.Client, sessions *session.Manager) *Service {
	if client == nil {
		client = telegram.NopClient{}
	}
	return &Service{channels: channels, telegram: client, sessions: sessions}
}

func (s *Service) SyncAccount(ctx context.Context, account model.Account) ([]model.Channel, error) {
	return s.SyncAccountWithProgress(ctx, account, nil)
}

func (s *Service) SyncAccountWithProgress(ctx context.Context, account model.Account, progress taskpkg.ProgressSink) ([]model.Channel, error) {
	sessionPath := ""
	if s.sessions != nil {
		sessionPath = s.sessions.PathForAccount(account.ID)
	}
	items, err := s.telegram.ListChannels(ctx, telegram.AccountSession{
		AccountID:   account.ID,
		Phone:       account.Phone,
		SessionPath: sessionPath,
	})
	if err != nil {
		return nil, fmt.Errorf("list telegram channels: %w", err)
	}
	out := make([]model.Channel, 0, len(items))
	for i, item := range items {
		if err := checkProgressStatus(ctx, progress); err != nil {
			return out, err
		}
		channel := model.Channel{
			AccountID:          account.ID,
			TelegramChannelID:  item.TelegramChannelID,
			AccessHash:         item.AccessHash,
			Title:              item.Title,
			Username:           item.Username,
			Type:               item.Type,
			MemberCount:        item.MemberCount,
			Description:        item.Description,
			AvatarState:        firstNonEmpty(item.AvatarState, "unknown"),
			PhotoID:            item.PhotoID,
			SyncState:          "metadata_only",
			ListenState:        "disabled",
		}
		id, err := s.channels.Save(ctx, channel)
		if err != nil {
			return nil, err
		}
		channel.ID = id
		out = append(out, channel)
		if progress != nil {
			if err := progress.Progress(ctx, int64(i+1), int64(len(items)), "metadata sync channel stored"); err != nil {
				return out, err
			}
		}
	}
	return out, nil
}

func checkProgressStatus(ctx context.Context, progress taskpkg.ProgressSink) error {
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

func firstNonEmpty(value string, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
