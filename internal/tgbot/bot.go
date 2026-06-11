package tgbot

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"tg-search/internal/model"
	"tg-search/internal/notification"
	"tg-search/internal/repository"
	"tg-search/internal/resource"
)

const (
	defaultBaseURL      = "https://api.telegram.org"
	defaultBatchSize    = 50
	defaultMaxTries     = 5
	defaultRetryDelay   = 30 * time.Second
	defaultPollInterval = 3 * time.Second
)

type BotAPI interface {
	GetUpdates(context.Context, int64) ([]Update, error)
	SendMessage(context.Context, int64, string) error
	SendHTMLMessage(context.Context, int64, string) error
	SendMessageWithMarkup(context.Context, int64, string, *InlineKeyboardMarkup) error
	EditMessageText(context.Context, int64, int64, string, *InlineKeyboardMarkup) error
	AnswerCallbackQuery(context.Context, string, string) error
	SetCommands(context.Context, []BotCommand) error
}

type API struct {
	Token   string
	BaseURL string
	Client  *http.Client
}

type Update struct {
	UpdateID      int64          `json:"update_id"`
	Message       *Message       `json:"message,omitempty"`
	CallbackQuery *CallbackQuery `json:"callback_query,omitempty"`
}

type Message struct {
	MessageID int64 `json:"message_id"`
	Chat      Chat  `json:"chat"`
	From      *User `json:"from,omitempty"`
	Text      string `json:"text"`
}

type CallbackQuery struct {
	ID      string   `json:"id"`
	From    *User    `json:"from,omitempty"`
	Message *Message `json:"message,omitempty"`
	Data    string   `json:"data"`
}

type User struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

type Chat struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type BotCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

type InlineKeyboardButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data,omitempty"`
}

type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}

type apiResponse[T any] struct {
	OK          bool   `json:"ok"`
	Result      T      `json:"result"`
	Description string `json:"description"`
}

func NewAPI(token string) *API {
	return &API{Token: token, BaseURL: defaultBaseURL, Client: &http.Client{Timeout: 15 * time.Second}}
}

func (a *API) GetUpdates(ctx context.Context, offset int64) ([]Update, error) {
	values := url.Values{}
	if offset > 0 {
		values.Set("offset", strconv.FormatInt(offset, 10))
	}
	values.Set("timeout", "0")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.endpoint("getUpdates")+"?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}
	var resp apiResponse[[]Update]
	if err := a.do(req, &resp); err != nil {
		return nil, err
	}
	return resp.Result, nil
}

func (a *API) SendMessage(ctx context.Context, chatID int64, text string) error {
	return a.sendMessage(ctx, chatID, text, "")
}

func (a *API) SendHTMLMessage(ctx context.Context, chatID int64, text string) error {
	return a.sendMessage(ctx, chatID, text, "HTML")
}

func (a *API) sendMessage(ctx context.Context, chatID int64, text string, parseMode string) error {
	payload := map[string]any{
		"chat_id":                  chatID,
		"text":                     text,
		"disable_web_page_preview": true,
	}
	if parseMode != "" {
		payload["parse_mode"] = parseMode
	}
	return a.post(ctx, "sendMessage", payload)
}

func (a *API) SendMessageWithMarkup(ctx context.Context, chatID int64, text string, markup *InlineKeyboardMarkup) error {
	payload := map[string]any{
		"chat_id":                  chatID,
		"text":                     text,
		"disable_web_page_preview": true,
	}
	if markup != nil {
		payload["reply_markup"] = markup
	}
	return a.post(ctx, "sendMessage", payload)
}

func (a *API) EditMessageText(ctx context.Context, chatID int64, messageID int64, text string, markup *InlineKeyboardMarkup) error {
	payload := map[string]any{
		"chat_id":                  chatID,
		"message_id":               messageID,
		"text":                     text,
		"disable_web_page_preview": true,
	}
	if markup != nil {
		payload["reply_markup"] = markup
	}
	return a.post(ctx, "editMessageText", payload)
}

