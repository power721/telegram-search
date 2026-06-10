package tgbot

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tg-search/internal/config"
	"tg-search/internal/db"
	"tg-search/internal/model"
	"tg-search/internal/notification"
	"tg-search/internal/repository"
	"tg-search/internal/resource"
)

type fakeAPI struct {
	updates  []Update
	sent     []sentMessage
	sentHTML []sentMessage
	commands []BotCommand
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

func (f *fakeAPI) SendHTMLMessage(ctx context.Context, chatID int64, text string) error {
	f.sentHTML = append(f.sentHTML, sentMessage{chatID: chatID, text: text})
	return nil
}

func (f *fakeAPI) SetCommands(ctx context.Context, commands []BotCommand) error {
	f.commands = append([]BotCommand{}, commands...)
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

	accountID, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", TelegramUserID: 42, Status: model.AccountStatusOnline})
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
	bot := NewBot(BotOptions{API: api, Accounts: accounts, Resources: resources, SavedSearches: searches, Subscriptions: subs})
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
	chats, err := subs.FindChats(ctx)
	if err != nil {
		t.Fatalf("find bot chats: %v", err)
	}
	if len(chats) != 1 || chats[0].ChatID != 42 {
		t.Fatalf("bot chats = %+v, want chat 42", chats)
	}
}

func TestBotRejectsUnknownTelegramAccount(t *testing.T) {
	ctx := context.Background()
	conn := setupBotDB(t)
	accounts := repository.NewAccountRepository(conn)
	searches := repository.NewSavedSearchRepository(conn)
	subs := repository.NewTelegramBotSubscriptionRepository(conn)
	api := &fakeAPI{updates: []Update{
		{UpdateID: 1, Message: &Message{Chat: Chat{ID: 99, Type: "private"}, Text: "/subscribe secret"}},
	}}
	bot := NewBot(BotOptions{API: api, Accounts: accounts, SavedSearches: searches, Subscriptions: subs})
	if err := bot.Run(ctx); err != nil {
		t.Fatalf("run bot: %v", err)
	}
	if len(api.sent) != 1 || !strings.Contains(api.sent[0].text, "not authorized") {
		t.Fatalf("sent = %+v, want authorization rejection", api.sent)
	}
	items, err := searches.FindAll(ctx)
	if err != nil {
		t.Fatalf("find saved searches: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("saved searches = %+v, want none", items)
	}
	chats, err := subs.FindChats(ctx)
	if err != nil {
		t.Fatalf("find bot chats: %v", err)
	}
	if len(chats) != 0 {
		t.Fatalf("bot chats = %+v, want none", chats)
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
	payload := `{"saved_search_id":1,"saved_search_name":"哪吒3","keyword":"哪吒3","resource_id":"link:x","resource_title":"哪吒3 4K","resource_type":"quark","resource_category":"cloud_drive","resource_url":"https://pan.quark.cn/s/nezha3","source_channel_id":1,"source_channel_name":"电影频道","source_channel_username":"movie_channel","telegram_channel_id":1001234567890,"telegram_message_id":9,"datetime":"2026-06-10T12:00:00Z"}`
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
	if len(api.sent) != 0 {
		t.Fatalf("plain sent = %+v, want none", api.sent)
	}
	if len(api.sentHTML) != 1 || api.sentHTML[0].chatID != 42 || !strings.Contains(api.sentHTML[0].text, "哪吒3 4K") {
		t.Fatalf("html sent = %+v, want match message", api.sentHTML)
	}
	if !strings.Contains(api.sentHTML[0].text, `来源: <a href="https://t.me/movie_channel/9">电影频道</a>`) {
		t.Fatalf("html sent = %q, want linked source channel", api.sentHTML[0].text)
	}
	stored, err := deliveries.FindByID(ctx, deliveryID)
	if err != nil {
		t.Fatalf("find delivery: %v", err)
	}
	if stored.Status != model.NotificationDeliverySucceeded || stored.DeliveredAt == nil {
		t.Fatalf("delivery = %+v, want succeeded", stored)
	}
}

func TestDeliveryDispatcherSkipsDuplicateResourceForSameChat(t *testing.T) {
	ctx := context.Background()
	conn := setupBotDB(t)
	searches := repository.NewSavedSearchRepository(conn)
	subs := repository.NewTelegramBotSubscriptionRepository(conn)
	deliveries := repository.NewNotificationDeliveryRepository(conn)
	firstSearchID, err := searches.Create(ctx, model.SavedSearch{Name: "香港", Keyword: "香港", NotifyTelegram: true, Enabled: true})
	if err != nil {
		t.Fatalf("create first saved search: %v", err)
	}
	secondSearchID, err := searches.Create(ctx, model.SavedSearch{Name: "地图", Keyword: "地图", NotifyTelegram: true, Enabled: true})
	if err != nil {
		t.Fatalf("create second saved search: %v", err)
	}
	firstSubID, err := subs.Create(ctx, model.TelegramBotSubscription{ChatID: 42, SavedSearchID: firstSearchID, Enabled: true})
	if err != nil {
		t.Fatalf("create first subscription: %v", err)
	}
	secondSubID, err := subs.Create(ctx, model.TelegramBotSubscription{ChatID: 42, SavedSearchID: secondSearchID, Enabled: true})
	if err != nil {
		t.Fatalf("create second subscription: %v", err)
	}
	payload := `{"saved_search_id":1,"saved_search_name":"香港","keyword":"香港","resource_id":"link:https://115cdn.com/s/swsz4fd3zrk?password=t58d","resource_title":"香港探秘地图","resource_type":"115","resource_category":"cloud_drive","resource_url":"https://115cdn.com/s/swsz4fd3zrk?password=t58d","source_channel_id":1,"source_channel_name":"LEO网盘搜集","telegram_message_id":9,"datetime":"2026-06-10T12:00:00Z"}`
	firstDeliveryID, err := deliveries.Create(ctx, model.NotificationDelivery{
		EventType:   model.NotificationEventSavedSearchMatched,
		TargetType:  model.NotificationTargetTelegram,
		TargetID:    firstSubID,
		PayloadJSON: payload,
	})
	if err != nil {
		t.Fatalf("create first delivery: %v", err)
	}
	secondDeliveryID, err := deliveries.Create(ctx, model.NotificationDelivery{
		EventType:   model.NotificationEventSavedSearchMatched,
		TargetType:  model.NotificationTargetTelegram,
		TargetID:    secondSubID,
		PayloadJSON: payload,
	})
	if err != nil {
		t.Fatalf("create second delivery: %v", err)
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
	if len(api.sentHTML) != 1 || !strings.Contains(api.sentHTML[0].text, "香港探秘地图") {
		t.Fatalf("html sent = %+v, want one resource message", api.sentHTML)
	}
	for _, id := range []int64{firstDeliveryID, secondDeliveryID} {
		stored, err := deliveries.FindByID(ctx, id)
		if err != nil {
			t.Fatalf("find delivery %d: %v", id, err)
		}
		if stored.Status != model.NotificationDeliverySucceeded || stored.DeliveredAt == nil {
			t.Fatalf("delivery %d = %+v, want succeeded", id, stored)
		}
	}
}

func TestFormatMatchMessageLinksPrivateTelegramMessage(t *testing.T) {
	message := formatMatchMessage(notification.SavedSearchMatch{
		ResourceTitle:     "翘楚 & 4K",
		ResourceType:      "baidu",
		SourceChannelName: "盘链资源频道",
		TelegramChannelID: 1001234567890,
		TelegramMessageID: 42,
	})
	if !strings.Contains(message, `翘楚 &amp; 4K`) {
		t.Fatalf("message = %q, want escaped title", message)
	}
	if !strings.Contains(message, `来源: <a href="https://t.me/c/1234567890/42">盘链资源频道</a>`) {
		t.Fatalf("message = %q, want private message source link", message)
	}
}

func TestBotListsAndBindsWebSavedSearchSubscriptions(t *testing.T) {
	ctx := context.Background()
	conn := setupBotDB(t)
	accounts := repository.NewAccountRepository(conn)
	searches := repository.NewSavedSearchRepository(conn)
	subs := repository.NewTelegramBotSubscriptionRepository(conn)
	if _, err := accounts.Save(ctx, model.Account{Phone: "+10000000000", TelegramUserID: 42, Status: model.AccountStatusOnline}); err != nil {
		t.Fatalf("save account: %v", err)
	}
	searchID, err := searches.Create(ctx, model.SavedSearch{
		Name:           "网页订阅",
		Keyword:        "哪吒3",
		NotifyRSS:      true,
		NotifyTelegram: true,
		Enabled:        true,
	})
	if err != nil {
		t.Fatalf("create saved search: %v", err)
	}
	api := &fakeAPI{updates: []Update{
		{UpdateID: 1, Message: &Message{Chat: Chat{ID: 42}, Text: "/subscriptions"}},
		{UpdateID: 2, Message: &Message{Chat: Chat{ID: 42}, Text: fmt.Sprintf("/subscribe %d", searchID)}},
		{UpdateID: 3, Message: &Message{Chat: Chat{ID: 42}, Text: "/subscriptions"}},
	}}
	bot := NewBot(BotOptions{API: api, Accounts: accounts, SavedSearches: searches, Subscriptions: subs})
	if err := bot.Run(ctx); err != nil {
		t.Fatalf("run bot: %v", err)
	}
	if len(api.sent) != 3 {
		t.Fatalf("sent = %+v, want 3 messages", api.sent)
	}
	if !strings.Contains(api.sent[0].text, "Available saved searches:") ||
		!strings.Contains(api.sent[0].text, fmt.Sprintf("saved #%d", searchID)) ||
		!strings.Contains(api.sent[0].text, "/subscribe") {
		t.Fatalf("first subscriptions response = %q, want available web saved search", api.sent[0].text)
	}
	if !strings.Contains(api.sent[1].text, fmt.Sprintf("saved search #%d", searchID)) {
		t.Fatalf("subscribe response = %q, want bound saved search", api.sent[1].text)
	}
	if !strings.Contains(api.sent[2].text, "My subscriptions:") ||
		!strings.Contains(api.sent[2].text, "sub #") ||
		!strings.Contains(api.sent[2].text, fmt.Sprintf("saved #%d", searchID)) ||
		strings.Contains(api.sent[2].text, "Available saved searches:") {
		t.Fatalf("second subscriptions response = %q, want only bound subscription", api.sent[2].text)
	}
}

func TestBotUnsubscribeAcceptsSavedSearchIDFallback(t *testing.T) {
	ctx := context.Background()
	conn := setupBotDB(t)
	accounts := repository.NewAccountRepository(conn)
	searches := repository.NewSavedSearchRepository(conn)
	subs := repository.NewTelegramBotSubscriptionRepository(conn)
	if _, err := accounts.Save(ctx, model.Account{Phone: "+10000000000", TelegramUserID: 42, Status: model.AccountStatusOnline}); err != nil {
		t.Fatalf("save account: %v", err)
	}
	if _, err := searches.Create(ctx, model.SavedSearch{Name: "翘楚", Keyword: "翘楚", NotifyTelegram: true, Enabled: true}); err != nil {
		t.Fatalf("create first saved search: %v", err)
	}
	searchID, err := searches.Create(ctx, model.SavedSearch{Name: "香港探秘地图", Keyword: "香港探秘地图", NotifyTelegram: true, Enabled: true})
	if err != nil {
		t.Fatalf("create second saved search: %v", err)
	}
	if searchID != 2 {
		t.Fatalf("second saved search id = %d, want 2", searchID)
	}
	subID, err := subs.Create(ctx, model.TelegramBotSubscription{ChatID: 42, SavedSearchID: searchID, Enabled: true})
	if err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	if subID == searchID {
		t.Fatalf("subscription id unexpectedly equals saved search id: %d", subID)
	}
	api := &fakeAPI{updates: []Update{
		{UpdateID: 1, Message: &Message{Chat: Chat{ID: 42}, Text: fmt.Sprintf("/unsubscribe %d", searchID)}},
		{UpdateID: 2, Message: &Message{Chat: Chat{ID: 42}, Text: "/subscriptions"}},
		{UpdateID: 3, Message: &Message{Chat: Chat{ID: 42}, Text: "/unsubscribe 99"}},
	}}
	bot := NewBot(BotOptions{API: api, Accounts: accounts, SavedSearches: searches, Subscriptions: subs})
	if err := bot.Run(ctx); err != nil {
		t.Fatalf("run bot: %v", err)
	}
	if len(api.sent) != 3 {
		t.Fatalf("sent = %+v, want 3 messages", api.sent)
	}
	if !strings.Contains(api.sent[0].text, "Unsubscribed from saved search #2.") {
		t.Fatalf("unsubscribe response = %q, want saved search fallback", api.sent[0].text)
	}
	if !strings.Contains(api.sent[1].text, "Available saved searches:") || strings.Contains(api.sent[1].text, "My subscriptions:") {
		t.Fatalf("subscriptions response = %q, want available searches only", api.sent[1].text)
	}
	if !strings.Contains(api.sent[2].text, "not subscribed to subscription or saved search #99") {
		t.Fatalf("missing unsubscribe response = %q, want clear miss", api.sent[2].text)
	}
}

func TestRuntimeAppliesTelegramBotSettingsWithoutRestart(t *testing.T) {
	ctx := context.Background()
	conn := setupBotDB(t)
	settings := repository.NewSettingsRepository(conn)
	accounts := repository.NewAccountRepository(conn)
	searches := repository.NewSavedSearchRepository(conn)
	subs := repository.NewTelegramBotSubscriptionRepository(conn)
	deliveries := repository.NewNotificationDeliveryRepository(conn)
	if _, err := accounts.Save(ctx, model.Account{Phone: "+10000000000", TelegramUserID: 42, Status: model.AccountStatusOnline}); err != nil {
		t.Fatalf("save account: %v", err)
	}
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
		Accounts:      accounts,
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
	if len(firstAPI.commands) == 0 || firstAPI.commands[0].Command != "search" {
		t.Fatalf("first token commands = %+v, want menu commands", firstAPI.commands)
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
	if len(secondAPI.commands) == 0 || secondAPI.commands[0].Command != "search" {
		t.Fatalf("second token commands = %+v, want menu commands", secondAPI.commands)
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
