package notification

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"tg-search/internal/db"
	"tg-search/internal/model"
	"tg-search/internal/repository"
	"tg-search/internal/resource"
)

func TestSavedSearchMatchesResource(t *testing.T) {
	item := resource.Item{
		ID:        "link:https://pan.quark.cn/s/nezha3",
		Kind:      "link",
		Type:      "quark",
		Category:  "cloud_drive",
		Title:     "哪吒3 4K",
		AccountID: 7,
		ChannelID: 9,
	}
	search := model.SavedSearch{
		Keyword: "哪吒3",
		Filters: model.SavedSearchFilters{
			CloudTypes: []string{"quark"},
			AccountID:  7,
			ChannelID:  9,
		},
		Enabled: true,
	}
	if !SavedSearchMatchesResource(search, item) {
		t.Fatalf("SavedSearchMatchesResource returned false, want true")
	}
	search.Filters.CloudTypes = []string{"aliyun"}
	if SavedSearchMatchesResource(search, item) {
		t.Fatalf("SavedSearchMatchesResource returned true for wrong provider")
	}
	search.Filters.CloudTypes = []string{"quark"}
	search.Enabled = false
	if SavedSearchMatchesResource(search, item) {
		t.Fatalf("SavedSearchMatchesResource returned true for disabled search")
	}
}

func TestServiceEnqueueResourceCreated(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "tg-search.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	searches := repository.NewSavedSearchRepository(conn)
	deliveries := repository.NewNotificationDeliveryRepository(conn)
	webhooks := repository.NewWebhookRepository(conn)
	botSubs := repository.NewTelegramBotSubscriptionRepository(conn)
	id, err := searches.Create(ctx, model.SavedSearch{
		Name:           "哪吒3",
		Keyword:        "哪吒3",
		Filters:        model.SavedSearchFilters{Category: "cloud_drive"},
		NotifyRSS:      true,
		NotifyWebhook:  true,
		NotifyTelegram: true,
		Enabled:        true,
	})
	if err != nil {
		t.Fatalf("create saved search: %v", err)
	}
	subID, err := botSubs.Create(ctx, model.TelegramBotSubscription{ChatID: 42, SavedSearchID: id, Enabled: true})
	if err != nil {
		t.Fatalf("create bot subscription: %v", err)
	}
	resourceWebhookID, err := webhooks.Create(ctx, model.Webhook{
		Name:    "resource",
		URL:     "https://example.com/resource",
		Events:  []string{model.NotificationEventResourceCreated},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("create resource webhook: %v", err)
	}
	matchWebhookID, err := webhooks.Create(ctx, model.Webhook{
		Name:    "match",
		URL:     "https://example.com/match",
		Events:  []string{model.NotificationEventSavedSearchMatched},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("create match webhook: %v", err)
	}
	service := NewService(Options{SavedSearches: searches, Deliveries: deliveries, Webhooks: webhooks, BotSubs: botSubs})
	created, err := service.EnqueueResourceCreated(ctx, resource.Item{
		ID:                "link:https://pan.quark.cn/s/nezha3",
		Kind:              "link",
		Type:              "quark",
		Category:          "cloud_drive",
		Title:             "哪吒3",
		URL:               "https://pan.quark.cn/s/nezha3",
		ChannelID:         5,
		ChannelTitle:      "电影频道",
		TelegramMessageID: 99,
		Datetime:          time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("enqueue resource created: %v", err)
	}
	if len(created) != 4 {
		t.Fatalf("deliveries = %+v, want resource webhook, saved search, match webhook, and telegram deliveries", created)
	}
	items, err := deliveries.List(ctx, repository.NotificationDeliveryListParams{Status: model.NotificationDeliveryPending})
	if err != nil {
		t.Fatalf("list deliveries: %v", err)
	}
	seen := map[string]bool{}
	for _, item := range items {
		seen[item.EventType+":"+item.TargetType] = true
		if item.EventType == model.NotificationEventSavedSearchMatched && item.TargetType == model.NotificationTargetSavedSearch && item.TargetID != id {
			t.Fatalf("saved search delivery target = %d, want %d", item.TargetID, id)
		}
		if item.EventType == model.NotificationEventResourceCreated && item.TargetID != resourceWebhookID {
			t.Fatalf("resource webhook target = %d, want %d", item.TargetID, resourceWebhookID)
		}
		if item.EventType == model.NotificationEventSavedSearchMatched && item.TargetType == model.NotificationTargetWebhook && item.TargetID != matchWebhookID {
			t.Fatalf("match webhook target = %d, want %d", item.TargetID, matchWebhookID)
		}
		if item.EventType == model.NotificationEventSavedSearchMatched && item.TargetType == model.NotificationTargetTelegram && item.TargetID != subID {
			t.Fatalf("telegram target = %d, want %d", item.TargetID, subID)
		}
	}
	if !seen[model.NotificationEventResourceCreated+":"+model.NotificationTargetWebhook] ||
		!seen[model.NotificationEventSavedSearchMatched+":"+model.NotificationTargetSavedSearch] ||
		!seen[model.NotificationEventSavedSearchMatched+":"+model.NotificationTargetWebhook] ||
		!seen[model.NotificationEventSavedSearchMatched+":"+model.NotificationTargetTelegram] {
		t.Fatalf("stored deliveries = %+v, want resource webhook, saved search, match webhook, and telegram", items)
	}
}