func (a *API) AnswerCallbackQuery(ctx context.Context, callbackID string, text string) error {
	payload := map[string]any{"callback_query_id": callbackID}
	if text != "" {
		payload["text"] = text
	}
	return a.post(ctx, "answerCallbackQuery", payload)
}

func (a *API) SetCommands(ctx context.Context, commands []BotCommand) error {
	return a.post(ctx, "setMyCommands", map[string]any{"commands": commands})
}

func (a *API) post(ctx context.Context, method string, payload map[string]any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint(method), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	var resp apiResponse[json.RawMessage]
	return a.do(req, &resp)
}

func (a *API) endpoint(method string) string {
	base := strings.TrimRight(a.BaseURL, "/")
	if base == "" {
		base = defaultBaseURL
	}
	return base + "/bot" + a.Token + "/" + method
}

func (a *API) do(req *http.Request, out any) error {
	client := a.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telegram bot api returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if err := json.Unmarshal(data, out); err != nil {
		return err
	}
	ok := false
	description := ""
	switch value := out.(type) {
	case *apiResponse[[]Update]:
		ok = value.OK
		description = value.Description
	case *apiResponse[json.RawMessage]:
		ok = value.OK
		description = value.Description
	}
	if !ok {
		if description == "" {
			description = "telegram bot api returned ok=false"
		}
		return errors.New(description)
	}
	return nil
}

type Bot struct {
	api           BotAPI
	accounts      *repository.AccountRepository
	resources     *resource.Service
	savedSearches *repository.SavedSearchRepository
	subscriptions *repository.TelegramBotSubscriptionRepository
	logger        *zap.Logger
	offset        int64
}

type BotOptions struct {
	API           BotAPI
	Accounts      *repository.AccountRepository
	Resources     *resource.Service
	SavedSearches *repository.SavedSearchRepository
	Subscriptions *repository.TelegramBotSubscriptionRepository
	Logger        *zap.Logger
}

func NewBot(opts BotOptions) *Bot {
	if opts.Logger == nil {
		opts.Logger = zap.NewNop()
	}
	return &Bot{api: opts.API, accounts: opts.Accounts, resources: opts.Resources, savedSearches: opts.SavedSearches, subscriptions: opts.Subscriptions, logger: opts.Logger}
}

func (b *Bot) Name() string {
	return "telegram-bot-poll"
}

func (b *Bot) Run(ctx context.Context) error {
	if b.api == nil {
		return nil
	}
	updates, err := b.api.GetUpdates(ctx, b.offset+1)
	if err != nil {
		return err
	}
	for _, update := range updates {
		if update.UpdateID > b.offset {
			b.offset = update.UpdateID
		}
		switch {
		case update.Message != nil:
			msg := *update.Message
			if strings.TrimSpace(msg.Text) == "" {
				continue
			}
			allowed, err := b.authorizeMessage(ctx, msg)
			if err != nil {
				b.logger.Warn("telegram bot authorization failed", zap.Int64("chat_id", msg.Chat.ID), zap.Error(err))
				continue
			}
			if !allowed {
				if err := b.api.SendMessage(ctx, msg.Chat.ID, "This Telegram account is not authorized to use this bot."); err != nil {
					b.logger.Warn("telegram bot authorization response failed", zap.Int64("chat_id", msg.Chat.ID), zap.Error(err))
				}
				continue
			}
			if err := b.rememberChat(ctx, msg.Chat); err != nil {
				b.logger.Warn("telegram bot chat registry update failed", zap.Int64("chat_id", msg.Chat.ID), zap.Error(err))
			}
			if err := b.handleMessage(ctx, msg); err != nil {
				b.logger.Warn("telegram bot command failed", zap.Int64("chat_id", msg.Chat.ID), zap.Error(err))
			}
		case update.CallbackQuery != nil:
			cq := *update.CallbackQuery
			if cq.Message == nil {
				continue
			}
			allowed, err := b.authorizeTelegramUser(ctx, callbackTelegramUserID(cq))
			if err != nil {
				b.logger.Warn("telegram bot callback authorization failed", zap.Int64("chat_id", cq.Message.Chat.ID), zap.Error(err))
				continue
			}
			if !allowed {
				if err := b.api.AnswerCallbackQuery(ctx, cq.ID, "This Telegram account is not authorized to use this bot."); err != nil {
					b.logger.Warn("telegram bot callback authorization response failed", zap.Int64("chat_id", cq.Message.Chat.ID), zap.Error(err))
				}
				continue
			}
			if err := b.rememberChat(ctx, cq.Message.Chat); err != nil {
				b.logger.Warn("telegram bot chat registry update failed", zap.Int64("chat_id", cq.Message.Chat.ID), zap.Error(err))
			}
			if err := b.handleCallback(ctx, cq); err != nil {
				b.logger.Warn("telegram bot callback command failed", zap.Int64("chat_id", cq.Message.Chat.ID), zap.Error(err))
			}
		}
	}
	return nil
}

