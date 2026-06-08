package channel

import (
	"context"
	"fmt"

	"tg-search/internal/model"
	"tg-search/internal/repository"
	"tg-search/internal/session"
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
	for _, item := range items {
		channel := model.Channel{
			AccountID:         account.ID,
			TelegramChannelID: item.TelegramChannelID,
			AccessHash:        item.AccessHash,
			Title:             item.Title,
			Username:          item.Username,
			Type:              item.Type,
		}
		id, err := s.channels.Save(ctx, channel)
		if err != nil {
			return nil, err
		}
		channel.ID = id
		out = append(out, channel)
	}
	return out, nil
}
