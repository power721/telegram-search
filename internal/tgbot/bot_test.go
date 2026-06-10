package tgbot

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tg-search/internal/config"
	"tg-search/internal/db"
	"tg-search/internal/model"
	"tg-search/internal/repository"
	"tg-search/internal/resource"
)

type fakeAPI struct {
	updates []Update
	sent    []sentMessage
}

type sentMessage struct {
	chatID int64
	text   string
}

func (f *fakeAPI) GetUpdates(ctx context.Context, offset int64) ([]Update, error) {
	out := []Update{}
	for _, update := range f.updates {
		if update.UpdateID >= offset {
			out = append(out, update)
		}
	}
	return out, nil
}

func (f *fakeAPI) SendMessage(ctx context.Context, chatID int64, text string) error {
	f.sent = append(f.sent, sentMessage{chatID: chatID, text: text})
	return nil
}

func TestBotCommands(t *testing.T) {
	ctx := context.Background()
	conn := setupBotDB(t)
	accounts := repository.NewAccountRepository(conn)
	channels := repository.NewChannelRepository(conn)
	messages := repository.NewMessageRepository(conn)
	links := repository.NewLinkRepository(conn)
	files := repository.NewFileRepository(conn)
	searches := repository.NewSavedSearchRepository(conn)
	subs := repository.NewTelegramBotSubscriptionRepository(conn)
	resources := resource.NewService(links, files)

	accountID, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 1, Title: "电影频道", Type: model.ChannelTypeChannel})
	stored, err := messages.SaveBatch(ctx, []model.Message{{
		AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1,
		Text: "流浪地球2 4K", RawJSON: "{}", Date: time.Now().UTC(),
	}})
	if err != nil {
		t.Fatalf("save message: %v", err)
	}
	if _, err := links.SaveBatch(ctx, stored[0].ID, []model.Link{{Type: "quark", URL: "https://pan.quark.cn/s/wandering", Note: "流浪地球2 4K"}}); err != nil {
		t.Fatalf("save link: %v", err)
	}
	api := &fakeAPI{updates: []Update{
		{UpdateID: 1, Message: &Message{Chat: Chat{ID: 42}, Text: "/search 流浪地球"}},
		{UpdateID: 2, Message: &Message{Chat: Chat{ID: 42}, Text: "/subscribe 哪吒3"}},
		{UpdateID: 3, Message: &Message{Chat: Chat{ID: 42}, Text: "/subscriptions"}},
	}}
	bot := NewBot(BotOptions{API: api, Resources: resources, SavedSearches: searches, Subscriptions: subs})
	if err := bot.Run(ctx); err != nil {
		t.Fatalf("run bot: %v", err)
	}
	if len(api.sent) != 3 {
		t.Fatalf("sent messages = %+v, want 3", api.sent)
	}
	if !strings.Contains(api.sent[0].text, "流浪地球2 4K") || !strings.Contains(api.sent[0].text, "https://pan.quark.cn/s/wandering") {
		t.Fatalf("search response = %q, want resource", api.sent[0].text)
	}
	if !strings.Contains(api.sent[1].text, "Subscribed #") {
		t.Fatalf("subscribe response = %q, want subscription id", api.sent[1].text)
	}
	if !strings.Contains(api.sent[2].text, "哪吒3") {
		t.Fatalf("subscriptions response = %q, want keyword", api.sent[2].text)
	}
}

