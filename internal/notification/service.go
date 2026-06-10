package notification

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"tg-search/internal/model"
	"tg-search/internal/repository"
	"tg-search/internal/resource"
)

type Service struct {
	savedSearches *repository.SavedSearchRepository
	deliveries    *repository.NotificationDeliveryRepository
	webhooks      *repository.WebhookRepository
}

type Options struct {
	SavedSearches *repository.SavedSearchRepository
	Deliveries    *repository.NotificationDeliveryRepository
	Webhooks      *repository.WebhookRepository
}

type SavedSearchMatch struct {
	SavedSearchID     int64     `json:"saved_search_id"`
	SavedSearchName   string    `json:"saved_search_name"`
	Keyword           string    `json:"keyword"`
	ResourceID        string    `json:"resource_id"`
	ResourceTitle     string    `json:"resource_title"`
	ResourceType      string    `json:"resource_type"`
	ResourceCategory  string    `json:"resource_category"`
	ResourceURL       string    `json:"resource_url,omitempty"`
	SourceChannelID   int64     `json:"source_channel_id"`
	SourceChannelName string    `json:"source_channel_name"`
	TelegramMessageID int64     `json:"telegram_message_id"`
	Datetime          time.Time `json:"datetime"`
}

type ResourceEvent struct {
	ResourceID        string    `json:"resource_id"`
	ResourceTitle     string    `json:"resource_title"`
	ResourceType      string    `json:"resource_type"`
	ResourceCategory  string    `json:"resource_category"`
	ResourceURL       string    `json:"resource_url,omitempty"`
	SourceChannelID   int64     `json:"source_channel_id"`
	SourceChannelName string    `json:"source_channel_name"`
	TelegramMessageID int64     `json:"telegram_message_id"`
	Datetime          time.Time `json:"datetime"`
}

func NewService(opts Options) *Service {
	return &Service{savedSearches: opts.SavedSearches, deliveries: opts.Deliveries, webhooks: opts.Webhooks}
}

func (s *Service) MatchResource(ctx context.Context, item resource.Item) ([]model.SavedSearch, error) {
	if s.savedSearches == nil {
		return nil, nil
	}
	searches, err := s.savedSearches.FindEnabled(ctx)
	if err != nil {
		return nil, err
	}
	matches := make([]model.SavedSearch, 0, len(searches))
	for _, search := range searches {
		if SavedSearchMatchesResource(search, item) {
			matches = append(matches, search)
		}
	}
	return matches, nil
}

func (s *Service) EnqueueResourceCreated(ctx context.Context, item resource.Item) ([]model.NotificationDelivery, error) {
	if s.deliveries == nil {
		return nil, nil
	}
	deliveries, err := s.EnqueueEvent(ctx, model.NotificationEventResourceCreated, resourcePayload(item))
	if err != nil {
		return nil, err
	}
	searches, err := s.MatchResource(ctx, item)
	if err != nil {
		return nil, err
	}
	for _, search := range searches {
		match := matchPayload(search, item)
		payload, err := json.Marshal(match)
		if err != nil {
			return nil, err
		}
		delivery := model.NotificationDelivery{
			EventType:   model.NotificationEventSavedSearchMatched,
			TargetType:  model.NotificationTargetSavedSearch,
			TargetID:    search.ID,
			PayloadJSON: string(payload),
			Status:      model.NotificationDeliveryPending,
		}
		id, err := s.deliveries.Create(ctx, delivery)
		if err != nil {
			return nil, err
		}
		delivery.ID = id
		deliveries = append(deliveries, delivery)
		if search.NotifyWebhook {
			webhookDeliveries, err := s.EnqueueEvent(ctx, model.NotificationEventSavedSearchMatched, match)
			if err != nil {
				return nil, err
			}
			deliveries = append(deliveries, webhookDeliveries...)
		}
	}
	return deliveries, nil
}

func (s *Service) EnqueueEvent(ctx context.Context, eventType string, payload any) ([]model.NotificationDelivery, error) {
	if s.deliveries == nil || s.webhooks == nil {
		return nil, nil
	}
	webhooks, err := s.webhooks.FindEnabledForEvent(ctx, eventType)
	if err != nil {
		return nil, err
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	deliveries := make([]model.NotificationDelivery, 0, len(webhooks))
	for _, webhook := range webhooks {
		delivery := model.NotificationDelivery{
			EventType:   eventType,
			TargetType:  model.NotificationTargetWebhook,
			TargetID:    webhook.ID,
			PayloadJSON: string(payloadJSON),
			Status:      model.NotificationDeliveryPending,
		}
		id, err := s.deliveries.Create(ctx, delivery)
		if err != nil {
			return nil, err
		}
		delivery.ID = id
		deliveries = append(deliveries, delivery)
	}
	return deliveries, nil
}

func SavedSearchMatchesResource(search model.SavedSearch, item resource.Item) bool {
	if !search.Enabled {
		return false
	}
	if !filtersMatch(search.Filters, item) {
		return false
	}
	keyword := normalize(search.Keyword)
	if keyword == "" {
		return true
	}
	for _, field := range []string{
		item.Title,
		item.Note,
		item.FileName,
		item.SourceSnippet,
		item.MediaTitle,
		item.MediaTags,
		item.MediaSummary,
		item.URL,
		item.ChannelTitle,
	} {
		if strings.Contains(normalize(field), keyword) {
			return true
		}
	}
	return false
}

func filtersMatch(filters model.SavedSearchFilters, item resource.Item) bool {
	if filters.Type != "" && filters.Type != item.Type && filters.Type != item.Kind {
		return false
	}
	if filters.Category != "" && filters.Category != item.Category {
		return false
	}
	if filters.AccountID != 0 && filters.AccountID != item.AccountID {
		return false
	}
	if filters.ChannelID != 0 && filters.ChannelID != item.ChannelID {
		return false
	}
	if len(filters.CloudTypes) > 0 {
		found := false
		for _, typ := range filters.CloudTypes {
			typ = strings.TrimSpace(typ)
			if typ == item.Type || typ == item.Category {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func matchPayload(search model.SavedSearch, item resource.Item) SavedSearchMatch {
	return SavedSearchMatch{
		SavedSearchID:     search.ID,
		SavedSearchName:   search.Name,
		Keyword:           search.Keyword,
		ResourceID:        item.ID,
		ResourceTitle:     item.Title,
		ResourceType:      item.Type,
		ResourceCategory:  item.Category,
		ResourceURL:       item.URL,
		SourceChannelID:   item.ChannelID,
		SourceChannelName: item.ChannelTitle,
		TelegramMessageID: item.TelegramMessageID,
		Datetime:          item.Datetime,
	}
}

func resourcePayload(item resource.Item) ResourceEvent {
	return ResourceEvent{
		ResourceID:        item.ID,
		ResourceTitle:     item.Title,
		ResourceType:      item.Type,
		ResourceCategory:  item.Category,
		ResourceURL:       item.URL,
		SourceChannelID:   item.ChannelID,
		SourceChannelName: item.ChannelTitle,
		TelegramMessageID: item.TelegramMessageID,
		Datetime:          item.Datetime,
	}
}

func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
