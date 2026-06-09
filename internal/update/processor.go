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
	"tg-search/internal/resource"
	taskpkg "tg-search/internal/task"
)

type ProcessorOptions struct {
	DB        *sql.DB
	Channels  *repository.ChannelRepository
	Messages  *repository.MessageRepository
	Links     *repository.LinkRepository
	Files     *repository.FileRepository
	Resources *resource.Service
	Cursors   *repository.SyncCursorRepository
	Tasks     *taskpkg.Service
	Extractor *link.Extractor
	Filter    *messagefilter.Filter
}

type Processor struct {
	db        *sql.DB
	channels  *repository.ChannelRepository
	messages  *repository.MessageRepository
	links     *repository.LinkRepository
	files     *repository.FileRepository
	resources *resource.Service
	cursors   *repository.SyncCursorRepository
	tasks     *taskpkg.Service
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
		files:     opts.Files,
		resources: opts.Resources,
		cursors:   opts.Cursors,
		tasks:     opts.Tasks,
		extractor: opts.Extractor,
		filter:    opts.Filter,
	}
}

func (p *Processor) Process(ctx context.Context, event Event) error {
	channel, err := p.channels.FindByTelegramID(ctx, event.AccountID, event.TelegramChannelID)
	if err != nil {
		return fmt.Errorf("find update channel: %w", err)
	}
	if !channel.ListenEnabled {
		return nil
	}

	switch event.Type {
	case EventNewMessage, EventEditMessage:
		if err := p.enqueueGapRecovery(ctx, channel, event); err != nil {
			return err
		}
		if err := p.storeMessage(ctx, channel, event); err != nil {
			return err
		}
		if event.Type == EventNewMessage {
			return p.advanceHistoryCursor(ctx, channel, event)
		}
		return nil
	case EventDeleteMessage:
		return p.deleteMessage(ctx, channel, event)
	default:
		return fmt.Errorf("unsupported update event type %q", event.Type)
	}
}

func (p *Processor) enqueueGapRecovery(ctx context.Context, channel model.Channel, event Event) error {
	if p.cursors == nil || p.tasks == nil || event.Type != EventNewMessage || event.MessageID <= 0 {
		return nil
	}
	cursor, err := p.cursors.Find(ctx, event.AccountID, channel.ID, "history")
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("load history cursor for gap detection: %w", err)
	}
	if cursor.LastMessageID <= 0 || event.MessageID <= cursor.LastMessageID+1 {
		return nil
	}
	_, err = p.tasks.Enqueue(ctx, model.TaskTypeGapRecovery, taskpkg.GapRecoveryPayload{
		AccountID:         event.AccountID,
		ChannelID:         channel.ID,
		FromMessageID:     cursor.LastMessageID + 1,
		ToMessageID:       event.MessageID - 1,
		TriggerMessageID:  event.MessageID,
		TelegramChannelID: event.TelegramChannelID,
	})
	if err != nil {
		return fmt.Errorf("enqueue gap recovery task: %w", err)
	}
	return nil
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
				return p.refreshResourceStats(ctx)
			}
			return nil
		}
		extracted = result.Links
	}
	if err := dbpkg.WithTx(ctx, p.db, func(tx *sql.Tx) error {
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
			Files:             event.Files,
		}})
		if err != nil {
			return err
		}
		if _, err := p.links.ReplaceForMessageTx(ctx, tx, stored[0].ID, extracted); err != nil {
			return err
		}
		if p.files == nil {
			return nil
		}
		_, err = p.files.ReplaceForMessageTx(ctx, tx, stored[0].ID, stored[0].Files)
		return err
	}); err != nil {
		return err
	}
	return p.refreshResourceStats(ctx)
}

func (p *Processor) deleteMessage(ctx context.Context, channel model.Channel, event Event) error {
	if err := dbpkg.WithTx(ctx, p.db, func(tx *sql.Tx) error {
		return p.messages.MarkDeletedTx(ctx, tx, channel.ID, event.MessageID)
	}); err != nil {
		return err
	}
	return p.refreshResourceStats(ctx)
}

func (p *Processor) refreshResourceStats(ctx context.Context) error {
	if p.resources == nil {
		return nil
	}
	return p.resources.RefreshGlobalGrouped(ctx)
}

func (p *Processor) advanceHistoryCursor(ctx context.Context, channel model.Channel, event Event) error {
	if p.cursors == nil || event.MessageID <= 0 {
		return nil
	}
	cursor, err := p.cursors.Find(ctx, event.AccountID, channel.ID, "history")
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("load history cursor for update advance: %w", err)
	}
	if err == nil {
		if cursor.LastMessageID >= event.MessageID {
			return nil
		}
		if event.MessageID > cursor.LastMessageID+1 {
			return nil
		}
	}
	date := event.Date
	if date.IsZero() {
		date = event.EditDateOrNow()
	}
	return p.cursors.Save(ctx, model.SyncCursor{
		AccountID:     event.AccountID,
		ChannelID:     channel.ID,
		CursorType:    "history",
		LastMessageID: event.MessageID,
		Date:          date,
	})
}

func (e Event) EditDateOrNow() time.Time {
	if e.EditDate != nil {
		return *e.EditDate
	}
	return time.Now().UTC()
}