func TestDeliveryDispatcherSendsTelegramMatch(t *testing.T) {
	ctx := context.Background()
	conn := setupBotDB(t)
	searches := repository.NewSavedSearchRepository(conn)
	subs := repository.NewTelegramBotSubscriptionRepository(conn)
	deliveries := repository.NewNotificationDeliveryRepository(conn)
	searchID, err := searches.Create(ctx, model.SavedSearch{Name: "哪吒3", Keyword: "哪吒3", NotifyTelegram: true, Enabled: true})
	if err != nil {
		t.Fatalf("create saved search: %v", err)
	}
	subID, err := subs.Create(ctx, model.TelegramBotSubscription{ChatID: 42, SavedSearchID: searchID, Enabled: true})
	if err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	payload := `{"saved_search_id":1,"saved_search_name":"哪吒3","keyword":"哪吒3","resource_id":"link:x","resource_title":"哪吒3 4K","resource_type":"quark","resource_category":"cloud_drive","resource_url":"https://pan.quark.cn/s/nezha3","source_channel_id":1,"source_channel_name":"电影频道","telegram_message_id":9,"datetime":"2026-06-10T12:00:00Z"}`
	deliveryID, err := deliveries.Create(ctx, model.NotificationDelivery{
		EventType:   model.NotificationEventSavedSearchMatched,
		TargetType:  model.NotificationTargetTelegram,
		TargetID:    subID,
		PayloadJSON: payload,
	})
	if err != nil {
		t.Fatalf("create delivery: %v", err)
	}
	api := &fakeAPI{}
	dispatcher := NewDeliveryDispatcher(DeliveryDispatcherOptions{
		API:           api,
		Deliveries:    deliveries,
		Subscriptions: subs,
		Now:           func() time.Time { return time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC) },
	})
	if err := dispatcher.Run(ctx); err != nil {
		t.Fatalf("run dispatcher: %v", err)
	}
	if len(api.sent) != 1 || api.sent[0].chatID != 42 || !strings.Contains(api.sent[0].text, "哪吒3 4K") {
		t.Fatalf("sent = %+v, want match message", api.sent)
	}
	stored, err := deliveries.FindByID(ctx, deliveryID)
	if err != nil {
		t.Fatalf("find delivery: %v", err)
	}
	if stored.Status != model.NotificationDeliverySucceeded || stored.DeliveredAt == nil {
		t.Fatalf("delivery = %+v, want succeeded", stored)
	}
}

func TestRuntimeAppliesTelegramBotSettingsWithoutRestart(t *testing.T) {
	ctx := context.Background()
	conn := setupBotDB(t)
	settings := repository.NewSettingsRepository(conn)
	searches := repository.NewSavedSearchRepository(conn)
	subs := repository.NewTelegramBotSubscriptionRepository(conn)
	deliveries := repository.NewNotificationDeliveryRepository(conn)
	firstAPI := &fakeAPI{updates: []Update{{UpdateID: 1, Message: &Message{Chat: Chat{ID: 42}, Text: "/help"}}}}
	secondAPI := &fakeAPI{updates: []Update{{UpdateID: 1, Message: &Message{Chat: Chat{ID: 42}, Text: "/help"}}}}
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	runtime := NewRuntime(RuntimeOptions{
		Settings: settings,
		Defaults: config.BotConfig{PollInterval: config.Duration(3 * time.Second)},
		APIFactory: func(token string) BotAPI {
			if token == "second-token" {
				return secondAPI
			}
			return firstAPI
		},
		SavedSearches: searches,
		Subscriptions: subs,
		Deliveries:    deliveries,
		Now:           func() time.Time { return now },
	})

	if err := runtime.Run(ctx); err != nil {
		t.Fatalf("run disabled runtime: %v", err)
	}
	if len(firstAPI.sent) != 0 {
		t.Fatalf("disabled runtime sent = %+v, want none", firstAPI.sent)
	}
	if err := settings.SaveTelegramBot(ctx, config.BotConfig{Enabled: true, Token: "first-token", PollInterval: config.Duration(5 * time.Second)}); err != nil {
		t.Fatalf("save first bot settings: %v", err)
	}
	if err := runtime.Run(ctx); err != nil {
		t.Fatalf("run enabled runtime: %v", err)
	}
	if len(firstAPI.sent) != 1 || !strings.Contains(firstAPI.sent[0].text, "/search <keyword>") {
		t.Fatalf("first token sent = %+v, want help message", firstAPI.sent)
	}
	now = now.Add(time.Second)
	if err := runtime.Run(ctx); err != nil {
		t.Fatalf("run before interval: %v", err)
	}
	if len(firstAPI.sent) != 1 {
		t.Fatalf("sent before interval = %+v, want unchanged", firstAPI.sent)
	}
	if err := settings.SaveTelegramBot(ctx, config.BotConfig{Enabled: true, Token: "second-token", PollInterval: config.Duration(5 * time.Second)}); err != nil {
		t.Fatalf("save second bot settings: %v", err)
	}
	if err := runtime.Run(ctx); err != nil {
		t.Fatalf("run after token change: %v", err)
	}
	if len(secondAPI.sent) != 1 {
		t.Fatalf("second token sent = %+v, want one message", secondAPI.sent)
	}
}

func setupBotDB(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "tg-search.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(context.Background(), conn); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	return conn
}
