package config

type RuntimeSettings struct {
	Sync     RuntimeSyncSettings     `json:"sync"`
	Storage  RuntimeStorageSettings  `json:"storage"`
	Telegram RuntimeTelegramSettings `json:"telegram"`
}

type RuntimeSyncSettings struct {
	Workers                 int      `json:"workers"`
	HistoryBatchSize        int      `json:"history_batch_size"`
	TelegramRequestInterval Duration `json:"telegram_request_interval"`
}

type RuntimeStorageSettings struct {
	MaxDBSize     Size `json:"max_db_size"`
	MaxMediaCache Size `json:"max_media_cache"`
}

type RuntimeTelegramSettings struct {
	Proxy            string                  `json:"proxy"`
	ReconnectTimeout Duration                `json:"reconnect_timeout"`
	DialTimeout      Duration                `json:"dial_timeout"`
	RateLimit        TelegramRateLimitConfig `json:"rate_limit"`
	Stream           TelegramStreamConfig    `json:"stream"`
	Media            TelegramMediaConfig     `json:"media"`
}

func RuntimeSettingsFromConfig(cfg Config) RuntimeSettings {
	return RuntimeSettings{
		Sync: RuntimeSyncSettings{
			Workers:                 cfg.Sync.Workers,
			HistoryBatchSize:        cfg.Sync.HistoryBatchSize,
			TelegramRequestInterval: cfg.Sync.TelegramRequestInterval,
		},
		Storage: RuntimeStorageSettings{
			MaxDBSize:     cfg.Storage.MaxDBSize,
			MaxMediaCache: cfg.Storage.MaxMediaCache,
		},
		Telegram: RuntimeTelegramSettings{
			Proxy:            cfg.Telegram.Proxy,
			ReconnectTimeout: cfg.Telegram.ReconnectTimeout,
			DialTimeout:      cfg.Telegram.DialTimeout,
			RateLimit:        cfg.Telegram.RateLimit,
			Stream:           cfg.Telegram.Stream,
			Media:            cfg.Telegram.Media,
		},
	}
}

func ApplyRuntimeSettings(cfg Config, settings RuntimeSettings) (Config, error) {
	applyDefaults(&cfg)
	cfg.Sync.Workers = settings.Sync.Workers
	cfg.Sync.HistoryBatchSize = settings.Sync.HistoryBatchSize
	cfg.Sync.TelegramRequestInterval = settings.Sync.TelegramRequestInterval
	cfg.Storage.MaxDBSize = settings.Storage.MaxDBSize
	cfg.Storage.MaxMediaCache = settings.Storage.MaxMediaCache
	cfg.Telegram.Proxy = settings.Telegram.Proxy
	cfg.Telegram.ReconnectTimeout = settings.Telegram.ReconnectTimeout
	cfg.Telegram.DialTimeout = settings.Telegram.DialTimeout
	cfg.Telegram.RateLimit = settings.Telegram.RateLimit
	cfg.Telegram.Stream = settings.Telegram.Stream
	cfg.Telegram.Media = settings.Telegram.Media
	if err := validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
