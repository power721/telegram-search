package update

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	dbpkg "tg-search/internal/db"
	"tg-search/internal/link"
	"tg-search/internal/messagefilter"
	"tg-search/internal/model"
	"tg-search/internal/repository"
)

type ProcessorOptions struct {
	DB        *sql.DB
	Channels  *repository.ChannelRepository
	Messages  *repository.MessageRepository
	Links     *repository.LinkRepository
	Extractor *link.Extractor
	Filter    *messagefilter.Filter
}

type Processor struct {
	db        *sql.DB
	channels  *repository.ChannelRepository
	messages  *repository.MessageRepository
	links     *repository.LinkRepository
	extractor *link.Extractor
	filter    *messagefilter.Filter
}

func NewProcessor(opts ProcessorOptions) *Processor {
	if opts.Extractor == nil {
		opts.Extractor = link.NewExtractor()
	}
	return &Processor{
		db:        opts.DB,
		channels:  opts.Channels,
		messages:  opts.Messages,
		links:     opts.Links,
		extractor: opts.Extractor,
		filter:    opts.Filter,
	}
}

func (p *Processor) Process(ctx context.Context, event Event) error {
	channel, err := p.channels.FindByTelegramID(ctx, event.AccountID, event.TelegramChannelID)
	if err != nil {
		return fmt.Errorf("find update channel: %w", err)
	}

	switch event.Type {
	case EventNewMessage, EventEditMessage:
		return p.storeMessage(ctx, channel, event)
	case EventDeleteMessage:
		return p.deleteMessage(ctx, channel, event)
	default:
		return fmt.Errorf("unsupported update event type %q", event.Type)
	}
}

func (p *Processor) storeMessage(ctx context.Context, channel model.Channel, event Event) error {
	extracted := p.extractor.Extract(event.Text)
	if p.filter != nil {
		result, err := p.filter.Apply(ctx, messagefilter.Request{
			ChannelID:      channel.ID,
			Text:           event.Text,
			RequireRule:    true,
			RequireEnabled: true,
		})
		if err != nil {
			return err
		}
		if !result.Keep {
			if event.Type == EventEditMessage && result.RuleApplied {
				if err := p.messages.MarkDeleted(ctx, channel.ID, event.MessageID); err != nil && !errors.Is(err, sql.ErrNoRows) {
					return err
				}
			}
			return nil
		}
		extracted = result.Links
	}
	return dbpkg.WithTx(ctx, p.db, func(tx *sql.Tx) error {
		date := event.Date
		if date.IsZero() {
			date = event.EditDateOrNow()
		}
		stored, err := p.messages.SaveBatchTx(ctx, tx, []model.Message{{
			AccountID:         event.AccountID,
			ChannelID:         channel.ID,
			TelegramMessageID: event.MessageID,
			SenderID:          event.SenderID,
			Text:              event.Text,
			RawJSON:           event.RawJSON,
			Date:              date,
			EditDate:          event.EditDate,
		}})
		if err != nil {
			return err
		}
		_, err = p.links.ReplaceForMessageTx(ctx, tx, stored[0].ID, extracted)
		return err
	})
}

func (p *Processor) deleteMessage(ctx context.Context, channel model.Channel, event Event) error {
	return dbpkg.WithTx(ctx, p.db, func(tx *sql.Tx) error {
		return p.messages.MarkDeletedTx(ctx, tx, channel.ID, event.MessageID)
	})
}

func (e Event) EditDateOrNow() time.Time {
	if e.EditDate != nil {
		return *e.EditDate
	}
	return time.Now().UTC()
}
