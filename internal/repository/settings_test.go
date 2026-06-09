package repository

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tg-search/internal/config"
	"tg-search/internal/db"
	"tg-search/internal/model"
)

func TestSettingsRepositoryUpsertsValues(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "tg-search.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := NewSettingsRepository(conn)
	if err := repo.Set(ctx, "setup.complete", `true`); err != nil {
		t.Fatalf("set: %v", err)
	}
	value, ok, err := repo.Get(ctx, "setup.complete")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !ok || value != `true` {
		t.Fatalf("value=%q ok=%v, want true", value, ok)
	}
}

func TestTelegramAPISettingsRoundTripAndRedaction(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "tg-search.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := NewSettingsRepository(conn)

	settings := model.TelegramAPISettings{AppID: 123456, AppHash: "hash-secret"}
	if err := repo.SaveTelegramAPI(ctx, settings); err != nil {
		t.Fatalf("save telegram api: %v", err)
	}

	raw, ok, err := repo.Get(ctx, "telegram_api")
	if err != nil {
		t.Fatalf("get raw telegram api: %v", err)
	}
	if !ok || !strings.Contains(raw, "hash-secret") {
		t.Fatalf("raw setting = %q ok=%v, want stored secret", raw, ok)
	}

	loaded, err := repo.LoadTelegramAPI(ctx)
	if err != nil {
		t.Fatalf("load telegram api: %v", err)
	}
	if loaded.AppID != settings.AppID || loaded.AppHash != settings.AppHash {
		t.Fatalf("loaded = %+v, want %+v", loaded, settings)
	}

	redacted := RedactTelegramAPI(loaded)
	if !redacted.Configured || redacted.AppID != 123456 || !redacted.AppHashSet {
		t.Fatalf("redacted = %+v, want configured app id with app_hash_set", redacted)
	}
	redactedJSON, err := json.Marshal(redacted)
	if err != nil {
		t.Fatalf("marshal redacted: %v", err)
	}
	if strings.Contains(string(redactedJSON), "hash-secret") {
		t.Fatalf("redacted response leaked app hash: %+v", redacted)
	}
}

func TestTelegramAPISettingsDefaultsWhenNotStored(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "tg-search.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := NewSettingsRepository(conn)

	settings, err := repo.LoadTelegramAPI(ctx)
	if err != nil {
		t.Fatalf("load telegram api: %v", err)
	}
	if settings.AppID != 26375241 || settings.AppHash != "70f574f48a016d683c64f2f7a217d04f" {
		t.Fatalf("settings = %+v, want default Telegram API credentials", settings)
	}
	redacted := RedactTelegramAPI(settings)
	if redacted.Configured || redacted.AppID != 0 || redacted.AppHashSet {
		t.Fatalf("redacted = %+v, want default credentials hidden from settings response", redacted)
	}
}

func TestRuntimeSettingsDefaultsMissingStoredFieldsFromConfig(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "tg-search.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := NewSettingsRepository(conn)
	defaults := config.Config{
		Sync: config.SyncConfig{
			Workers:                 5,
			HistoryBatchSize:        100,
			TelegramRequestInterval: config.Duration(2 * time.Second),
		},
		Storage: config.StorageConfig{
			MaxDBSize:     config.Size(10 * 1000 * 1000 * 1000),
			MaxMediaCache: config.Size(20 * 1000 * 1000 * 1000),
		},
		Telegram: config.TelegramConfig{
			ReconnectTimeout: config.Duration(5 * time.Minute),
			DialTimeout:      config.Duration(10 * time.Second),
			RateLimit:        config.TelegramRateLimitConfig{Enabled: true, RatePerSecond: 10, Burst: 5},
			Stream:           config.TelegramStreamConfig{Concurrency: 2, Buffers: 4, ChunkTimeout: config.Duration(20 * time.Second)},
			Media:            config.TelegramMediaConfig{Concurrency: 2},
		},
	}
	if err := repo.Set(ctx, "runtime", `{"sync":{"workers":8}}`); err != nil {
		t.Fatalf("set runtime setting: %v", err)
	}

	settings, err := repo.LoadRuntimeSettings(ctx, defaults)
	if err != nil {
		t.Fatalf("load runtime settings: %v", err)
	}

	if settings.Sync.Workers != 8 {
		t.Fatalf("workers = %d, want stored override 8", settings.Sync.Workers)
	}
	if settings.Sync.HistoryBatchSize != 100 {
		t.Fatalf("history batch size = %d, want default 100", settings.Sync.HistoryBatchSize)
	}
	if settings.Telegram.Stream.ChunkTimeout != config.Duration(20*time.Second) {
		t.Fatalf("chunk timeout = %s, want default 20s", settings.Telegram.Stream.ChunkTimeout)
	}
}

func TestTelegramCredentialsProviderLoadsStoredSettings(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "tg-search.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := NewSettingsRepository(conn)
	if err := repo.SaveTelegramAPI(ctx, model.TelegramAPISettings{AppID: 654321, AppHash: "db-hash"}); err != nil {
		t.Fatalf("save telegram api: %v", err)
	}

	credentials, err := NewTelegramCredentialsProvider(repo).TelegramCredentials(ctx)
	if err != nil {
		t.Fatalf("telegram credentials: %v", err)
	}
	if credentials.APIID != 654321 || credentials.APIHash != "db-hash" {
		t.Fatalf("credentials = %+v, want DB settings", credentials)
	}
}

func TestTelegramCredentialsProviderUsesDefaultsWhenNotStored(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "tg-search.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	credentials, err := NewTelegramCredentialsProvider(NewSettingsRepository(conn)).TelegramCredentials(ctx)
	if err != nil {
		t.Fatalf("telegram credentials: %v", err)
	}
	if credentials.APIID != 26375241 || credentials.APIHash != "70f574f48a016d683c64f2f7a217d04f" {
		t.Fatalf("credentials = %+v, want default Telegram API credentials", credentials)
	}
}