func (b *Bot) authorizeMessage(ctx context.Context, msg Message) (bool, error) {
	return b.authorizeTelegramUser(ctx, messageTelegramUserID(msg))
}

func (b *Bot) authorizeTelegramUser(ctx context.Context, telegramUserID int64) (bool, error) {
	if b.accounts == nil {
		return false, nil
	}
	if telegramUserID <= 0 {
		return false, nil
	}
	_, err := b.accounts.FindByTelegramUserID(ctx, telegramUserID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func callbackTelegramUserID(cq CallbackQuery) int64 {
	if cq.From != nil && cq.From.ID > 0 {
		return cq.From.ID
	}
	return 0
}

func messageTelegramUserID(msg Message) int64 {
	if msg.From != nil && msg.From.ID > 0 {
		return msg.From.ID
	}
	if msg.Chat.ID > 0 && (msg.Chat.Type == "" || msg.Chat.Type == "private") {
		return msg.Chat.ID
	}
	return 0
}

func (b *Bot) rememberChat(ctx context.Context, chat Chat) error {
	if b.subscriptions == nil {
		return nil
	}
	return b.subscriptions.UpsertChat(ctx, model.TelegramBotChat{
		ChatID:     chat.ID,
		Title:      chat.Title,
		Username:   chat.Username,
		FirstName:  chat.FirstName,
		LastName:   chat.LastName,
		Type:       chat.Type,
		LastSeenAt: time.Now().UTC(),
	})
}

func (b *Bot) handleMessage(ctx context.Context, msg Message) error {
	text := strings.TrimSpace(msg.Text)
	command, arg := splitCommand(text)
	switch command {
	case "/start", "/help":
		return b.api.SendMessage(ctx, msg.Chat.ID, helpText())
	case "/search":
		return b.handleSearch(ctx, msg.Chat.ID, arg)
	case "/subscribe":
		return b.handleSubscribe(ctx, msg.Chat.ID, arg)
	case "/unsubscribe":
		return b.handleUnsubscribe(ctx, msg.Chat.ID, arg)
	case "/subscriptions":
		return b.handleSubscriptions(ctx, msg.Chat.ID)
	default:
		return b.api.SendMessage(ctx, msg.Chat.ID, helpText())
	}
}

func (b *Bot) handleSearch(ctx context.Context, chatID int64, keyword string) error {
	if b.resources == nil {
		return b.api.SendMessage(ctx, chatID, "Resource search is unavailable.")
	}
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return b.api.SendMessage(ctx, chatID, "Usage: /search <keyword>")
	}
	result, err := b.resources.List(ctx, resource.Query{Keyword: keyword, Limit: 5, MaxLimit: 5, Sort: "date_desc"})
	if err != nil {
		return err
	}
	return b.api.SendMessage(ctx, chatID, formatSearchResults(keyword, result.Items))
}

func (b *Bot) handleSubscribe(ctx context.Context, chatID int64, keyword string) error {
	if b.savedSearches == nil || b.subscriptions == nil {
		return b.api.SendMessage(ctx, chatID, "Subscription service is unavailable.")
	}
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return b.api.SendMessage(ctx, chatID, "Usage: /subscribe <keyword or saved_search_id>")
	}
	if id, ok := parseSavedSearchID(keyword); ok {
		search, err := b.savedSearches.FindByID(ctx, id)
		if err == nil {
			if !search.Enabled {
				return b.api.SendMessage(ctx, chatID, fmt.Sprintf("Saved search #%d is disabled.", search.ID))
			}
			if !search.NotifyTelegram {
				return b.api.SendMessage(ctx, chatID, fmt.Sprintf("Saved search #%d is not enabled for Telegram notifications.", search.ID))
			}
			subID, err := b.subscriptions.Create(ctx, model.TelegramBotSubscription{ChatID: chatID, SavedSearchID: search.ID, Enabled: true})
			if err != nil {
				return err
			}
			return b.api.SendMessage(ctx, chatID, fmt.Sprintf("Subscribed #%d to saved search #%d: %s", subID, search.ID, firstText(search.Name, search.Keyword)))
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
	}
	searchID, err := b.savedSearches.Create(ctx, model.SavedSearch{
		Name:           keyword,
		Keyword:        keyword,
		NotifyRSS:      true,
		NotifyTelegram: true,
		Enabled:        true,
	})
	if err != nil {
		return err
	}
	subID, err := b.subscriptions.Create(ctx, model.TelegramBotSubscription{ChatID: chatID, SavedSearchID: searchID, Enabled: true})
	if err != nil {
		return err
	}
	return b.api.SendMessage(ctx, chatID, fmt.Sprintf("Subscribed #%d: %s", subID, keyword))
}

func (b *Bot) handleUnsubscribe(ctx context.Context, chatID int64, arg string) error {
	if b.subscriptions == nil {
		return b.api.SendMessage(ctx, chatID, "Subscription service is unavailable.")
	}
	id, err := strconv.ParseInt(strings.TrimSpace(arg), 10, 64)
	if err != nil || id <= 0 {
		return b.api.SendMessage(ctx, chatID, "Usage: /unsubscribe <subscription_id or saved_search_id>")
	}
	if err := b.subscriptions.DeleteForChat(ctx, chatID, id); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err := b.subscriptions.DeleteForChatSavedSearch(ctx, chatID, id); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return b.api.SendMessage(ctx, chatID, fmt.Sprintf("This chat is not subscribed to subscription or saved search #%d.", id))
			}
			return err
		}
		return b.api.SendMessage(ctx, chatID, fmt.Sprintf("Unsubscribed from saved search #%d.", id))
	}
	return b.api.SendMessage(ctx, chatID, fmt.Sprintf("Unsubscribed subscription #%d.", id))
}

