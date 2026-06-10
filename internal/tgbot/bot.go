package tgbot

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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
	SetCommands(context.Context, []BotCommand) error
}

type API struct {
	Token   string
	BaseURL string
	Client  *http.Client
}

type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message,omitempty"`
}

type Message struct {
	Chat Chat   `json:"chat"`
	Text string `json:"text"`
}

type Chat struct {
	ID int64 `json:"id"`
}

type BotCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
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
	body, err := json.Marshal(map[string]any{
		"chat_id":                  chatID,
		"text":                     text,
		"disable_web_page_preview": true,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint("sendMessage"), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	var resp apiResponse[json.RawMessage]
	return a.do(req, &resp)
}

func (a *API) SetCommands(ctx context.Context, commands []BotCommand) error {
	body, err := json.Marshal(map[string]any{"commands": commands})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint("setMyCommands"), bytes.NewReader(body))
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
	resources     *resource.Service
	savedSearches *repository.SavedSearchRepository
	subscriptions *repository.TelegramBotSubscriptionRepository
	logger        *zap.Logger
	offset        int64
}

type BotOptions struct {
	API           BotAPI
	Resources     *resource.Service
	SavedSearches *repository.SavedSearchRepository
	Subscriptions *repository.TelegramBotSubscriptionRepository
	Logger        *zap.Logger
}

func NewBot(opts BotOptions) *Bot {
	if opts.Logger == nil {
		opts.Logger = zap.NewNop()
	}
	return &Bot{api: opts.API, resources: opts.Resources, savedSearches: opts.SavedSearches, subscriptions: opts.Subscriptions, logger: opts.Logger}
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
		if update.Message == nil || strings.TrimSpace(update.Message.Text) == "" {
			continue
		}
		if err := b.handleMessage(ctx, *update.Message); err != nil {
			b.logger.Warn("telegram bot command failed", zap.Int64("chat_id", update.Message.Chat.ID), zap.Error(err))
		}
	}
	return nil
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
		return b.api.SendMessage(ctx, chatID, "Usage: /unsubscribe <subscription_id>")
	}
	if err := b.subscriptions.DeleteForChat(ctx, chatID, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return b.api.SendMessage(ctx, chatID, "Subscription not found.")
		}
		return err
	}
	return b.api.SendMessage(ctx, chatID, fmt.Sprintf("Unsubscribed #%d.", id))
}

func (b *Bot) handleSubscriptions(ctx context.Context, chatID int64) error {
	if b.subscriptions == nil {
		return b.api.SendMessage(ctx, chatID, "Subscription service is unavailable.")
	}
	items, err := b.subscriptions.FindByChat(ctx, chatID)
	if err != nil {
		return err
	}
	lines := []string{}
	subscribedSearches := map[int64]struct{}{}
	if len(items) > 0 {
		lines = append(lines, "My subscriptions:")
	}
	for _, item := range items {
		subscribedSearches[item.SavedSearchID] = struct{}{}
		lines = append(lines, fmt.Sprintf("#%d saved #%d %s", item.ID, item.SavedSearchID, firstText(item.SavedSearch, item.Keyword)))
	}
	if b.savedSearches != nil {
		searches, err := b.savedSearches.FindAll(ctx)
		if err != nil {
			return err
		}
		available := []model.SavedSearch{}
		for _, search := range searches {
			if !search.Enabled || !search.NotifyTelegram {
				continue
			}
			if _, ok := subscribedSearches[search.ID]; ok {
				continue
			}
			available = append(available, search)
		}
		if len(available) > 0 {
			if len(lines) > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, "Available saved searches:")
			for _, search := range available {
				lines = append(lines, fmt.Sprintf("saved #%d %s - /subscribe %d", search.ID, firstText(search.Name, search.Keyword), search.ID))
			}
		}
	}
	if len(lines) == 0 {
		lines = append(lines, "No subscriptions. Use /subscribe <keyword> or create a saved search in Settings.")
	}
	return b.api.SendMessage(ctx, chatID, strings.Join(lines, "\n"))
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
	if err := d.api.SendMessage(ctx, sub.ChatID, formatMatchMessage(match)); err != nil {
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
		"/unsubscribe <subscription_id>",
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
		"发现新资源",
		firstText(match.ResourceTitle, match.ResourceURL, match.ResourceID),
	}
	meta := []string{}
	if match.ResourceType != "" {
		meta = append(meta, match.ResourceType)
	}
	if match.SourceChannelName != "" {
		meta = append(meta, "来源: "+match.SourceChannelName)
	}
	if len(meta) > 0 {
		lines = append(lines, strings.Join(meta, " | "))
	}
	if match.ResourceURL != "" {
		lines = append(lines, match.ResourceURL)
	}
	return strings.Join(lines, "\n")
}

func firstText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
