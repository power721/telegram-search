package telegram

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"runtime"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/gotd/contrib/middleware/floodwait"
	"github.com/gotd/contrib/middleware/ratelimit"
	gotdtelegram "github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/dcs"
	"go.uber.org/zap"
	"golang.org/x/net/proxy"
	"golang.org/x/time/rate"

	"tg-search/internal/build"
	configpkg "tg-search/internal/config"
)

type RateLimitConfig struct {
	Enabled       bool
	RatePerSecond int
	Burst         int
}

type RuntimeConfig struct {
	Proxy            string
	ReconnectTimeout time.Duration
	DialTimeout      time.Duration
	RateLimit        RateLimitConfig
}

type BuildOptionsInput struct {
	Runtime        RuntimeConfig
	SessionStorage gotdtelegram.SessionStorage
	UpdateHandler  gotdtelegram.UpdateHandler
	Logger         *zap.Logger
	NoUpdates      bool
}

func DefaultRuntimeConfig() RuntimeConfig {
	return RuntimeConfig{
		ReconnectTimeout: 5 * time.Minute,
		DialTimeout:      10 * time.Second,
		RateLimit: RateLimitConfig{
			Enabled:       true,
			RatePerSecond: 10,
			Burst:         5,
		},
	}
}

func RuntimeConfigFromConfig(cfg configpkg.TelegramConfig) RuntimeConfig {
	return RuntimeConfig{
		Proxy:            cfg.Proxy,
		ReconnectTimeout: cfg.ReconnectTimeout.Std(),
		DialTimeout:      cfg.DialTimeout.Std(),
		RateLimit: RateLimitConfig{
			Enabled:       cfg.RateLimit.Enabled,
			RatePerSecond: cfg.RateLimit.RatePerSecond,
			Burst:         cfg.RateLimit.Burst,
		},
	}
}

func BuildOptions(ctx context.Context, input BuildOptionsInput) (gotdtelegram.Options, error) {
	runtimeCfg := normalizeRuntimeConfig(input.Runtime)
	dial, err := telegramDialer(runtimeCfg.Proxy)
	if err != nil {
		return gotdtelegram.Options{}, err
	}
	middlewares := []gotdtelegram.Middleware{
		floodwait.NewSimpleWaiter(),
	}
	if runtimeCfg.RateLimit.Enabled {
		middlewares = append(middlewares, ratelimit.New(rate.Limit(runtimeCfg.RateLimit.RatePerSecond), runtimeCfg.RateLimit.Burst))
	}
	return gotdtelegram.Options{
		Resolver: dcs.Plain(dcs.PlainOptions{
			Dial: dialContext(ctx, dial),
		}),
		ReconnectionBackoff: func() backoff.BackOff {
			return newTelegramBackoff(runtimeCfg.ReconnectTimeout)
		},
		SessionStorage: input.SessionStorage,
		UpdateHandler:  input.UpdateHandler,
		Logger:         input.Logger,
		NoUpdates:      input.NoUpdates,
		RetryInterval:  2 * time.Second,
		MaxRetries:     20,
		DialTimeout:    runtimeCfg.DialTimeout,
		Middlewares:    middlewares,
		Device: gotdtelegram.DeviceConfig{
			DeviceModel:    "TG Search",
			SystemVersion:  runtime.GOOS,
			AppVersion:     build.Version,
			SystemLangCode: "zh-CN",
			LangCode:       "zh",
		},
	}, nil
}

func normalizeRuntimeConfig(cfg RuntimeConfig) RuntimeConfig {
	defaults := DefaultRuntimeConfig()
	if cfg.ReconnectTimeout <= 0 {
		cfg.ReconnectTimeout = defaults.ReconnectTimeout
	}
	if cfg.DialTimeout <= 0 {
		cfg.DialTimeout = defaults.DialTimeout
	}
	if cfg.RateLimit == (RateLimitConfig{}) {
		cfg.RateLimit = defaults.RateLimit
	}
	if cfg.RateLimit.RatePerSecond <= 0 {
		cfg.RateLimit.RatePerSecond = defaults.RateLimit.RatePerSecond
	}
	if cfg.RateLimit.Burst <= 0 {
		cfg.RateLimit.Burst = defaults.RateLimit.Burst
	}
	return cfg
}

func telegramDialer(proxyURL string) (proxy.Dialer, error) {
	if proxyURL == "" {
		return proxy.Direct, nil
	}
	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("telegram proxy %q: %w", proxyURL, err)
	}
	dialer, err := proxy.FromURL(parsed, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("telegram proxy %q: %w", proxyURL, err)
	}
	return dialer, nil
}

func dialContext(parent context.Context, dialer proxy.Dialer) dcs.DialFunc {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		if parent != nil {
			select {
			case <-parent.Done():
				return nil, parent.Err()
			default:
			}
		}
		type result struct {
			conn net.Conn
			err  error
		}
		done := make(chan result, 1)
		go func() {
			conn, err := dialer.Dial(network, addr)
			done <- result{conn: conn, err: err}
		}()
		select {
		case res := <-done:
			return res.conn, res.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func newTelegramBackoff(timeout time.Duration) backoff.BackOff {
	b := backoff.NewExponentialBackOff()
	b.Multiplier = 1.1
	b.MaxElapsedTime = timeout
	b.MaxInterval = 10 * time.Second
	return b
}