func (b *Bot) handleSubscriptions(ctx context.Context, chatID int64) error {
	if b.subscriptions == nil {
		return b.api.SendMessage(ctx, chatID, "Subscription service is unavailable.")
	}
	items, err := b.subscriptions.FindByChat(ctx, chatID)
	if err != nil {
		return err
	}
	subscribedSearches := map[int64]struct{}{}
	sent := false
	for _, item := range items {
		subscribedSearches[item.SavedSearchID] = struct{}{}
		if err := b.api.SendMessageWithMarkup(ctx, chatID, subscriptionCardText(item), unsubscribeMarkup(item.ID)); err != nil {
			return err
		}
		sent = true
	}
	if b.savedSearches != nil {
		searches, err := b.savedSearches.FindAll(ctx)
		if err != nil {
			return err
		}
		for _, search := range searches {
			if !search.Enabled || !search.NotifyTelegram {
				continue
			}
			if _, ok := subscribedSearches[search.ID]; ok {
				continue
			}
			if err := b.api.SendMessageWithMarkup(ctx, chatID, availableCardText(search), subscribeMarkup(search.ID)); err != nil {
				return err
			}
			sent = true
		}
	}
	if !sent {
		return b.api.SendMessage(ctx, chatID, "No subscriptions. Use /subscribe <keyword> or create a saved search in Settings.")
	}
	return nil
}

