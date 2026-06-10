package tgbot

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"

	"tg-search/internal/config"
	"tg-search/internal/repository"
	"tg-search/internal/resource"
)

type Runtime struct {
	settings      *repository.SettingsRepository
	defaults      config.BotConfig
	apiFactory    func(string) BotAPI
	resources     *resource.Service
	savedSearches *repository.SavedSearchRepository
	subscriptions *repository.TelegramBotSubscriptionRepository
	deliveries    *repository.NotificationDeliveryRepository
	logger        *zap.Logger
	now           func() time.Time

	token      string
	interval   time.Duration
	nextRun    time.Time
	bot        *Bot
	dispatcher *DeliveryDispatcher
}

type RuntimeOptions struct {
	Settings      *repository.SettingsRepository
	Defaults      config.BotConfig
	APIFactory    func(string) BotAPI
	Resources     *resource.Service
	SavedSearches *repository.SavedSearchRepository
	Subscriptions *repository.TelegramBotSubscriptionRepository
	Deliveries    *repository.NotificationDeliveryRepository
	Logger        *zap.Logger
	Now           func() time.Time
}

func NewRuntime(opts RuntimeOptions) *Runtime {
	if opts.APIFactory == nil {
		opts.APIFactory = func(token string) BotAPI { return NewAPI(token) }
	}
	if opts.Logger == nil {
		opts.Logger = zap.NewNop()
	}
	if opts.Now == nil {
		opts.Now = func() time.Time { return time.Now().UTC() }
	}
	return &Runtime{
		settings:      opts.Settings,
		defaults:      opts.Defaults,
		apiFactory:    opts.APIFactory,
		resources:     opts.Resources,
		savedSearches: opts.SavedSearches,
		subscriptions: opts.Subscriptions,
		deliveries:    opts.Deliveries,
		logger:        opts.Logger,
		now:           opts.Now,
	}
}

func (r *Runtime) Name() string {
	return "telegram-bot-runtime"
}

func (r *Runtime) Run(ctx context.Context) error {
	if r.settings == nil {
		return nil
	}
	settings, err := r.settings.LoadTelegramBot(ctx, r.defaults)
	if err != nil {
		return err
	}
	token := strings.TrimSpace(settings.Token)
	if !settings.Enabled || token == "" {
		r.nextRun = time.Time{}
		return nil
	}
	interval := settings.PollInterval.Std()
	if interval <= 0 {
		interval = defaultPollInterval
	}
	if r.bot == nil || r.dispatcher == nil || r.token != token {
		r.configure(token, interval)
	}
	if r.interval != interval {
		r.interval = interval
		r.nextRun = time.Time{}
	}
	now := r.now()
	if !r.nextRun.IsZero() && now.Before(r.nextRun) {
		return nil
	}
	r.nextRun = now.Add(interval)
	var firstErr error
	if r.bot != nil {
		if err := r.bot.Run(ctx); err != nil {
			firstErr = err
		}
	}
	if r.dispatcher != nil {
		if err := r.dispatcher.Run(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (r *Runtime) configure(token string, interval time.Duration) {
	api := r.apiFactory(token)
	r.token = token
	r.interval = interval
	r.nextRun = time.Time{}
	r.bot = NewBot(BotOptions{
		API:           api,
		Resources:     r.resources,
		SavedSearches: r.savedSearches,
		Subscriptions: r.subscriptions,
		Logger:        r.logger,
	})
	r.dispatcher = NewDeliveryDispatcher(DeliveryDispatcherOptions{
		API:           api,
		Deliveries:    r.deliveries,
		Subscriptions: r.subscriptions,
		Logger:        r.logger,
	})
}
