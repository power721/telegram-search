package telegram

import (
	"context"
	"strings"
	"testing"
	"time"

	gotdsession "github.com/gotd/td/session"
	"go.uber.org/zap"

	"tg-search/internal/config"
)

func TestBuildOptionsAddsReliabilityDefaults(t *testing.T) {
	opts, err := BuildOptions(context.Background(), BuildOptionsInput{
		Runtime:        DefaultRuntimeConfig(),
		SessionStorage: new(gotdsession.StorageMemory),
		Logger:         zap.NewNop(),
		NoUpdates:      true,
	})
	if err != nil {
		t.Fatalf("BuildOptions returned error: %v", err)
	}

	if opts.SessionStorage == nil {
		t.Fatal("SessionStorage is nil")
	}
	if opts.Logger == nil {
		t.Fatal("Logger is nil")
	}
	if !opts.NoUpdates {
		t.Fatal("NoUpdates = false, want true")
	}
	if opts.DialTimeout != 10*time.Second {
		t.Fatalf("DialTimeout = %s, want 10s", opts.DialTimeout)
	}
	if opts.ReconnectionBackoff == nil {
		t.Fatal("ReconnectionBackoff is nil")
	}
	if opts.Resolver == nil {
		t.Fatal("Resolver is nil")
	}
	if len(opts.Middlewares) != 2 {
		t.Fatalf("middlewares = %d, want floodwait and ratelimit", len(opts.Middlewares))
	}
}

func TestBuildOptionsCanDisableRateLimit(t *testing.T) {
	runtime := DefaultRuntimeConfig()
	runtime.RateLimit.Enabled = false

	opts, err := BuildOptions(context.Background(), BuildOptionsInput{
		Runtime:        runtime,
		SessionStorage: new(gotdsession.StorageMemory),
	})
	if err != nil {
		t.Fatalf("BuildOptions returned error: %v", err)
	}

	if len(opts.Middlewares) != 1 {
		t.Fatalf("middlewares = %d, want only floodwait", len(opts.Middlewares))
	}
}

func TestRuntimeConfigFromConfig(t *testing.T) {
	runtime := RuntimeConfigFromConfig(config.TelegramConfig{
		Proxy:            "socks5://127.0.0.1:1080",
		ReconnectTimeout: config.Duration(2 * time.Minute),
		DialTimeout:      config.Duration(3 * time.Second),
		RateLimit: config.TelegramRateLimitConfig{
			Enabled:       false,
			RatePerSecond: 7,
			Burst:         2,
		},
	})

	if runtime.Proxy != "socks5://127.0.0.1:1080" {
		t.Fatalf("Proxy = %q", runtime.Proxy)
	}
	if runtime.ReconnectTimeout != 2*time.Minute {
		t.Fatalf("ReconnectTimeout = %s, want 2m", runtime.ReconnectTimeout)
	}
	if runtime.DialTimeout != 3*time.Second {
		t.Fatalf("DialTimeout = %s, want 3s", runtime.DialTimeout)
	}
	if runtime.RateLimit.Enabled {
		t.Fatal("rate limit enabled = true, want false")
	}
	if runtime.RateLimit.RatePerSecond != 7 || runtime.RateLimit.Burst != 2 {
		t.Fatalf("rate limit = %+v, want rate 7 burst 2", runtime.RateLimit)
	}
}

func TestBuildOptionsRejectsInvalidProxy(t *testing.T) {
	runtime := DefaultRuntimeConfig()
	runtime.Proxy = "://bad"

	_, err := BuildOptions(context.Background(), BuildOptionsInput{
		Runtime:        runtime,
		SessionStorage: new(gotdsession.StorageMemory),
	})
	if err == nil || !strings.Contains(err.Error(), "telegram proxy") {
		t.Fatalf("BuildOptions error = %v, want telegram proxy error", err)
	}
}