func (b *Bot) handleCallback(ctx context.Context, cq CallbackQuery) error {
	if b.subscriptions == nil {
		return b.api.AnswerCallbackQuery(ctx, cq.ID, "订阅服务不可用")
	}
	chatID := cq.Message.Chat.ID
	messageID := cq.Message.MessageID
	action, idText, _ := strings.Cut(cq.Data, ":")
	id, err := strconv.ParseInt(strings.TrimSpace(idText), 10, 64)
	if err != nil || id <= 0 {
		return b.api.AnswerCallbackQuery(ctx, cq.ID, "无效操作")
	}
	switch action {
	case "unsub":
		return b.handleUnsubscribeCallback(ctx, cq, chatID, messageID, id)
	case "sub":
		return b.handleSubscribeCallback(ctx, cq, chatID, messageID, id)
	default:
		return b.api.AnswerCallbackQuery(ctx, cq.ID, "")
	}
}

func (b *Bot) handleUnsubscribeCallback(ctx context.Context, cq CallbackQuery, chatID, messageID, subscriptionID int64) error {
	sub, err := b.subscriptions.FindByID(ctx, subscriptionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if err := b.api.AnswerCallbackQuery(ctx, cq.ID, "订阅不存在"); err != nil {
				return err
			}
			return b.api.EditMessageText(ctx, chatID, messageID, "✅ 已取消订阅", nil)
		}
		return err
	}
	if sub.ChatID != chatID {
		return b.api.AnswerCallbackQuery(ctx, cq.ID, "无效操作")
	}
	if err := b.subscriptions.DeleteForChat(ctx, chatID, subscriptionID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err := b.api.AnswerCallbackQuery(ctx, cq.ID, "已取消订阅"); err != nil {
		return err
	}
	return b.api.EditMessageText(ctx, chatID, messageID, "✅ 已取消订阅："+firstText(sub.SavedSearch, sub.Keyword), nil)
}

func (b *Bot) handleSubscribeCallback(ctx context.Context, cq CallbackQuery, chatID, messageID, savedSearchID int64) error {
	if b.savedSearches == nil {
		return b.api.AnswerCallbackQuery(ctx, cq.ID, "订阅服务不可用")
	}
	search, err := b.savedSearches.FindByID(ctx, savedSearchID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return b.api.AnswerCallbackQuery(ctx, cq.ID, "订阅不存在")
		}
		return err
	}
	if !search.Enabled || !search.NotifyTelegram {
		return b.api.AnswerCallbackQuery(ctx, cq.ID, "该订阅不可用")
	}
	if _, err := b.subscriptions.Create(ctx, model.TelegramBotSubscription{ChatID: chatID, SavedSearchID: search.ID, Enabled: true}); err != nil {
		return err
	}
	if err := b.api.AnswerCallbackQuery(ctx, cq.ID, "已订阅"); err != nil {
		return err
	}
	return b.api.EditMessageText(ctx, chatID, messageID, "✅ 已订阅："+firstText(search.Name, search.Keyword), nil)
}

func subscriptionCardText(item model.TelegramBotSubscription) string {
	return cardText("🔔 已订阅", firstText(item.SavedSearch, item.Keyword), item.Keyword)
}

func availableCardText(search model.SavedSearch) string {
	return cardText("🔍 可订阅", firstText(search.Name, search.Keyword), search.Keyword)
}

func cardText(prefix, title, keyword string) string {
	lines := []string{prefix + "：" + title}
	keyword = strings.TrimSpace(keyword)
	if keyword != "" && keyword != strings.TrimSpace(title) {
		lines = append(lines, "关键词："+keyword)
	}
	return strings.Join(lines, "\n")
}

func unsubscribeMarkup(subscriptionID int64) *InlineKeyboardMarkup {
	return singleButtonMarkup("取消订阅", fmt.Sprintf("unsub:%d", subscriptionID))
}

func subscribeMarkup(savedSearchID int64) *InlineKeyboardMarkup {
	return singleButtonMarkup("订阅", fmt.Sprintf("sub:%d", savedSearchID))
}

func singleButtonMarkup(text, data string) *InlineKeyboardMarkup {
	return &InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{{{Text: text, CallbackData: data}}}}
}

type DeliveryDispatcher struct {
	api           BotAPI
	deliveries    *repository.NotificationDeliveryRepository
	subscriptions *repository.TelegramBotSubscriptionRepository
	logger        *zap.Logger
	batchSize     int
	maxTries      int64
	retryDelay    time.Duration
	now           func() time.Time
}

type DeliveryDispatcherOptions struct {
	API           BotAPI
	Deliveries    *repository.NotificationDeliveryRepository
	Subscriptions *repository.TelegramBotSubscriptionRepository
	Logger        *zap.Logger
	BatchSize     int
	MaxTries      int64
	RetryDelay    time.Duration
	Now           func() time.Time
}

func NewDeliveryDispatcher(opts DeliveryDispatcherOptions) *DeliveryDispatcher {
	if opts.Logger == nil {
		opts.Logger = zap.NewNop()
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = defaultBatchSize
	}
	if opts.MaxTries <= 0 {
		opts.MaxTries = defaultMaxTries
	}
	if opts.RetryDelay <= 0 {
		opts.RetryDelay = defaultRetryDelay
	}
	if opts.Now == nil {
		opts.Now = func() time.Time { return time.Now().UTC() }
	}
	return &DeliveryDispatcher{
		api:           opts.API,
		deliveries:    opts.Deliveries,
		subscriptions: opts.Subscriptions,
		logger:        opts.Logger,
		batchSize:     opts.BatchSize,
		maxTries:      opts.MaxTries,
		retryDelay:    opts.RetryDelay,
		now:           opts.Now,
	}
}

func (d *DeliveryDispatcher) Name() string {
	return "telegram-bot-delivery-dispatch"
}

func (d *DeliveryDispatcher) Run(ctx context.Context) error {
	if d.api == nil || d.deliveries == nil || d.subscriptions == nil {
		return nil
	}
	items, err := d.deliveries.DueTelegramDeliveries(ctx, d.now(), d.batchSize)
	if err != nil {
		return err
	}
	for _, item := range items {
		if err := d.deliver(ctx, item); err != nil {
			d.logger.Warn("telegram bot delivery failed", zap.Int64("delivery_id", item.ID), zap.Error(err))
		}
	}
	return nil
}

func (d *DeliveryDispatcher) deliver(ctx context.Context, delivery model.NotificationDelivery) error {
	sub, err := d.subscriptions.FindByID(ctx, delivery.TargetID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return d.markTerminalFailure(ctx, delivery, "telegram bot subscription not found")
		}
		return err
	}
	var match notification.SavedSearchMatch
	if err := json.Unmarshal([]byte(delivery.PayloadJSON), &match); err != nil {
		return d.markTerminalFailure(ctx, delivery, "invalid telegram delivery payload")
	}
	if match.ResourceID != "" {
		sent, err := d.subscriptions.ResourceSent(ctx, sub.ChatID, match.ResourceID)
		if err != nil {
			return err
		}
		if sent {
			return d.deliveries.MarkSucceeded(ctx, delivery.ID, d.now())
		}
	}
	if err := d.api.SendHTMLMessage(ctx, sub.ChatID, formatMatchMessage(match)); err != nil {
		return d.markRetryableFailure(ctx, delivery, err.Error())
	}
	if match.ResourceID != "" {
		if err := d.subscriptions.MarkResourceSent(ctx, sub.ChatID, match.ResourceID); err != nil {
			d.logger.Warn("mark telegram resource sent failed", zap.Int64("delivery_id", delivery.ID), zap.Error(err))
		}
	}
	return d.deliveries.MarkSucceeded(ctx, delivery.ID, d.now())
}

func (d *DeliveryDispatcher) markRetryableFailure(ctx context.Context, delivery model.NotificationDelivery, message string) error {
	nextRetryCount := delivery.RetryCount + 1
	var nextRunAt *time.Time
	if nextRetryCount < d.maxTries {
		next := d.now().Add(d.retryDelay * time.Duration(nextRetryCount))
		nextRunAt = &next
	}
	if err := d.deliveries.MarkFailed(ctx, delivery.ID, message, nextRunAt); err != nil {
		return err
	}
	return fmt.Errorf("%s", message)
}

func (d *DeliveryDispatcher) markTerminalFailure(ctx context.Context, delivery model.NotificationDelivery, message string) error {
	if err := d.deliveries.MarkFailed(ctx, delivery.ID, message, nil); err != nil {
		return err
	}
	return fmt.Errorf("%s", message)
}

func splitCommand(text string) (string, string) {
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return "", ""
	}
	command := strings.ToLower(parts[0])
	if at := strings.Index(command, "@"); at >= 0 {
		command = command[:at]
	}
	arg := ""
	if len(parts) > 1 {
		arg = strings.TrimSpace(text[len(parts[0]):])
	}
	return command, arg
}

func parseSavedSearchID(value string) (int64, bool) {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "#")
	value = strings.TrimPrefix(value, "saved:")
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func DefaultCommands() []BotCommand {
	return []BotCommand{
		{Command: "search", Description: "Search resources"},
		{Command: "subscribe", Description: "Subscribe to a keyword or saved search ID"},
		{Command: "subscriptions", Description: "List my subscriptions and available saved searches"},
		{Command: "unsubscribe", Description: "Remove a subscription"},
		{Command: "help", Description: "Show command help"},
	}
}

func helpText() string {
	return strings.Join([]string{
		"tg-search bot commands:",
		"/search <keyword>",
		"/subscribe <keyword or saved_search_id>",
		"/unsubscribe <subscription_id or saved_search_id>",
		"/subscriptions",
	}, "\n")
}

func formatSearchResults(keyword string, items []resource.Item) string {
	if len(items) == 0 {
		return "No resources found for: " + keyword
	}
	lines := []string{"Search results for: " + keyword}
	for i, item := range items {
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, firstText(item.Title, item.Note, item.FileName, item.URL, item.ID)))
		meta := []string{}
		if item.Type != "" {
			meta = append(meta, item.Type)
		}
		if item.ChannelTitle != "" {
			meta = append(meta, item.ChannelTitle)
		}
		if len(meta) > 0 {
			lines = append(lines, "   "+strings.Join(meta, " | "))
		}
		if item.URL != "" {
			lines = append(lines, "   "+item.URL)
		}
	}
	return strings.Join(lines, "\n")
}

func formatMatchMessage(match notification.SavedSearchMatch) string {
	lines := []string{
		html.EscapeString("发现新资源"),
		html.EscapeString(firstText(match.ResourceTitle, match.ResourceURL, match.ResourceID)),
	}
	meta := []string{}
	if match.ResourceType != "" {
		meta = append(meta, html.EscapeString(match.ResourceType))
	}
	if match.SourceChannelName != "" {
		source := html.EscapeString(match.SourceChannelName)
		if link := telegramMessageLink(match); link != "" {
			source = fmt.Sprintf(`<a href="%s">%s</a>`, html.EscapeString(link), source)
		}
		meta = append(meta, "来源: "+source)
	}
	if len(meta) > 0 {
		lines = append(lines, strings.Join(meta, " | "))
	}
	if match.ResourceURL != "" {
		lines = append(lines, html.EscapeString(match.ResourceURL))
	}
	return strings.Join(lines, "\n")
}

func telegramMessageLink(match notification.SavedSearchMatch) string {
	if match.TelegramMessageID <= 0 {
		return ""
	}
	username := strings.TrimPrefix(strings.TrimSpace(match.SourceChannelUsername), "@")
	if username != "" {
		return "https://t.me/" + url.PathEscape(username) + "/" + strconv.FormatInt(match.TelegramMessageID, 10)
	}
	channelID := normalizedPrivateChannelID(match.TelegramChannelID)
	if channelID == "" {
		return ""
	}
	return "https://t.me/c/" + channelID + "/" + strconv.FormatInt(match.TelegramMessageID, 10)
}

func normalizedPrivateChannelID(value int64) string {
	if value == 0 {
		return ""
	}
	raw := strconv.FormatInt(value, 10)
	raw = strings.TrimPrefix(raw, "-")
	if strings.HasPrefix(raw, "100") && len(raw) > 10 {
		return raw[3:]
	}
	return raw
}

func firstText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
